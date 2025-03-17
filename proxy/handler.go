package proxy

import (
	"fmt"
	"io"
	"log"
	"net/http"
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

	return &ProxyHandler{
		cache:      cache,
		client:     client,
		config:     cfg,
		cacheables: cacheables,
	}
}

// ServeHTTP implements the http.Handler interface
func (p *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Log the request
	log.Printf("Proxying request: %s %s", r.Method, r.URL.String())

	// Check if the request URL is properly formed
	if r.URL.Scheme == "" || r.URL.Host == "" {
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
			response := item.Value
			
			// Write headers from cached response
			p.writeCachedHeaders(w, response)
			
			// Write body from cached response
			p.writeCachedBody(w, response)
			
			return
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
	return !strings.Contains(cacheControl, "no-store")

	// return true
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

// We'll implement these methods in the next steps
func (p *ProxyHandler) writeCachedHeaders(w http.ResponseWriter, response []byte) {
	// This will be implemented in the next step
	// For now, set a placeholder header
	w.Header().Set("X-Cache", "HIT")
}

func (p *ProxyHandler) writeCachedBody(w http.ResponseWriter, response []byte) {
	// This will be implemented in the next step
	// For now, write the response directly
	w.Write(response)
}

func (p *ProxyHandler) cacheResponse(key string, resp *http.Response, body []byte) {
	// This will be implemented in the next step
	// For now, just log that we would cache this
	log.Printf("Would cache response for %s (%d bytes)", key, len(body))
}