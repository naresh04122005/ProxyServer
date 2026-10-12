package proxy

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/naresh04122005/ProxyServer/cache"
	"github.com/naresh04122005/ProxyServer/config"
)

// ProxyHandler handles both HTTP forwarding and HTTPS CONNECT tunneling
type ProxyHandler struct {
	cache      cache.Cache
	client     *http.Client
	config     *config.Config
	cacheables map[string]bool
	workerPool *WorkerPool
}

// NewProxyHandler creates a new ProxyHandler
func NewProxyHandler(cache cache.Cache, cfg *config.Config) *ProxyHandler {
	client := &http.Client{
		Timeout: time.Duration(cfg.ProxyTimeout) * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			return nil
		},
	}

	cacheables := map[string]bool{
		http.MethodGet:  true,
		http.MethodHead: true,
	}

	workerPool := NewWorkerPool(cfg.MaxConnections)

	return &ProxyHandler{
		cache:      cache,
		client:     client,
		config:     cfg,
		cacheables: cacheables,
		workerPool: workerPool,
	}
}

// ServeHTTP implements http.Handler and routes requests through the worker pool
func (p *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.handleRequest(w, r)
	})

	p.workerPool.Enqueue(w, r, handler)
}

// handleRequest processes incoming HTTP or HTTPS requests
func (p *ProxyHandler) handleRequest(w http.ResponseWriter, r *http.Request) {
	// 1. Handle HTTPS CONNECT tunneling
	if r.Method == http.MethodConnect {
		p.handleConnect(w, r)
		return
	}

	// 2. Resolve Target URL (supports both standard forward proxy and ?url= parameter)
	targetURLStr := r.URL.Query().Get("url")
	if targetURLStr != "" {
		parsedURL, err := url.Parse(targetURLStr)
		if err != nil {
			http.Error(w, "Invalid URL format in query parameter", http.StatusBadRequest)
			return
		}
		r.URL = parsedURL
	} else {
		// Standard forward proxy request
		if r.URL.Scheme == "" {
			r.URL.Scheme = "http"
		}
		if r.URL.Host == "" && r.Host != "" {
			r.URL.Host = r.Host
		}
	}

	if r.URL.Host == "" {
		http.Error(w, "Invalid proxy request: missing destination host", http.StatusBadRequest)
		return
	}

	// 3. Domain policy check (Allowlist / Blocklist)
	if !p.isDomainAllowed(r.URL.Host) {
		http.Error(w, "Access Denied: Domain blocked by network policy", http.StatusForbidden)
		return
	}

	// 4. Cache Lookup (for GET / HEAD requests)
	if p.isCacheable(r) {
		cacheKey := p.createCacheKey(r)
		if item, found := p.cache.Get(cacheKey); found {
			log.Printf("[CACHE HIT] %s", cacheKey)

			cachedResp, err := p.parseCachedResponse(item.Value)
			if err == nil {
				for key, values := range cachedResp.Header {
					for _, value := range values {
						w.Header().Add(key, value)
					}
				}
				w.Header().Set("X-Proxy-Server", "Go-Proxy-Server/1.0")
				w.Header().Set("X-Cache", "HIT")
				w.WriteHeader(cachedResp.StatusCode)
				w.Write(cachedResp.Body)
				return
			}
		}
		log.Printf("[CACHE MISS] %s", cacheKey)
	}

	// 5. Clone request and forward to target origin server
	proxyReq, err := p.cloneRequest(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error creating proxy request: %v", err), http.StatusInternalServerError)
		return
	}

	resp, err := p.client.Do(proxyReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error forwarding request: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// 6. Copy origin response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.Header().Set("X-Proxy-Server", "Go-Proxy-Server/1.0")
	w.Header().Set("X-Cache", "MISS")
	w.WriteHeader(resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response body: %v", err)
		return
	}

	// 7. Store in cache if eligible
	if p.isCacheable(r) && p.isResponseCacheable(resp) {
		cacheKey := p.createCacheKey(r)
		p.cacheResponse(cacheKey, resp, body)
	}

	if _, err := w.Write(body); err != nil {
		log.Printf("Error writing response body: %v", err)
	}
}

// handleConnect implements RFC 2817 HTTP CONNECT tunneling for HTTPS
func (p *ProxyHandler) handleConnect(w http.ResponseWriter, r *http.Request) {
	host := r.URL.Host
	if host == "" {
		host = r.Host
	}

	// Domain policy check
	if !p.isDomainAllowed(host) {
		http.Error(w, "Access Denied: Domain blocked by network policy", http.StatusForbidden)
		return
	}

	// Dial the remote destination server
	destConn, err := net.DialTimeout("tcp", host, time.Duration(p.config.ProxyTimeout)*time.Second)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to connect to %s: %v", host, err), http.StatusBadGateway)
		return
	}
	defer destConn.Close()

	// Hijack the client connection to take over raw TCP
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "HTTP Hijacking not supported", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to hijack connection: %v", err), http.StatusServiceUnavailable)
		return
	}
	defer clientConn.Close()

	// Inform client that tunnel is established
	_, err = clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	if err != nil {
		return
	}

	// Bidirectional raw TCP copy between client and destination
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		io.Copy(destConn, clientConn)
	}()

	go func() {
		defer wg.Done()
		io.Copy(clientConn, destConn)
	}()

	wg.Wait()
}

// Shutdown gracefully stops the worker pool
func (p *ProxyHandler) Shutdown() {
	if p.workerPool != nil {
		p.workerPool.Stop()
	}
}

// isDomainAllowed checks if a host is allowed based on blocklists and allowlists
func (p *ProxyHandler) isDomainAllowed(host string) bool {
	// Strip port if present (e.g. "google.com:443" -> "google.com")
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.ToLower(strings.TrimSpace(host))

	// 1. Check blocked domains first
	for _, blocked := range p.config.BlockedDomains {
		blocked = strings.ToLower(strings.TrimSpace(blocked))
		if host == blocked || strings.HasSuffix(host, "."+blocked) {
			return false
		}
	}

	// 2. If allowlist is empty, allow everything not explicitly blocked
	if len(p.config.AllowedDomains) == 0 {
		return true
	}

	// 3. Check allowed domains
	for _, allowed := range p.config.AllowedDomains {
		allowed = strings.ToLower(strings.TrimSpace(allowed))
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return true
		}
	}

	return false
}

// isCacheable verifies if an HTTP request is eligible for caching
func (p *ProxyHandler) isCacheable(r *http.Request) bool {
	if !p.cacheables[r.Method] {
		return false
	}
	if r.Header.Get("Authorization") != "" {
		return false
	}
	cacheControl := r.Header.Get("Cache-Control")
	return !strings.Contains(cacheControl, "no-store")
}

// isResponseCacheable verifies if the upstream response can be stored
func (p *ProxyHandler) isResponseCacheable(resp *http.Response) bool {
	if resp.StatusCode != http.StatusOK {
		return false
	}
	cacheControl := resp.Header.Get("Cache-Control")
	if strings.Contains(cacheControl, "no-store") || strings.Contains(cacheControl, "private") {
		return false
	}
	if resp.Header.Get("Set-Cookie") != "" {
		return false
	}
	return true
}

func (p *ProxyHandler) createCacheKey(r *http.Request) string {
	return fmt.Sprintf("%s:%s", r.Method, r.URL.String())
}

func (p *ProxyHandler) cloneRequest(r *http.Request) (*http.Request, error) {
	targetURL := *r.URL
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

	// Add proxy metadata headers
	proxyReq.Header.Set("X-Forwarded-For", r.RemoteAddr)
	proxyReq.Header.Set("X-Forwarded-Host", r.Host)

	// Remove hop-by-hop headers
	proxyReq.Header.Del("Connection")
	proxyReq.Header.Del("Keep-Alive")
	proxyReq.Header.Del("Proxy-Authenticate")
	proxyReq.Header.Del("Proxy-Authorization")

	return proxyReq, nil
}

type CachedResponse struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

func (p *ProxyHandler) cacheResponse(key string, resp *http.Response, body []byte) {
	ttl := p.calculateTTL(resp)
	if ttl <= 0 {
		ttl = time.Duration(p.config.CacheTTL) * time.Second
	}

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

	p.cache.Set(key, serialized, ttl)
	log.Printf("Cached %s (%d bytes, TTL: %v)", key, len(serialized), ttl)
}

func (p *ProxyHandler) calculateTTL(resp *http.Response) time.Duration {
	cacheControl := resp.Header.Get("Cache-Control")
	if cacheControl != "" {
		for _, directive := range strings.Split(cacheControl, ",") {
			directive = strings.TrimSpace(directive)
			if strings.HasPrefix(directive, "max-age=") {
				value := strings.TrimPrefix(directive, "max-age=")
				if seconds, err := strconv.Atoi(value); err == nil {
					return time.Duration(seconds) * time.Second
				}
			}
		}
	}

	if expires := resp.Header.Get("Expires"); expires != "" {
		formats := []string{
			time.RFC1123,
			time.RFC1123Z,
			"Mon, 02-Jan-2006 15:04:05 MST",
		}
		for _, format := range formats {
			if expiresTime, err := time.Parse(format, expires); err == nil {
				return time.Until(expiresTime)
			}
		}
	}

	return time.Duration(p.config.CacheTTL) * time.Second
}

func (p *ProxyHandler) serializeResponse(resp *CachedResponse) ([]byte, error) {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%d\r\n", resp.StatusCode)
	for key, values := range resp.Header {
		for _, value := range values {
			fmt.Fprintf(&buf, "%s: %s\r\n", key, value)
		}
	}
	buf.WriteString("\r\n")
	buf.Write(resp.Body)
	return buf.Bytes(), nil
}

func (p *ProxyHandler) parseCachedResponse(data []byte) (*CachedResponse, error) {
	parts := bytes.SplitN(data, []byte("\r\n\r\n"), 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid cached response format")
	}

	headerLines := bytes.Split(parts[0], []byte("\r\n"))
	if len(headerLines) < 1 {
		return nil, fmt.Errorf("invalid cached response headers")
	}

	statusCode := 0
	if _, err := fmt.Sscanf(string(headerLines[0]), "%d", &statusCode); err != nil {
		return nil, fmt.Errorf("invalid status code: %v", err)
	}

	headers := make(http.Header)
	for _, line := range headerLines[1:] {
		headerParts := bytes.SplitN(line, []byte(": "), 2)
		if len(headerParts) == 2 {
			headers.Add(string(headerParts[0]), string(headerParts[1]))
		}
	}

	return &CachedResponse{
		StatusCode: statusCode,
		Header:     headers,
		Body:       parts[1],
	}, nil
}