package tests

import (
	"testing"
	"time"
	"fmt"
	"github.com/Jovial-Kanwadia/proxy-server/cache"
)

func TestLRUCache_BasicOperations(t *testing.T) {
	c := cache.NewLRUCache(3)

	// Test initial state
	if c.Size() != 0 {
		t.Errorf("Expected size 0, got %d", c.Size())
	}

	// Test Set and Get
	c.Set("key1", []byte("value1"), 0)
	item, found := c.Get("key1")
	if !found {
		t.Error("Expected to find key1")
	}
	if string(item.Value) != "value1" {
		t.Errorf("Expected value1, got %s", string(item.Value))
	}

	// Test overwriting a key
	c.Set("key1", []byte("new-value1"), 0)
	item, found = c.Get("key1")
	if !found {
		t.Error("Expected to find key1")
	}
	if string(item.Value) != "new-value1" {
		t.Errorf("Expected new-value1, got %s", string(item.Value))
	}

	// Test removing a key
	removed := c.Remove("key1")
	if !removed {
		t.Error("Expected key1 to be removed")
	}
	_, found = c.Get("key1")
	if found {
		t.Error("Expected key1 to be gone")
	}

	// Test removing a non-existent key
	removed = c.Remove("nonexistent")
	if removed {
		t.Error("Expected nonexistent key removal to return false")
	}
}

func TestLRUCache_EvictionPolicy(t *testing.T) {
	c := cache.NewLRUCache(3)

	// Fill the cache
	c.Set("key1", []byte("value1"), 0)
	c.Set("key2", []byte("value2"), 0)
	c.Set("key3", []byte("value3"), 0)

	// All items should be in the cache
	for _, key := range []string{"key1", "key2", "key3"} {
		_, found := c.Get(key)
		if !found {
			t.Errorf("Expected to find %s", key)
		}
	}

	// Access key1 to make it most recently used
	c.Get("key1")

	// Add a new item, which should evict the least recently used (key2)
	c.Set("key4", []byte("value4"), 0)

	// key2 should be evicted
	_, found := c.Get("key2")
	if found {
		t.Error("Expected key2 to be evicted")
	}

	// key1, key3, and key4 should still be in the cache
	for _, key := range []string{"key1", "key3", "key4"} {
		_, found := c.Get(key)
		if !found {
			t.Errorf("Expected to find %s", key)
		}
	}
}

func TestLRUCache_TTL(t *testing.T) {
	c := cache.NewLRUCache(3)

	// Add an item with a 100ms TTL
	c.Set("key1", []byte("value1"), 100*time.Millisecond)

	// Item should be available immediately
	_, found := c.Get("key1")
	if !found {
		t.Error("Expected to find key1")
	}

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Item should be expired
	_, found = c.Get("key1")
	if found {
		t.Error("Expected key1 to be expired")
	}
}

func TestLRUCache_Clear(t *testing.T) {
	c := cache.NewLRUCache(3)

	// Fill the cache
	c.Set("key1", []byte("value1"), 0)
	c.Set("key2", []byte("value2"), 0)

	// Clear the cache
	c.Clear()

	// Cache should be empty
	if c.Size() != 0 {
		t.Errorf("Expected size 0, got %d", c.Size())
	}

	// Items should be gone
	for _, key := range []string{"key1", "key2"} {
		_, found := c.Get(key)
		if found {
			t.Errorf("Expected %s to be gone", key)
		}
	}
}

func TestLRUCache_Stats(t *testing.T) {
	c := cache.NewLRUCache(3)

	// Add some items
	c.Set("key1", []byte("value1"), 0)
	c.Set("key2", []byte("value2"), 0)

	// Generate some hits and misses
	c.Get("key1") // Hit
	c.Get("key1") // Hit
	c.Get("key2") // Hit
	c.Get("key3") // Miss

	// Check stats
	stats := c.Stats()
	if stats.Size != 2 {
		t.Errorf("Expected size 2, got %d", stats.Size)
	}
	if stats.Capacity != 3 {
		t.Errorf("Expected capacity 3, got %d", stats.Capacity)
	}
	if stats.Hits != 3 {
		t.Errorf("Expected 3 hits, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Expected 1 miss, got %d", stats.Misses)
	}
	if stats.HitRate != 0.75 {
		t.Errorf("Expected hit rate 0.75, got %f", stats.HitRate)
	}

	// Generate an eviction
	c.Set("key3", []byte("value3"), 0)
	c.Set("key4", []byte("value4"), 0) // This should evict key1

	// Check eviction stats
	stats = c.Stats()
	if stats.Evictions != 1 {
		t.Errorf("Expected 1 eviction, got %d", stats.Evictions)
	}
}

func TestLRUCache_Concurrency(t *testing.T) {
	c := cache.NewLRUCache(100)
	done := make(chan bool)

	// Spawn 10 goroutines to read and write concurrently
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				key := string(byte('a' + id))
				c.Set(key, []byte("value"), 0)
				c.Get(key)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to finish
	for i := 0; i < 10; i++ {
		<-done
	}

	// Check that the cache is in a valid state
	if c.Size() > c.Capacity() {
		t.Errorf("Cache size %d exceeds capacity %d", c.Size(), c.Capacity())
	}
}

func TestLRUCache_LargeValues(t *testing.T) {
	c := cache.NewLRUCache(10)
	
	// Create a large value (1MB)
	largeValue := make([]byte, 1024*1024)
	for i := range largeValue {
		largeValue[i] = byte(i % 256)
	}
	
	// Add the large value to the cache
	c.Set("large", largeValue, 0)
	
	// Retrieve the large value
	item, found := c.Get("large")
	if !found {
		t.Error("Expected to find large value")
	}
	
	// Check that the value is correct
	if len(item.Value) != len(largeValue) {
		t.Errorf("Expected value length %d, got %d", len(largeValue), len(item.Value))
	}
	
	// Check a few bytes to ensure the value is intact
	for i := 0; i < 10; i++ {
		if item.Value[i] != largeValue[i] {
			t.Errorf("Value mismatch at index %d", i)
		}
	}
}

func TestLRUCache_StressTest(t *testing.T) {
	// Skip in short mode
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}
	
	c := cache.NewLRUCache(1000)
	
	// Add a lot of items
	for i := 0; i < 5000; i++ {
		key := fmt.Sprintf("key%d", i)
		value := []byte(fmt.Sprintf("value%d", i))
		c.Set(key, value, 0)
	}
	
	// Check that the cache size is correct
	if c.Size() != 1000 {
		t.Errorf("Expected size 1000, got %d", c.Size())
	}
	
	// Check that we can find recent items
	for i := 4000; i < 5000; i++ {
		key := fmt.Sprintf("key%d", i)
		item, found := c.Get(key)
		if !found {
			t.Errorf("Expected to find %s", key)
		}
		expectedValue := fmt.Sprintf("value%d", i)
		if string(item.Value) != expectedValue {
			t.Errorf("Expected %s, got %s", expectedValue, string(item.Value))
		}
	}
	
	// Check that old items were evicted
	for i := 0; i < 4000; i++ {
		key := fmt.Sprintf("key%d", i)
		_, found := c.Get(key)
		if found {
			t.Errorf("Expected %s to be evicted", key)
		}
	}
}

func TestLRUCache_VariableTTL(t *testing.T) {
	c := cache.NewLRUCache(5)
	
	// Add items with different TTLs
	c.Set("instant", []byte("instant"), 1*time.Millisecond)
	c.Set("short", []byte("short"), 100*time.Millisecond)
	c.Set("medium", []byte("medium"), 200*time.Millisecond)
	c.Set("long", []byte("long"), 300*time.Millisecond)
	c.Set("forever", []byte("forever"), 0) // No TTL
	
	// Wait for the instant TTL to expire
	time.Sleep(10 * time.Millisecond)
	
	// Check that the instant TTL item is gone
	_, found := c.Get("instant")
	if found {
		t.Error("Expected instant TTL item to be gone")
	}
	
	// Check that other items are still there
	for _, key := range []string{"short", "medium", "long", "forever"} {
		_, found := c.Get(key)
		if !found {
			t.Errorf("Expected to find %s", key)
		}
	}
	
	// Wait for the short TTL to expire
	time.Sleep(100 * time.Millisecond)
	
	// Check that the short TTL item is gone
	_, found = c.Get("short")
	if found {
		t.Error("Expected short TTL item to be gone")
	}
	
	// Check that other items are still there
	for _, key := range []string{"medium", "long", "forever"} {
		_, found := c.Get(key)
		if !found {
			t.Errorf("Expected to find %s", key)
		}
	}
	
	// Wait for all TTLs to expire
	time.Sleep(200 * time.Millisecond)
	
	// Check that only the forever item is still there
	_, found = c.Get("medium")
	if found {
		t.Error("Expected medium TTL item to be gone")
	}
	_, found = c.Get("long")
	if found {
		t.Error("Expected long TTL item to be gone")
	}
	_, found = c.Get("forever")
	if !found {
		t.Error("Expected forever item to still be there")
	}
}

func TestLRUCache_ZeroCapacity(t *testing.T) {
	// Create a cache with zero capacity
	c := cache.NewLRUCache(0)
	
	// Try to add an item
	c.Set("key", []byte("value"), 0)
	
	// Check that the item was not added
	_, found := c.Get("key")
	if found {
		t.Error("Expected item not to be added to zero-capacity cache")
	}
}

func BenchmarkLRUCache_Get(b *testing.B) {
	c := cache.NewLRUCache(1000)
	
	// Add some items
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key%d", i)
		value := []byte(fmt.Sprintf("value%d", i))
		c.Set(key, value, 0)
	}
	
	b.ResetTimer()
	
	// Benchmark Get operations
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key%d", i%1000)
		c.Get(key)
	}
}

func BenchmarkLRUCache_Set(b *testing.B) {
	c := cache.NewLRUCache(1000)
	
	b.ResetTimer()
	
	// Benchmark Set operations
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key%d", i%1000)
		value := []byte(fmt.Sprintf("value%d", i))
		c.Set(key, value, 0)
	}
}

func BenchmarkLRUCache_MixedOperations(b *testing.B) {
	c := cache.NewLRUCache(1000)
	
	// Add some initial items
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key%d", i)
		value := []byte(fmt.Sprintf("value%d", i))
		c.Set(key, value, 0)
	}
	
	b.ResetTimer()
	
	// Benchmark mixed operations
	for i := 0; i < b.N; i++ {
		op := i % 3
		key := fmt.Sprintf("key%d", i%1000)
		
		switch op {
		case 0: // Get
			c.Get(key)
		case 1: // Set
			value := []byte(fmt.Sprintf("value%d", i))
			c.Set(key, value, 0)
		case 2: // Remove
			c.Remove(key)
		}
	}
}