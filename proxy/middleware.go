package proxy

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Jovial-Kanwadia/proxy-server/config"
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

// Logger middleware logs HTTP requests
func Logger() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			
			// Create a response writer wrapper to capture status code
			rw := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}
			
			// Call the next handler
			next.ServeHTTP(rw, r)
			
			// Log the request details
			duration := time.Since(start)
			log.Printf(
				"%s %s %s %d %s %s",
				r.RemoteAddr,
				r.Method,
				r.URL.Path,
				rw.statusCode,
				duration,
				r.UserAgent(),
			)
		})
	}
}

// CORS middleware adds CORS headers to responses
func CORS() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Set CORS headers
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			
			// Handle preflight requests
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}
			
			// Call the next handler
			next.ServeHTTP(w, r)
		})
	}
}

// Compress middleware compresses responses using gzip
func Compress() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if the client accepts gzip encoding
			if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
				next.ServeHTTP(w, r)
				return
			}
			
			// Create a gzip writer
			gz, err := gzip.NewWriterLevel(w, gzip.BestSpeed)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			defer gz.Close()
			
			// Set the Content-Encoding header
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Del("Content-Length")
			
			// Create a gzip response writer
			gzw := &gzipResponseWriter{
				ResponseWriter: w,
				Writer:         gz,
			}
			
			// Call the next handler with the gzip writer
			next.ServeHTTP(gzw, r)
		})
	}
}

// RateLimit middleware limits the number of requests from a single IP address (for production)
func RateLimit(requestsPerMinute int) Middleware {
	type client struct {
		count      int
		lastAccess time.Time
	}
	
	var (
		clients = make(map[string]*client)
		mu      sync.Mutex
	)
	
	// Start a goroutine to clean up expired clients
	go func() {
		for {
			time.Sleep(time.Minute)
			mu.Lock()
			for ip, client := range clients {
				if time.Since(client.lastAccess) > time.Minute {
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()
	
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get the client IP address
			ip := r.RemoteAddr
			if idx := strings.LastIndex(ip, ":"); idx != -1 {
				ip = ip[:idx]
			}
			
			// Check if the client has exceeded the rate limit
			mu.Lock()
			c, exists := clients[ip]
			if !exists {
				c = &client{count: 0, lastAccess: time.Now()}
				clients[ip] = c
			}
			
			c.count++
			c.lastAccess = time.Now()
			
			if c.count > requestsPerMinute {
				mu.Unlock()
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			mu.Unlock()
			
			// Call the next handler
			next.ServeHTTP(w, r)
		})
	}
}


// responseWriter is a wrapper for http.ResponseWriter that captures the status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code and calls the underlying ResponseWriter's WriteHeader
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// gzipResponseWriter is a wrapper for http.ResponseWriter that writes to a gzip writer
type gzipResponseWriter struct {
	http.ResponseWriter
	Writer io.Writer
}

// Write writes the data to the gzip writer
func (gzw *gzipResponseWriter) Write(data []byte) (int, error) {
	return gzw.Writer.Write(data)
}

// CreateMiddlewareChain creates a chain of middleware based on the configuration
func CreateMiddlewareChain(handler http.Handler, cfg *config.Config) http.Handler {
	middlewares := []Middleware{
		Logger(), // Always include logger middleware
	}
	
	// Add compression middleware
	middlewares = append(middlewares, Compress())
	
	// Add CORS middleware
	middlewares = append(middlewares, CORS())
	
	// Add rate limiting middleware if max connections is configured
	if cfg.MaxConnections > 0 {
		// Calculate requests per minute based on MaxConnections
		// This is a simplistic approach - adjust as needed
		requestsPerMinute := cfg.MaxConnections * 60
		middlewares = append(middlewares, RateLimit(requestsPerMinute))
	}
	
	// Apply all middlewares to the handler
	return Chain(handler, middlewares...)
}

// SecurityHeaders adds security-related headers to responses
func SecurityHeaders() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add security headers
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			
			// Call the next handler
			next.ServeHTTP(w, r)
		})
	}
}

// RequestID adds a unique ID to each request for tracing
func RequestID() Middleware {
	var requestID int64 = 0
	var mu sync.Mutex
	
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Generate a unique request ID
			mu.Lock()
			requestID++
			id := requestID
			mu.Unlock()
			
			// Add the request ID as a header
			w.Header().Set("X-Request-ID", fmt.Sprintf("%d", id))
			
			// Store the request ID in the context
			ctx := context.WithValue(r.Context(), "requestID", id)
			r = r.WithContext(ctx)
			
			// Call the next handler
			next.ServeHTTP(w, r)
		})
	}
}

// RequestTimer measures and logs the time taken to process a request
func RequestTimer() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			
			// Call the next handler
			next.ServeHTTP(w, r)
			
			// Calculate and log the duration
			duration := time.Since(start)
			log.Printf("Request %s %s took %s", r.Method, r.URL.Path, duration)
		})
	}
}