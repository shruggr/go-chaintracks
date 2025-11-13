package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/bsv-blockchain/go-chaintracks/pkg/chaintracks"
)

func main() {
	config := LoadConfig()

	log.Printf("Starting chaintracks-server")
	log.Printf("  Network: %s", config.Network)
	log.Printf("  Port: %d", config.Port)
	log.Printf("  Storage Path: %s", config.StoragePath)

	if err := ensureHeadersExist(config.StoragePath, config.Network); err != nil {
		log.Fatalf("Failed to initialize headers: %v", err)
	}

	cm, err := chaintracks.NewChainManager(config.Network, config.StoragePath)
	if err != nil {
		log.Fatalf("Failed to create chain manager: %v", err)
	}

	height := cm.GetHeight()
	log.Printf("Loaded %d headers", height)

	if tip := cm.GetTip(); tip != nil {
		log.Printf("Chain tip: %s at height %d", tip.Header.Hash().String(), tip.Height)
	}

	server := NewServer(cm)

	mux := http.NewServeMux()
	server.SetupRoutes(mux)

	handler := loggingMiddleware(corsMiddleware(mux))

	addr := fmt.Sprintf(":%d", config.Port)
	httpServer := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	go func() {
		log.Printf("Server listening on http://localhost%s", addr)
		log.Printf("Available endpoints:")
		log.Printf("  GET  http://localhost%s/docs - API Documentation (Swagger UI)", addr)
		log.Printf("  GET  http://localhost%s/getInfo", addr)
		log.Printf("  GET  http://localhost%s/getPresentHeight", addr)
		log.Printf("  GET  http://localhost%s/findChainTipHeaderHex", addr)
		log.Printf("Press Ctrl+C to stop")

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down gracefully...")
	if err := httpServer.Close(); err != nil {
		log.Printf("Error closing server: %v", err)
	}
	log.Println("Server stopped")
}

// ensureHeadersExist checks if headers exist at storagePath, and if not, copies from checkpoint
func ensureHeadersExist(storagePath, network string) error {
	metadataFile := filepath.Join(storagePath, network+"NetBlockHeaders.json")

	if _, err := os.Stat(metadataFile); err == nil {
		return nil
	}

	log.Printf("No headers found at %s, initializing from checkpoint...", storagePath)

	checkpointPath := filepath.Join("data", "headers")
	checkpointMetadata := filepath.Join(checkpointPath, network+"NetBlockHeaders.json")

	if _, err := os.Stat(checkpointMetadata); os.IsNotExist(err) {
		log.Printf("Warning: No checkpoint headers found at %s", checkpointPath)
		return nil
	}

	if err := os.MkdirAll(storagePath, 0755); err != nil {
		return fmt.Errorf("failed to create storage directory: %w", err)
	}

	files, err := filepath.Glob(filepath.Join(checkpointPath, network+"*"))
	if err != nil {
		return fmt.Errorf("failed to list checkpoint files: %w", err)
	}

	log.Printf("Copying %d checkpoint files to %s...", len(files), storagePath)
	for _, srcFile := range files {
		dstFile := filepath.Join(storagePath, filepath.Base(srcFile))
		if err := copyFile(srcFile, dstFile); err != nil {
			return fmt.Errorf("failed to copy %s: %w", srcFile, err)
		}
	}

	log.Printf("Checkpoint headers initialized successfully")
	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return err
	}

	return destFile.Sync()
}
