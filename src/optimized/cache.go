package optimized

import "sync"

// Cache stores compiled representations keyed by node instances.
type Cache struct {
	mu      sync.Mutex
	cache   map[interface{}]string
	maxSize int
}

// NewCache creates a cache with default size limit.
func NewCache() *Cache {
	return &Cache{cache: make(map[interface{}]string), maxSize: 1000}
}

// Fetch retrieves the cached value or computes and stores it using fn.
func (c *Cache) Fetch(node interface{}, fn func() string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if v, ok := c.cache[node]; ok {
		return v
	}
	val := fn()
	if len(c.cache) >= c.maxSize {
		for k := range c.cache {
			delete(c.cache, k)
			break
		}
	}
	c.cache[node] = val
	return val
}

var defaultCache = NewCache()

// DefaultCache provides a package-level cache instance.
func DefaultCache() *Cache { return defaultCache }
