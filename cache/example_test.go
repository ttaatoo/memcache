package cache_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"memcache/cache"
)

func ExampleCache() {
	// Create a cache with capacity of 3 items
	c := cache.New[string, int](3)

	// Set some values
	c.Set("one", 1, 0)             // No expiration
	c.Set("two", 2, time.Hour)     // Expires in 1 hour
	c.Set("three", 3, time.Minute) // Expires in 1 minute

	// Get values
	val, err := c.Get("one", nil)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("one:", val)

	// Add more items (will trigger LRU eviction)
	c.Set("four", 4, 0)
	c.Set("five", 5, 0)

	// Check which items remain (LRU eviction)
	_, err1 := c.Get("one", nil)
	_, err2 := c.Get("two", nil)
	_, err3 := c.Get("three", nil)
	_, err4 := c.Get("four", nil)
	_, err5 := c.Get("five", nil)

	fmt.Println("one exists:", err1 == nil)
	fmt.Println("two exists:", err2 == nil)
	fmt.Println("three exists:", err3 == nil)
	fmt.Println("four exists:", err4 == nil)
	fmt.Println("five exists:", err5 == nil)

	// Output:
	// one: 1
	// one exists: false
	// two exists: true
	// three exists: false
	// four exists: true
	// five exists: true
}

func ExampleCache_expiration() {
	c := cache.New[string, int](10)

	// Set value with 1 second expiration
	c.Set("key", 42, time.Second)

	// Get immediately
	val, err := c.Get("key", nil)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Before expiration:", val)

	// Wait for expiration
	time.Sleep(2 * time.Second)

	// Try to get expired value
	val, err = c.Get("key", func() (int, error) {
		return 0, nil // This will be called since key expired
	})
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("After expiration:", val)

	// Output:
	// Before expiration: 42
	// After expiration: 0
}

func ExampleCache_concurrent() {
	c := cache.New[string, int](100)
	var wg sync.WaitGroup

	// Simulate concurrent access
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("key%d", id)

			// Get or compute value
			val, err := c.Get(key, func() (int, error) {
				time.Sleep(100 * time.Millisecond) // Simulate expensive computation
				return id * 10, nil
			})
			if err != nil {
				fmt.Printf("Error in goroutine %d: %v\n", id, err)
				return
			}

			fmt.Printf("Goroutine %d got value: %d\n", id, val)
		}(i)
	}

	wg.Wait()
}

func TestCache(t *testing.T) {
	// Create cache with small capacity to test LRU
	c := cache.New[string, int](2)

	// Test basic set/get
	c.Set("one", 1, 0)
	val, err := c.Get("one", nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if val != 1 {
		t.Errorf("Expected 1, got %d", val)
	}

	// Test LRU eviction
	c.Set("two", 2, 0)
	c.Set("three", 3, 0) // Should evict "one"

	if _, err := c.Get("one", nil); err == nil {
		t.Error("Expected 'one' to be evicted")
	}

	// Test expiration
	c.Set("expired", 42, time.Millisecond)
	time.Sleep(2 * time.Millisecond)
	if _, err := c.Get("expired", nil); err == nil {
		t.Error("Expected 'expired' to be expired")
	}

	// Test concurrent access
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("key%d", i)
			c.Set(key, i, 0)
			val, err := c.Get(key, nil)
			if err != nil {
				t.Errorf("Unexpected error for key %s: %v", key, err)
				return
			}
			if val != i {
				t.Errorf("Expected %d, got %d", i, val)
			}
		}(i)
	}
	wg.Wait()

	// Test cleanup
	c.Set("expired1", 1, time.Millisecond)
	c.Set("expired2", 2, time.Millisecond)
	time.Sleep(2 * time.Millisecond)
	c.Cleanup()
	if c.Size() != 0 {
		t.Errorf("Expected size 0 after cleanup, got %d", c.Size())
	}
}
