package chaintracks

import (
	"math/big"
	"sync"

	"github.com/bsv-blockchain/go-sdk/block"
	"github.com/bsv-blockchain/go-sdk/chainhash"
)

// BlockHeader extends the base block.Header with additional chain-specific metadata
type BlockHeader struct {
	*block.Header
	Height    uint32   // Block height in the chain
	ChainWork *big.Int // Cumulative chain work up to and including this block
}

// HeaderStore manages the in-memory storage of block headers with concurrent access
type HeaderStore struct {
	mu sync.RWMutex

	byHeight []chainhash.Hash                // Main chain hashes indexed by height
	byHash   map[chainhash.Hash]*BlockHeader // Hash → Header (all headers: main + orphans)
	tip      *BlockHeader                    // Current chain tip

	network string // "main" or "test"
}

// NewHeaderStore creates a new HeaderStore
func NewHeaderStore(network string) *HeaderStore {
	return &HeaderStore{
		byHeight: make([]chainhash.Hash, 0, 1000000), // Pre-allocate for ~1M blocks
		byHash:   make(map[chainhash.Hash]*BlockHeader),
		network:  network,
	}
}

// GetHeaderByHeight retrieves a header from the main chain by height
func (s *HeaderStore) GetHeaderByHeight(height uint32) (*BlockHeader, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if height >= uint32(len(s.byHeight)) {
		return nil, ErrHeaderNotFound
	}

	hash := s.byHeight[height]
	header, ok := s.byHash[hash]
	if !ok {
		return nil, ErrHeaderNotFound
	}

	return header, nil
}

// GetHeaderByHash retrieves a header by hash (main chain or orphan)
func (s *HeaderStore) GetHeaderByHash(hash *chainhash.Hash) (*BlockHeader, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	header, ok := s.byHash[*hash]
	if !ok {
		return nil, ErrHeaderNotFound
	}

	return header, nil
}

// GetTip returns the current chain tip
func (s *HeaderStore) GetTip() *BlockHeader {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tip
}

// GetHeight returns the current chain height
func (s *HeaderStore) GetHeight() uint32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.tip == nil {
		return 0
	}
	return s.tip.Height
}

// AddHeader adds a header to the store
func (s *HeaderStore) AddHeader(header *BlockHeader) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	hash := header.Hash()

	// Check if already exists
	if _, exists := s.byHash[hash]; exists {
		return ErrDuplicateHeader
	}

	// Add to byHash (all headers)
	s.byHash[hash] = header

	// Ensure slice is large enough for this height
	for uint32(len(s.byHeight)) <= header.Height {
		s.byHeight = append(s.byHeight, chainhash.Hash{})
	}

	// Add to byHeight (main chain only)
	s.byHeight[header.Height] = hash

	// Update tip if this is the new highest block
	if s.tip == nil || header.Height > s.tip.Height {
		s.tip = header
	}

	return nil
}

// AddOrphan adds an orphaned header (not on main chain)
func (s *HeaderStore) AddOrphan(header *BlockHeader) {
	s.mu.Lock()
	defer s.mu.Unlock()

	hash := header.Hash()
	s.byHash[hash] = header
}

// RemoveOrphan removes a header entirely
func (s *HeaderStore) RemoveOrphan(hash *chainhash.Hash) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.byHash, *hash)
}

// GetOrphan retrieves a header by hash (same as GetHeaderByHash now)
func (s *HeaderStore) GetOrphan(hash *chainhash.Hash) (*BlockHeader, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	header, ok := s.byHash[*hash]
	return header, ok
}

// PruneOrphans removes orphans older than maxDepth from current tip
func (s *HeaderStore) PruneOrphans(maxDepth uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.tip == nil {
		return
	}

	pruneHeight := uint32(0)
	if s.tip.Height > maxDepth {
		pruneHeight = s.tip.Height - maxDepth
	}

	// Remove headers that are not in byHeight (orphans) and too old
	for hash, header := range s.byHash {
		// Check if it's in the main chain
		if header.Height < uint32(len(s.byHeight)) && s.byHeight[header.Height] == hash {
			continue
		}
		// It's an orphan, check if too old
		if header.Height < pruneHeight {
			delete(s.byHash, hash)
		}
	}
}

// HasHeader checks if a header exists by hash
func (s *HeaderStore) HasHeader(hash *chainhash.Hash) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, ok := s.byHash[*hash]
	return ok
}

// Count returns the number of headers in the main chain
func (s *HeaderStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.byHeight)
}

// OrphanCount returns the number of orphaned headers
func (s *HeaderStore) OrphanCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.byHash) - len(s.byHeight)
}
