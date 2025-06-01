package test

import (
	"fmt"
	"testing"
	"time"

	"github.com/dark-elixir/mukhostho-db/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_BasicOperations(t *testing.T) {
	store := core.NewStore()

	// Test Set and Get
	err := store.Set("key1", "value1", nil)
	require.NoError(t, err)

	value, exists, err := store.Get("key1")
	require.NoError(t, err)
	assert.True(t, exists, "Key should exist")
	assert.Equal(t, "value1", value)

	// Test Get non-existent key
	value, exists, err = store.Get("nonexistent")
	require.NoError(t, err)
	assert.False(t, exists, "Key should not exist")
	// Test Delete
	deleted := store.Delete("key1")
	assert.True(t, deleted, "Delete should return true")

	value, exists, err = store.Get("key1")
	require.NoError(t, err)
	assert.False(t, exists, "Key should not exist after deletion")

	// Test Delete non-existent key
	deleted = store.Delete("nonexistent")
	assert.False(t, deleted, "Delete of non-existent key should return false")
}

func TestStore_TTL(t *testing.T) {
	store := core.NewStore()

	// Set with TTL
	ttl := 100 * time.Millisecond
	err := store.Set("ttl_key", "ttl_value", &ttl)
	require.NoError(t, err)

	// Should exist immediately
	value, exists, err := store.Get("ttl_key")
	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "ttl_value", value)

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should be expired
	value, exists, err = store.Get("ttl_key")
	require.NoError(t, err)
	assert.False(t, exists)
	assert.Nil(t, value)
}

func TestStore_ListOperations(t *testing.T) {
	store := core.NewStore()

	// Add some test data
	testData := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
		"key3": true,
		"key4": 3.14,
	}

	for k, v := range testData {
		err := store.Set(k, v, nil)
		require.NoError(t, err)
	}

	// Test ListKeys
	keys := store.ListKeys()
	assert.Len(t, keys, 4)
	for _, key := range keys {
		assert.Contains(t, testData, key)
	}

	// Test ListItems
	items := store.ListItems()
	assert.Len(t, items, 4)
	for k, v := range testData {
		assert.Equal(t, v, items[k])
	}
}

func TestStore_DataTypes(t *testing.T) {
	store := core.NewStore()

	testCases := []struct {
		key   string
		value interface{}
	}{
		{"string", "hello world"},
		{"int", 42},
		{"int64", int64(9223372036854775807)},
		{"float64", 3.141592653589793},
		{"bool_true", true},
		{"bool_false", false},
		{"slice", []string{"a", "b", "c"}},
		{"map", map[string]int{"x": 1, "y": 2}},
	}

	for _, tc := range testCases {
		err := store.Set(tc.key, tc.value, nil)
		require.NoError(t, err, "Failed to set %s", tc.key)

		value, exists, err := store.Get(tc.key)
		require.NoError(t, err, "Failed to get %s", tc.key)
		assert.True(t, exists, "Key %s should exist", tc.key)
		assert.Equal(t, tc.value, value, "Value mismatch for key %s", tc.key)
	}
}

func TestStore_MemoryTracking(t *testing.T) {
	store := core.NewStore()

	initialMemory := store.GetMemoryUsage()
	initialCount := store.GetKeyCount()

	// Add some data
	err := store.Set("large_string", string(make([]byte, 1000)), nil)
	require.NoError(t, err)

	// Memory usage should increase
	assert.Greater(t, store.GetMemoryUsage(), initialMemory)
	assert.Equal(t, initialCount+1, store.GetKeyCount())

	// Delete the data
	store.Delete("large_string")

	// Memory should decrease and count should be back to initial
	assert.Equal(t, initialCount, store.GetKeyCount())
}

func TestStore_CleanupExpired(t *testing.T) {
	store := core.NewStore()

	// Add some keys with short TTL
	shortTTL := 50 * time.Millisecond
	longTTL := 1 * time.Hour

	store.Set("short1", "value1", &shortTTL)
	store.Set("short2", "value2", &shortTTL)
	store.Set("long1", "value3", &longTTL)
	store.Set("permanent", "value4", nil)

	assert.Equal(t, int64(4), store.GetKeyCount())

	// Wait for short TTL to expire
	time.Sleep(100 * time.Millisecond)

	// Cleanup expired keys
	cleaned := store.CleanupExpired()
	assert.Equal(t, int64(2), cleaned)
	assert.Equal(t, int64(2), store.GetKeyCount())

	// Verify remaining keys
	keys := store.ListKeys()
	assert.Contains(t, keys, "long1")
	assert.Contains(t, keys, "permanent")
}

func TestStore_ConcurrentAccess(t *testing.T) {
	store := core.NewStore()
	numGoroutines := 10
	numOperations := 100

	// Concurrent writes
	done := make(chan bool, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("key_%d_%d", id, j)
				value := fmt.Sprintf("value_%d_%d", id, j)
				store.Set(key, value, nil)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify all keys were written
	expectedCount := int64(numGoroutines * numOperations)
	assert.Equal(t, expectedCount, store.GetKeyCount())

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("key_%d_%d", id, j)
				expectedValue := fmt.Sprintf("value_%d_%d", id, j)

				value, exists, err := store.Get(key)
				assert.NoError(t, err)
				assert.True(t, exists)
				assert.Equal(t, expectedValue, value)
			}
			done <- true
		}(i)
	}

	// Wait for all read goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

func TestStore_Snapshot(t *testing.T) {
	store := core.NewStore()

	// Add test data
	testData := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
		"key3": true,
	}

	for k, v := range testData {
		err := store.Set(k, v, nil)
		require.NoError(t, err)
	}

	// Get snapshot
	snapshot, err := store.GetSnapshot()
	require.NoError(t, err)
	assert.Len(t, snapshot, 3)

	// Create new store and load snapshot
	newStore := core.NewStore()
	err = newStore.LoadSnapshot(snapshot)
	require.NoError(t, err)

	// Verify data
	assert.Equal(t, store.GetKeyCount(), newStore.GetKeyCount())
	for k, v := range testData {
		value, exists, err := newStore.Get(k)
		require.NoError(t, err)
		assert.True(t, exists)
		assert.Equal(t, v, value)
	}
}

func TestStore_ErrorCases(t *testing.T) {
	store := core.NewStore()

	// Test empty key
	err := store.Set("", "value", nil)
	assert.Error(t, err)

	_, _, err = store.Get("")
	assert.Error(t, err)

	deleted := store.Delete("")
	assert.False(t, deleted)
}

func BenchmarkStore_Set(b *testing.B) {
	store := core.NewStore()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key_%d", i)
		store.Set(key, "benchmark_value", nil)
	}
}

func BenchmarkStore_Get(b *testing.B) {
	store := core.NewStore()

	// Pre-populate store
	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("key_%d", i)
		store.Set(key, "benchmark_value", nil)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key_%d", i%10000)
		store.Get(key)
	}
}

func BenchmarkStore_ConcurrentSet(b *testing.B) {
	store := core.NewStore()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key_%d", i)
			store.Set(key, "concurrent_value", nil)
			i++
		}
	})
}

func BenchmarkStore_ConcurrentGet(b *testing.B) {
	store := core.NewStore()

	// Pre-populate store
	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("key_%d", i)
		store.Set(key, "benchmark_value", nil)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key_%d", i%10000)
			store.Get(key)
			i++
		}
	})
}
