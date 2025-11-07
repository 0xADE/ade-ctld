package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/0xADE/ade-ctld/internal/config"
	"github.com/0xADE/ade-ctld/internal/indexer"
	"github.com/0xADE/ade-ctld/server"
)

func main() {
	// Initialize configuration
	if err := config.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize config: %v\n", err)
		os.Exit(1)
	}

	// Start config watcher
	if err := config.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start config watcher: %v\n", err)
		os.Exit(1)
	}

	// Create indexer
	idx := indexer.NewIndexer()

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start indexing
	if err := idx.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start indexer: %v\n", err)
		os.Exit(1)
	}

	// Create server
	srv, err := server.NewServer(idx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create server: %v\n", err)
		os.Exit(1)
	}

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- srv.Start(ctx)
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	fmt.Println("ade-exe-ctld started")

	select {
	case sig := <-sigChan:
		fmt.Printf("\nReceived signal: %v\n", sig)
		cancel()
		idx.Stop()
		if err := srv.Stop(); err != nil {
			fmt.Fprintf(os.Stderr, "Error stopping server: %v\n", err)
		}
	case err := <-serverErr:
		if err != nil {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Println("ade-exe-ctld stopped")
}
