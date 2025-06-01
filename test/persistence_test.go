package test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dark-elixir/mukhostho-db/internal/core"
	"github.com/dark-elixir/mukhostho-db/internal/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWALPersistence(t *testing.T) {
	tempDir := t.TempDir()

	// Create WAL manager
	walManager, err := persistence.NewWALManager(tempDir, "always")
	require.NoError(t, err)
	defer walManager.Close()

	// Log some operations
	err = walManager.LogSet("key1", "value1", nil)
	require.NoError(t, err)

	err = walManager.LogSet("key2", "value2", nil)
	require.NoError(t, err)

	err = walManager.LogDelete("key1")
	require.NoError(t, err)

	// Force sync
	err = walManager.Flush()
	require.NoError(t, err)

	// Close and create new store to test recovery
	walManager.Close()

	// Create new store and replay WAL
	store := core.NewStore()
	newWalManager, err := persistence.NewWALManager(tempDir, "always")
	require.NoError(t, err)
	defer newWalManager.Close()

	// Get WAL files and replay them
	walFiles, err := newWalManager.GetWALFiles()
	require.NoError(t, err)

	for _, walFile := range walFiles {
		err = newWalManager.ReplayWAL(store, walFile)
		require.NoError(t, err)
	}

	// Verify final state
	// key1 should be deleted
	_, exists, err := store.Get("key1")
	require.NoError(t, err)
	assert.False(t, exists, "key1 should be deleted")

	// key2 should exist
	value, exists, err := store.Get("key2")
	require.NoError(t, err)
	assert.True(t, exists, "key2 should exist")
	assert.Equal(t, "value2", value)
}

func TestSnapshotPersistence(t *testing.T) {
	tempDir := t.TempDir()

	// Create store with test data
	store := core.NewStore()
	err := store.Set("key1", "value1", nil)
	require.NoError(t, err)
	err = store.Set("key2", 42, nil)
	require.NoError(t, err)
	err = store.Set("key3", true, nil)
	require.NoError(t, err)

	ttl := time.Hour
	err = store.Set("key4", "value4", &ttl)
	require.NoError(t, err)

	// Create snapshot manager
	snapshotManager, err := persistence.NewSnapshotManager(tempDir, "gzip")
	require.NoError(t, err)
	// Create snapshot
	snapshotPath, err := snapshotManager.CreateSnapshot(store, "test-snapshot")
	require.NoError(t, err)

	// Verify snapshot file exists
	_, err = os.Stat(snapshotPath)
	require.NoError(t, err, "Snapshot file should exist")

	// Create new store and restore from snapshot
	newStore := core.NewStore()
	err = snapshotManager.RestoreSnapshot(newStore, snapshotPath)
	require.NoError(t, err)

	// Verify data
	testCases := []struct {
		key      string
		expected interface{}
	}{
		{"key1", "value1"},
		{"key2", 42},
		{"key3", true},
		{"key4", "value4"},
	}

	for _, tc := range testCases {
		value, exists, err := newStore.Get(tc.key)
		require.NoError(t, err)
		assert.True(t, exists, "Key %s should exist after restore", tc.key)
		assert.Equal(t, tc.expected, value, "Key %s should have correct value", tc.key)
	}
}

func TestSnapshotCleanup(t *testing.T) {
	tempDir := t.TempDir()
	snapshotManager, err := persistence.NewSnapshotManager(tempDir, "gzip")
	require.NoError(t, err)

	store := core.NewStore()

	// Create multiple snapshots
	var snapshots []string
	for i := 0; i < 5; i++ {
		err := store.Set("key", i, nil)
		require.NoError(t, err)

		snapshot, err := snapshotManager.CreateSnapshot(store, "test-snapshot-multiple")
		require.NoError(t, err)

		snapshots = append(snapshots, snapshot)
		// Small delay to ensure different timestamps
		time.Sleep(10 * time.Millisecond)
	}

	// Verify all snapshots exist
	for i, snapshot := range snapshots {
		_, err := os.Stat(snapshot)
		require.NoError(t, err, "Snapshot %d should exist: %s", i, snapshot)
	}
	// Clean up old snapshots (keep only 3, max age 1 hour)
	err = snapshotManager.CleanupOldSnapshots(time.Hour, 3)
	require.NoError(t, err)
	// Check that only the 3 most recent snapshots remain
	files, err := filepath.Glob(filepath.Join(tempDir, "snapshot-*.json.gz"))
	require.NoError(t, err)
	assert.Equal(t, 3, len(files), "Expected 3 snapshots after cleanup")

	// Verify the most recent snapshots still exist
	for i := 2; i < 5; i++ { // Last 3 snapshots
		_, err := os.Stat(snapshots[i])
		assert.NoError(t, err, "Recent snapshot should still exist: %s", snapshots[i])
	}
}

func TestRecoveryFromWALAndSnapshot(t *testing.T) {
	tempDir := t.TempDir()

	// Create initial store and snapshot
	store := core.NewStore()
	err := store.Set("initial_key", "initial_value", nil)
	require.NoError(t, err)

	snapshotManager, err := persistence.NewSnapshotManager(tempDir, "gzip")
	require.NoError(t, err)

	snapshotPath, err := snapshotManager.CreateSnapshot(store, "recovery-test-snapshot")
	require.NoError(t, err)

	// Create WAL manager and log additional operations
	walManager, err := persistence.NewWALManager(tempDir, "always")
	require.NoError(t, err)

	// Log operations that modify the snapshot state
	err = walManager.LogSet("wal_key1", "wal_value1", nil)
	require.NoError(t, err)

	err = walManager.LogSet("wal_key2", "wal_value2", nil)
	require.NoError(t, err)

	err = walManager.LogDelete("initial_key")
	require.NoError(t, err)

	err = walManager.Flush()
	require.NoError(t, err)
	walManager.Close()

	// Create new store and perform recovery
	recoveredStore := core.NewStore()

	// Restore from snapshot first
	err = snapshotManager.RestoreSnapshot(recoveredStore, snapshotPath)
	require.NoError(t, err)

	// Apply WAL operations
	newWalManager, err := persistence.NewWALManager(tempDir, "always")
	require.NoError(t, err)
	defer newWalManager.Close()

	walFiles, err := newWalManager.GetWALFiles()
	require.NoError(t, err)

	for _, walFile := range walFiles {
		err = newWalManager.ReplayWAL(recoveredStore, walFile)
		require.NoError(t, err)
	}

	// Verify recovered state
	// initial_key should be deleted by WAL
	_, exists, err := recoveredStore.Get("initial_key")
	require.NoError(t, err)
	assert.False(t, exists, "initial_key should be deleted after WAL replay")

	// WAL keys should exist
	value1, exists1, err := recoveredStore.Get("wal_key1")
	require.NoError(t, err)
	assert.True(t, exists1, "wal_key1 should exist")
	assert.Equal(t, "wal_value1", value1)

	value2, exists2, err := recoveredStore.Get("wal_key2")
	require.NoError(t, err)
	assert.True(t, exists2, "wal_key2 should exist")
	assert.Equal(t, "wal_value2", value2)
}

func TestWALCorruption(t *testing.T) {
	tempDir := t.TempDir()

	// Create WAL manager and write some data
	walManager, err := persistence.NewWALManager(tempDir, "always")
	require.NoError(t, err)

	// Log valid operations
	err = walManager.LogSet("key1", "value1", nil)
	require.NoError(t, err)

	err = walManager.LogSet("key2", "value2", nil)
	require.NoError(t, err)

	err = walManager.Flush()
	require.NoError(t, err)
	walManager.Close()

	// Get the WAL files to corrupt one
	newWalManager, err := persistence.NewWALManager(tempDir, "always")
	require.NoError(t, err)

	walFiles, err := newWalManager.GetWALFiles()
	require.NoError(t, err)
	require.NotEmpty(t, walFiles, "Should have at least one WAL file")

	newWalManager.Close()

	// Corrupt the WAL file by truncating it
	walFile := walFiles[0]
	file, err := os.OpenFile(walFile, os.O_WRONLY, 0644)
	require.NoError(t, err)

	err = file.Truncate(10) // Truncate to invalid size
	require.NoError(t, err)
	file.Close()

	// Try to replay corrupted WAL - should handle gracefully
	store := core.NewStore()
	corruptWalManager, err := persistence.NewWALManager(tempDir, "always")
	require.NoError(t, err)
	defer corruptWalManager.Close()

	// Replaying corrupted WAL should not crash and may return error
	err = corruptWalManager.ReplayWAL(store, walFile)
	if err != nil {
		t.Logf("Expected error reading corrupted WAL: %v", err)
	}

	// Store should still be functional even after corruption
	err = store.Set("recovery_key", "recovery_value", nil)
	require.NoError(t, err)

	value, exists, err := store.Get("recovery_key")
	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "recovery_value", value)
}
