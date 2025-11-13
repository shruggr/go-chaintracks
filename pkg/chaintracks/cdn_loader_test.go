package chaintracks

import (
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-sdk/block"
)

const testCDNPath = "../../../chaintracks-server/public/headers"

func TestParseMetadata(t *testing.T) {
	metadataPath := filepath.Join(testCDNPath, "mainNetBlockHeaders.json")

	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		t.Skipf("Test CDN data not found at %s", metadataPath)
	}

	metadata, err := parseMetadata(metadataPath)
	if err != nil {
		t.Fatalf("Failed to parse metadata: %v", err)
	}

	if metadata.HeadersPerFile != 100000 {
		t.Errorf("Expected 100000 headers per file, got %d", metadata.HeadersPerFile)
	}

	if len(metadata.Files) == 0 {
		t.Error("Expected at least one file entry")
	}

	t.Logf("Parsed metadata with %d files", len(metadata.Files))
	for i, file := range metadata.Files {
		t.Logf("  File %d: %s, height %d-%d, count %d",
			i, file.FileName, file.FirstHeight, file.FirstHeight+uint32(file.Count)-1, file.Count)
	}
}

func TestLoadHeadersFromFile(t *testing.T) {
	filePath := filepath.Join(testCDNPath, "mainNet_0.headers")

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Skipf("Test file not found at %s", filePath)
	}

	headers, err := loadHeadersFromFile(filePath)
	if err != nil {
		t.Fatalf("Failed to load headers: %v", err)
	}

	if len(headers) != 100000 {
		t.Errorf("Expected 100000 headers, got %d", len(headers))
	}

	genesisHash := headers[0].Hash()
	expectedGenesisHash := "000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f"
	if genesisHash.String() != expectedGenesisHash {
		t.Errorf("Genesis hash mismatch:\n  got:      %s\n  expected: %s",
			genesisHash.String(), expectedGenesisHash)
	}

	t.Logf("Loaded %d headers successfully", len(headers))
	t.Logf("Genesis hash: %s", genesisHash.String())
}

func TestLoadFromLocalFiles(t *testing.T) {
	if _, err := os.Stat(testCDNPath); os.IsNotExist(err) {
		t.Skipf("Test CDN data not found at %s", testCDNPath)
	}

	cm, err := NewChainManager("main", testCDNPath)
	if err != nil {
		t.Fatalf("Failed to create ChainManager: %v", err)
	}

	if cm.GetHeight() == 0 {
		t.Skip("No local files found, this is expected")
	}

	t.Logf("Loaded chain to height %d", cm.GetHeight())

	tip := cm.GetTip()
	if tip == nil {
		t.Fatal("Chain tip is nil")
	}

	t.Logf("Chain tip: height=%d, hash=%s", tip.Height, tip.Header.Hash().String())
}

func TestSyncFromCDNSingleFile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping CDN sync test in short mode")
	}

	if _, err := os.Stat(testCDNPath); os.IsNotExist(err) {
		t.Skipf("Test CDN data not found at %s", testCDNPath)
	}

	tempDir := t.TempDir()
	cm, err := NewChainManager("main", tempDir)
	if err != nil {
		t.Fatalf("Failed to create ChainManager: %v", err)
	}

	metadataPath := filepath.Join(testCDNPath, "mainNetBlockHeaders.json")
	metadata, err := parseMetadata(metadataPath)
	if err != nil {
		t.Fatalf("Failed to parse metadata: %v", err)
	}

	if len(metadata.Files) == 0 {
		t.Fatal("No files in metadata")
	}

	fileEntry := metadata.Files[0]

	data, err := os.ReadFile(filepath.Join(testCDNPath, fileEntry.FileName))
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	start := time.Now()

	headerCount := len(data) / 80
	for i := 0; i < headerCount; i++ {
		headerBytes := data[i*80 : (i+1)*80]
		header, err := block.NewHeaderFromBytes(headerBytes)
		if err != nil {
			t.Fatalf("Failed to parse header at index %d: %v", i, err)
		}

		height := fileEntry.FirstHeight + uint32(i)

		var chainWork *big.Int
		if height == 0 {
			chainWork = big.NewInt(0)
		} else {
			prevHeader, err := cm.store.GetHeaderByHeight(height - 1)
			if err != nil {
				t.Fatalf("Failed to get previous header: %v", err)
			}

			work := CalculateWork(header.Bits)
			chainWork = new(big.Int).Add(prevHeader.ChainWork, work)
		}

		blockHeader := &BlockHeader{
			Header:    header,
			Height:    height,
			ChainWork: chainWork,
		}

		if err := cm.store.AddHeader(blockHeader); err != nil {
			t.Fatalf("Failed to add header: %v", err)
		}
	}

	elapsed := time.Since(start)

	if cm.GetHeight() != 99999 {
		t.Errorf("Expected height 99999, got %d", cm.GetHeight())
	}

	tip := cm.GetTip()
	t.Logf("Validated and loaded %d headers in %v", headerCount, elapsed)
	t.Logf("Chain tip: height=%d, hash=%s", tip.Height, tip.Header.Hash().String())
	t.Logf("Performance: %.0f headers/sec", float64(headerCount)/elapsed.Seconds())
}

func TestSyncFullChain(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping full chain sync test in short mode")
	}

	if _, err := os.Stat(testCDNPath); os.IsNotExist(err) {
		t.Skipf("Test CDN data not found at %s", testCDNPath)
	}

	tempDir := t.TempDir()
	cm, err := NewChainManager("main", tempDir)
	if err != nil {
		t.Fatalf("Failed to create ChainManager: %v", err)
	}

	metadataPath := filepath.Join(testCDNPath, "mainNetBlockHeaders.json")
	metadata, err := parseMetadata(metadataPath)
	if err != nil {
		t.Fatalf("Failed to parse metadata: %v", err)
	}

	start := time.Now()
	totalHeaders := 0

	for fileIdx, fileEntry := range metadata.Files {
		data, err := os.ReadFile(filepath.Join(testCDNPath, fileEntry.FileName))
		if err != nil {
			t.Fatalf("Failed to read file %s: %v", fileEntry.FileName, err)
		}

		headerCount := len(data) / 80
		for i := 0; i < headerCount; i++ {
			headerBytes := data[i*80 : (i+1)*80]
			header, err := block.NewHeaderFromBytes(headerBytes)
			if err != nil {
				t.Fatalf("Failed to parse header at index %d in file %d: %v", i, fileIdx, err)
			}

			height := fileEntry.FirstHeight + uint32(i)

			var chainWork *big.Int
			if height == 0 {
				chainWork = big.NewInt(0)
			} else {
				prevHeader, err := cm.store.GetHeaderByHeight(height - 1)
				if err != nil {
					t.Fatalf("Failed to get previous header at height %d: %v", height-1, err)
				}

				work := CalculateWork(header.Bits)
				chainWork = new(big.Int).Add(prevHeader.ChainWork, work)
			}

			blockHeader := &BlockHeader{
				Header:    header,
				Height:    height,
				ChainWork: chainWork,
			}

			if err := cm.store.AddHeader(blockHeader); err != nil {
				t.Fatalf("Failed to add header at height %d: %v", height, err)
			}
		}

		totalHeaders += headerCount
		t.Logf("Processed file %d/%d: %s (%d headers, total: %d)",
			fileIdx+1, len(metadata.Files), fileEntry.FileName, headerCount, totalHeaders)
	}

	elapsed := time.Since(start)

	tip := cm.GetTip()
	t.Logf("Successfully validated and loaded %d headers in %v", totalHeaders, elapsed)
	t.Logf("Final chain tip: height=%d, hash=%s", tip.Height, tip.Header.Hash().String())
	t.Logf("Performance: %.0f headers/sec", float64(totalHeaders)/elapsed.Seconds())

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	t.Logf("Memory usage: %.2f MB allocated", float64(memStats.Alloc)/(1024*1024))
}
