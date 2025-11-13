package chaintracks

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"

	"github.com/bsv-blockchain/go-sdk/block"
	"github.com/bsv-blockchain/go-sdk/chainhash"
)

// loadHeadersFromFile reads a binary .headers file and returns a slice of headers
// This function performs no validation - just parsing
func loadHeadersFromFile(path string) ([]*block.Header, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	if len(data)%80 != 0 {
		return nil, fmt.Errorf("invalid file size: %d bytes (not multiple of 80)", len(data))
	}

	headerCount := len(data) / 80
	headers := make([]*block.Header, 0, headerCount)

	for i := 0; i < headerCount; i++ {
		headerBytes := data[i*80 : (i+1)*80]
		header, err := block.NewHeaderFromBytes(headerBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse header at index %d: %w", i, err)
		}
		headers = append(headers, header)
	}

	return headers, nil
}

// parseMetadata reads and parses the metadata JSON file
func parseMetadata(path string) (*CDNMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	var metadata CDNMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata JSON: %w", err)
	}

	return &metadata, nil
}


// loadFromLocalFiles restores the chain from local header files
// No validation is performed - we trust our own checkpoint and exported files
func (cm *ChainManager) loadFromLocalFiles() error {
	if cm.localStoragePath == "" {
		return fmt.Errorf("no local storage path configured")
	}

	metadataPath := filepath.Join(cm.localStoragePath, cm.network+"NetBlockHeaders.json")
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		return nil
	}

	metadata, err := parseMetadata(metadataPath)
	if err != nil {
		return fmt.Errorf("failed to parse local metadata: %w", err)
	}

	for _, fileEntry := range metadata.Files {
		filePath := filepath.Join(cm.localStoragePath, fileEntry.FileName)
		headers, err := loadHeadersFromFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to load file %s: %w", fileEntry.FileName, err)
		}

		for i, header := range headers {
			height := fileEntry.FirstHeight + uint32(i)

			var chainWork *big.Int
			if height == 0 {
				chainWork = big.NewInt(0)
			} else {
				prevHeader, _ := cm.store.GetHeaderByHeight(height - 1)
				work := CalculateWork(header.Bits)
				chainWork = new(big.Int).Add(prevHeader.ChainWork, work)
			}

			blockHeader := &BlockHeader{
				Header:    header,
				Height:    height,
				ChainWork: chainWork,
			}

			if err := cm.store.AddHeader(blockHeader); err != nil {
				return fmt.Errorf("failed to add header at height %d: %w", height, err)
			}
		}
	}

	return nil
}


// SetChainTip updates the chain tip with a new branch of headers
// branchHeaders should be ordered from oldest to newest
// The parent of branchHeaders[0] must exist in our current chain
func (cm *ChainManager) SetChainTip(branchHeaders []*BlockHeader) error {
	if len(branchHeaders) == 0 {
		return nil
	}

	// Update in-memory chain
	cm.store.mu.Lock()
	for _, header := range branchHeaders {
		hash := header.Hash()

		// Ensure slice is large enough
		for uint32(len(cm.store.byHeight)) <= header.Height {
			cm.store.byHeight = append(cm.store.byHeight, chainhash.Hash{})
		}

		// Update byHeight and byHash
		cm.store.byHeight[header.Height] = hash
		cm.store.byHash[hash] = header

		// Update tip if this is the new highest block
		if cm.store.tip == nil || header.Height > cm.store.tip.Height {
			cm.store.tip = header
		}
	}
	cm.store.mu.Unlock()

	// Write headers to files
	if err := cm.writeHeadersToFiles(branchHeaders); err != nil {
		return fmt.Errorf("failed to write headers to files: %w", err)
	}

	// Update metadata
	if err := cm.updateMetadataForTip(); err != nil {
		return fmt.Errorf("failed to update metadata: %w", err)
	}

	return nil
}

// writeHeadersToFiles writes headers to the appropriate .headers files
func (cm *ChainManager) writeHeadersToFiles(headers []*BlockHeader) error {
	if cm.localStoragePath == "" {
		return nil
	}

	if err := os.MkdirAll(cm.localStoragePath, 0755); err != nil {
		return fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Group headers by file
	fileHeaders := make(map[uint32][]*BlockHeader)
	for _, header := range headers {
		fileIndex := header.Height / 100000
		fileHeaders[fileIndex] = append(fileHeaders[fileIndex], header)
	}

	// Write to each file
	for fileIndex, hdrs := range fileHeaders {
		fileName := fmt.Sprintf("%sNet_%d.headers", cm.network, fileIndex)
		filePath := filepath.Join(cm.localStoragePath, fileName)

		// Open file for read/write (create if doesn't exist)
		f, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			return fmt.Errorf("failed to open file %s: %w", fileName, err)
		}

		// Write each header at its position
		for _, header := range hdrs {
			positionInFile := (header.Height % 100000) * 80
			if _, err := f.Seek(int64(positionInFile), 0); err != nil {
				f.Close()
				return fmt.Errorf("failed to seek in file: %w", err)
			}

			headerBytes := header.Header.Bytes()
			if _, err := f.Write(headerBytes); err != nil {
				f.Close()
				return fmt.Errorf("failed to write header: %w", err)
			}
		}

		f.Close()
	}

	return nil
}

// updateMetadataForTip updates the metadata JSON with current chain tip info
func (cm *ChainManager) updateMetadataForTip() error {
	if cm.localStoragePath == "" {
		return nil
	}

	metadataPath := filepath.Join(cm.localStoragePath, cm.network+"NetBlockHeaders.json")

	// Read existing metadata or create new
	var metadata *CDNMetadata
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		metadata = &CDNMetadata{
			RootFolder:     "",
			JSONFilename:   cm.network + "NetBlockHeaders.json",
			HeadersPerFile: 100000,
			Files:          []CDNFileEntry{},
		}
	} else {
		metadata, err = parseMetadata(metadataPath)
		if err != nil {
			return fmt.Errorf("failed to parse existing metadata: %w", err)
		}
	}

	// Update file entries based on current chain
	tip := cm.store.GetTip()
	if tip == nil {
		return nil
	}

	fileIndex := tip.Height / 100000

	// Ensure we have entries for all files up to the current tip
	for i := uint32(len(metadata.Files)); i <= fileIndex; i++ {
		metadata.Files = append(metadata.Files, CDNFileEntry{
			Chain:         cm.network,
			Count:         0,
			FileHash:      "",
			FileName:      fmt.Sprintf("%sNet_%d.headers", cm.network, i),
			FirstHeight:   i * 100000,
			LastChainWork: "0000000000000000000000000000000000000000000000000000000000000000",
			LastHash:      "0000000000000000000000000000000000000000000000000000000000000000",
			PrevChainWork: "0000000000000000000000000000000000000000000000000000000000000000",
			PrevHash:      "0000000000000000000000000000000000000000000000000000000000000000",
			SourceURL:     "",
		})
	}

	// Update the last file entry with current tip info
	lastFileEntry := &metadata.Files[fileIndex]
	lastFileEntry.Count = int((tip.Height % 100000) + 1)
	lastFileEntry.LastChainWork = ChainWorkToHex(tip.ChainWork)
	lastFileEntry.LastHash = tip.Hash().String()

	// Get previous header for prevChainWork and prevHash
	if tip.Height > 0 {
		prevHeader, err := cm.store.GetHeaderByHeight(tip.Height - 1)
		if err == nil {
			lastFileEntry.PrevChainWork = ChainWorkToHex(prevHeader.ChainWork)
			lastFileEntry.PrevHash = prevHeader.Hash().String()
		}
	}

	// Write updated metadata
	return cm.writeLocalMetadata(metadata)
}

// writeLocalMetadata writes the metadata JSON to local storage
func (cm *ChainManager) writeLocalMetadata(metadata *CDNMetadata) error {
	if cm.localStoragePath == "" {
		return nil
	}

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	metadataPath := filepath.Join(cm.localStoragePath, cm.network+"NetBlockHeaders.json")
	if err := os.WriteFile(metadataPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	return nil
}
