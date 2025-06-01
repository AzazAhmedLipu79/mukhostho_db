package persistence

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dark-elixir/mukhostho-db/internal/core"
)

// Snapshot represents a point-in-time snapshot of the database
type Snapshot struct {
	Timestamp time.Time              `json:"timestamp"`
	Version   string                 `json:"version"`
	KeyCount  int64                  `json:"key_count"`
	Data      map[string]*core.Value `json:"data"`
	Metadata  SnapshotMetadata       `json:"metadata"`
}

// SnapshotMetadata contains additional information about the snapshot
type SnapshotMetadata struct {
	TotalSize   int64  `json:"total_size"`
	Compression string `json:"compression"`
	CreatedBy   string `json:"created_by"`
	Description string `json:"description"`
}

// SnapshotManager handles creating and restoring database snapshots
type SnapshotManager struct {
	snapshotDir     string
	compressionType string
	mu              sync.RWMutex
}

// NewSnapshotManager creates a new snapshot manager
func NewSnapshotManager(snapshotDir string, compressionType string) (*SnapshotManager, error) {
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create snapshot directory: %v", err)
	}

	return &SnapshotManager{
		snapshotDir:     snapshotDir,
		compressionType: compressionType,
	}, nil
}

// CreateSnapshot creates a new snapshot of the store
func (sm *SnapshotManager) CreateSnapshot(store *core.Store, description string) (string, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Get store data
	data, err := store.GetSnapshot()
	if err != nil {
		return "", fmt.Errorf("failed to get store snapshot: %v", err)
	}

	// Create snapshot
	snapshot := &Snapshot{
		Timestamp: time.Now(),
		Version:   "1.0.0",
		KeyCount:  store.GetKeyCount(),
		Data:      data,
		Metadata: SnapshotMetadata{
			TotalSize:   store.GetMemoryUsage(),
			Compression: sm.compressionType,
			CreatedBy:   "mukhostho-db",
			Description: description,
		},
	}
	// Generate filename with nanosecond precision to avoid collisions
	filename := fmt.Sprintf("snapshot-%d.json", time.Now().UnixNano())
	if sm.compressionType == "gzip" {
		filename += ".gz"
	}

	filepath := filepath.Join(sm.snapshotDir, filename)

	// Write snapshot to file
	if err := sm.writeSnapshot(snapshot, filepath); err != nil {
		return "", fmt.Errorf("failed to write snapshot: %v", err)
	}

	return filepath, nil
}

// writeSnapshot writes a snapshot to file with optional compression
func (sm *SnapshotManager) writeSnapshot(snapshot *Snapshot, filepath string) error {
	file, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create snapshot file: %v", err)
	}
	defer file.Close()

	var writer interface {
		Write([]byte) (int, error)
	} = file

	// Apply compression if enabled
	if sm.compressionType == "gzip" {
		gzipWriter := gzip.NewWriter(file)
		defer gzipWriter.Close()
		writer = gzipWriter
	}

	// Encode JSON
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(snapshot); err != nil {
		return fmt.Errorf("failed to encode snapshot: %v", err)
	}

	return nil
}

// RestoreSnapshot restores the store from a snapshot file
func (sm *SnapshotManager) RestoreSnapshot(store *core.Store, filepath string) error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	file, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf("failed to open snapshot file: %v", err)
	}
	defer file.Close()

	var reader interface {
		Read([]byte) (int, error)
	} = file

	// Handle compression
	if strings.HasSuffix(filepath, ".gz") {
		gzipReader, err := gzip.NewReader(file)
		if err != nil {
			return fmt.Errorf("failed to create gzip reader: %v", err)
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	// Decode snapshot
	var snapshot Snapshot
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(&snapshot); err != nil {
		return fmt.Errorf("failed to decode snapshot: %v", err)
	}

	// Validate snapshot
	if err := sm.validateSnapshot(&snapshot); err != nil {
		return fmt.Errorf("invalid snapshot: %v", err)
	}

	// Load data into store
	if err := store.LoadSnapshot(snapshot.Data); err != nil {
		return fmt.Errorf("failed to load snapshot data: %v", err)
	}

	fmt.Printf("Restored snapshot from %s: %d keys, %d bytes\n",
		snapshot.Timestamp.Format(time.RFC3339),
		snapshot.KeyCount,
		snapshot.Metadata.TotalSize)

	return nil
}

// validateSnapshot performs basic validation on a snapshot
func (sm *SnapshotManager) validateSnapshot(snapshot *Snapshot) error {
	if snapshot.Version == "" {
		return fmt.Errorf("missing snapshot version")
	}

	if snapshot.Data == nil {
		return fmt.Errorf("missing snapshot data")
	}

	if int64(len(snapshot.Data)) != snapshot.KeyCount {
		return fmt.Errorf("key count mismatch: expected %d, got %d",
			snapshot.KeyCount, len(snapshot.Data))
	}

	return nil
}

// ListSnapshots returns a list of available snapshots
func (sm *SnapshotManager) ListSnapshots() ([]SnapshotInfo, error) {
	files, err := os.ReadDir(sm.snapshotDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read snapshot directory: %v", err)
	}

	var snapshots []SnapshotInfo
	for _, file := range files {
		if file.IsDir() || !strings.HasPrefix(file.Name(), "snapshot-") {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		snapshots = append(snapshots, SnapshotInfo{
			Filename:  file.Name(),
			Path:      filepath.Join(sm.snapshotDir, file.Name()),
			Size:      info.Size(),
			CreatedAt: info.ModTime(),
		})
	}

	// Sort by creation time (newest first)
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt.After(snapshots[j].CreatedAt)
	})

	return snapshots, nil
}

// SnapshotInfo contains information about a snapshot file
type SnapshotInfo struct {
	Filename  string    `json:"filename"`
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}

// GetLatestSnapshot returns the path to the most recent snapshot
func (sm *SnapshotManager) GetLatestSnapshot() (string, error) {
	snapshots, err := sm.ListSnapshots()
	if err != nil {
		return "", err
	}

	if len(snapshots) == 0 {
		return "", fmt.Errorf("no snapshots found")
	}

	return snapshots[0].Path, nil
}

// CleanupOldSnapshots removes old snapshots based on age and count limits
func (sm *SnapshotManager) CleanupOldSnapshots(maxAge time.Duration, keepMinimum int) error {
	snapshots, err := sm.ListSnapshots()
	if err != nil {
		return err
	}

	if len(snapshots) <= keepMinimum {
		return nil // Don't delete if we have fewer than minimum
	}

	deleted := 0

	// Process snapshots (they are sorted newest first)
	for i, snapshot := range snapshots {
		// Always keep the first keepMinimum snapshots (most recent)
		if i < keepMinimum {
			continue
		}

		// For snapshots beyond the keepMinimum, delete them
		if err := os.Remove(snapshot.Path); err != nil {
			fmt.Printf("Failed to delete snapshot %s: %v\n", snapshot.Filename, err)
			continue
		}
		deleted++
	}

	fmt.Printf("Cleaned up %d old snapshots\n", deleted)
	return nil
}

// GetSnapshotInfo reads metadata from a snapshot file without loading all data
func (sm *SnapshotManager) GetSnapshotInfo(filepath string) (*SnapshotMetadata, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open snapshot file: %v", err)
	}
	defer file.Close()

	var reader interface {
		Read([]byte) (int, error)
	} = file

	if strings.HasSuffix(filepath, ".gz") {
		gzipReader, err := gzip.NewReader(file)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %v", err)
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	// Read only the metadata part
	var partial struct {
		Metadata SnapshotMetadata `json:"metadata"`
	}

	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(&partial); err != nil {
		return nil, fmt.Errorf("failed to decode snapshot metadata: %v", err)
	}

	return &partial.Metadata, nil
}
