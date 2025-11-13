package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"

	"github.com/bsv-blockchain/go-chaintracks/pkg/chaintracks"
	"github.com/bsv-blockchain/go-sdk/block"
	p2p "github.com/bsv-blockchain/go-p2p-message-bus"
	"github.com/libp2p/go-libp2p/core/crypto"
)

// BlockMessage represents a block announcement from the P2P network
type BlockMessage struct {
	PeerID     string `json:"PeerID"`
	ClientName string `json:"ClientName"`
	DataHubURL string `json:"DataHubURL"`
	Hash       string `json:"Hash"`
	Height     uint32 `json:"Height"`
	Header     string `json:"Header"`
	Coinbase   string `json:"Coinbase"`
}

// PeerInfo contains information about a connected peer
type PeerInfo struct {
	ID    string
	Name  string
	Addrs []string
}

// P2PListener handles incoming block messages from the P2P network
type P2PListener struct {
	client      p2p.Client
	chainMgr    *chaintracks.ChainManager
	network     string
	storagePath string
}

// NewP2PListener creates a new P2P listener
func NewP2PListener(chainMgr *chaintracks.ChainManager, network, storagePath string) (*P2PListener, error) {
	// Load or generate private key
	privKey, err := loadOrGeneratePrivateKey(storagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load private key: %w", err)
	}

	// Create P2P client
	client, err := p2p.NewClient(p2p.Config{
		Name:          "go-chaintracks",
		Logger:        &p2p.DefaultLogger{},
		PrivateKey:    privKey,
		Port:          0, // Random port
		PeerCacheFile: filepath.Join(storagePath, "peer_cache.json"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create P2P client: %w", err)
	}

	return &P2PListener{
		client:      client,
		chainMgr:    chainMgr,
		network:     network,
		storagePath: storagePath,
	}, nil
}

// Start begins listening for block messages
func (l *P2PListener) Start(ctx context.Context) error {
	topic := fmt.Sprintf("teranode/bitcoin/1.0.0/%s-block", l.network)
	log.Printf("Subscribing to P2P topic: %s", topic)

	msgChan := l.client.Subscribe(topic)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-msgChan:
				if err := l.handleBlockMessage(ctx, msg.Data); err != nil {
					log.Printf("Error handling block message: %v", err)
				}
			}
		}
	}()

	return nil
}

// Close shuts down the P2P listener
func (l *P2PListener) Close() error {
	return l.client.Close()
}

// GetPeers returns information about connected peers
func (l *P2PListener) GetPeers() []PeerInfo {
	p2pPeers := l.client.GetPeers()
	peers := make([]PeerInfo, len(p2pPeers))
	for i, p := range p2pPeers {
		peers[i] = PeerInfo{
			ID:    p.ID,
			Name:  p.Name,
			Addrs: p.Addrs,
		}
	}
	return peers
}

// handleBlockMessage processes a received block message
func (l *P2PListener) handleBlockMessage(ctx context.Context, data []byte) error {
	log.Printf("Raw block message: %s", string(data))

	var blockMsg BlockMessage
	if err := json.Unmarshal(data, &blockMsg); err != nil {
		return fmt.Errorf("failed to unmarshal block message: %w", err)
	}

	log.Printf("Received block: height=%d hash=%s from=%s datahub=%s", blockMsg.Height, blockMsg.Hash, blockMsg.PeerID, blockMsg.DataHubURL)

	// Decode header from hex
	headerBytes, err := hex.DecodeString(blockMsg.Header)
	if err != nil {
		return fmt.Errorf("failed to decode header hex: %w", err)
	}

	if len(headerBytes) != 80 {
		return fmt.Errorf("invalid header size: %d bytes", len(headerBytes))
	}

	header, err := block.NewHeaderFromBytes(headerBytes)
	if err != nil {
		return fmt.Errorf("failed to parse header: %w", err)
	}

	// Check if parent exists in our chain
	parentHash := header.PrevBlock
	_, err = l.chainMgr.GetHeaderByHash(&parentHash)
	if err == nil {
		// Parent exists - simple case
		return l.addBlockToChain(header, blockMsg.Height)
	}

	// Parent doesn't exist - need to crawl back
	log.Printf("Parent not found for block %s, crawling back...", blockMsg.Hash)
	return l.crawlBackAndMerge(ctx, header, blockMsg.Height, blockMsg.DataHubURL)
}

// addBlockToChain adds a single block that extends our current chain
func (l *P2PListener) addBlockToChain(header *block.Header, height uint32) error {
	// Get parent to calculate chainwork
	parentHash := header.PrevBlock
	parentHeader, err := l.chainMgr.GetHeaderByHash(&parentHash)
	if err != nil {
		return fmt.Errorf("failed to get parent header: %w", err)
	}

	// Calculate chainwork
	work := chaintracks.CalculateWork(header.Bits)
	chainWork := new(big.Int).Add(parentHeader.ChainWork, work)

	// Create BlockHeader
	blockHeader := &chaintracks.BlockHeader{
		Header:    header,
		Height:    height,
		ChainWork: chainWork,
	}

	// Check if this is the new tip
	currentTip := l.chainMgr.GetTip()
	if currentTip == nil || blockHeader.ChainWork.Cmp(currentTip.ChainWork) > 0 {
		log.Printf("New tip: height=%d chainwork=%s", blockHeader.Height, blockHeader.ChainWork.String())
		return l.chainMgr.SetChainTip([]*chaintracks.BlockHeader{blockHeader})
	}

	// Not the new tip - add to byHash for lookups without changing tip
	return l.chainMgr.AddHeader(blockHeader)
}

// crawlBackAndMerge fetches missing parents until we find a connection to our chain
// Uses the shared SyncFromRemoteTip function for efficient batch fetching
func (l *P2PListener) crawlBackAndMerge(ctx context.Context, header *block.Header, height uint32, dataHubURL string) error {
	// Use the shared sync logic to walk backwards and find common ancestor
	blockHash := header.Hash()
	return SyncFromRemoteTip(l.chainMgr, blockHash, dataHubURL)
}

// loadOrGeneratePrivateKey loads a private key from file or generates a new one
func loadOrGeneratePrivateKey(storagePath string) (crypto.PrivKey, error) {
	keyPath := filepath.Join(storagePath, "p2p_key.hex")

	// Try to load existing key
	if _, err := os.Stat(keyPath); err == nil {
		data, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read key file: %w", err)
		}

		keyHex := string(data)
		privKey, err := p2p.PrivateKeyFromHex(keyHex)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}

		log.Printf("Loaded P2P private key from %s", keyPath)
		return privKey, nil
	}

	// Generate new key
	privKey, err := p2p.GeneratePrivateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	// Save to file
	keyHex, err := p2p.PrivateKeyToHex(privKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encode private key: %w", err)
	}

	if err := os.MkdirAll(storagePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	if err := os.WriteFile(keyPath, []byte(keyHex), 0600); err != nil {
		return nil, fmt.Errorf("failed to write key file: %w", err)
	}

	log.Printf("Generated new P2P private key: %s", keyPath)
	return privKey, nil
}
