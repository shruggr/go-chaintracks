package chaintracks

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	p2p "github.com/bsv-blockchain/go-p2p-message-bus"
)

// ChainManager is the main orchestrator for chain management
type ChainManager struct {
	mu sync.RWMutex

	byHeight []chainhash.Hash                // Main chain hashes indexed by height
	byHash   map[chainhash.Hash]*BlockHeader // Hash → Header (all headers: main + orphans)
	tip      *BlockHeader                    // Current chain tip

	localStoragePath string
	network          string

	// P2P fields
	p2pClient p2p.Client        // P2P client for network communication
	msgChan   chan *BlockHeader // Channel for broadcasting tip changes to consumers
}

// NewChainManager creates a new ChainManager and restores from local files if present
// If bootstrapURL is provided, it will sync from a remote teranode before returning
func NewChainManager(network, localStoragePath string, bootstrapURL ...string) (*ChainManager, error) {
	// Default to ~/.chaintracks if no path provided
	if localStoragePath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home directory: %w", err)
		}
		localStoragePath = filepath.Join(homeDir, ".chaintracks")
	} else if localStoragePath[0] == '~' {
		// Expand ~ to home directory
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home directory: %w", err)
		}
		localStoragePath = filepath.Join(homeDir, localStoragePath[1:])
	}

	cm := &ChainManager{
		byHeight:         make([]chainhash.Hash, 0, 1000000),
		byHash:           make(map[chainhash.Hash]*BlockHeader),
		network:          network,
		localStoragePath: localStoragePath,
	}

	log.Printf("ChainManager initializing: network=%s, path=%s", network, localStoragePath)

	// Auto-restore from local files if they exist
	if err := cm.loadFromLocalFiles(); err != nil {
		return nil, fmt.Errorf("failed to load checkpoint files: %w", err)
	}

	// Run bootstrap sync if configured (optional parameter)
	if len(bootstrapURL) > 0 && bootstrapURL[0] != "" {
		log.Printf("Bootstrap URL configured: %s", bootstrapURL[0])

		// Get the latest block hash from the bootstrap node
		remoteTipHash, err := FetchLatestBlock(bootstrapURL[0])
		if err != nil {
			log.Printf("Failed to get bootstrap node tip: %v (will continue with P2P sync)", err)
		} else {
			log.Printf("Bootstrap node tip: %s", remoteTipHash.String())
			if err := cm.SyncFromRemoteTip(remoteTipHash, bootstrapURL[0]); err != nil {
				log.Printf("Bootstrap sync failed: %v (will continue with P2P sync)", err)
			}
		}

		// Log updated chain state after bootstrap
		if tip := cm.GetTip(); tip != nil {
			log.Printf("Chain tip after bootstrap: %s at height %d", tip.Header.Hash().String(), tip.Height)
		}
	}

	return cm, nil
}

// GetHeaderByHeight retrieves a header by height
func (cm *ChainManager) GetHeaderByHeight(height uint32) (*BlockHeader, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if height >= uint32(len(cm.byHeight)) {
		return nil, ErrHeaderNotFound
	}

	hash := cm.byHeight[height]
	header, ok := cm.byHash[hash]
	if !ok {
		return nil, ErrHeaderNotFound
	}

	return header, nil
}

// GetHeaderByHash retrieves a header by hash
func (cm *ChainManager) GetHeaderByHash(hash *chainhash.Hash) (*BlockHeader, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	header, ok := cm.byHash[*hash]
	if !ok {
		return nil, ErrHeaderNotFound
	}

	return header, nil
}

// GetTip returns the current chain tip
func (cm *ChainManager) GetTip() *BlockHeader {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.tip
}

// GetHeight returns the current chain height
func (cm *ChainManager) GetHeight() uint32 {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	if cm.tip == nil {
		return 0
	}
	return cm.tip.Height
}

// AddHeader adds a header to byHash for lookups without modifying the chain tip
func (cm *ChainManager) AddHeader(header *BlockHeader) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	hash := header.Hash()
	cm.byHash[hash] = header

	return nil
}

// GetNetwork returns the network name
func (cm *ChainManager) GetNetwork() string {
	return cm.network
}

// pruneOrphans removes old orphaned headers (must be called with lock held)
func (cm *ChainManager) pruneOrphans() {
	if cm.tip == nil {
		return
	}

	pruneHeight := uint32(0)
	if cm.tip.Height > 100 {
		pruneHeight = cm.tip.Height - 100
	}

	// Remove headers that are not in byHeight (orphans) and too old
	for hash, header := range cm.byHash {
		// Check if it's in the main chain
		if header.Height < uint32(len(cm.byHeight)) && cm.byHeight[header.Height] == hash {
			continue
		}
		// It's an orphan, check if too old
		if header.Height < pruneHeight {
			delete(cm.byHash, hash)
		}
	}
}
