# 🛡️ High-Concurrency Forward Proxy Server in Go

A lightweight, high-performance **Forward Proxy Server** built with Go, designed for enterprise and campus networks. It sits at the network gateway between client devices and the internet to enforce access control policies, optimize bandwidth through an in-memory thread-safe LRU cache, and protect upstream network resources through backpressure worker pools and rate limiting.

---

## 🌟 Key Features

* **Standard HTTP & HTTPS Proxying**: Supports standard forward proxying and full RFC 2817 **HTTPS `CONNECT` TCP tunneling** (bidirectional `io.Copy`).
* **Access Control & Domain Filtering**: Granular domain enforcement using configurable **Allowlists** (exam/kiosk mode) and **Blocklists** (blocking unauthorized or malicious domains).
* **Thread-Safe In-Memory LRU Cache**: Implements an O(1) LRU cache using a Hash Map + Doubly Linked List (`container/list`), protected by mutexes with passive **TTL expiration**.
* **Worker Pool & Backpressure**: Buffers incoming client connections through a bounded worker pool to prevent socket exhaustion and throttling from upstream servers.
* **IP-Based Rate Limiting**: Windowed rate limiting to prevent client abuse and DDoS attacks, with automatic memory cleanup routines.
* **Bandwidth Optimization**: Automatic Gzip compression for clients accepting compressed payloads.
* **Production Reliability**: Configured with strict `http.Server` timeouts (Read/Write/Idle/MaxHeaderBytes) to defend against Slowloris attacks, and handles **graceful shutdown** using OS interrupt signals (`SIGINT`/`SIGTERM`).

---

## 🏗️ Architecture Overview

```text
                      +-------------------------------------------------+
                      |              FORWARD PROXY SERVER               |
                      |                                                 |
[ Client / Student ]  |  [ Rate Limiter ] -> [ Domain Policy Engine ]   |
         |            |                                |                |
         |            |       +------------------------+                |
         |            |       |                                         |
         |            |   (HTTP Request)                (HTTPS Tunnel)  |
         |            |       |                                |        |
         |            |  [ LRU Cache Check ]              [ TCP Hijack] |
         |            |    |-- Hit  -> Return 200              |        |
         |            |    \-- Miss -> Worker Pool             |        |
         v            |                     |                  v        |
 (Forward Request) -> |                     +------------------+------> | -> [ Internet / Target ]
                      +-------------------------------------------------+
```

---

## 🚀 Getting Started

### 1. Prerequisites
* Go 1.22+ or newer

### 2. Installation
```bash
# Clone the repository
git clone https://github.com/naresh04122005/ProxyServer.git
cd ProxyServer

# Install and tidy dependencies
go mod tidy
```

### 3. Run Tests & Benchmarks
```bash
# Run all unit, concurrency, and stress tests
go test ./... -v

# Run cache benchmarks
go test -bench=. ./tests
```

### 4. Running the Server
```bash
# Start with default configuration (loads config.json automatically)
go run main.go

# Or specify custom CLI flags
go run main.go --port=8080 --cache-size=2048 --blocked-domains="torrent.com,gambling.com"
```

---

## 🧪 Testing & Verification

Configure your tool or browser to use the proxy (`localhost:8080`):

### 1. HTTPS Tunneling (`CONNECT`)
```bash
curl -v -x http://localhost:8080 https://httpbin.org/get
```
*Tunnels encrypted TLS traffic directly to destination over raw TCP sockets.*

### 2. HTTP Caching
```bash
# 1st request (Cache Miss)
curl -v -x http://localhost:8080 http://example.com

# 2nd request (Cache Hit - Served instantly from RAM!)
curl -v -x http://localhost:8080 http://example.com
```

### 3. Domain Blocking (Firewall Policy)
```bash
curl -v -x http://localhost:8080 http://torrent.com
```
*Returns `403 Forbidden: Access Denied: Domain blocked by network policy`.*

---

## ⚙️ Configuration (`config.json`)

```json
{
  "port": 8080,
  "host": "localhost",
  "cache_size": 2048,
  "cache_ttl": 3600,
  "proxy_timeout": 30,
  "allowed_domains": [],
  "blocked_domains": [
    "torrent.com",
    "gambling.com",
    "malicious-site.com"
  ],
  "max_connections": 100,
  "log_level": "info"
}
```

---

## 🤝 License
MIT