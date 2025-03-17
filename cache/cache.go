package cache

import (
	"time"
)

// CacheItem represents an item stored in the cache
type CacheItem struct {
	Key       string
	Value     []byte
	Size      int
	CreatedAt time.Time
	ExpiresAt time.Time
}

// Cache defines the interface for our caching mechanism
type Cache interface {
	// Get retrieves an item from the cache by key
	// Returns the item and a boolean indicating if it was found
	Get(key string) (*CacheItem, bool)

	// Set adds or updates an item in the cache
	// Returns true if the item was added, false if it was updated
	Set(key string, value []byte, ttl time.Duration) bool

	// Remove deletes an item from the cache
	// Returns true if the item was found and removed
	Remove(key string) bool

	// Clear removes all items from the cache
	Clear()

	// Size returns the current number of items in the cache
	Size() int

	// Capacity returns the maximum number of items the cache can hold
	Capacity() int

	// Stats returns statistics about the cache usage
	Stats() CacheStats
}

// CacheStats contains statistics about cache usage
type CacheStats struct {
	Size      int     // Current number of items
	Capacity  int     // Maximum number of items
	Hits      int64   // Number of cache hits
	Misses    int64   // Number of cache misses
	HitRate   float64 // Hit rate (hits / (hits + misses))
	Evictions int64   // Number of items evicted
	AvgSize   int     // Average size of items in bytes
}