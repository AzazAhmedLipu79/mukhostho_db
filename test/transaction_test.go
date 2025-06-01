package test

import (
	"testing"

	"github.com/dark-elixir/mukhostho-db/internal/core"
	"github.com/dark-elixir/mukhostho-db/internal/persistence"
	"github.com/dark-elixir/mukhostho-db/internal/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test function for transaction delete operations
func TestTransactionDelete(t *testing.T) {
	store := core.NewStore()
	walManager, err := persistence.NewWALManager(t.TempDir(), "always")
	require.NoError(t, err)
	defer walManager.Close()

	manager := transaction.NewTransactionManager(store, walManager)
	// Set up initial state
	err = store.Set("delete_test1", "value1", nil)
	require.NoError(t, err)
	err = store.Set("delete_test2", "value2", nil)
	require.NoError(t, err)

	// Begin transaction
	txn, err := manager.BeginTransaction()
	require.NoError(t, err)

	// Delete one key
	err = manager.Delete(txn.ID, "delete_test1")
	require.NoError(t, err)

	// Add a new key
	err = manager.Set(txn.ID, "delete_test3", "value3", nil)
	require.NoError(t, err)

	// Commit transaction
	err = manager.CommitTransaction(txn.ID)
	require.NoError(t, err)

	// Verify final state
	_, exists, err := store.Get("delete_test1")
	require.NoError(t, err)
	assert.False(t, exists, "Deleted key should not exist after commit")

	_, exists, err = store.Get("delete_test2")
	require.NoError(t, err)
	assert.True(t, exists, "Unmodified key should still exist")

	value, exists, err := store.Get("delete_test3")
	require.NoError(t, err)
	assert.True(t, exists, "New key should exist after commit")
	assert.Equal(t, "value3", value)
}
