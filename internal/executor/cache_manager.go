package executor

import (
	"sync"
	"time"
)

type cacheEntry struct {
	Value     string
	ExpiresAt time.Time
}

type CacheManager struct {
	mu    sync.Mutex
	l1TTL time.Duration
	l2TTL time.Duration
	now   func() time.Time
	l1    map[string]cacheEntry
	l2    map[string]cacheEntry
	l3    map[string]string
}

func NewCacheManager() *CacheManager {
	return &CacheManager{
		l1TTL: 60 * time.Second,
		l2TTL: 5 * time.Minute,
		now:   func() time.Time { return time.Now().UTC() },
		l1:    map[string]cacheEntry{},
		l2:    map[string]cacheEntry{},
		l3:    map[string]string{},
	}
}

func (c *CacheManager) SetNow(now func() time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if now != nil {
		c.now = now
	}
}

func (c *CacheManager) WriteThrough(workspaceID, entityType, entityID, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := cacheKey(workspaceID, entityType, entityID)
	now := c.now()
	c.l3[key] = value
	c.l2[key] = cacheEntry{Value: value, ExpiresAt: now.Add(c.l2TTL)}
	c.l1[key] = cacheEntry{Value: value, ExpiresAt: now.Add(c.l1TTL)}
}

func (c *CacheManager) Read(workspaceID, entityType, entityID string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := cacheKey(workspaceID, entityType, entityID)
	now := c.now()

	if l1, ok := c.l1[key]; ok && now.Before(l1.ExpiresAt) {
		return l1.Value, true
	}
	if l2, ok := c.l2[key]; ok && now.Before(l2.ExpiresAt) {
		c.l1[key] = cacheEntry{Value: l2.Value, ExpiresAt: now.Add(c.l1TTL)}
		return l2.Value, true
	}
	l3, ok := c.l3[key]
	if !ok {
		return "", false
	}
	c.l2[key] = cacheEntry{Value: l3, ExpiresAt: now.Add(c.l2TTL)}
	c.l1[key] = cacheEntry{Value: l3, ExpiresAt: now.Add(c.l1TTL)}
	return l3, true
}

func (c *CacheManager) Invalidate(workspaceID, entityType, entityID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := cacheKey(workspaceID, entityType, entityID)
	delete(c.l1, key)
	delete(c.l2, key)
	delete(c.l3, key)
}

func cacheKey(workspaceID, entityType, entityID string) string {
	return "cache:{layer}:" + workspaceID + ":" + entityType + ":" + entityID
}
