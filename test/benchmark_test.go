package test

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/dark-elixir/mukhostho-db/internal/config"
	"github.com/dark-elixir/mukhostho-db/internal/core"
	"github.com/dark-elixir/mukhostho-db/internal/memory"
)

// BenchmarkStoreSet benchmarks the SET operation
func BenchmarkStoreSet(b *testing.B) {
	store := core.NewStore()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key_%d", i)
			value := fmt.Sprintf("value_%d", i)
			store.Set(key, value, nil)
			i++
		}
	})
}

// BenchmarkStoreGet benchmarks the GET operation
func BenchmarkStoreGet(b *testing.B) {
	store := core.NewStore()

	// Prepare test data
	numKeys := 10000
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := fmt.Sprintf("value_%d", i)
		store.Set(key, value, nil)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			key := fmt.Sprintf("key_%d", rand.Intn(numKeys))
			store.Get(key)
		}
	})
}

// BenchmarkStoreDelete benchmarks the DELETE operation
func BenchmarkStoreDelete(b *testing.B) {
	store := core.NewStore()

	// Prepare test data
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := fmt.Sprintf("value_%d", i)
		store.Set(key, value, nil)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key_%d", i)
		store.Delete(key)
	}
}

// BenchmarkStoreMixed benchmarks mixed operations
func BenchmarkStoreMixed(b *testing.B) {
	store := core.NewStore()

	// Prepare some initial data
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := fmt.Sprintf("value_%d", i)
		store.Set(key, value, nil)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			switch i % 10 {
			case 0, 1, 2, 3, 4, 5: // 60% reads
				key := fmt.Sprintf("key_%d", rand.Intn(1000))
				store.Get(key)
			case 6, 7, 8: // 30% writes
				key := fmt.Sprintf("key_%d", i)
				value := fmt.Sprintf("value_%d", i)
				store.Set(key, value, nil)
			case 9: // 10% deletes
				key := fmt.Sprintf("key_%d", rand.Intn(1000))
				store.Delete(key)
			}
			i++
		}
	})
}

// BenchmarkLRUCache benchmarks the LRU cache implementation
func BenchmarkLRUCache(b *testing.B) {
	cache := core.NewLRUCacheWithCapacity(1000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key_%d", i%1000)
			value := fmt.Sprintf("value_%d", i)
			cache.Put(key, value)
			i++
		}
	})
}

// BenchmarkLRUCacheGet benchmarks LRU cache GET operations
func BenchmarkLRUCacheGet(b *testing.B) {
	cache := core.NewLRUCacheWithCapacity(1000)

	// Prepare test data
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := fmt.Sprintf("value_%d", i)
		cache.Put(key, value)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			key := fmt.Sprintf("key_%d", rand.Intn(1000))
			cache.Get(key)
		}
	})
}

// BenchmarkMemoryManager benchmarks the memory manager
func BenchmarkMemoryManager(b *testing.B) {
	cfg := &config.MemoryConfig{
		MaxMemoryMB:     100, // 100MB
		EvictionPolicy:  "lru",
		TTLCleanupDelay: 60 * time.Second,
		GCInterval:      60 * time.Second,
	}

	store := core.NewStore()
	manager := memory.NewMemoryManager(store, cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.GetMemoryStats()
	}
}

// BenchmarkConcurrentAccess benchmarks concurrent access patterns
func BenchmarkConcurrentAccess(b *testing.B) {
	store := core.NewStore()
	numGoroutines := 100

	b.ResetTimer()

	var wg sync.WaitGroup
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for i := 0; i < b.N/numGoroutines; i++ {
				key := fmt.Sprintf("key_%d_%d", goroutineID, i)
				value := fmt.Sprintf("value_%d_%d", goroutineID, i)

				// Mixed operations
				switch i % 3 {
				case 0:
					store.Set(key, value, nil)
				case 1:
					store.Get(key)
				case 2:
					store.Delete(key)
				}
			}
		}(g)
	}

	wg.Wait()
}

// BenchmarkTTLOperations benchmarks TTL operations
func BenchmarkTTLOperations(b *testing.B) {
	store := core.NewStore()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := fmt.Sprintf("value_%d", i)
		ttl := 1 * time.Hour // 1 hour
		store.Set(key, value, &ttl)
	}
}

// BenchmarkLargeValues benchmarks operations with large values
func BenchmarkLargeValues(b *testing.B) {
	store := core.NewStore()

	// Create a large value (1KB)
	largeValue := make([]byte, 1024)
	for i := range largeValue {
		largeValue[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key_%d", i)
		store.Set(key, string(largeValue), nil)
	}
}

// BenchmarkRangeOperation benchmarks the Range operation using ListItems
func BenchmarkRangeOperation(b *testing.B) {
	store := core.NewStore()

	// Prepare test data
	numKeys := 10000
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := fmt.Sprintf("value_%d", i)
		store.Set(key, value, nil)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		count := 0
		items := store.ListItems()
		for range items {
			count++
			if count >= 100 { // Stop after 100 items
				break
			}
		}
	}
}

// BenchmarkKeyExists benchmarks the key existence check using Get
func BenchmarkKeyExists(b *testing.B) {
	store := core.NewStore()

	// Prepare test data
	numKeys := 10000
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := fmt.Sprintf("value_%d", i)
		store.Set(key, value, nil)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			key := fmt.Sprintf("key_%d", rand.Intn(numKeys))
			_, exists, _ := store.Get(key)
			_ = exists // Use the variable to avoid unused warning
		}
	})
}

// BenchmarkSize benchmarks the size operation using GetKeyCount
func BenchmarkSize(b *testing.B) {
	store := core.NewStore()

	// Prepare test data
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := fmt.Sprintf("value_%d", i)
		store.Set(key, value, nil)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.GetKeyCount()
	}
}

// BenchmarkMemoryUsage benchmarks memory usage calculation using GetMemoryUsage
func BenchmarkMemoryUsage(b *testing.B) {
	store := core.NewStore()

	// Prepare test data
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := fmt.Sprintf("value_%d", i)
		store.Set(key, value, nil)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.GetMemoryUsage()
	}
}
