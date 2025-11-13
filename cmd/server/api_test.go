package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bsv-blockchain/go-chaintracks/pkg/chaintracks"
	"github.com/bsv-blockchain/go-sdk/chainhash"
)

func setupTestServer(t *testing.T) (*Server, *chaintracks.ChainManager) {
	cm, err := chaintracks.NewChainManager("main", "../testdata/headers")
	if err != nil {
		t.Fatalf("Failed to create chain manager: %v", err)
	}

	return NewServer(cm), cm
}

func TestHandleRoot(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	server.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Status != "success" {
		t.Errorf("Expected status 'success', got '%s'", resp.Status)
	}

	if resp.Value != "chaintracks-server" {
		t.Errorf("Expected value 'chaintracks-server', got '%v'", resp.Value)
	}
}

func TestHandleGetChain(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest("GET", "/getChain", nil)
	w := httptest.NewRecorder()

	server.HandleGetChain(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Status != "success" {
		t.Errorf("Expected status 'success', got '%s'", resp.Status)
	}

	if resp.Value != "main" {
		t.Errorf("Expected value 'main', got '%v'", resp.Value)
	}
}

func TestHandleGetInfo(t *testing.T) {
	server, cm := setupTestServer(t)

	req := httptest.NewRequest("GET", "/getInfo", nil)
	w := httptest.NewRecorder()

	server.HandleGetInfo(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp struct {
		Status string       `json:"status"`
		Value  InfoResponse `json:"value"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Status != "success" {
		t.Errorf("Expected status 'success', got '%s'", resp.Status)
	}

	if resp.Value.Chain != "main" {
		t.Errorf("Expected chain 'main', got '%s'", resp.Value.Chain)
	}

	expectedHeight := cm.GetHeight()
	if resp.Value.HeightBulk != expectedHeight {
		t.Errorf("Expected heightBulk %d, got %d", expectedHeight, resp.Value.HeightBulk)
	}
}

func TestHandleGetPresentHeight(t *testing.T) {
	server, cm := setupTestServer(t)

	req := httptest.NewRequest("GET", "/getPresentHeight", nil)
	w := httptest.NewRecorder()

	server.HandleGetPresentHeight(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	cacheControl := w.Header().Get("Cache-Control")
	if cacheControl != "public, max-age=60" {
		t.Errorf("Expected Cache-Control 'public, max-age=60', got '%s'", cacheControl)
	}

	var resp struct {
		Status string  `json:"status"`
		Value  float64 `json:"value"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Status != "success" {
		t.Errorf("Expected status 'success', got '%s'", resp.Status)
	}

	expectedHeight := cm.GetHeight()
	if uint32(resp.Value) != expectedHeight {
		t.Errorf("Expected height %d, got %d", expectedHeight, uint32(resp.Value))
	}
}

func TestHandleFindChainTipHashHex(t *testing.T) {
	server, cm := setupTestServer(t)

	req := httptest.NewRequest("GET", "/findChainTipHashHex", nil)
	w := httptest.NewRecorder()

	server.HandleFindChainTipHashHex(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	cacheControl := w.Header().Get("Cache-Control")
	if cacheControl != "no-cache" {
		t.Errorf("Expected Cache-Control 'no-cache', got '%s'", cacheControl)
	}

	var resp struct {
		Status string `json:"status"`
		Value  string `json:"value"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Status != "success" {
		t.Errorf("Expected status 'success', got '%s'", resp.Status)
	}

	tip := cm.GetTip()
	expectedHash := tip.Header.Hash().String()
	if resp.Value != expectedHash {
		t.Errorf("Expected hash %s, got %s", expectedHash, resp.Value)
	}
}

func TestHandleFindChainTipHeaderHex(t *testing.T) {
	server, cm := setupTestServer(t)

	req := httptest.NewRequest("GET", "/findChainTipHeaderHex", nil)
	w := httptest.NewRecorder()

	server.HandleFindChainTipHeaderHex(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp struct {
		Status string              `json:"status"`
		Value  BlockHeaderResponse `json:"value"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Status != "success" {
		t.Errorf("Expected status 'success', got '%s'", resp.Status)
	}

	tip := cm.GetTip()
	if resp.Value.Height != tip.Height {
		t.Errorf("Expected height %d, got %d", tip.Height, resp.Value.Height)
	}
}

func TestHandleFindHeaderHexForHeight(t *testing.T) {
	server, cm := setupTestServer(t)

	height := cm.GetHeight()
	if height < 100 {
		t.Skip("Not enough headers to test")
	}

	req := httptest.NewRequest("GET", "/findHeaderHexForHeight?height=100", nil)
	w := httptest.NewRecorder()

	server.HandleFindHeaderHexForHeight(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp struct {
		Status string              `json:"status"`
		Value  BlockHeaderResponse `json:"value"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Status != "success" {
		t.Errorf("Expected status 'success', got '%s'", resp.Status)
	}

	if resp.Value.Height != 100 {
		t.Errorf("Expected height 100, got %d", resp.Value.Height)
	}
}

func TestHandleFindHeaderHexForHeight_NotFound(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest("GET", "/findHeaderHexForHeight?height=99999999", nil)
	w := httptest.NewRecorder()

	server.HandleFindHeaderHexForHeight(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp struct {
		Status string      `json:"status"`
		Value  interface{} `json:"value"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Status != "success" {
		t.Errorf("Expected status 'success', got '%s'", resp.Status)
	}

	if resp.Value != nil {
		t.Errorf("Expected null value for non-existent header, got %v", resp.Value)
	}
}

func TestHandleFindHeaderHexForBlockHash(t *testing.T) {
	server, cm := setupTestServer(t)

	tip := cm.GetTip()
	hash := tip.Header.Hash().String()

	req := httptest.NewRequest("GET", "/findHeaderHexForBlockHash?hash="+hash, nil)
	w := httptest.NewRecorder()

	server.HandleFindHeaderHexForBlockHash(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp struct {
		Status string              `json:"status"`
		Value  BlockHeaderResponse `json:"value"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Status != "success" {
		t.Errorf("Expected status 'success', got '%s'", resp.Status)
	}

	if resp.Value.Height != tip.Height {
		t.Errorf("Expected height %d, got %d", tip.Height, resp.Value.Height)
	}
}

func TestHandleFindHeaderHexForBlockHash_InvalidHash(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest("GET", "/findHeaderHexForBlockHash?hash=invalid", nil)
	w := httptest.NewRecorder()

	server.HandleFindHeaderHexForBlockHash(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var resp Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Status != "error" {
		t.Errorf("Expected status 'error', got '%s'", resp.Status)
	}
}

func TestHandleFindHeaderHexForBlockHash_NotFound(t *testing.T) {
	server, _ := setupTestServer(t)

	nonExistentHash := chainhash.Hash{}
	req := httptest.NewRequest("GET", "/findHeaderHexForBlockHash?hash="+nonExistentHash.String(), nil)
	w := httptest.NewRecorder()

	server.HandleFindHeaderHexForBlockHash(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp struct {
		Status string      `json:"status"`
		Value  interface{} `json:"value"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Status != "success" {
		t.Errorf("Expected status 'success', got '%s'", resp.Status)
	}

	if resp.Value != nil {
		t.Errorf("Expected null value for non-existent header, got %v", resp.Value)
	}
}

func TestHandleGetHeaders(t *testing.T) {
	server, cm := setupTestServer(t)

	height := cm.GetHeight()
	if height < 10 {
		t.Skip("Not enough headers to test")
	}

	req := httptest.NewRequest("GET", "/getHeaders?height=0&count=10", nil)
	w := httptest.NewRecorder()

	server.HandleGetHeaders(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp struct {
		Status string `json:"status"`
		Value  string `json:"value"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Status != "success" {
		t.Errorf("Expected status 'success', got '%s'", resp.Status)
	}

	expectedLen := 10 * 80 * 2
	if len(resp.Value) != expectedLen {
		t.Errorf("Expected hex string length %d (10 headers * 80 bytes * 2), got %d", expectedLen, len(resp.Value))
	}
}

func TestHandleGetHeaders_MissingParams(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest("GET", "/getHeaders?height=0", nil)
	w := httptest.NewRecorder()

	server.HandleGetHeaders(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var resp Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Status != "error" {
		t.Errorf("Expected status 'error', got '%s'", resp.Status)
	}
}
