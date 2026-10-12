package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

// Config holds all configuration settings for the proxy server
type Config struct {
	// Server settings
	Port           int      `json:"port"`
	Host           string   `json:"host"`
	ReadTimeout    int      `json:"read_timeout"`    // In seconds
	WriteTimeout   int      `json:"write_timeout"`   // In seconds
	IdleTimeout    int      `json:"idle_timeout"`    // In seconds
	MaxHeaderBytes int      `json:"max_header_bytes"`

	// Cache settings
	CacheSize      int      `json:"cache_size"`      // Number of items in LRU cache
	CacheTTL       int      `json:"cache_ttl"`       // Default TTL in seconds

	// Proxy settings
	ProxyTimeout   int      `json:"proxy_timeout"`   // In seconds
	AllowedDomains []string `json:"allowed_domains"` // If specified, ONLY these domains are accessible
	BlockedDomains []string `json:"blocked_domains"` // These domains are forbidden
	MaxConnections int      `json:"max_connections"` // Maximum concurrent worker connections

	// Logging settings
	LogLevel       string   `json:"log_level"`
	LogFile        string   `json:"log_file"`
}

// NewDefaultConfig returns a new Config with production defaults
func NewDefaultConfig() *Config {
	return &Config{
		Port:           8080,
		Host:           "localhost",
		ReadTimeout:    30,
		WriteTimeout:   30,
		IdleTimeout:    60,
		MaxHeaderBytes: 1 << 20, // 1MB

		CacheSize:      1024,
		CacheTTL:       3600, // 1 hour

		ProxyTimeout:   30,
		AllowedDomains: []string{},
		BlockedDomains: []string{},
		MaxConnections: 100,

		LogLevel:       "info",
		LogFile:        "",
	}
}

// LoadFromFile loads configuration from a JSON file
func LoadFromFile(filename string) (*Config, error) {
	config := NewDefaultConfig()

	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("error opening config file: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(config); err != nil {
		return nil, fmt.Errorf("error decoding config file: %w", err)
	}

	return config, nil
}

// SaveToFile saves the configuration to a formatted JSON file
func (c *Config) SaveToFile(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("error creating config file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(c)
}

// ParseFlags parses command line flags and overrides settings
func (c *Config) ParseFlags() {
	var configFile string
	var allowedDomainsFlag, blockedDomainsFlag string

	flag.IntVar(&c.Port, "port", c.Port, "Port to listen on")
	flag.StringVar(&c.Host, "host", c.Host, "Host to listen on")
	flag.IntVar(&c.ReadTimeout, "read-timeout", c.ReadTimeout, "Read timeout in seconds")
	flag.IntVar(&c.WriteTimeout, "write-timeout", c.WriteTimeout, "Write timeout in seconds")
	flag.IntVar(&c.CacheSize, "cache-size", c.CacheSize, "LRU cache size (number of items)")
	flag.IntVar(&c.CacheTTL, "cache-ttl", c.CacheTTL, "Cache TTL in seconds")
	flag.IntVar(&c.ProxyTimeout, "proxy-timeout", c.ProxyTimeout, "Proxy timeout in seconds")
	flag.IntVar(&c.MaxConnections, "max-connections", c.MaxConnections, "Maximum concurrent worker connections")
	flag.StringVar(&configFile, "config", "", "Path to configuration file")
	flag.StringVar(&allowedDomainsFlag, "allowed-domains", "", "Comma-separated list of allowed domains")
	flag.StringVar(&blockedDomainsFlag, "blocked-domains", "", "Comma-separated list of blocked domains")

	flag.Parse()

	if configFile == "" {
		if _, err := os.Stat("config.json"); err == nil {
			configFile = "config.json"
		}
	}

	// If a config file was specified, load it first
	if configFile != "" {
		if fileConfig, err := LoadFromFile(configFile); err == nil {
			*c = *fileConfig
		}
	}

	// CLI domain flags take precedence if supplied
	if allowedDomainsFlag != "" {
		c.AllowedDomains = splitAndTrim(allowedDomainsFlag)
	}
	if blockedDomainsFlag != "" {
		c.BlockedDomains = splitAndTrim(blockedDomainsFlag)
	}
}

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// Validate checks if the configuration values are sound
func (c *Config) Validate() error {
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("invalid port number: %d", c.Port)
	}
	if c.ReadTimeout <= 0 {
		return fmt.Errorf("invalid read timeout: %d", c.ReadTimeout)
	}
	if c.WriteTimeout <= 0 {
		return fmt.Errorf("invalid write timeout: %d", c.WriteTimeout)
	}
	if c.CacheSize <= 0 {
		return fmt.Errorf("invalid cache size: %d", c.CacheSize)
	}
	if c.CacheTTL <= 0 {
		return fmt.Errorf("invalid cache TTL: %d", c.CacheTTL)
	}
	if c.ProxyTimeout <= 0 {
		return fmt.Errorf("invalid proxy timeout: %d", c.ProxyTimeout)
	}
	if c.MaxConnections <= 0 {
		return fmt.Errorf("invalid max connections: %d", c.MaxConnections)
	}
	return nil
}

// String returns a pretty string representation of the config
func (c *Config) String() string {
	return fmt.Sprintf(`Configuration:
  Server:
    Host: %s
    Port: %d
    ReadTimeout: %d seconds
    WriteTimeout: %d seconds
    IdleTimeout: %d seconds
    MaxHeaderBytes: %d bytes

  Cache:
    Size: %d items
    TTL: %d seconds

  Proxy:
    Timeout: %d seconds
    AllowedDomains: %v
    BlockedDomains: %v
    MaxConnections: %d

  Logging:
    Level: %s
`,
		c.Host, c.Port, c.ReadTimeout, c.WriteTimeout, c.IdleTimeout, c.MaxHeaderBytes,
		c.CacheSize, c.CacheTTL,
		c.ProxyTimeout, c.AllowedDomains, c.BlockedDomains, c.MaxConnections,
		c.LogLevel)
}