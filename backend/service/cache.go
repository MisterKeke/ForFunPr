package service

import (
	"sync"
	"time"
)

const (
	favoriteCacheTTL       = 5 * time.Minute
	telegramCacheCapacity  = 128
	youTubeCacheCapacity   = 128
	handleCacheCapacity    = 256
)

type ttlCacheEntry[T any] struct {
	value      T
	expiresAt  time.Time
	lastAccess uint64
}

// boundedTTLCache is an instance-owned TTL/LRU cache. clone prevents callers
// from mutating stored slices (or receiving storage that another caller owns).
type boundedTTLCache[T any] struct {
	mu      sync.Mutex
	items   map[string]ttlCacheEntry[T]
	max     int
	ttl     time.Duration
	clock   func() time.Time
	clone   func(T) T
	access  uint64
}

func newBoundedTTLCache[T any](max int, ttl time.Duration, clone func(T) T) *boundedTTLCache[T] {
	if max < 1 { max = 1 }
	if ttl <= 0 { ttl = time.Minute }
	if clone == nil { clone = func(value T) T { return value } }
	return &boundedTTLCache[T]{
		items: make(map[string]ttlCacheEntry[T]),
		max: max,
		ttl: ttl,
		clock: time.Now,
		clone: clone,
	}
}

func (c *boundedTTLCache[T]) get(key string) (T, bool) {
	var zero T
	if c == nil { return zero, false }
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.clock()
	c.removeExpired(now)
	entry, ok := c.items[key]
	if !ok { return zero, false }
	c.access++
	entry.lastAccess = c.access
	c.items[key] = entry
	return c.clone(entry.value), true
}

func (c *boundedTTLCache[T]) set(key string, value T) {
	if c == nil { return }
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.clock()
	c.removeExpired(now)
	if _, exists := c.items[key]; !exists && len(c.items) >= c.max {
		c.removeLeastRecentlyUsed()
	}
	c.access++
	c.items[key] = ttlCacheEntry[T]{
		value: c.clone(value),
		expiresAt: now.Add(c.ttl),
		lastAccess: c.access,
	}
}

func (c *boundedTTLCache[T]) invalidate(key string) {
	if c == nil { return }
	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()
}

func (c *boundedTTLCache[T]) clear() {
	if c == nil { return }
	c.mu.Lock()
	c.items = make(map[string]ttlCacheEntry[T])
	c.mu.Unlock()
}

func (c *boundedTTLCache[T]) removeExpired(now time.Time) {
	for key, entry := range c.items {
		if !now.Before(entry.expiresAt) { delete(c.items, key) }
	}
}

func (c *boundedTTLCache[T]) removeLeastRecentlyUsed() {
	var oldestKey string
	var oldestAccess uint64
	first := true
	for key, entry := range c.items {
		if first || entry.lastAccess < oldestAccess {
			oldestKey = key
			oldestAccess = entry.lastAccess
			first = false
		}
	}
	if !first { delete(c.items, oldestKey) }
}

func cloneTelegramPosts(posts []TelegramPost) []TelegramPost {
	result := make([]TelegramPost, len(posts))
	copy(result, posts)
	for index := range result {
		result[index].Images = append([]string(nil), posts[index].Images...)
	}
	return result
}

func cloneYouTubeVideos(videos []YouTubeVideo) []YouTubeVideo {
	return append([]YouTubeVideo(nil), videos...)
}
