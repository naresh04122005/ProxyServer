package proxy

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Jovial-Kanwadia/proxy-server/cache"
	"github.com/Jovial-Kanwadia/proxy-server/config"
)

// ProxyHandler handles HTTP requests by forwarding them to the target server
type ProxyHandler struct {
	cache      cache.Cache
	client     *http.Client
	config     *config.Config
	cacheables map[string]bool // Map of cacheable HTTP methods
	workerPool *WorkerPool     // Worker pool for concurrent request handling
}

// NewProxyHandler creates a new ProxyHandler
func NewProxyHandler(cache cache.Cache, cfg *config.Config) *ProxyHandler {
	// Create HTTP client with timeouts
	client := &http.Client{
		Timeout: time.Duration(cfg.ProxyTimeout) * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Follow up to 10 redirects
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			return nil
		},
	}

	// Define cacheable HTTP methods
	cacheables := map[string]bool{
		http.MethodGet:  true,
		http.MethodHead: true,
	}

	// Create a new worker pool
	workerPool := NewWorkerPool(cfg.MaxConnections)

	return &ProxyHandler{
		cache:      cache,
		client:     client,
		config:     cfg,
		cacheables: cacheables,
		workerPool: workerPool,
	}
}

// ServeHTTP implements the http.Handler interface
func (p *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Create a handler for the request
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.handleRequest(w, r)
	})

	// Enqueue the request to be processed by a worker
	p.workerPool.Enqueue(w, r, handler)
}

// handleRequest processes a single HTTP request
func (p *ProxyHandler) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Check if the URL is provided as a query parameter
    targetURLStr := r.URL.Query().Get("url")
    
    if targetURLStr != "" {
        // Parse the target URL from the query parameter
        parsedURL, err := url.Parse(targetURLStr)
        if err != nil {
            http.Error(w, "Invalid URL format", http.StatusBadRequest)
            return
        }
        
        // Update the request URL
        r.URL = parsedURL
    } else if r.URL.Scheme == "" || r.URL.Host == "" {
        // This is likely a direct request to the proxy without the target URL
        http.Error(w, "Invalid proxy request. URL must include scheme and host.", http.StatusBadRequest)
        return
    }

	// Check if the domain is allowed
	if !p.isDomainAllowed(r.URL.Host) {
		http.Error(w, "Domain not allowed", http.StatusForbidden)
		return
	}

	// Check if we can use the cache for this request
	if p.isCacheable(r) {
		cacheKey := p.createCacheKey(r)
		
		// Try to get from cache
		if item, found := p.cache.Get(cacheKey); found {
			log.Printf("Cache hit for %s", cacheKey)
			
			// Parse the cached response
			cachedResp, err := p.parseCachedResponse(item.Value)
			if err != nil {
				log.Printf("Error parsing cached response: %v", err)
			} else {
				// Write headers from cached response
				for key, values := range cachedResp.Header {
					for _, value := range values {
						w.Header().Add(key, value)
					}
				}
				
				// Add cache header
				w.Header().Set("X-Cache", "HIT")
				
				// Set status code
				w.WriteHeader(cachedResp.StatusCode)
				
				// Write body
				if _, err := w.Write(cachedResp.Body); err != nil {
					log.Printf("Error writing cached response body: %v", err)
				}
				
				return
			}
		}
		
		log.Printf("Cache miss for %s", cacheKey)
	}

	// Clone the request for the target server
	proxyReq, err := p.cloneRequest(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error creating proxy request: %v", err), http.StatusInternalServerError)
		return
	}

	// Forward the request to the target server
	resp, err := p.client.Do(proxyReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error forwarding request: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy headers from target response to client response
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Add proxy headers
	w.Header().Set("X-Proxy-Server", "Go-Proxy-Server/1.0")
	w.Header().Set("X-Cache", "MISS")

	// Set status code
	w.WriteHeader(resp.StatusCode)

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response body: %v", err)
		return
	}

	// Check if we should cache this response
	if p.isCacheable(r) && p.isResponseCacheable(resp) {
		cacheKey := p.createCacheKey(r)
		
		// Store response in cache
		p.cacheResponse(cacheKey, resp, body)
	}

	// Write response body to client
	if _, err := w.Write(body); err != nil {
		log.Printf("Error writing response body: %v", err)
	}
}

// Shutdown gracefully shuts down the proxy handler
func (p *ProxyHandler) Shutdown() {
	if p.workerPool != nil {
		p.workerPool.Stop()
	}
}

// isDomainAllowed checks if the domain is allowed based on configuration
func (p *ProxyHandler) isDomainAllowed(host string) bool {
	// If no allowed domains are specified, all domains are allowed
	if len(p.config.AllowedDomains) == 0 {
		return true
	}

	// Check if the host is in the allowed domains list
	for _, domain := range p.config.AllowedDomains {
		if strings.HasSuffix(host, domain) {
			return true
		}
	}

	return false
}

// isCacheable checks if the request can be cached
func (p *ProxyHandler) isCacheable(r *http.Request) bool {
	// Check HTTP method
	if !p.cacheables[r.Method] {
		return false
	}

	// Don't cache if there's an Authorization header
	if r.Header.Get("Authorization") != "" {
		return false
	}

	// Don't cache if there's a Cache-Control: no-store header
	cacheControl := r.Header.Get("Cache-Control")
	if strings.Contains(cacheControl, "no-store") {
		return false
	}

	return true
}

// isResponseCacheable checks if the response can be cached
func (p *ProxyHandler) isResponseCacheable(resp *http.Response) bool {
	// Only cache successful responses
	if resp.StatusCode != http.StatusOK {
		return false
	}

	// Don't cache if there's a Cache-Control: no-store header
	cacheControl := resp.Header.Get("Cache-Control")
	if strings.Contains(cacheControl, "no-store") {
		return false
	}

	// Don't cache if there's a Set-Cookie header
	if resp.Header.Get("Set-Cookie") != "" {
		return false
	}

	return true
}

// createCacheKey creates a unique key for the request
func (p *ProxyHandler) createCacheKey(r *http.Request) string {
	// Simple key format: METHOD:URL
	return fmt.Sprintf("%s:%s", r.Method, r.URL.String())
}

// cloneRequest creates a new request for the target server
func (p *ProxyHandler) cloneRequest(r *http.Request) (*http.Request, error) {
	// Create a new URL from the request URL
	targetURL := *r.URL

	// Create a new request
	proxyReq, err := http.NewRequest(r.Method, targetURL.String(), r.Body)
	if err != nil {
		return nil, err
	}

	// Copy headers
	proxyReq.Header = make(http.Header)
	for key, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(key, value)
		}
	}

	// Update specific headers
	proxyReq.Header.Set("X-Forwarded-For", r.RemoteAddr)
	proxyReq.Header.Set("X-Forwarded-Host", r.Host)

	// Don't pass the Connection header
	proxyReq.Header.Del("Connection")

	return proxyReq, nil
}

// CachedResponse represents a cached HTTP response
type CachedResponse struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// cacheResponse stores a response in the cache
func (p *ProxyHandler) cacheResponse(key string, resp *http.Response, body []byte) {
	// Determine cache TTL from Cache-Control header
	ttl := p.calculateTTL(resp)
	if ttl <= 0 {
		// Use default TTL from config
		ttl = time.Duration(p.config.CacheTTL) * time.Second
	}

	// Serialize the response
	cachedResp := &CachedResponse{
		StatusCode: resp.StatusCode,
		Header:     resp.Header.Clone(),
		Body:       body,
	}

	serialized, err := p.serializeResponse(cachedResp)
	if err != nil {
		log.Printf("Error serializing response: %v", err)
		return
	}

	// Store in cache
	p.cache.Set(key, serialized, ttl)
	log.Printf("Cached response for %s (%d bytes) with TTL %v", key, len(serialized), ttl)
}

// calculateTTL calculates the TTL from Cache-Control header
func (p *ProxyHandler) calculateTTL(resp *http.Response) time.Duration {
    // Check for Cache-Control: max-age
    cacheControl := resp.Header.Get("Cache-Control")
    if cacheControl != "" {
        directives := strings.Split(cacheControl, ",")
        for _, directive := range directives {
            directive = strings.TrimSpace(directive)
            if strings.HasPrefix(directive, "max-age=") {
                value := strings.TrimPrefix(directive, "max-age=")
                if seconds, err := strconv.Atoi(value); err == nil {
                    return time.Duration(seconds) * time.Second
                }
            }
        }
    }

    // Check for Expires header
    if expires := resp.Header.Get("Expires"); expires != "" {
        // Try multiple time formats that might be used in HTTP headers
        formats := []string{
            time.RFC1123,
            time.RFC1123Z,
            "Mon, 02-Jan-2006 15:04:05 MST",
            "Monday, 02-Jan-2006 15:04:05 MST",
        }
        
        for _, format := range formats {
            if expiresTime, err := time.Parse(format, expires); err == nil {
                return time.Until(expiresTime)
            }
        }
    }

    // Return default TTL from config
    return time.Duration(p.config.CacheTTL) * time.Second
}
// serializeResponse serializes a CachedResponse to a byte array
func (p *ProxyHandler) serializeResponse(resp *CachedResponse) ([]byte, error) {
	// For simplicity, we'll use a simple format:
	// - First line: Status code
	// - Headers (one per line, key: value)
	// - Empty line
	// - Body
	var buf bytes.Buffer

	// Write status code
	fmt.Fprintf(&buf, "%d\r\n", resp.StatusCode)

	// Write headers
	for key, values := range resp.Header {
		for _, value := range values {
			fmt.Fprintf(&buf, "%s: %s\r\n", key, value)
		}
	}

	// Empty line to separate headers from body
	buf.WriteString("\r\n")

	// Write body
	buf.Write(resp.Body)

	return buf.Bytes(), nil
}

// parseCachedResponse deserializes a byte array to a CachedResponse
func (p *ProxyHandler) parseCachedResponse(data []byte) (*CachedResponse, error) {
	// Split data into headers and body
	parts := bytes.SplitN(data, []byte("\r\n\r\n"), 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid cached response format")
	}

	// Parse headers
	headerLines := bytes.Split(parts[0], []byte("\r\n"))
	if len(headerLines) < 1 {
		return nil, fmt.Errorf("invalid cached response headers")
	}

	// Parse status code
	statusCode := 0
	if _, err := fmt.Sscanf(string(headerLines[0]), "%d", &statusCode); err != nil {
		return nil, fmt.Errorf("invalid status code: %v", err)
	}

	// Parse headers
	headers := make(http.Header)
	for _, line := range headerLines[1:] {
		headerParts := bytes.SplitN(line, []byte(": "), 2)
		if len(headerParts) == 2 {
			key := string(headerParts[0])
			value := string(headerParts[1])
			headers.Add(key, value)
		}
	}

	// Create response
	resp := &CachedResponse{
		StatusCode: statusCode,
		Header:     headers,
		Body:       parts[1],
	}

	return resp, nil
}