package test

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dark-elixir/mukhostho-db/internal/config"
	"github.com/dark-elixir/mukhostho-db/internal/core"
	"github.com/dark-elixir/mukhostho-db/internal/memory"
	"github.com/dark-elixir/mukhostho-db/internal/persistence"
	"github.com/dark-elixir/mukhostho-db/internal/transaction"
)

func setupTestComponents(t *testing.T) (*core.Store, *memory.MemoryManager, *persistence.WALManager, *persistence.SnapshotManager, *transaction.TransactionManager) {
	// Create temporary directories
	tempDir := t.TempDir()
	walDir := filepath.Join(tempDir, "wal")
	snapshotDir := filepath.Join(tempDir, "snapshots")

	cfg := &config.Config{
		Memory: config.MemoryConfig{
			MaxMemoryMB:     100, // 100MB
			EvictionPolicy:  "lru",
			TTLCleanupDelay: 1 * time.Minute,
			GCInterval:      5 * time.Minute,
		},
		Persistence: config.PersistenceConfig{
			DataDir:          tempDir,
			WALDir:           walDir,
			SnapshotDir:      snapshotDir,
			BackupDir:        filepath.Join(tempDir, "backups"),
			SnapshotInterval: 10 * time.Minute,
			WALSyncPolicy:    "always",
			CompressionType:  "gzip",
		},
	}

	store := core.NewStore()
	memManager := memory.NewMemoryManager(store, &cfg.Memory)

	walManager, err := persistence.NewWALManager(cfg.Persistence.WALDir, cfg.Persistence.WALSyncPolicy)
	require.NoError(t, err)

	snapshotManager, err := persistence.NewSnapshotManager(cfg.Persistence.SnapshotDir, cfg.Persistence.CompressionType)
	require.NoError(t, err)

	txnManager := transaction.NewTransactionManager(store, walManager)

	return store, memManager, walManager, snapshotManager, txnManager
}

func TestFullSystemIntegration(t *testing.T) {
	store, memManager, walManager, snapshotManager, txnManager := setupTestComponents(t)

	// Start memory manager
	memManager.Start()
	defer memManager.Stop()

	// Test basic operations
	t.Run("Basic CRUD operations", func(t *testing.T) {
		// Set operation
		err := store.Set("test_key", "test_value", nil)
		require.NoError(t, err)

		// Get operation
		value, exists, err := store.Get("test_key")
		require.NoError(t, err)
		assert.True(t, exists)
		assert.Equal(t, "test_value", value)

		// Delete operation
		deleted := store.Delete("test_key")
		assert.True(t, deleted)

		// Verify deletion
		_, exists, err = store.Get("test_key")
		require.NoError(t, err)
		assert.False(t, exists)
	})

	// Test TTL operations
	t.Run("TTL operations", func(t *testing.T) {
		ttl := 100 * time.Millisecond
		err := store.Set("ttl_key", "ttl_value", &ttl)
		require.NoError(t, err)

		// Key should exist initially
		value, exists, err := store.Get("ttl_key")
		require.NoError(t, err)
		assert.True(t, exists)
		assert.Equal(t, "ttl_value", value)

		// Wait for expiration
		time.Sleep(150 * time.Millisecond)

		// Key should be expired
		_, exists, err = store.Get("ttl_key")
		require.NoError(t, err)
		assert.False(t, exists)
	})

	// Test persistence operations
	t.Run("Persistence operations", func(t *testing.T) {
		// Add some data
		err := store.Set("persist_key1", "persist_value1", nil)
		require.NoError(t, err)

		err = store.Set("persist_key2", "persist_value2", nil)
		require.NoError(t, err)

		// Create snapshot
		snapshotPath, err := snapshotManager.CreateSnapshot(store, "Test snapshot")
		require.NoError(t, err)
		assert.NotEmpty(t, snapshotPath)

		// Verify snapshot can be restored
		newStore := core.NewStore()
		err = snapshotManager.RestoreSnapshot(newStore, snapshotPath)
		require.NoError(t, err)

		// Verify data was restored
		value, exists, err := newStore.Get("persist_key1")
		require.NoError(t, err)
		assert.True(t, exists)
		assert.Equal(t, "persist_value1", value)

		value, exists, err = newStore.Get("persist_key2")
		require.NoError(t, err)
		assert.True(t, exists)
		assert.Equal(t, "persist_value2", value)
	})

	// Test transaction operations
	t.Run("Transaction operations", func(t *testing.T) {
		// Begin transaction
		txn, err := txnManager.BeginTransaction()
		require.NoError(t, err)
		// Add operations to transaction
		err = txnManager.Set(txn.ID, "txn_key1", "txn_value1", nil)
		require.NoError(t, err)

		err = txnManager.Set(txn.ID, "txn_key2", "txn_value2", nil)
		require.NoError(t, err)

		// Values should not be visible before commit
		_, exists, err := store.Get("txn_key1")
		require.NoError(t, err)
		assert.False(t, exists)

		// Commit transaction
		err = txnManager.CommitTransaction(txn.ID)
		require.NoError(t, err)

		// Values should now be visible
		value, exists, err := store.Get("txn_key1")
		require.NoError(t, err)
		assert.True(t, exists)
		assert.Equal(t, "txn_value1", value)

		value, exists, err = store.Get("txn_key2")
		require.NoError(t, err)
		assert.True(t, exists)
		assert.Equal(t, "txn_value2", value)
	})

	// Test transaction rollback
	t.Run("Transaction rollback", func(t *testing.T) {
		// Begin transaction
		txn, err := txnManager.BeginTransaction()
		require.NoError(t, err)
		// Add operations to transaction
		err = txnManager.Set(txn.ID, "rollback_key", "rollback_value", nil)
		require.NoError(t, err)

		// Rollback transaction
		err = txnManager.RollbackTransaction(txn.ID)
		require.NoError(t, err)

		// Value should not be visible after rollback
		_, exists, err := store.Get("rollback_key")
		require.NoError(t, err)
		assert.False(t, exists)
	})

	// Test memory management
	t.Run("Memory management", func(t *testing.T) {
		initialCount := store.GetKeyCount()
		initialMemory := store.GetMemoryUsage()

		// Add some data
		for i := 0; i < 100; i++ {
			key := fmt.Sprintf("mem_key_%d", i)
			value := fmt.Sprintf("mem_value_%d", i)
			err := store.Set(key, value, nil)
			require.NoError(t, err)
		}

		// Verify memory usage increased
		assert.Greater(t, store.GetKeyCount(), initialCount)
		assert.Greater(t, store.GetMemoryUsage(), initialMemory)

		// Test memory stats
		stats := memManager.GetMemoryStats()
		assert.NotNil(t, stats)
		assert.Greater(t, stats.KeyCount, initialCount)
		assert.Greater(t, stats.CurrentMemoryBytes, initialMemory)
	})

	// Clean up
	walManager.Close()
}

func TestConcurrentOperations(t *testing.T) {
	store, memManager, walManager, _, _ := setupTestComponents(t)

	memManager.Start()
	defer memManager.Stop()
	defer walManager.Close()

	const numGoroutines = 10
	const numOperations = 100

	t.Run("Concurrent SET operations", func(t *testing.T) {
		done := make(chan bool, numGoroutines)

		for g := 0; g < numGoroutines; g++ {
			go func(goroutineID int) {
				defer func() { done <- true }()

				for i := 0; i < numOperations; i++ {
					key := fmt.Sprintf("concurrent_key_%d_%d", goroutineID, i)
					value := fmt.Sprintf("concurrent_value_%d_%d", goroutineID, i)

					err := store.Set(key, value, nil)
					if err != nil {
						t.Errorf("Failed to set key %s: %v", key, err)
						return
					}
				}
			}(g)
		}

		// Wait for all goroutines to complete
		for i := 0; i < numGoroutines; i++ {
			<-done
		}

		// Verify all keys were set correctly
		for g := 0; g < numGoroutines; g++ {
			for i := 0; i < numOperations; i++ {
				key := fmt.Sprintf("concurrent_key_%d_%d", g, i)
				expectedValue := fmt.Sprintf("concurrent_value_%d_%d", g, i)

				value, exists, err := store.Get(key)
				require.NoError(t, err)
				assert.True(t, exists, "Key %s should exist", key)
				assert.Equal(t, expectedValue, value, "Value for key %s should match", key)
			}
		}
	})
}

func TestMemoryPressure(t *testing.T) {
	store, memManager, walManager, _, _ := setupTestComponents(t)

	// Set a very low memory limit for testing
	memManager.SetMemoryLimit(1) // 1MB

	memManager.Start()
	defer memManager.Stop()
	defer walManager.Close()

	t.Run("Memory eviction under pressure", func(t *testing.T) {
		// Add data until we exceed memory limit
		for i := 0; i < 1000; i++ {
			key := fmt.Sprintf("pressure_key_%d", i)
			// Create a large value
			value := string(make([]byte, 1024)) // 1KB value

			err := store.Set(key, value, nil)
			if err != nil {
				// This is expected when memory limit is exceeded
				break
			}
		}

		// Verify memory manager is functioning
		stats := memManager.GetMemoryStats()
		assert.NotNil(t, stats)

		// Memory pressure should trigger evictions
		assert.True(t, memManager.IsMemoryPressure())
	})
}
