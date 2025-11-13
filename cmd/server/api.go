package main

import (
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/bsv-blockchain/go-chaintracks/pkg/chaintracks"
	"github.com/bsv-blockchain/go-sdk/chainhash"
)

//go:embed openapi.yaml
var openapiSpec string

// Server wraps the ChainManager with HTTP handlers
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

// writeJSON writes a JSON response
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeSuccess writes a success response
func (s *Server) writeSuccess(w http.ResponseWriter, value interface{}) {
	writeJSON(w, http.StatusOK, Response{
		Status: "success",
		Value:  value,
	})
}

// writeError writes an error response
func (s *Server) writeError(w http.ResponseWriter, code string, description string, status int) {
	writeJSON(w, status, Response{
		Status:      "error",
		Code:        code,
		Description: description,
	})
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
func (s *Server) HandleRoot(w http.ResponseWriter, r *http.Request) {
	s.writeSuccess(w, "chaintracks-server")
}

// HandleRobots returns robots.txt
func (s *Server) HandleRobots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("User-agent: *\nDisallow: /\n"))
}

// HandleGetChain returns the network name
func (s *Server) HandleGetChain(w http.ResponseWriter, r *http.Request) {
	s.writeSuccess(w, s.cm.GetNetwork())
}

// HandleGetInfo returns service state information
func (s *Server) HandleGetInfo(w http.ResponseWriter, r *http.Request) {
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

	s.writeSuccess(w, info)
}

// HandleGetPresentHeight returns the current blockchain height
func (s *Server) HandleGetPresentHeight(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=60")
	s.writeSuccess(w, s.cm.GetHeight())
}

// HandleFindChainTipHashHex returns the chain tip hash
func (s *Server) HandleFindChainTipHashHex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache")

	tip := s.cm.GetTip()
	if tip == nil {
		s.writeError(w, "ERR_NO_TIP", "Chain tip not found", http.StatusNotFound)
		return
	}

	hash := tip.Header.Hash()
	s.writeSuccess(w, &hash)
}

// HandleFindChainTipHeaderHex returns the full chain tip header
func (s *Server) HandleFindChainTipHeaderHex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache")

	tip := s.cm.GetTip()
	if tip == nil {
		s.writeError(w, "ERR_NO_TIP", "Chain tip not found", http.StatusNotFound)
		return
	}

	s.writeSuccess(w, toBlockHeaderResponse(tip))
}

// HandleFindHeaderHexForHeight returns a header by height
func (s *Server) HandleFindHeaderHexForHeight(w http.ResponseWriter, r *http.Request) {
	heightStr := r.URL.Query().Get("height")
	if heightStr == "" {
		s.writeError(w, "ERR_INVALID_PARAMS", "Missing height parameter", http.StatusBadRequest)
		return
	}

	height, err := strconv.ParseUint(heightStr, 10, 32)
	if err != nil {
		s.writeError(w, "ERR_INVALID_PARAMS", "Invalid height parameter", http.StatusBadRequest)
		return
	}

	tip := s.cm.GetHeight()
	if uint32(height) < tip-100 {
		w.Header().Set("Cache-Control", "public, max-age=3600")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}

	header, err := s.cm.GetHeaderByHeight(uint32(height))
	if err != nil {
		s.writeSuccess(w, nil)
		return
	}

	s.writeSuccess(w, toBlockHeaderResponse(header))
}

// HandleFindHeaderHexForBlockHash returns a header by hash
func (s *Server) HandleFindHeaderHexForBlockHash(w http.ResponseWriter, r *http.Request) {
	hashStr := r.URL.Query().Get("hash")
	if hashStr == "" {
		s.writeError(w, "ERR_INVALID_PARAMS", "Missing hash parameter", http.StatusBadRequest)
		return
	}

	hash, err := chainhash.NewHashFromHex(hashStr)
	if err != nil {
		s.writeError(w, "ERR_INVALID_PARAMS", "Invalid hash parameter", http.StatusBadRequest)
		return
	}

	header, err := s.cm.GetHeaderByHash(hash)
	if err != nil {
		s.writeSuccess(w, nil)
		return
	}

	tip := s.cm.GetHeight()
	if header.Height < tip-100 {
		w.Header().Set("Cache-Control", "public, max-age=3600")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}

	s.writeSuccess(w, toBlockHeaderResponse(header))
}

// HandleGetHeaders returns multiple headers as concatenated hex
func (s *Server) HandleGetHeaders(w http.ResponseWriter, r *http.Request) {
	heightStr := r.URL.Query().Get("height")
	countStr := r.URL.Query().Get("count")

	if heightStr == "" || countStr == "" {
		s.writeError(w, "ERR_INVALID_PARAMS", "Missing height or count parameter", http.StatusBadRequest)
		return
	}

	height, err := strconv.ParseUint(heightStr, 10, 32)
	if err != nil {
		s.writeError(w, "ERR_INVALID_PARAMS", "Invalid height parameter", http.StatusBadRequest)
		return
	}

	count, err := strconv.ParseUint(countStr, 10, 32)
	if err != nil {
		s.writeError(w, "ERR_INVALID_PARAMS", "Invalid count parameter", http.StatusBadRequest)
		return
	}

	tip := s.cm.GetHeight()
	if uint32(height) < tip-100 {
		w.Header().Set("Cache-Control", "public, max-age=3600")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
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

	s.writeSuccess(w, hexData)
}

// HandleOpenAPISpec serves the OpenAPI specification
func (s *Server) HandleOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	w.Write([]byte(openapiSpec))
}

// HandleSwaggerUI serves the Swagger UI
func (s *Server) HandleSwaggerUI(w http.ResponseWriter, r *http.Request) {
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
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// SetupRoutes configures all HTTP routes
func (s *Server) SetupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/", s.HandleRoot)
	mux.HandleFunc("/robots.txt", s.HandleRobots)
	mux.HandleFunc("/docs", s.HandleSwaggerUI)
	mux.HandleFunc("/openapi.yaml", s.HandleOpenAPISpec)
	mux.HandleFunc("/getChain", s.HandleGetChain)
	mux.HandleFunc("/getInfo", s.HandleGetInfo)
	mux.HandleFunc("/getPresentHeight", s.HandleGetPresentHeight)
	mux.HandleFunc("/findChainTipHashHex", s.HandleFindChainTipHashHex)
	mux.HandleFunc("/findChainTipHeaderHex", s.HandleFindChainTipHeaderHex)
	mux.HandleFunc("/findHeaderHexForHeight", s.HandleFindHeaderHexForHeight)
	mux.HandleFunc("/findHeaderHexForBlockHash", s.HandleFindHeaderHexForBlockHash)
	mux.HandleFunc("/getHeaders", s.HandleGetHeaders)
}
