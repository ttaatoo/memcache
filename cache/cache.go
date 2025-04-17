package cache

import (
	"sync"
	"time"
)

// CacheEntry represents a cached value with its metadata
type CacheEntry[V any] struct {
	value      V
	expiresAt  time.Time
	ready      chan struct{}  // closed when value is ready
	prev, next *CacheEntry[V] // for LRU list
}

// Cache is a concurrent-safe in-memory cache
type Cache[K comparable, V any] struct {
	mu       sync.RWMutex
	items    map[K]*CacheEntry[V]
	capacity int
	head     *CacheEntry[V] // most recently used
	tail     *CacheEntry[V] // least recently used
}

// New creates a new cache instance
func New[K comparable, V any](capacity int) *Cache[K, V] {
	if capacity <= 0 {
		capacity = 1000 // default capacity
	}
	return &Cache[K, V]{
		items:    make(map[K]*CacheEntry[V]),
		capacity: capacity,
	}
}

// moveToFront moves the entry to the front of the LRU list
func (c *Cache[K, V]) moveToFront(entry *CacheEntry[V]) {
	if entry == c.head {
		return
	}

	// Remove from current position
	if entry.prev != nil {
		entry.prev.next = entry.next
	}
	if entry.next != nil {
		entry.next.prev = entry.prev
	}
	if entry == c.tail {
		c.tail = entry.prev
	}

	// Add to front
	entry.prev = nil
	entry.next = c.head
	if c.head != nil {
		c.head.prev = entry
	}
	c.head = entry
	if c.tail == nil {
		c.tail = entry
	}
}

// removeTail removes the least recently used entry
func (c *Cache[K, V]) removeTail() {
	if c.tail == nil {
		return
	}

	// Find the key for this entry
	var key K
	for k, v := range c.items {
		if v == c.tail {
			key = k
			break
		}
	}

	delete(c.items, key)
	if c.tail.prev != nil {
		c.tail.prev.next = nil
	}
	c.tail = c.tail.prev
	if c.tail == nil {
		c.head = nil
	}
}

// Get retrieves a value from the cache. If the value doesn't exist,
// it calls the provided function to compute it.
func (c *Cache[K, V]) Get(key K, compute func() (V, error)) (V, error) {
	// First try a fast path with read lock
	c.mu.RLock()
	entry, exists := c.items[key]
	c.mu.RUnlock()

	if exists {
		// Wait for the value to be ready if it's being computed
		<-entry.ready

		// Check if the entry has expired
		if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
			// Entry expired, remove it and recompute
			c.mu.Lock()
			delete(c.items, key)
			c.mu.Unlock()
			return c.Get(key, compute)
		}

		// Move to front of LRU list
		c.mu.Lock()
		c.moveToFront(entry)
		c.mu.Unlock()

		return entry.value, nil
	}

	// Value doesn't exist, need to compute it
	c.mu.Lock()

	// Double check after acquiring write lock
	entry, exists = c.items[key]
	if exists {
		c.mu.Unlock()
		<-entry.ready
		return entry.value, nil
	}

	// Check if we need to evict
	if len(c.items) >= c.capacity {
		c.removeTail()
	}

	// Create new entry and start computation
	entry = &CacheEntry[V]{
		ready: make(chan struct{}),
	}
	c.items[key] = entry
	c.moveToFront(entry)
	c.mu.Unlock()

	// Compute the value
	value, err := compute()
	if err != nil {
		// Remove the entry if computation failed
		c.mu.Lock()
		delete(c.items, key)
		c.mu.Unlock()
		close(entry.ready)
		return value, err
	}

	// Store the computed value
	entry.value = value
	close(entry.ready)
	return value, nil
}

// Set stores a value in the cache with an optional expiration time
func (c *Cache[K, V]) Set(key K, value V, expiration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if we need to evict
	if len(c.items) >= c.capacity {
		c.removeTail()
	}

	entry := &CacheEntry[V]{
		value: value,
		ready: make(chan struct{}),
	}

	if expiration > 0 {
		entry.expiresAt = time.Now().Add(expiration)
	}

	close(entry.ready)
	c.items[key] = entry
	c.moveToFront(entry)
}

// Delete removes a value from the cache
func (c *Cache[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.items[key]
	if !exists {
		return
	}

	// Remove from LRU list
	if entry.prev != nil {
		entry.prev.next = entry.next
	}
	if entry.next != nil {
		entry.next.prev = entry.prev
	}
	if entry == c.head {
		c.head = entry.next
	}
	if entry == c.tail {
		c.tail = entry.prev
	}

	delete(c.items, key)
}

// Clear removes all values from the cache
func (c *Cache[K, V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[K]*CacheEntry[V])
	c.head = nil
	c.tail = nil
}

// Size returns the number of items in the cache
func (c *Cache[K, V]) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Cleanup removes expired entries from the cache
func (c *Cache[K, V]) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, entry := range c.items {
		if !entry.expiresAt.IsZero() && now.After(entry.expiresAt) {
			// Remove from LRU list
			if entry.prev != nil {
				entry.prev.next = entry.next
			}
			if entry.next != nil {
				entry.next.prev = entry.prev
			}
			if entry == c.head {
				c.head = entry.next
			}
			if entry == c.tail {
				c.tail = entry.prev
			}

			delete(c.items, key)
		}
	}
}
