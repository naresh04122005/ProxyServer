package proxy

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/naresh04122005/ProxyServer/config"
)

// Middleware is a function that wraps an http.Handler
type Middleware func(http.Handler) http.Handler

// Chain applies multiple middleware to a handler
func Chain(handler http.Handler, middlewares ...Middleware) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

// Logger middleware logs incoming proxy requests with client IP, method, and destination host
func Logger() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap ResponseWriter to capture status code
			rw := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// Call next handler
			next.ServeHTTP(rw, r)

			// Log request details
			duration := time.Since(start)
			targetHost := r.Host
			if targetHost == "" {
				targetHost = r.URL.Host
			}

			log.Printf(
				"[%s] %s -> %s %s | Status: %d | Duration: %s",
				r.RemoteAddr,
				r.Method,
				targetHost,
				r.URL.Path,
				rw.statusCode,
				duration,
			)
		})
	}
}

// Compress middleware compresses responses using gzip when supported by client
func Compress() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Don't compress if client doesn't accept gzip or if it's a CONNECT tunnel
			if r.Method == http.MethodConnect || !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
				next.ServeHTTP(w, r)
				return
			}

			// Create gzip writer
			gz, err := gzip.NewWriterLevel(w, gzip.BestSpeed)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			defer gz.Close()

			// Set Content-Encoding header
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Del("Content-Length")

			// Wrap response writer
			gzw := &gzipResponseWriter{
				ResponseWriter: w,
				Writer:         gz,
			}

			next.ServeHTTP(gzw, r)
		})
	}
}

// RateLimit middleware limits requests per minute per IP address
func RateLimit(requestsPerMinute int) Middleware {
	type client struct {
		count      int
		lastAccess time.Time
	}

	var (
		clients = make(map[string]*client)
		mu      sync.Mutex
	)

	// Background worker to clean up stale IP records every minute
	go func() {
		for {
			time.Sleep(time.Minute)
			mu.Lock()
			for ip, c := range clients {
				if time.Since(c.lastAccess) > time.Minute {
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract client IP (stripping port)
			ip := r.RemoteAddr
			if idx := strings.LastIndex(ip, ":"); idx != -1 {
				ip = ip[:idx]
			}

			mu.Lock()
			c, exists := clients[ip]
			if !exists {
				c = &client{count: 0, lastAccess: time.Now()}
				clients[ip] = c
			}

			// Reset count if more than a minute has passed
			if time.Since(c.lastAccess) > time.Minute {
				c.count = 0
				c.lastAccess = time.Now()
			}

			c.count++
			c.lastAccess = time.Now()

			if c.count > requestsPerMinute {
				mu.Unlock()
				http.Error(w, "Rate limit exceeded. Please slow down.", http.StatusTooManyRequests)
				return
			}
			mu.Unlock()

			next.ServeHTTP(w, r)
		})
	}
}

// responseWriter captures the HTTP status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, fmt.Errorf("underlying ResponseWriter does not support http.Hijacker")
}

// gzipResponseWriter writes HTTP body data to gzip
type gzipResponseWriter struct {
	http.ResponseWriter
	Writer io.Writer
}

func (gzw *gzipResponseWriter) Write(data []byte) (int, error) {
	return gzw.Writer.Write(data)
}

// CreateMiddlewareChain creates the active forward proxy middleware pipeline
func CreateMiddlewareChain(handler http.Handler, cfg *config.Config) http.Handler {
	middlewares := []Middleware{
		Logger(), // 1. Always log client requests
	}

	// 2. Bandwidth optimization via Gzip compression
	middlewares = append(middlewares, Compress())

	// 3. Network abuse prevention via IP rate limiting
	if cfg.MaxConnections > 0 {
		requestsPerMinute := cfg.MaxConnections * 60
		middlewares = append(middlewares, RateLimit(requestsPerMinute))
	}

	return Chain(handler, middlewares...)
}

// RequestID adds a unique ID header to trace requests
func RequestID() Middleware {
	var requestID int64
	var mu sync.Mutex

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			requestID++
			id := requestID
			mu.Unlock()

			w.Header().Set("X-Request-ID", fmt.Sprintf("%d", id))
			ctx := context.WithValue(r.Context(), "requestID", id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}