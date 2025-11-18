# go-chaintracks

A Go implementation of blockchain header management for Bitcoin SV (BSV).

## Features

- In-memory chain tracking with height and hash indexes
- Chainwork calculation and comparison
- Automatic orphan pruning (keeps last 100 blocks)
- P2P live sync via message bus
- REST API compatible with TypeScript wallet-toolbox
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
cm, err := chaintracks.NewChainManager("main", "~/.chaintracks", "")
if err != nil {
    log.Fatal(err)
}

// Get current chain tip
tip := cm.GetTip()
height := cm.GetHeight()

// Get header by height
header, err := cm.GetHeaderByHeight(123456)

// Get header by hash
header, err := cm.GetHeaderByHash(&hash)

// Update chain tip with new headers
err = cm.SetChainTip(newHeaders)
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

- `GET /network` - Network name (main, test, or teratest)
- `GET /info` - Service state and configuration
- `GET /height` - Current blockchain height
- `GET /tip/hash` - Chain tip hash
- `GET /tip/header` - Chain tip header object
- `GET /header/height/?height=N` - Header by height (query param)
- `GET /header/hash/?hash=H` - Header by hash (query param)
- `GET /headers?height=N&count=C` - Multiple headers

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
- Standard library only

## License

See repository license.
