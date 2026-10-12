package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/naresh04122005/ProxyServer/cache"
	"github.com/naresh04122005/ProxyServer/config"
	"github.com/naresh04122005/ProxyServer/proxy"
)

func main() {
	// 1. Load configuration
	cfg := config.NewDefaultConfig()
	cfg.ParseFlags()

	// 2. Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Print active configuration
	fmt.Println(cfg)

	// 3. Initialize Thread-Safe LRU Cache
	lruCache := cache.NewLRUCache(cfg.CacheSize)
	fmt.Printf("Initialized LRU cache with capacity: %d items\n", lruCache.Capacity())

	// 4. Initialize Proxy Engine and Middleware Chain
	proxyHandler := proxy.NewProxyHandler(lruCache, cfg)
	handler := proxy.CreateMiddlewareChain(proxyHandler, cfg)

	// 5. Initialize Production-Grade HTTP Server with Timeouts
	server := &http.Server{
		Addr:           fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler:        handler,
		ReadTimeout:    time.Duration(cfg.ReadTimeout) * time.Second,
		WriteTimeout:   time.Duration(cfg.WriteTimeout) * time.Second,
		IdleTimeout:    time.Duration(cfg.IdleTimeout) * time.Second,
		MaxHeaderBytes: cfg.MaxHeaderBytes,
	}

	// 6. Start server in background goroutine
	go func() {
		fmt.Printf("🛡️  Forward Proxy Server running on %s:%d\n", cfg.Host, cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Error starting server: %v", err)
		}
	}()

	// 7. Listen for OS interrupt signals (SIGINT / SIGTERM)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Wait for shutdown signal (Ctrl + C)
	<-stop
	fmt.Println("\nReceived shutdown signal. Stopping gracefully...")

	// Create 5-second timeout window to drain active requests
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Step A: Stop accepting new requests and drain active connections
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Error during server shutdown: %v", err)
	}

	// Step B: Safely terminate background workers and release pool
	proxyHandler.Shutdown()

	fmt.Println("Server gracefully stopped. Goodbye!")
}