package cache

import (
	"container/list"
	"sync"
	"time"
)

// LRUCache is a thread-safe LRU cache implementation
type LRUCache struct {
	capacity    int
	evictions   int64
	hits        int64
	misses      int64
	totalSize   int
	items       map[string]*list.Element
	evictionList *list.List
	mutex       sync.RWMutex
}

// NewLRUCache creates a new LRU cache with the given capacity
func NewLRUCache(capacity int) *LRUCache {
	return &LRUCache{
		capacity:    capacity,
		items:       make(map[string]*list.Element),
		evictionList: list.New(),
	}
}

// Get retrieves an item from the cache
func (c *LRUCache) Get(key string) (*CacheItem, bool) {
	c.mutex.RLock()
	element, exists := c.items[key]
	c.mutex.RUnlock()

	if !exists {
		c.mutex.Lock()
		c.misses++
		c.mutex.Unlock()
		return nil, false
	}

	item := element.Value.(*CacheItem)

	// Check if the item has expired
	if !item.ExpiresAt.IsZero() && time.Now().After(item.ExpiresAt) {
		c.mutex.Lock()
		c.evictElement(element)
		c.misses++
		c.mutex.Unlock()
		return nil, false
	}

	// Move to front (most recently used)
	c.mutex.Lock()
	c.evictionList.MoveToFront(element)
	c.hits++
	c.mutex.Unlock()

	return item, true
}

// Set adds or updates an item in the cache
func (c *LRUCache) Set(key string, value []byte, ttl time.Duration) bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Calculate expiration time
	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	// Create cache item
	item := &CacheItem{
		Key:       key,
		Value:     value,
		Size:      len(value),
		CreatedAt: time.Now(),
		ExpiresAt: expiresAt,
	}

	// Check if the key already exists
	if element, exists := c.items[key]; exists {
		// Update existing item
		oldItem := element.Value.(*CacheItem)
		c.totalSize = c.totalSize - oldItem.Size + item.Size
		element.Value = item
		c.evictionList.MoveToFront(element)
		return false
	}

	// Add new item
	element := c.evictionList.PushFront(item)
	c.items[key] = element
	c.totalSize += item.Size

	// Evict items if we're over capacity
	for c.evictionList.Len() > c.capacity {
		c.evictOldest()
	}

	return true
}

// Remove deletes an item from the cache
func (c *LRUCache) Remove(key string) bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if element, exists := c.items[key]; exists {
		return c.evictElement(element)
	}
	return false
}

// Clear removes all items from the cache
func (c *LRUCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.items = make(map[string]*list.Element)
	c.evictionList = list.New()
	c.totalSize = 0
	// Don't reset statistics
}

// Size returns the current number of items in the cache
func (c *LRUCache) Size() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.evictionList.Len()
}

// Capacity returns the maximum number of items the cache can hold
func (c *LRUCache) Capacity() int {
	return c.capacity
}

// Stats returns statistics about the cache usage
func (c *LRUCache) Stats() CacheStats {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	size := c.evictionList.Len()
	total := c.hits + c.misses
	hitRate := 0.0
	avgSize := 0

	if total > 0 {
		hitRate = float64(c.hits) / float64(total)
	}

	if size > 0 {
		avgSize = c.totalSize / size
	}

	return CacheStats{
		Size:      size,
		Capacity:  c.capacity,
		Hits:      c.hits,
		Misses:    c.misses,
		HitRate:   hitRate,
		Evictions: c.evictions,
		AvgSize:   avgSize,
	}
}

// evictOldest removes the least recently used item from the cache
func (c *LRUCache) evictOldest() bool {
	if element := c.evictionList.Back(); element != nil {
		return c.evictElement(element)
	}
	return false
}

// evictElement removes an item from the cache
func (c *LRUCache) evictElement(element *list.Element) bool {
	item := element.Value.(*CacheItem)
	c.evictionList.Remove(element)
	delete(c.items, item.Key)
	c.totalSize -= item.Size
	c.evictions++
	return true
}