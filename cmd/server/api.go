package main

import (
	_ "embed"
	"encoding/hex"
	"strconv"

	"github.com/bsv-blockchain/go-chaintracks/pkg/chaintracks"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/gofiber/fiber/v2"
)

//go:embed openapi.yaml
var openapiSpec string

// Server wraps the ChainManager with Fiber handlers
type Server struct {
	cm *chaintracks.ChainManager
}

// NewServer creates a new API server
func NewServer(cm *chaintracks.ChainManager) *Server {
	return &Server{cm: cm}
}

// Response represents the standard API response format
type Response struct {
	Status      string      `json:"status"`
	Value       interface{} `json:"value,omitempty"`
	Code        string      `json:"code,omitempty"`
	Description string      `json:"description,omitempty"`
}

// BlockHeaderResponse represents a block header in API format
type BlockHeaderResponse struct {
	Version      int32           `json:"version"`
	PreviousHash *chainhash.Hash `json:"previousHash"`
	MerkleRoot   *chainhash.Hash `json:"merkleRoot"`
	Time         uint32          `json:"time"`
	Bits         uint32          `json:"bits"`
	Nonce        uint32          `json:"nonce"`
	Height       uint32          `json:"height"`
	Hash         *chainhash.Hash `json:"hash"`
}

// InfoResponse represents the service info response
type InfoResponse struct {
	Chain         string    `json:"chain"`
	HeightBulk    uint32    `json:"heightBulk"`
	HeightLive    uint32    `json:"heightLive"`
	Storage       string    `json:"storage"`
	BulkIngestors []string  `json:"bulkIngestors"`
	LiveIngestors []string  `json:"liveIngestors"`
	Packages      []Package `json:"packages"`
}

// Package represents a package version info
type Package struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// toBlockHeaderResponse converts a BlockHeader to API response format
func toBlockHeaderResponse(bh *chaintracks.BlockHeader) BlockHeaderResponse {
	hash := bh.Header.Hash()
	return BlockHeaderResponse{
		Version:      bh.Header.Version,
		PreviousHash: &bh.Header.PrevBlock,
		MerkleRoot:   &bh.Header.MerkleRoot,
		Time:         bh.Header.Timestamp,
		Bits:         bh.Header.Bits,
		Nonce:        bh.Header.Nonce,
		Height:       bh.Height,
		Hash:         &hash,
	}
}

// HandleRoot returns service identification
func (s *Server) HandleRoot(c *fiber.Ctx) error {
	return c.JSON(Response{
		Status: "success",
		Value:  "chaintracks-server",
	})
}

// HandleRobots returns robots.txt
func (s *Server) HandleRobots(c *fiber.Ctx) error {
	c.Set("Content-Type", "text/plain")
	return c.SendString("User-agent: *\nDisallow: /\n")
}

// HandleGetChain returns the network name
func (s *Server) HandleGetChain(c *fiber.Ctx) error {
	return c.JSON(Response{
		Status: "success",
		Value:  s.cm.GetNetwork(),
	})
}

// HandleGetInfo returns service state information
func (s *Server) HandleGetInfo(c *fiber.Ctx) error {
	height := s.cm.GetHeight()

	info := InfoResponse{
		Chain:         s.cm.GetNetwork(),
		HeightBulk:    height,
		HeightLive:    height,
		Storage:       "memory",
		BulkIngestors: []string{"local-files"},
		LiveIngestors: []string{},
		Packages: []Package{
			{Name: "go-chaintracks", Version: "0.1.0"},
		},
	}

	return c.JSON(Response{
		Status: "success",
		Value:  info,
	})
}

// HandleGetPresentHeight returns the current blockchain height
func (s *Server) HandleGetPresentHeight(c *fiber.Ctx) error {
	c.Set("Cache-Control", "public, max-age=60")
	return c.JSON(Response{
		Status: "success",
		Value:  s.cm.GetHeight(),
	})
}

// HandleFindChainTipHashHex returns the chain tip hash
func (s *Server) HandleFindChainTipHashHex(c *fiber.Ctx) error {
	c.Set("Cache-Control", "no-cache")

	tip := s.cm.GetTip()
	if tip == nil {
		return c.Status(fiber.StatusNotFound).JSON(Response{
			Status:      "error",
			Code:        "ERR_NO_TIP",
			Description: "Chain tip not found",
		})
	}

	hash := tip.Header.Hash()
	return c.JSON(Response{
		Status: "success",
		Value:  &hash,
	})
}

// HandleFindChainTipHeaderHex returns the full chain tip header
func (s *Server) HandleFindChainTipHeaderHex(c *fiber.Ctx) error {
	c.Set("Cache-Control", "no-cache")

	tip := s.cm.GetTip()
	if tip == nil {
		return c.Status(fiber.StatusNotFound).JSON(Response{
			Status:      "error",
			Code:        "ERR_NO_TIP",
			Description: "Chain tip not found",
		})
	}

	return c.JSON(Response{
		Status: "success",
		Value:  toBlockHeaderResponse(tip),
	})
}

// HandleFindHeaderHexForHeight returns a header by height
func (s *Server) HandleFindHeaderHexForHeight(c *fiber.Ctx) error {
	heightStr := c.Query("height")
	if heightStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(Response{
			Status:      "error",
			Code:        "ERR_INVALID_PARAMS",
			Description: "Missing height parameter",
		})
	}

	height, err := strconv.ParseUint(heightStr, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(Response{
			Status:      "error",
			Code:        "ERR_INVALID_PARAMS",
			Description: "Invalid height parameter",
		})
	}

	tip := s.cm.GetHeight()
	if uint32(height) < tip-100 {
		c.Set("Cache-Control", "public, max-age=3600")
	} else {
		c.Set("Cache-Control", "no-cache")
	}

	header, err := s.cm.GetHeaderByHeight(uint32(height))
	if err != nil {
		return c.JSON(Response{
			Status: "success",
			Value:  nil,
		})
	}

	return c.JSON(Response{
		Status: "success",
		Value:  toBlockHeaderResponse(header),
	})
}

// HandleFindHeaderHexForBlockHash returns a header by hash
func (s *Server) HandleFindHeaderHexForBlockHash(c *fiber.Ctx) error {
	hashStr := c.Query("hash")
	if hashStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(Response{
			Status:      "error",
			Code:        "ERR_INVALID_PARAMS",
			Description: "Missing hash parameter",
		})
	}

	hash, err := chainhash.NewHashFromHex(hashStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(Response{
			Status:      "error",
			Code:        "ERR_INVALID_PARAMS",
			Description: "Invalid hash parameter",
		})
	}

	header, err := s.cm.GetHeaderByHash(hash)
	if err != nil {
		return c.JSON(Response{
			Status: "success",
			Value:  nil,
		})
	}

	tip := s.cm.GetHeight()
	if header.Height < tip-100 {
		c.Set("Cache-Control", "public, max-age=3600")
	} else {
		c.Set("Cache-Control", "no-cache")
	}

	return c.JSON(Response{
		Status: "success",
		Value:  toBlockHeaderResponse(header),
	})
}

// HandleGetHeaders returns multiple headers as concatenated hex
func (s *Server) HandleGetHeaders(c *fiber.Ctx) error {
	heightStr := c.Query("height")
	countStr := c.Query("count")

	if heightStr == "" || countStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(Response{
			Status:      "error",
			Code:        "ERR_INVALID_PARAMS",
			Description: "Missing height or count parameter",
		})
	}

	height, err := strconv.ParseUint(heightStr, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(Response{
			Status:      "error",
			Code:        "ERR_INVALID_PARAMS",
			Description: "Invalid height parameter",
		})
	}

	count, err := strconv.ParseUint(countStr, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(Response{
			Status:      "error",
			Code:        "ERR_INVALID_PARAMS",
			Description: "Invalid count parameter",
		})
	}

	tip := s.cm.GetHeight()
	if uint32(height) < tip-100 {
		c.Set("Cache-Control", "public, max-age=3600")
	} else {
		c.Set("Cache-Control", "no-cache")
	}

	var hexData string
	for i := uint32(0); i < uint32(count); i++ {
		h := uint32(height) + i
		header, err := s.cm.GetHeaderByHeight(h)
		if err != nil {
			break
		}

		headerBytes := header.Header.Bytes()
		hexData += hex.EncodeToString(headerBytes)
	}

	return c.JSON(Response{
		Status: "success",
		Value:  hexData,
	})
}

// HandleOpenAPISpec serves the OpenAPI specification
func (s *Server) HandleOpenAPISpec(c *fiber.Ctx) error {
	c.Set("Content-Type", "application/yaml")
	return c.SendString(openapiSpec)
}

// HandleSwaggerUI serves the Swagger UI
func (s *Server) HandleSwaggerUI(c *fiber.Ctx) error {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Chaintracks API Documentation</title>
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.10.0/swagger-ui.css">
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5.10.0/swagger-ui-bundle.js"></script>
    <script>
        window.onload = function() {
            SwaggerUIBundle({
                url: '/openapi.yaml',
                dom_id: '#swagger-ui',
                deepLinking: true,
                presets: [
                    SwaggerUIBundle.presets.apis,
                    SwaggerUIBundle.SwaggerUIStandalonePreset
                ]
            });
        };
    </script>
</body>
</html>`
	c.Set("Content-Type", "text/html")
	return c.SendString(html)
}

// SetupRoutes configures all Fiber routes
func (s *Server) SetupRoutes(app *fiber.App, dashboard *DashboardHandler) {
	app.Get("/", dashboard.HandleStatus)
	app.Get("/robots.txt", s.HandleRobots)
	app.Get("/docs", s.HandleSwaggerUI)
	app.Get("/openapi.yaml", s.HandleOpenAPISpec)
	app.Get("/network", s.HandleGetChain)
	app.Get("/info", s.HandleGetInfo)
	app.Get("/height", s.HandleGetPresentHeight)
	app.Get("/tip/hash", s.HandleFindChainTipHashHex)
	app.Get("/tip/header", s.HandleFindChainTipHeaderHex)
	app.Get("/header/height/", s.HandleFindHeaderHexForHeight)
	app.Get("/header/hash/", s.HandleFindHeaderHexForBlockHash)
	app.Get("/headers", s.HandleGetHeaders)
}
