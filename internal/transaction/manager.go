package transaction

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/dark-elixir/mukhostho-db/internal/core"
	"github.com/dark-elixir/mukhostho-db/internal/persistence"
)

// TransactionState represents the state of a transaction
type TransactionState int

const (
	TransactionActive TransactionState = iota
	TransactionCommitted
	TransactionAborted
)

// Operation represents a single operation within a transaction
type Operation struct {
	Type     string      `json:"type"` // "SET", "DELETE"
	Key      string      `json:"key"`
	Value    interface{} `json:"value,omitempty"`
	TTL      *time.Time  `json:"ttl,omitempty"`
	OldValue interface{} `json:"old_value,omitempty"` // For rollback
	Existed  bool        `json:"existed"`             // Whether key existed before operation
}

// Transaction represents an ACID transaction
type Transaction struct {
	ID         string           `json:"id"`
	State      TransactionState `json:"state"`
	Operations []Operation      `json:"operations"`
	StartTime  time.Time        `json:"start_time"`
	CommitTime *time.Time       `json:"commit_time,omitempty"`
	Timeout    time.Duration    `json:"timeout"`
	mu         sync.RWMutex
}

// TransactionManager manages database transactions
type TransactionManager struct {
	store          *core.Store
	walManager     *persistence.WALManager
	transactions   sync.Map // map[string]*Transaction
	locks          sync.Map // map[string]*sync.RWMutex (key-level locks)
	mu             sync.RWMutex
	nextTxnID      int64
	defaultTimeout time.Duration
}

// NewTransactionManager creates a new transaction manager
func NewTransactionManager(store *core.Store, walManager *persistence.WALManager) *TransactionManager {
	return &TransactionManager{
		store:          store,
		walManager:     walManager,
		defaultTimeout: 30 * time.Second,
	}
}

// BeginTransaction starts a new transaction
func (tm *TransactionManager) BeginTransaction() (*Transaction, error) {
	tm.mu.Lock()
	tm.nextTxnID++
	txnID := fmt.Sprintf("txn-%d-%d", time.Now().Unix(), tm.nextTxnID)
	tm.mu.Unlock()

	txn := &Transaction{
		ID:         txnID,
		State:      TransactionActive,
		Operations: make([]Operation, 0),
		StartTime:  time.Now(),
		Timeout:    tm.defaultTimeout,
	}

	tm.transactions.Store(txnID, txn)

	// Set timeout for automatic rollback
	go tm.timeoutTransaction(txn)

	return txn, nil
}

// CommitTransaction commits all operations in a transaction
func (tm *TransactionManager) CommitTransaction(txnID string) error {
	txnInterface, exists := tm.transactions.Load(txnID)
	if !exists {
		return errors.New("transaction not found")
	}

	txn := txnInterface.(*Transaction)
	txn.mu.Lock()
	defer txn.mu.Unlock()

	if txn.State != TransactionActive {
		return fmt.Errorf("transaction is not active (state: %v)", txn.State)
	}

	// Acquire all necessary locks in sorted order to prevent deadlocks
	lockedKeys := tm.acquireLocksForTransaction(txn)
	defer tm.releaseLocksForTransaction(lockedKeys)

	// Apply all operations atomically
	for _, op := range txn.Operations {
		switch op.Type {
		case "SET":
			var ttl *time.Duration
			if op.TTL != nil {
				duration := op.TTL.Sub(time.Now())
				ttl = &duration
			}

			if err := tm.store.Set(op.Key, op.Value, ttl); err != nil {
				// Rollback on error
				tm.rollbackOperations(txn.Operations[:len(txn.Operations)])
				return fmt.Errorf("failed to apply SET operation: %v", err)
			}

			// Log to WAL
			if tm.walManager != nil {
				if err := tm.walManager.LogSet(op.Key, op.Value, op.TTL); err != nil {
					// Rollback on WAL error
					tm.rollbackOperations(txn.Operations[:len(txn.Operations)])
					return fmt.Errorf("failed to log SET operation: %v", err)
				}
			}

		case "DELETE":
			if !tm.store.Delete(op.Key) && op.Existed {
				// Key should have existed but doesn't - data inconsistency
				return fmt.Errorf("data inconsistency: key %s should exist", op.Key)
			}

			// Log to WAL
			if tm.walManager != nil {
				if err := tm.walManager.LogDelete(op.Key); err != nil {
					// Rollback on WAL error
					tm.rollbackOperations(txn.Operations[:len(txn.Operations)])
					return fmt.Errorf("failed to log DELETE operation: %v", err)
				}
			}
		}
	}

	// Mark transaction as committed
	now := time.Now()
	txn.CommitTime = &now
	txn.State = TransactionCommitted

	// Clean up
	tm.transactions.Delete(txnID)

	return nil
}

// RollbackTransaction rolls back all operations in a transaction
func (tm *TransactionManager) RollbackTransaction(txnID string) error {
	txnInterface, exists := tm.transactions.Load(txnID)
	if !exists {
		return errors.New("transaction not found")
	}

	txn := txnInterface.(*Transaction)
	txn.mu.Lock()
	defer txn.mu.Unlock()

	if txn.State != TransactionActive {
		return fmt.Errorf("transaction is not active (state: %v)", txn.State)
	}

	// Acquire locks for rollback
	lockedKeys := tm.acquireLocksForTransaction(txn)
	defer tm.releaseLocksForTransaction(lockedKeys)

	// Rollback operations
	if err := tm.rollbackOperations(txn.Operations); err != nil {
		return fmt.Errorf("failed to rollback operations: %v", err)
	}

	// Mark transaction as aborted
	txn.State = TransactionAborted

	// Clean up
	tm.transactions.Delete(txnID)

	return nil
}

// Set adds a SET operation to a transaction
func (tm *TransactionManager) Set(txnID string, key string, value interface{}, ttl *time.Duration) error {
	txnInterface, exists := tm.transactions.Load(txnID)
	if !exists {
		return errors.New("transaction not found")
	}

	txn := txnInterface.(*Transaction)
	txn.mu.Lock()
	defer txn.mu.Unlock()

	if txn.State != TransactionActive {
		return errors.New("transaction is not active")
	}

	// Get old value for potential rollback
	oldValue, existed, _ := tm.store.Get(key)

	var ttlTime *time.Time
	if ttl != nil {
		t := time.Now().Add(*ttl)
		ttlTime = &t
	}

	operation := Operation{
		Type:     "SET",
		Key:      key,
		Value:    value,
		TTL:      ttlTime,
		OldValue: oldValue,
		Existed:  existed,
	}

	txn.Operations = append(txn.Operations, operation)
	return nil
}

// Delete adds a DELETE operation to a transaction
func (tm *TransactionManager) Delete(txnID string, key string) error {
	txnInterface, exists := tm.transactions.Load(txnID)
	if !exists {
		return errors.New("transaction not found")
	}

	txn := txnInterface.(*Transaction)
	txn.mu.Lock()
	defer txn.mu.Unlock()

	if txn.State != TransactionActive {
		return errors.New("transaction is not active")
	}

	// Get old value for potential rollback
	oldValue, existed, _ := tm.store.Get(key)

	operation := Operation{
		Type:     "DELETE",
		Key:      key,
		OldValue: oldValue,
		Existed:  existed,
	}

	txn.Operations = append(txn.Operations, operation)
	return nil
}

// GetTransaction returns transaction information
func (tm *TransactionManager) GetTransaction(txnID string) (*Transaction, error) {
	txnInterface, exists := tm.transactions.Load(txnID)
	if !exists {
		return nil, errors.New("transaction not found")
	}

	txn := txnInterface.(*Transaction)
	txn.mu.RLock()
	defer txn.mu.RUnlock()

	// Return a copy to avoid race conditions
	result := &Transaction{
		ID:         txn.ID,
		State:      txn.State,
		Operations: make([]Operation, len(txn.Operations)),
		StartTime:  txn.StartTime,
		CommitTime: txn.CommitTime,
		Timeout:    txn.Timeout,
	}

	copy(result.Operations, txn.Operations)
	return result, nil
}

// ListActiveTransactions returns all active transactions
func (tm *TransactionManager) ListActiveTransactions() []*Transaction {
	var transactions []*Transaction

	tm.transactions.Range(func(key, value interface{}) bool {
		txn := value.(*Transaction)
		txn.mu.RLock()
		if txn.State == TransactionActive {
			// Create a copy
			txnCopy := &Transaction{
				ID:         txn.ID,
				State:      txn.State,
				Operations: make([]Operation, len(txn.Operations)),
				StartTime:  txn.StartTime,
				CommitTime: txn.CommitTime,
				Timeout:    txn.Timeout,
			}
			copy(txnCopy.Operations, txn.Operations)
			transactions = append(transactions, txnCopy)
		}
		txn.mu.RUnlock()
		return true
	})

	return transactions
}

// acquireLocksForTransaction acquires locks for all keys in a transaction
func (tm *TransactionManager) acquireLocksForTransaction(txn *Transaction) []string {
	// Get unique keys and sort them to prevent deadlocks
	keySet := make(map[string]bool)
	for _, op := range txn.Operations {
		keySet[op.Key] = true
	}

	keys := make([]string, 0, len(keySet))
	for key := range keySet {
		keys = append(keys, key)
	}

	// Sort keys to ensure consistent lock ordering
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}

	// Acquire locks
	for _, key := range keys {
		lock := tm.getKeyLock(key)
		lock.Lock()
	}

	return keys
}

// releaseLocksForTransaction releases locks for all keys
func (tm *TransactionManager) releaseLocksForTransaction(keys []string) {
	// Release in reverse order
	for i := len(keys) - 1; i >= 0; i-- {
		lock := tm.getKeyLock(keys[i])
		lock.Unlock()
	}
}

// getKeyLock gets or creates a lock for a specific key
func (tm *TransactionManager) getKeyLock(key string) *sync.RWMutex {
	if lock, exists := tm.locks.Load(key); exists {
		return lock.(*sync.RWMutex)
	}

	newLock := &sync.RWMutex{}
	actual, _ := tm.locks.LoadOrStore(key, newLock)
	return actual.(*sync.RWMutex)
}

// rollbackOperations reverses the effects of operations
func (tm *TransactionManager) rollbackOperations(operations []Operation) error {
	// Apply operations in reverse order
	for i := len(operations) - 1; i >= 0; i-- {
		op := operations[i]
		switch op.Type {
		case "SET":
			if op.Existed {
				// Restore old value
				tm.store.Set(op.Key, op.OldValue, nil)
			} else {
				// Delete the key that was added
				tm.store.Delete(op.Key)
			}
		case "DELETE":
			if op.Existed {
				// Restore the deleted key
				tm.store.Set(op.Key, op.OldValue, nil)
			}
		}
	}
	return nil
}

// timeoutTransaction automatically rolls back a transaction after timeout
func (tm *TransactionManager) timeoutTransaction(txn *Transaction) {
	time.Sleep(txn.Timeout)

	// Check if transaction is still active
	if txnInterface, exists := tm.transactions.Load(txn.ID); exists {
		activeTxn := txnInterface.(*Transaction)
		activeTxn.mu.Lock()
		if activeTxn.State == TransactionActive {
			activeTxn.mu.Unlock()
			fmt.Printf("Transaction %s timed out, rolling back\n", txn.ID)
			tm.RollbackTransaction(txn.ID)
		} else {
			activeTxn.mu.Unlock()
		}
	}
}

// CleanupCompletedTransactions removes completed transactions from memory
func (tm *TransactionManager) CleanupCompletedTransactions(maxAge time.Duration) {
	cutoff := time.Now().Add(-maxAge)

	tm.transactions.Range(func(key, value interface{}) bool {
		txn := value.(*Transaction)
		txn.mu.RLock()
		shouldRemove := txn.State != TransactionActive && txn.StartTime.Before(cutoff)
		txn.mu.RUnlock()

		if shouldRemove {
			tm.transactions.Delete(key)
		}
		return true
	})
}
