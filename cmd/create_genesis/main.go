package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/bsv-blockchain/go-chaintracks/pkg/chaintracks"
)

// Teratestnet genesis block header (reversed endianness for hash: 000000000499eabba0a88f5b3747231c74b9191c1a4a04b2c2ea817976b7776d)
const teratestnetGenesisHex = "0100000000000000000000000000000000000000000000000000000000000000000000003ba3edfd7a7b12b27ac72c3e67768f617fc81bc3888a51323a9fb8aa4b1e5e4a45f24d67ffff001d1b3cc368"

// Testnet genesis block header (reversed endianness for hash: 000000000933ea01ad0ee984209779baaec3ced90fa3f408719526f8d77f4943)
const testnetGenesisHex = "0100000000000000000000000000000000000000000000000000000000000000000000003ba3edfd7a7b12b27ac72c3e67768f617fc81bc3888a51323a9fb8aa4b1e5e4adae5494dffff001d1aa4ae18"

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: create_genesis <network>")
	}

	network := os.Args[1]

	var genesisHex string
	switch network {
	case "teratestnet":
		genesisHex = teratestnetGenesisHex
	case "testnet":
		genesisHex = testnetGenesisHex
	default:
		log.Fatalf("Unknown network: %s", network)
	}

	// Decode genesis header
	headerBytes, err := hex.DecodeString(genesisHex)
	if err != nil {
		log.Fatalf("Failed to decode genesis hex: %v", err)
	}

	if len(headerBytes) != 80 {
		log.Fatalf("Invalid header size: %d", len(headerBytes))
	}

	// Create output directory
	outDir := filepath.Join("data", "headers")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Write genesis header file
	headerFile := filepath.Join(outDir, fmt.Sprintf("%sNet_0.headers", network))
	if err := os.WriteFile(headerFile, headerBytes, 0644); err != nil {
		log.Fatalf("Failed to write header file: %v", err)
	}

	log.Printf("Created %s", headerFile)

	// Get genesis hash based on network
	var genesisHash string
	switch network {
	case "teratestnet":
		genesisHash = "000000000499eabba0a88f5b3747231c74b9191c1a4a04b2c2ea817976b7776d"
	case "testnet":
		genesisHash = "000000000933ea01ad0ee984209779baaec3ced90fa3f408719526f8d77f4943"
	default:
		log.Fatalf("Unknown network for hash: %s", network)
	}

	// Create metadata file
	metadata := chaintracks.CDNMetadata{
		RootFolder:     "",
		JSONFilename:   fmt.Sprintf("%sNetBlockHeaders.json", network),
		HeadersPerFile: 100000,
		Files: []chaintracks.CDNFileEntry{
			{
				Chain:         network,
				Count:         1,
				FileHash:      "",
				FileName:      fmt.Sprintf("%sNet_0.headers", network),
				FirstHeight:   0,
				LastChainWork: "0000000000000000000000000000000000000000000000000000000100010001",
				LastHash:      genesisHash,
				PrevChainWork: "0000000000000000000000000000000000000000000000000000000000000000",
				PrevHash:      "0000000000000000000000000000000000000000000000000000000000000000",
				SourceURL:     "",
			},
		},
	}

	metadataBytes, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal metadata: %v", err)
	}

	metadataFile := filepath.Join(outDir, fmt.Sprintf("%sNetBlockHeaders.json", network))
	if err := os.WriteFile(metadataFile, metadataBytes, 0644); err != nil {
		log.Fatalf("Failed to write metadata file: %v", err)
	}

	log.Printf("Created %s", metadataFile)
	log.Printf("Genesis files created for %s network", network)
}
