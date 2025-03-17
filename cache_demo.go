package main

// import (
// 	"fmt"
// 	"time"
// 	"github.com/Jovial-Kanwadia/proxy-server/cache"
// )

// func main() {
// 	// Create a new LRU cache with capacity 5
// 	lruCache := cache.NewLRUCache(5)
	
// 	fmt.Println("=== LRU Cache Demo ===")
// 	fmt.Printf("Initial size: %d\n", lruCache.Size())
	
// 	// Add some items
// 	fmt.Println("\nAdding items...")
// 	for i := 1; i <= 3; i++ {
// 		key := fmt.Sprintf("key%d", i)
// 		value := []byte(fmt.Sprintf("value%d", i))
// 		lruCache.Set(key, value, 0)
// 		fmt.Printf("Added %s: %s\n", key, value)
// 	}
	
// 	// Set an item with TTL
// 	fmt.Println("\nAdding item with TTL...")
// 	lruCache.Set("ttl-key", []byte("expires-soon"), 2*time.Second)
// 	fmt.Printf("Added ttl-key with 2-second TTL\n")
	
// 	// Get items
// 	fmt.Println("\nRetrieving items...")
// 	for i := 1; i <= 3; i++ {
// 		key := fmt.Sprintf("key%d", i)
// 		if item, found := lruCache.Get(key); found {
// 			fmt.Printf("Found %s: %s\n", key, item.Value)
// 		} else {
// 			fmt.Printf("Item %s not found\n", key)
// 		}
// 	}
	
// 	// Get the TTL item
// 	if item, found := lruCache.Get("ttl-key"); found {
// 		fmt.Printf("Found ttl-key: %s\n", item.Value)
// 	} else {
// 		fmt.Printf("Item ttl-key not found\n")
// 	}
	
// 	// Add more items to trigger eviction
// 	fmt.Println("\nAdding more items to trigger eviction...")
// 	for i := 4; i <= 6; i++ {
// 		key := fmt.Sprintf("key%d", i)
// 		value := []byte(fmt.Sprintf("value%d", i))
// 		lruCache.Set(key, value, 0)
// 		fmt.Printf("Added %s: %s\n", key, value)
// 	}
	
// 	// Check which items are still in the cache
// 	fmt.Println("\nChecking cache contents after eviction...")
// 	for i := 1; i <= 6; i++ {
// 		key := fmt.Sprintf("key%d", i)
// 		if item, found := lruCache.Get(key); found {
// 			fmt.Printf("Found %s: %s\n", key, item.Value)
// 		} else {
// 			fmt.Printf("Item %s not found (evicted)\n", key)
// 		}
// 	}
	
// 	// Wait for TTL to expire
// 	fmt.Println("\nWaiting for TTL to expire...")
// 	time.Sleep(3 * time.Second)
	
// 	// Check the TTL item again
// 	if item, found := lruCache.Get("ttl-key"); found {
// 		fmt.Printf("Found ttl-key: %s\n", item.Value)
// 	} else {
// 		fmt.Printf("Item ttl-key not found (expired)\n")
// 	}
	
// 	// Get cache statistics
// 	fmt.Println("\nCache statistics:")
// 	stats := lruCache.Stats()
// 	fmt.Printf("Size: %d/%d\n", stats.Size, stats.Capacity)
// 	fmt.Printf("Hits: %d\n", stats.Hits)
// 	fmt.Printf("Misses: %d\n", stats.Misses)
// 	fmt.Printf("Hit Rate: %.2f%%\n", stats.HitRate*100)
// 	fmt.Printf("Evictions: %d\n", stats.Evictions)
// 	fmt.Printf("Average Item Size: %d bytes\n", stats.AvgSize)
// }