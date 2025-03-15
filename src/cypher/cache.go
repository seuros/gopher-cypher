package cypher

import "sync"

// SimpleCache stores compiled query fragments keyed by a string.
// Thread-safe with RWMutex and FIFO eviction.
type SimpleCache struct {
	mu      sync.RWMutex
	cache   map[string]string
	order   []string // FIFO insertion order
	maxSize int
}

// NewSimpleCache creates a cache with default size limit.
func NewSimpleCache() *SimpleCache {
	return &SimpleCache{
		cache:   make(map[string]string),
		order:   make([]string, 0),
		maxSize: 1000,
	}
}

// Fetch retrieves the cached value or builds and stores it using fn.
func (c *SimpleCache) Fetch(key string, fn func() string) string {
	// Fast path: check if key exists with read lock
	c.mu.RLock()
	if v, ok := c.cache[key]; ok {
		c.mu.RUnlock()
		return v
	}
	c.mu.RUnlock()

	// Slow path: acquire write lock and check again (double-check locking)
	c.mu.Lock()
	defer c.mu.Unlock()

	// Re-check after acquiring write lock
	if v, ok := c.cache[key]; ok {
		return v
	}

	val := fn()

	// FIFO eviction: remove oldest entry if at capacity
	if len(c.cache) >= c.maxSize && len(c.order) > 0 {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.cache, oldest)
	}

	c.cache[key] = val
	c.order = append(c.order, key)
	return val
}

var simpleCache = NewSimpleCache()

// OptimizedCache is a specialized cache keyed by node instances.
// Thread-safe with RWMutex and FIFO eviction.
type OptimizedCache struct {
	mu      sync.RWMutex
	cache   map[interface{}]string
	order   []interface{} // FIFO insertion order
	maxSize int
}

func NewOptimizedCache() *OptimizedCache {
	return &OptimizedCache{
		cache:   make(map[interface{}]string),
		order:   make([]interface{}, 0),
		maxSize: 1000,
	}
}

func (c *OptimizedCache) Fetch(node interface{}, fn func() string) string {
	// Fast path: check if key exists with read lock
	c.mu.RLock()
	if v, ok := c.cache[node]; ok {
		c.mu.RUnlock()
		return v
	}
	c.mu.RUnlock()

	// Slow path: acquire write lock and check again (double-check locking)
	c.mu.Lock()
	defer c.mu.Unlock()

	// Re-check after acquiring write lock
	if v, ok := c.cache[node]; ok {
		return v
	}

	val := fn()

	// FIFO eviction: remove oldest entry if at capacity
	if len(c.cache) >= c.maxSize && len(c.order) > 0 {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.cache, oldest)
	}

	c.cache[node] = val
	c.order = append(c.order, node)
	return val
}

var optimizedCache = NewOptimizedCache()
