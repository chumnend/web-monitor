package sites

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

type cacheItem struct {
	value     any
	expiresAt time.Time
}

// Cache provides a thread-safe in-memory TTL cache for expensive aggregate queries.
type Cache struct {
	mu      sync.RWMutex
	items   map[string]cacheItem
	ttl     time.Duration
	stopCh  chan struct{}
	stopped bool
}

// NewCache creates a new Cache with the given default TTL.
func NewCache(ttl time.Duration) *Cache {
	if ttl <= 0 {
		ttl = 15 * time.Second
	}
	c := &Cache{
		items:  make(map[string]cacheItem),
		ttl:    ttl,
		stopCh: make(chan struct{}),
	}
	go c.cleanupLoop(2 * ttl)
	return c
}

// Close stops the background eviction loop.
func (c *Cache) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped {
		return
	}
	c.stopped = true
	close(c.stopCh)
}

func (c *Cache) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.cleanup()
		}
	}
}

func (c *Cache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for k, item := range c.items {
		if now.After(item.expiresAt) {
			delete(c.items, k)
		}
	}
}

// Get retrieves an item from cache if present and not expired.
func (c *Cache) Get(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, ok := c.items[key]
	if !ok || time.Now().After(item.expiresAt) {
		return nil, false
	}
	return item.value, true
}

// Set stores an item in cache with the default TTL.
func (c *Cache) Set(key string, val any) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = cacheItem{
		value:     val,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// Delete removes an item by key.
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

// InvalidateUser invalidates all cached entries for a user (e.g. dashboard summary).
func (c *Cache) InvalidateUser(userID uuid.UUID) {
	c.Delete("summary:" + userID.String())
}

// InvalidateSite invalidates all cached entries for a site (e.g. uptime stats).
func (c *Cache) InvalidateSite(siteID uuid.UUID) {
	c.Delete("uptime:" + siteID.String())
}
