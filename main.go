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

	"github.com/Jovial-Kanwadia/proxy-server/cache"
	"github.com/Jovial-Kanwadia/proxy-server/config"
	"github.com/Jovial-Kanwadia/proxy-server/proxy"
)

func main() {
	// Load configuration
	cfg := config.NewDefaultConfig()
	cfg.ParseFlags()

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Print configuration for debugging
	fmt.Println(cfg)

	// Create LRU cache
	lruCache := cache.NewLRUCache(cfg.CacheSize)
	fmt.Printf("Initialized LRU cache with capacity: %d\n", lruCache.Capacity())

	// Create proxy handler
	proxyHandler := proxy.NewProxyHandler(lruCache, cfg)
	
	// Apply middleware chain
	handler := proxy.CreateMiddlewareChain(proxyHandler, cfg)
	
	// Create server with timeouts
	server := &http.Server{
		Addr:           fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler:        handler,
		ReadTimeout:    time.Duration(cfg.ReadTimeout) * time.Second,
		WriteTimeout:   time.Duration(cfg.WriteTimeout) * time.Second,
		IdleTimeout:    time.Duration(cfg.IdleTimeout) * time.Second,
		MaxHeaderBytes: cfg.MaxHeaderBytes,
	}

	// Start server in goroutine to not block
	go func() {
		fmt.Printf("Starting proxy server on %s:%d\n", cfg.Host, cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Error starting server: %v", err)
		}
	}()

	// Set up graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Wait for interrupt signal
	<-stop
	fmt.Println("Shutting down server...")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Shutdown the proxy handler (which will stop the worker pool)
	proxyHandler.Shutdown()

	// Shutdown server
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Error during server shutdown: %v", err)
	}

	fmt.Println("Server gracefully stopped")
}