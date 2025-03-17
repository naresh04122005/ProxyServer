# 🛡️ Go Multithreaded Proxy Server

Welcome to the Go Proxy Server project! This multithreaded proxy server is built with high performance and flexibility in mind. It not only forwards HTTP requests and responses but also enhances them with powerful features such as caching, domain filtering, rate limiting, and much more.

## 📖 Project Overview
The Go Proxy Server is designed to serve as an intermediary between clients and target web servers. Its robust architecture supports:

- Request Forwarding: Transparently passes client requests to destination servers and returns responses.
- Response Caching: Stores frequently requested data to boost performance.
- Domain Filtering: Allows administrators to restrict access only to approved domains.
- Method Handling: Optimizes caching for idempotent methods (GET) while forwarding non-cacheable methods (POST, PUT) without caching.
- CORS & Compression: Adds necessary CORS headers and supports gzip compression to reduce bandwidth.
- Security Enhancements: Includes standard security headers to safeguard client interactions.
- Rate Limiting & Worker Pools: Manages incoming connections efficiently to prevent overload.
- Custom Configuration: Easily tailor server behavior via a JSON configuration file.
- Advanced Chaining: Supports chaining multiple proxy instances for complex network architectures.
- Robust Error Handling: Gracefully handles invalid requests and network issues.

This project is ideal for developers looking for a customizable proxy solution with enhanced performance and security.

## 🚀 Getting Started

Clone the repo to your local machine:
```bash
git clone https://github.com/Jovial-Kanwadia/ProxyServer.git
cd go-proxy-server
```

Build the project using Go:
```bash
# Navigate to your project directory
cd /path/to/your/proxy-server

# Build the project
go build -o proxy-server

# Run the server with default configuration (listens on localhost:8080)
./proxy-server
```
Server is now running and accessible at http://localhost:8080 🎉

## ⚙️ Features and Testing

### 1. Basic Proxy Functionality

The core functionality of the proxy server is to forward HTTP requests to target URLs and return the responses.

**Command:**
```bash
curl -v "http://localhost:8080/?url=http://example.com"
```

**Output:**
```
* Connected to localhost (127.0.0.1) port 8080
> GET /?url=http://example.com HTTP/1.1
< HTTP/1.1 200 OK
< X-Proxy-Server: Go-Proxy-Server/1.0
< X-Cache: MISS
... (example.com content) ...
```

**Explanation:**
This request demonstrates the basic forwarding capability of the proxy. The server receives a request for `example.com`, retrieves the content, and returns it to the client while adding custom headers to indicate it was processed by the proxy server.

### 2. Response Caching

The proxy server caches responses to improve performance for repeated requests.

**Commands:**
```bash
# First request (should be a cache miss)
curl -v "http://localhost:8080/?url=http://example.com"

# Second request (should be a cache hit)
curl -v "http://localhost:8080/?url=http://example.com"
```

**Output for Second Request:**
```
< HTTP/1.1 200 OK
< X-Cache: HIT
... (example.com content) ...
```

**Explanation:**
The first request is forwarded to the target server and the response is cached. The second request for the same URL retrieves the cached response rather than making another request to the target server, indicated by the `X-Cache: HIT` header.

### 3. Domain Filtering

The proxy server can be configured to restrict access to specific domains.

**Setup:**
```bash
# Start the server with allowed domains
./proxy-server --allowed-domains="example.com,google.com"
```

**Test Commands:**
```bash
# Test with allowed domain
curl -v "http://localhost:8080/?url=http://example.com"

# Test with disallowed domain
curl -v "http://localhost:8080/?url=http://reddit.com"
```

**Output for Disallowed Domain:**
```
< HTTP/1.1 403 Forbidden
< Content-Type: text/plain; charset=utf-8
Domain not allowed
```

**Explanation:**
The domain filtering feature allows administrators to restrict which websites can be accessed through the proxy. Requests to domains not in the allowlist are rejected with a 403 Forbidden response.

### 4. HTTP Method Handling

The proxy handles different HTTP methods appropriately, with special handling for cacheable vs. non-cacheable methods.

**Commands:**
```bash
# GET request (cacheable)
curl -v "http://localhost:8080/?url=http://httpbin.org/get"

# POST request (non-cacheable)
curl -v -X POST "http://localhost:8080/?url=http://httpbin.org/post"

# PUT request (non-cacheable)
curl -v -X PUT "http://localhost:8080/?url=http://httpbin.org/put"
```

**Behavior:**
- GET requests are cached and will show `X-Cache: HIT` on subsequent requests
- POST and PUT requests are not cached and will always show `X-Cache: MISS`

**Explanation:**
The proxy server intelligently handles different HTTP methods, caching GET requests (which are typically idempotent) while always forwarding POST and PUT requests (which may modify server state).

### 5. CORS Headers

The proxy server supports Cross-Origin Resource Sharing (CORS) to allow cross-domain requests.

**Command:**
```bash
# Test with OPTIONS request
curl -v -X OPTIONS "http://localhost:8080/?url=http://example.com"
```

**Output:**
```
< HTTP/1.1 200 OK
< Access-Control-Allow-Origin: *
< Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS
< Access-Control-Allow-Headers: Content-Type, Authorization
```

**Explanation:**
The proxy server adds CORS headers to responses, allowing web applications to make cross-origin requests. The OPTIONS request demonstrates the server's handling of CORS preflight requests.

### 6. Content Compression

The proxy server supports content compression to reduce bandwidth usage.

**Command:**
```bash
# Request with gzip encoding
curl -v -H "Accept-Encoding: gzip" "http://localhost:8080/?url=http://example.com"
```

**Output:**
```
< HTTP/1.1 200 OK
< Content-Encoding: gzip
```

**Explanation:**
When a client indicates support for compressed content, the proxy server compresses the response before sending it. This reduces the amount of data transferred over the network.

### 7. Custom Configuration

The proxy server supports customization through a configuration file.

**Setup:**
```bash
# Create a config file
cat > config.json << EOF
{
  "port": 8081,
  "host": "localhost",
  "cache_size": 2048,
  "cache_ttl": 7200,
  "proxy_timeout": 60,
  "allowed_domains": ["example.com", "httpbin.org"],
  "max_connections": 200
}
EOF

# Run with custom config
./proxy-server --config=config.json
```

**Test Command:**
```bash
curl -v "http://localhost:8081/?url=http://example.com"
```

**Output:**
```
* Connected to localhost (127.0.0.1) port 8081
> GET /?url=http://example.com HTTP/1.1
```

**Explanation:**
The configuration file allows administrators to customize various aspects of the proxy server's behavior without modifying the code. This example demonstrates changing the port from the default 8080 to 8081.

### 8. Security Headers

The proxy server adds security headers to responses to enhance client security.

**Command:**
```bash
curl -v "http://localhost:8080/?url=http://example.com" | grep -i "X-"
```

**Output:**
```
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
X-XSS-Protection: 1; mode=block
X-Proxy-Server: Go-Proxy-Server/1.0
X-Cache: MISS
```

**Explanation:**
The proxy server adds standard security headers to responses to help protect clients from common web vulnerabilities such as clickjacking and cross-site scripting.

### 9. Rate Limiting

The proxy server implements rate limiting to prevent abuse.

**Setup:**
```bash
# Run with a low rate limit
./proxy-server --max-connections=1
```

**Test Command:**
```bash
# Then run a loop to make multiple requests
for i in {1..20}; do
  curl -v "http://localhost:8080/?url=http://example.com"
  sleep 0.1
done
```

**Output:**
After several requests, you should see:
```
< HTTP/1.1 429 Too Many Requests
Rate limit exceeded
```

**Explanation:**
The rate limiting feature protects the proxy server from being overwhelmed by too many concurrent requests. When the limit is exceeded, the server returns a 429 status code.

### 10. Cache TTL (Time-to-Live)

The proxy server's cache has a configurable TTL to control how long responses are cached.

**Setup:**
```bash
# Start server with short TTL
./proxy-server --cache-ttl=5
```

**Test Commands:**
```bash
# Make initial request
curl -v "http://localhost:8080/?url=http://example.com"

# Immediately make another request (should be cache hit)
curl -v "http://localhost:8080/?url=http://example.com"

# Wait 6 seconds and make another request (should be cache miss)
sleep 6
curl -v "http://localhost:8080/?url=http://example.com"
```

**Behavior:**
- Second request: `X-Cache: HIT`
- Third request: `X-Cache: MISS`

**Explanation:**
The cache TTL controls how long responses stay in the cache before being evicted. In this example, responses are cached for 5 seconds, after which they are considered stale and will be fetched fresh from the target server.

### 11. Worker Pool

The proxy server uses a worker pool to handle concurrent requests efficiently.

**Setup:**
```bash
# Start server with limited connections
./proxy-server --max-connections=5
```

**Test Command:**
```bash
# Use Apache Bench to test concurrency
ab -n 100 -c 20 "http://localhost:8080/?url=http://example.com"
```

**Output:**
The test should complete with all requests processed, though some may be queued due to the worker pool limit.

**Explanation:**
The worker pool manages a limited number of concurrent requests, queuing additional requests until a worker becomes available. This prevents the server from being overwhelmed by a large number of simultaneous connections.

### 12. Error Handling

The proxy server implements robust error handling for various scenarios.

**Test Commands:**
```bash
# Invalid URL
curl -v "http://localhost:8080/?url=invalid-url"

# Timeout (with a short timeout)
./proxy-server --proxy-timeout=1
curl -v "http://localhost:8080/?url=http://slowwly.robertomurray.co.uk/delay/3000/url/http://www.google.com"
```

**Output:**
- Invalid URL: HTTP 400 Bad Request
- Timeout: HTTP 502 Bad Gateway

**Explanation:**
The proxy server handles various error conditions gracefully, returning appropriate HTTP status codes and error messages. This enhances reliability and provides better feedback to clients.

### 13. Advanced Scenario: Proxy Chaining

The proxy server can be used in a chained configuration for more complex setups.

**Setup:**
```bash
# Start two proxy instances
./proxy-server --port=8080 &
./proxy-server --port=8081 &
```

**Test Command:**
```bash
# Use one proxy to access the other
curl -v "http://localhost:8081/?url=http://localhost:8080/?url=http://example.com"
```

**Behavior:**
The request should be forwarded through both proxies and return the example.com content.

**Explanation:**
This advanced scenario demonstrates the proxy server's ability to work in a chained configuration, where one proxy forwards requests to another proxy. This can be useful for creating more complex network topologies or implementing additional layers of security.

## Monitoring and Logging

To monitor the proxy server's behavior during testing, you can capture its logs:

```bash
# Watch the logs while running tests
./proxy-server 2>&1 | tee proxy.log
```

This will display the logs in the terminal and save them to a file for later analysis.

## 🤝 Contributing
Contributions are welcome! Please open an issue or submit a pull request if you have ideas for improvements or bug fixes.