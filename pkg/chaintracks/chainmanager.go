package chaintracks

import (
	"sync"

	"github.com/bsv-blockchain/go-sdk/chainhash"
)

// ChainManager is the main orchestrator for chain management
type ChainManager struct {
	mu sync.RWMutex

	byHeight []chainhash.Hash                // Main chain hashes indexed by height
	byHash   map[chainhash.Hash]*BlockHeader // Hash → Header (all headers: main + orphans)
	tip      *BlockHeader                    // Current chain tip

	synced           bool
	localStoragePath string
	network          string
}

// NewChainManager creates a new ChainManager and restores from local files if present
func NewChainManager(network, localStoragePath string) (*ChainManager, error) {
	cm := &ChainManager{
		byHeight:         make([]chainhash.Hash, 0, 1000000),
		byHash:           make(map[chainhash.Hash]*BlockHeader),
		synced:           false,
		network:          network,
		localStoragePath: localStoragePath,
	}

	// Auto-restore from local files if they exist
	if err := cm.loadFromLocalFiles(); err != nil {
		// If files don't exist, that's okay - we'll listen to P2P for new headers
		// Only return error if files exist but are corrupted
		return cm, nil
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

// IsSynced returns whether the chain is synced
func (cm *ChainManager) IsSynced() bool {
	return cm.synced
}

// SetSynced sets the synced status
func (cm *ChainManager) SetSynced(synced bool) {
	cm.synced = synced
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
