package core

import (
	"container/list"
	"sync"
)

// LRUCache implements a thread-safe LRU cache for key eviction
type LRUCache struct {
	capacity int
	cache    map[string]*list.Element
	list     *list.List
	mu       sync.RWMutex
}

// CacheEntry represents an entry in the LRU cache
type CacheEntry struct {
	Key   string
	Value interface{}
}

// NewLRUCache creates a new LRU cache with default capacity
func NewLRUCache() *LRUCache {
	return &LRUCache{
		capacity: 10000, // Default capacity
		cache:    make(map[string]*list.Element),
		list:     list.New(),
	}
}

// NewLRUCacheWithCapacity creates a new LRU cache with specified capacity
func NewLRUCacheWithCapacity(capacity int) *LRUCache {
	return &LRUCache{
		capacity: capacity,
		cache:    make(map[string]*list.Element),
		list:     list.New(),
	}
}

// Get retrieves a value and marks it as recently used
func (lru *LRUCache) Get(key string) (interface{}, bool) {
	lru.mu.Lock()
	defer lru.mu.Unlock()

	if elem, exists := lru.cache[key]; exists {
		// Move to front (most recently used)
		lru.list.MoveToFront(elem)
		entry := elem.Value.(*CacheEntry)
		return entry.Value, true
	}
	return nil, false
}

// Put adds or updates a key-value pair
func (lru *LRUCache) Put(key string, value interface{}) {
	lru.mu.Lock()
	defer lru.mu.Unlock()

	if elem, exists := lru.cache[key]; exists {
		// Update existing entry and move to front
		lru.list.MoveToFront(elem)
		entry := elem.Value.(*CacheEntry)
		entry.Value = value
		return
	}

	// Add new entry
	entry := &CacheEntry{Key: key, Value: value}
	elem := lru.list.PushFront(entry)
	lru.cache[key] = elem

	// Check capacity and evict if necessary
	if lru.list.Len() > lru.capacity {
		lru.removeOldest()
	}
}

// Remove removes a key from the cache
func (lru *LRUCache) Remove(key string) bool {
	lru.mu.Lock()
	defer lru.mu.Unlock()

	if elem, exists := lru.cache[key]; exists {
		lru.removeElement(elem)
		return true
	}
	return false
}

// RemoveLRU removes and returns the least recently used key
func (lru *LRUCache) RemoveLRU() string {
	lru.mu.Lock()
	defer lru.mu.Unlock()

	if lru.list.Len() == 0 {
		return ""
	}

	elem := lru.list.Back()
	if elem != nil {
		entry := elem.Value.(*CacheEntry)
		lru.removeElement(elem)
		return entry.Key
	}
	return ""
}

// Len returns the current number of items in the cache
func (lru *LRUCache) Len() int {
	lru.mu.RLock()
	defer lru.mu.RUnlock()
	return lru.list.Len()
}

// Clear removes all items from the cache
func (lru *LRUCache) Clear() {
	lru.mu.Lock()
	defer lru.mu.Unlock()
	lru.list.Init()
	lru.cache = make(map[string]*list.Element)
}

// GetKeys returns all keys in LRU order (most recent first)
func (lru *LRUCache) GetKeys() []string {
	lru.mu.RLock()
	defer lru.mu.RUnlock()

	keys := make([]string, 0, lru.list.Len())
	for elem := lru.list.Front(); elem != nil; elem = elem.Next() {
		entry := elem.Value.(*CacheEntry)
		keys = append(keys, entry.Key)
	}
	return keys
}

// removeOldest removes the least recently used item
func (lru *LRUCache) removeOldest() {
	elem := lru.list.Back()
	if elem != nil {
		lru.removeElement(elem)
	}
}

// removeElement removes a specific element from the cache
func (lru *LRUCache) removeElement(elem *list.Element) {
	lru.list.Remove(elem)
	entry := elem.Value.(*CacheEntry)
	delete(lru.cache, entry.Key)
}

// SetCapacity updates the cache capacity and evicts items if necessary
func (lru *LRUCache) SetCapacity(capacity int) {
	lru.mu.Lock()
	defer lru.mu.Unlock()

	lru.capacity = capacity
	for lru.list.Len() > lru.capacity {
		lru.removeOldest()
	}
}
