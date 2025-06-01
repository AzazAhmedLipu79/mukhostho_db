package memory

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/dark-elixir/mukhostho-db/internal/config"
	"github.com/dark-elixir/mukhostho-db/internal/core"
)

// MemoryManager handles memory limits, eviction policies, and garbage collection
type MemoryManager struct {
	store          *core.Store
	config         *config.MemoryConfig
	maxMemoryBytes int64
	mu             sync.RWMutex
	running        bool
	stopChan       chan struct{}
	evictionPolicy EvictionPolicy
}

// EvictionPolicy defines the interface for eviction strategies
type EvictionPolicy interface {
	ShouldEvict(currentMemory, maxMemory int64) bool
	SelectVictim(store *core.Store) (string, bool)
	Name() string
}

// LRUEvictionPolicy implements Least Recently Used eviction
type LRUEvictionPolicy struct{}

func (p *LRUEvictionPolicy) ShouldEvict(currentMemory, maxMemory int64) bool {
	return currentMemory > maxMemory
}

func (p *LRUEvictionPolicy) SelectVictim(store *core.Store) (string, bool) {
	return store.EvictLRU()
}

func (p *LRUEvictionPolicy) Name() string {
	return "LRU"
}

// MemoryStats holds memory usage statistics
type MemoryStats struct {
	CurrentMemoryBytes int64     `json:"current_memory_bytes"`
	MaxMemoryBytes     int64     `json:"max_memory_bytes"`
	MemoryUtilization  float64   `json:"memory_utilization"`
	KeyCount           int64     `json:"key_count"`
	LastCleanup        time.Time `json:"last_cleanup"`
	LastEviction       time.Time `json:"last_eviction"`
	TotalEvictions     int64     `json:"total_evictions"`
	ExpiredKeysRemoved int64     `json:"expired_keys_removed"`
}

// NewMemoryManager creates a new memory manager
func NewMemoryManager(store *core.Store, config *config.MemoryConfig) *MemoryManager {
	maxMemoryBytes := config.MaxMemoryMB * 1024 * 1024 // Convert MB to bytes

	var evictionPolicy EvictionPolicy
	switch config.EvictionPolicy {
	case "lru":
		evictionPolicy = &LRUEvictionPolicy{}
	default:
		evictionPolicy = &LRUEvictionPolicy{} // Default to LRU
	}

	return &MemoryManager{
		store:          store,
		config:         config,
		maxMemoryBytes: maxMemoryBytes,
		stopChan:       make(chan struct{}),
		evictionPolicy: evictionPolicy,
	}
}

// Start begins the memory management background processes
func (mm *MemoryManager) Start() {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	if mm.running {
		return
	}

	mm.running = true

	// Start TTL cleanup goroutine
	go mm.ttlCleanupLoop()

	// Start memory monitoring goroutine
	go mm.memoryMonitorLoop()

	// Start garbage collection goroutine
	go mm.gcLoop()
}

// Stop stops all memory management background processes
func (mm *MemoryManager) Stop() {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	if !mm.running {
		return
	}

	mm.running = false
	close(mm.stopChan)
}

// CheckMemoryLimit checks if memory limit is exceeded and triggers eviction if necessary
func (mm *MemoryManager) CheckMemoryLimit() error {
	currentMemory := mm.store.GetMemoryUsage()

	if !mm.evictionPolicy.ShouldEvict(currentMemory, mm.maxMemoryBytes) {
		return nil
	}

	// Perform eviction
	evicted := 0
	for currentMemory > mm.maxMemoryBytes && evicted < 100 { // Limit evictions per cycle
		key, success := mm.evictionPolicy.SelectVictim(mm.store)
		if !success {
			break // No more keys to evict
		}

		fmt.Printf("Evicted key: %s due to memory pressure\n", key)
		evicted++
		currentMemory = mm.store.GetMemoryUsage()
	}

	if currentMemory > mm.maxMemoryBytes {
		return fmt.Errorf("memory limit exceeded: %d bytes (limit: %d bytes)",
			currentMemory, mm.maxMemoryBytes)
	}

	return nil
}

// CleanupExpiredKeys removes all expired keys from the store
func (mm *MemoryManager) CleanupExpiredKeys() int64 {
	return mm.store.CleanupExpired()
}

// GetMemoryStats returns current memory usage statistics
func (mm *MemoryManager) GetMemoryStats() *MemoryStats {
	currentMemory := mm.store.GetMemoryUsage()
	utilization := float64(currentMemory) / float64(mm.maxMemoryBytes) * 100

	return &MemoryStats{
		CurrentMemoryBytes: currentMemory,
		MaxMemoryBytes:     mm.maxMemoryBytes,
		MemoryUtilization:  utilization,
		KeyCount:           mm.store.GetKeyCount(),
		LastCleanup:        time.Now(), // This should be tracked properly
		// Other fields would be tracked by the manager
	}
}

// SetMemoryLimit updates the memory limit
func (mm *MemoryManager) SetMemoryLimit(limitMB int64) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	mm.maxMemoryBytes = limitMB * 1024 * 1024
	mm.config.MaxMemoryMB = limitMB
}

// ForceEviction forces eviction of a specified number of keys
func (mm *MemoryManager) ForceEviction(count int) int {
	evicted := 0
	for i := 0; i < count; i++ {
		_, success := mm.evictionPolicy.SelectVictim(mm.store)
		if !success {
			break
		}
		evicted++
	}
	return evicted
}

// ttlCleanupLoop runs periodically to clean up expired keys
func (mm *MemoryManager) ttlCleanupLoop() {
	ticker := time.NewTicker(mm.config.TTLCleanupDelay)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if !mm.running {
				return
			}

			expired := mm.CleanupExpiredKeys()
			if expired > 0 {
				fmt.Printf("Cleaned up %d expired keys\n", expired)
			}

		case <-mm.stopChan:
			return
		}
	}
}

// memoryMonitorLoop monitors memory usage and triggers eviction when necessary
func (mm *MemoryManager) memoryMonitorLoop() {
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if !mm.running {
				return
			}

			if err := mm.CheckMemoryLimit(); err != nil {
				fmt.Printf("Memory management warning: %v\n", err)
			}

		case <-mm.stopChan:
			return
		}
	}
}

// gcLoop runs garbage collection periodically
func (mm *MemoryManager) gcLoop() {
	ticker := time.NewTicker(mm.config.GCInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if !mm.running {
				return
			}

			// Force garbage collection
			runtime.GC()

			// Get memory stats after GC
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)

			fmt.Printf("GC completed: Heap size: %d KB, Objects: %d\n",
				memStats.HeapInuse/1024, memStats.HeapObjects)

		case <-mm.stopChan:
			return
		}
	}
}

// GetSystemMemoryInfo returns system memory information
func (mm *MemoryManager) GetSystemMemoryInfo() map[string]interface{} {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return map[string]interface{}{
		"heap_alloc_kb":    memStats.HeapAlloc / 1024,
		"heap_sys_kb":      memStats.HeapSys / 1024,
		"heap_inuse_kb":    memStats.HeapInuse / 1024,
		"heap_released_kb": memStats.HeapReleased / 1024,
		"heap_objects":     memStats.HeapObjects,
		"gc_cycles":        memStats.NumGC,
		"last_gc_time":     time.Unix(0, int64(memStats.LastGC)),
	}
}

// IsMemoryPressure returns true if the system is under memory pressure
func (mm *MemoryManager) IsMemoryPressure() bool {
	currentMemory := mm.store.GetMemoryUsage()
	threshold := float64(mm.maxMemoryBytes) * 0.8 // 80% threshold
	return float64(currentMemory) > threshold
}

// GetEvictionPolicy returns the current eviction policy name
func (mm *MemoryManager) GetEvictionPolicy() string {
	return mm.evictionPolicy.Name()
}

// EstimateKeySize estimates the memory size of a key-value pair
func (mm *MemoryManager) EstimateKeySize(key string, value interface{}) int64 {
	keySize := int64(len(key))

	switch v := value.(type) {
	case string:
		return keySize + int64(len(v)) + 64 // Add overhead
	case int, int64, float64:
		return keySize + 8 + 64
	case bool:
		return keySize + 1 + 64
	default:
		return keySize + 256 // Conservative estimate for complex objects
	}
}
