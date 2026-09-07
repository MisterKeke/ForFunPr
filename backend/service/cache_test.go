package service

import (
	"testing"
	"time"
)

func TestBoundedTTLCacheExpiryEvictionAndInvalidation(t *testing.T) {
	now := time.Unix(100, 0)
	cache := newBoundedTTLCache[int](2, time.Minute, nil)
	cache.clock = func() time.Time { return now }
	cache.set("first", 1)
	cache.set("second", 2)
	if _, ok := cache.get("first"); !ok {
		t.Fatal("first entry was not cached")
	}
	cache.set("third", 3)
	if _, ok := cache.get("second"); ok {
		t.Fatal("least recently used entry was not evicted")
	}
	cache.invalidate("first")
	if _, ok := cache.get("first"); ok {
		t.Fatal("targeted entry was not invalidated")
	}
	now = now.Add(2 * time.Minute)
	if _, ok := cache.get("third"); ok {
		t.Fatal("expired entry was returned")
	}
}

func TestTelegramCacheDoesNotExposeMutableSlices(t *testing.T) {
	cache := newBoundedTTLCache[[]TelegramPost](2, time.Minute, cloneTelegramPosts)
	original := []TelegramPost{{Text: "original", Images: []string{"one"}}}
	cache.set("channel", original)
	original[0].Text = "caller mutation"
	original[0].Images[0] = "caller image mutation"

	first, ok := cache.get("channel")
	if !ok {
		t.Fatal("entry was not cached")
	}
	first[0].Text = "returned mutation"
	first[0].Images[0] = "returned image mutation"
	second, _ := cache.get("channel")
	if second[0].Text != "original" || second[0].Images[0] != "one" {
		t.Fatalf("cached slice was mutated: %#v", second)
	}
}
