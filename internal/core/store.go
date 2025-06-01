package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// DataType represents the type of value stored
type DataType int

const (
	StringType DataType = iota
	IntType
	FloatType
	BoolType
	ObjectType
)

// Value represents a stored value with metadata
type Value struct {
	Data       interface{} `json:"data"`
	Type       DataType    `json:"type"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
	AccessedAt time.Time   `json:"accessed_at"`
	TTL        *time.Time  `json:"ttl,omitempty"`
	Size       int64       `json:"size"`
}

// UnmarshalJSON implements custom JSON unmarshaling to preserve data types
func (v *Value) UnmarshalJSON(data []byte) error {
	// Use a temporary struct to unmarshal the basic fields
	var temp struct {
		Data       interface{} `json:"data"`
		Type       DataType    `json:"type"`
		CreatedAt  time.Time   `json:"created_at"`
		UpdatedAt  time.Time   `json:"updated_at"`
		AccessedAt time.Time   `json:"accessed_at"`
		TTL        *time.Time  `json:"ttl,omitempty"`
		Size       int64       `json:"size"`
	}

	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	// Copy all fields
	v.Type = temp.Type
	v.CreatedAt = temp.CreatedAt
	v.UpdatedAt = temp.UpdatedAt
	v.AccessedAt = temp.AccessedAt
	v.TTL = temp.TTL
	v.Size = temp.Size

	// Convert data to the correct type based on stored type
	switch v.Type {
	case StringType:
		if str, ok := temp.Data.(string); ok {
			v.Data = str
		} else {
			return fmt.Errorf("expected string data for StringType, got %T", temp.Data)
		}
	case IntType:
		// JSON unmarshals numbers as float64, convert to int
		if f, ok := temp.Data.(float64); ok {
			v.Data = int(f)
		} else if i, ok := temp.Data.(int); ok {
			v.Data = i
		} else {
			return fmt.Errorf("expected numeric data for IntType, got %T", temp.Data)
		}
	case FloatType:
		if f, ok := temp.Data.(float64); ok {
			v.Data = f
		} else if f32, ok := temp.Data.(float32); ok {
			v.Data = float64(f32)
		} else {
			return fmt.Errorf("expected numeric data for FloatType, got %T", temp.Data)
		}
	case BoolType:
		if b, ok := temp.Data.(bool); ok {
			v.Data = b
		} else {
			return fmt.Errorf("expected boolean data for BoolType, got %T", temp.Data)
		}
	case ObjectType:
		// Keep as-is for complex objects
		v.Data = temp.Data
	default:
		return fmt.Errorf("unknown data type: %d", v.Type)
	}

	return nil
}

// IsExpired checks if the value has expired
func (v *Value) IsExpired() bool {
	if v.TTL == nil {
		return false
	}
	return time.Now().After(*v.TTL)
}

// UpdateAccess updates the last accessed time
func (v *Value) UpdateAccess() {
	v.AccessedAt = time.Now()
}

// Store represents the main in-memory data store
type Store struct {
	data      sync.Map     // Key -> *Value
	metadata  sync.Map     // Key -> *Metadata
	keyOrder  *LRUCache    // For LRU eviction
	keyCount  int64        // Total number of keys
	memoryUse int64        // Current memory usage in bytes
	mu        sync.RWMutex // Global lock for complex operations
}

// Metadata holds additional information about keys
type Metadata struct {
	Key         string    `json:"key"`
	Size        int64     `json:"size"`
	CreatedAt   time.Time `json:"created_at"`
	AccessedAt  time.Time `json:"accessed_at"`
	AccessCount int64     `json:"access_count"`
}

// NewStore creates a new instance of the store
func NewStore() *Store {
	return &Store{
		keyOrder: NewLRUCache(),
	}
}

// Set stores a key-value pair with optional TTL
func (s *Store) Set(key string, value interface{}, ttl *time.Duration) error {
	if key == "" {
		return errors.New("key cannot be empty")
	}

	val := &Value{
		Data:       value,
		Type:       inferType(value),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		AccessedAt: time.Now(),
		Size:       calculateSize(value),
	}

	if ttl != nil {
		expiry := time.Now().Add(*ttl)
		val.TTL = &expiry
	}

	// Check if key already exists
	if existing, exists := s.data.Load(key); exists {
		existingVal := existing.(*Value)
		s.memoryUse -= existingVal.Size
	} else {
		s.keyCount++
	}

	// Store the value
	s.data.Store(key, val)
	s.memoryUse += val.Size

	// Update LRU cache
	s.keyOrder.Put(key, val)

	// Update metadata
	meta := &Metadata{
		Key:         key,
		Size:        val.Size,
		CreatedAt:   val.CreatedAt,
		AccessedAt:  val.AccessedAt,
		AccessCount: 1,
	}
	s.metadata.Store(key, meta)

	return nil
}

// Get retrieves a value by key
func (s *Store) Get(key string) (interface{}, bool, error) {
	if key == "" {
		return nil, false, errors.New("key cannot be empty")
	}

	value, exists := s.data.Load(key)
	if !exists {
		return nil, false, nil
	}

	val := value.(*Value)

	// Check if expired
	if val.IsExpired() {
		s.Delete(key)
		return nil, false, nil
	}

	// Update access information
	val.UpdateAccess()
	s.keyOrder.Get(key) // Update LRU position

	// Update metadata
	if meta, exists := s.metadata.Load(key); exists {
		metadata := meta.(*Metadata)
		metadata.AccessedAt = time.Now()
		metadata.AccessCount++
	}

	return val.Data, true, nil
}

// Delete removes a key-value pair
func (s *Store) Delete(key string) bool {
	if key == "" {
		return false
	}

	value, exists := s.data.Load(key)
	if !exists {
		return false
	}

	val := value.(*Value)
	s.data.Delete(key)
	s.metadata.Delete(key)
	s.keyOrder.Remove(key)

	s.memoryUse -= val.Size
	s.keyCount--

	return true
}

// ListKeys returns all keys in the store
func (s *Store) ListKeys() []string {
	var keys []string
	s.data.Range(func(key, value interface{}) bool {
		k := key.(string)
		val := value.(*Value)

		// Skip expired keys
		if !val.IsExpired() {
			keys = append(keys, k)
		}
		return true
	})
	return keys
}

// ListItems returns all key-value pairs
func (s *Store) ListItems() map[string]interface{} {
	items := make(map[string]interface{})
	s.data.Range(func(key, value interface{}) bool {
		k := key.(string)
		val := value.(*Value)

		// Skip expired keys
		if !val.IsExpired() {
			items[k] = val.Data
		}
		return true
	})
	return items
}

// GetMemoryUsage returns current memory usage in bytes
func (s *Store) GetMemoryUsage() int64 {
	return s.memoryUse
}

// GetKeyCount returns the number of keys
func (s *Store) GetKeyCount() int64 {
	return s.keyCount
}

// CleanupExpired removes all expired keys
func (s *Store) CleanupExpired() int64 {
	var expiredKeys []string

	s.data.Range(func(key, value interface{}) bool {
		k := key.(string)
		val := value.(*Value)

		if val.IsExpired() {
			expiredKeys = append(expiredKeys, k)
		}
		return true
	})

	for _, key := range expiredKeys {
		s.Delete(key)
	}

	return int64(len(expiredKeys))
}

// EvictLRU evicts the least recently used key
func (s *Store) EvictLRU() (string, bool) {
	key := s.keyOrder.RemoveLRU()
	if key == "" {
		return "", false
	}

	s.Delete(key)
	return key, true
}

// GetSnapshot returns a snapshot of all data for persistence
func (s *Store) GetSnapshot() (map[string]*Value, error) {
	snapshot := make(map[string]*Value)

	s.data.Range(func(key, value interface{}) bool {
		k := key.(string)
		val := value.(*Value)

		// Only include non-expired keys
		if !val.IsExpired() {
			snapshot[k] = val
		}
		return true
	})

	return snapshot, nil
}

// LoadSnapshot loads data from a snapshot
func (s *Store) LoadSnapshot(snapshot map[string]*Value) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear existing data
	s.data = sync.Map{}
	s.metadata = sync.Map{}
	s.keyOrder = NewLRUCache()
	s.keyCount = 0
	s.memoryUse = 0

	// Load snapshot data
	for key, val := range snapshot {
		if !val.IsExpired() {
			s.data.Store(key, val)
			s.keyOrder.Put(key, val)
			s.keyCount++
			s.memoryUse += val.Size

			meta := &Metadata{
				Key:         key,
				Size:        val.Size,
				CreatedAt:   val.CreatedAt,
				AccessedAt:  val.AccessedAt,
				AccessCount: 1,
			}
			s.metadata.Store(key, meta)
		}
	}

	return nil
}

// Helper functions

func inferType(value interface{}) DataType {
	switch value.(type) {
	case string:
		return StringType
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return IntType
	case float32, float64:
		return FloatType
	case bool:
		return BoolType
	default:
		return ObjectType
	}
}

func calculateSize(value interface{}) int64 {
	switch v := value.(type) {
	case string:
		return int64(len(v))
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return 8 // Approximate size
	case float32, float64:
		return 8
	case bool:
		return 1
	default:
		// For complex objects, marshal to JSON to estimate size
		if data, err := json.Marshal(v); err == nil {
			return int64(len(data))
		}
		return 64 // Default estimate
	}
}
