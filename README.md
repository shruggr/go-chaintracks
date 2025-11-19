# go-chaintracks

A Go implementation of blockchain header management for Bitcoin SV (BSV).

## Features

- In-memory chain tracking with height and hash indexes
- Chainwork calculation and comparison
- Automatic orphan pruning (keeps last 100 blocks)
- P2P live sync with automatic updates
- Optional bootstrap sync from remote node
- REST API with v2 endpoints
- File-based persistence with metadata

## Installation

```bash
go get github.com/bsv-blockchain/go-chaintracks
```

## Usage

### As a Library

```go
import "github.com/bsv-blockchain/go-chaintracks/pkg/chaintracks"

// Create chain manager with local storage
// Network options: "main", "test", "teratest"
// Optional bootstrap URL for initial sync
cm, err := chaintracks.NewChainManager("main", "~/.chaintracks", "https://node.example.com")
if err != nil {
    log.Fatal(err)
}

// Start P2P sync for automatic updates
ctx := context.Background()
tipChanges, err := cm.Start(ctx)
if err != nil {
    log.Fatal(err)
}

// Listen for tip changes (optional)
go func() {
    for tip := range tipChanges {
        log.Printf("New tip: height=%d hash=%s", tip.Height, tip.Hash())
    }
}()

// Query methods
tip := cm.GetTip()
height := cm.GetHeight()
header, err := cm.GetHeaderByHeight(123456)
header, err := cm.GetHeaderByHash(&hash)

// Cleanup
defer cm.Stop()
```

### As a Client

```go
import "github.com/bsv-blockchain/go-chaintracks/pkg/chaintracks"

// Connect to remote chaintracks server
client := chaintracks.NewChainClient("http://localhost:3011")

// Start SSE connection for automatic updates
ctx := context.Background()
tipChanges, err := client.Start(ctx)
if err != nil {
    log.Fatal(err)
}

// Listen for tip changes (optional)
go func() {
    for tip := range tipChanges {
        log.Printf("New tip: height=%d hash=%s", tip.Height, tip.Hash)
    }
}()

// Query methods (same interface as ChainManager)
tip := client.GetTip()
height := client.GetHeight()
header, err := client.GetHeaderByHeight(123456)
header, err := client.GetHeaderByHash(&hash)

// Cleanup
defer client.Stop()
```

### As a Server

```bash
# Build and run
go build -o server ./cmd/server
./server

# Configure via .env file
cp .env.example .env
# Edit .env with your settings

# Or configure via environment variables
PORT=3011 CHAIN=main STORAGE_PATH=~/.chaintracks ./server
```

Server starts on port 3011 with Swagger UI at `/docs`.

## API Endpoints

- `GET /v2/network` - Network name (main, test, or teratest)
- `GET /v2/height` - Current blockchain height
- `GET /v2/tip/hash` - Chain tip hash
- `GET /v2/tip/header` - Chain tip header object
- `GET /v2/tip/stream` - SSE stream for real-time tip updates
- `GET /v2/header/height/:height` - Header by height (path param)
- `GET /v2/header/hash/:hash` - Header by hash (path param)
- `GET /v2/headers?height=N&count=C` - Multiple headers

Full API documentation available at `/docs` when running.

## Data Storage

Headers are stored in 100k-block files:
```
~/.chaintracks/
├── mainNetBlockHeaders.json    # Metadata
├── mainNet_0.headers            # Blocks 0-99999
├── mainNet_1.headers            # Blocks 100000-199999
└── ...
```

Each header is 80 bytes. Files use seek-based updates for efficient writes.

## Testing

```bash
go test ./pkg/chaintracks -v
```

## Architecture

- **ChainManager** - Main orchestrator for chain operations
- **BlockHeader** - Extends SDK header with height and chainwork
- **File I/O** - Local storage with seek-based updates
- **P2P Sync** - Live header updates via message bus
- **ChainTracker** - Implements go-sdk interface

## Dependencies

- `github.com/bsv-blockchain/go-sdk` - BSV blockchain SDK
- `github.com/gofiber/fiber/v2` - Web framework (server only)
- `github.com/joho/godotenv` - Environment configuration (server only)

## License

See repository license.
