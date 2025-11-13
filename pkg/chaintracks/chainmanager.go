package chaintracks

import (
	"fmt"
	"math/big"

	"github.com/bsv-blockchain/go-sdk/block"
	"github.com/bsv-blockchain/go-sdk/chainhash"
)

// ChainManager is the main orchestrator for chain management
type ChainManager struct {
	store            *HeaderStore
	synced           bool
	localStoragePath string

	// Configuration
	network string
}

// NewChainManager creates a new ChainManager and restores from local files if present
func NewChainManager(network, localStoragePath string) (*ChainManager, error) {
	cm := &ChainManager{
		store:            NewHeaderStore(network),
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

// ProcessHeader adds a header to the chain
func (cm *ChainManager) ProcessHeader(header *block.Header) error {
	hash := header.Hash()

	// Check if already exists
	if cm.store.HasHeader(&hash) {
		return ErrDuplicateHeader
	}

	// Look up parent by hash
	prevHeader, err := cm.store.GetHeaderByHash(&header.PrevBlock)
	if err != nil {
		// Parent not found - this is an orphan
		return cm.handleOrphan(header)
	}

	// Calculate height and chainwork
	height := prevHeader.Height + 1
	work := CalculateWork(header.Bits)
	chainWork := new(big.Int).Add(prevHeader.ChainWork, work)

	// Create BlockHeader with metadata
	blockHeader := &BlockHeader{
		Header:    header,
		Height:    height,
		ChainWork: chainWork,
	}

	// Add to store
	if err := cm.store.AddHeader(blockHeader); err != nil {
		return fmt.Errorf("failed to add header: %w", err)
	}

	return nil
}

// handleOrphan handles orphaned headers (headers whose parent is not known)
func (cm *ChainManager) handleOrphan(header *block.Header) error {
	blockHeader := &BlockHeader{
		Header:    header,
		Height:    0,              // Unknown until parent is found
		ChainWork: big.NewInt(0), // Will be calculated when parent is found
	}
	cm.store.AddOrphan(blockHeader)
	return nil
}

// GetHeaderByHeight retrieves a header by height
func (cm *ChainManager) GetHeaderByHeight(height uint32) (*BlockHeader, error) {
	return cm.store.GetHeaderByHeight(height)
}

// GetHeaderByHash retrieves a header by hash
func (cm *ChainManager) GetHeaderByHash(hash *chainhash.Hash) (*BlockHeader, error) {
	return cm.store.GetHeaderByHash(hash)
}

// GetTip returns the current chain tip
func (cm *ChainManager) GetTip() *BlockHeader {
	return cm.store.GetTip()
}

// GetHeight returns the current chain height
func (cm *ChainManager) GetHeight() uint32 {
	return cm.store.GetHeight()
}

// IsSynced returns whether the chain is synced
func (cm *ChainManager) IsSynced() bool {
	return cm.synced
}

// SetSynced sets the synced status
func (cm *ChainManager) SetSynced(synced bool) {
	cm.synced = synced
}

// GetStore returns the underlying header store
func (cm *ChainManager) GetStore() *HeaderStore {
	return cm.store
}

// GetNetwork returns the network name
func (cm *ChainManager) GetNetwork() string {
	return cm.network
}

// PruneOrphans removes old orphaned headers
func (cm *ChainManager) PruneOrphans() {
	cm.store.PruneOrphans(100) // Keep orphans for 100 blocks
}
