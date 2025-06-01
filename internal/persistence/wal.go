package persistence

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"sync"
	"time"

	"github.com/dark-elixir/mukhostho-db/internal/core"
)

// WALEntry represents a single entry in the Write-Ahead Log
type WALEntry struct {
	Timestamp time.Time   `json:"timestamp"`
	Operation string      `json:"operation"` // "SET", "DELETE"
	Key       string      `json:"key"`
	Value     interface{} `json:"value,omitempty"`
	TTL       *time.Time  `json:"ttl,omitempty"`
	Checksum  uint32      `json:"checksum"`
}

// WALManager handles Write-Ahead Logging for durability
type WALManager struct {
	walDir     string
	walFile    *os.File
	walWriter  *bufio.Writer
	mu         sync.Mutex
	syncPolicy string // "always", "periodic", "never"
	entryCount int64
}

// NewWALManager creates a new WAL manager
func NewWALManager(walDir string, syncPolicy string) (*WALManager, error) {
	if err := os.MkdirAll(walDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create WAL directory: %v", err)
	}

	walPath := fmt.Sprintf("%s/wal-%d.log", walDir, time.Now().Unix())
	walFile, err := os.OpenFile(walPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open WAL file: %v", err)
	}

	return &WALManager{
		walDir:     walDir,
		walFile:    walFile,
		walWriter:  bufio.NewWriter(walFile),
		syncPolicy: syncPolicy,
	}, nil
}

// LogSet logs a SET operation to WAL
func (w *WALManager) LogSet(key string, value interface{}, ttl *time.Time) error {
	return w.logEntry(&WALEntry{
		Timestamp: time.Now(),
		Operation: "SET",
		Key:       key,
		Value:     value,
		TTL:       ttl,
	})
}

// LogDelete logs a DELETE operation to WAL
func (w *WALManager) LogDelete(key string) error {
	return w.logEntry(&WALEntry{
		Timestamp: time.Now(),
		Operation: "DELETE",
		Key:       key,
	})
}

// logEntry writes a WAL entry to disk
func (w *WALManager) logEntry(entry *WALEntry) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Serialize the entry
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal WAL entry: %v", err)
	}

	// Calculate checksum
	entry.Checksum = crc32.ChecksumIEEE(data)

	// Re-serialize with checksum
	data, err = json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal WAL entry with checksum: %v", err)
	}

	// Write length prefix
	length := uint32(len(data))
	if err := binary.Write(w.walWriter, binary.LittleEndian, length); err != nil {
		return fmt.Errorf("failed to write WAL entry length: %v", err)
	}

	// Write entry data
	if _, err := w.walWriter.Write(data); err != nil {
		return fmt.Errorf("failed to write WAL entry: %v", err)
	}

	// Sync based on policy
	if w.syncPolicy == "always" {
		if err := w.walWriter.Flush(); err != nil {
			return fmt.Errorf("failed to flush WAL: %v", err)
		}
		if err := w.walFile.Sync(); err != nil {
			return fmt.Errorf("failed to sync WAL: %v", err)
		}
	}

	w.entryCount++
	return nil
}

// Flush ensures all WAL entries are written to disk
func (w *WALManager) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.walWriter.Flush(); err != nil {
		return fmt.Errorf("failed to flush WAL writer: %v", err)
	}

	return w.walFile.Sync()
}

// Close closes the WAL file
func (w *WALManager) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.walWriter.Flush(); err != nil {
		return err
	}

	return w.walFile.Close()
}

// ReplayWAL replays WAL entries to restore the store state
func (w *WALManager) ReplayWAL(store *core.Store, walPath string) error {
	file, err := os.Open(walPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No WAL file to replay
		}
		return fmt.Errorf("failed to open WAL file: %v", err)
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	entriesReplayed := 0

	for {
		// Read entry length
		var length uint32
		if err := binary.Read(reader, binary.LittleEndian, &length); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to read WAL entry length: %v", err)
		}

		// Read entry data
		data := make([]byte, length)
		if _, err := io.ReadFull(reader, data); err != nil {
			return fmt.Errorf("failed to read WAL entry data: %v", err)
		}

		// Deserialize entry
		var entry WALEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			return fmt.Errorf("failed to unmarshal WAL entry: %v", err)
		}

		// Verify checksum
		entry.Checksum = 0 // Reset for verification
		verifyData, _ := json.Marshal(entry)
		expectedChecksum := crc32.ChecksumIEEE(verifyData)

		var actualEntry WALEntry
		json.Unmarshal(data, &actualEntry)

		if actualEntry.Checksum != expectedChecksum {
			return fmt.Errorf("WAL entry checksum mismatch")
		}

		// Apply the operation
		switch entry.Operation {
		case "SET":
			var ttl *time.Duration
			if entry.TTL != nil {
				duration := entry.TTL.Sub(entry.Timestamp)
				ttl = &duration
			}
			store.Set(entry.Key, entry.Value, ttl)
		case "DELETE":
			store.Delete(entry.Key)
		}

		entriesReplayed++
	}

	fmt.Printf("Replayed %d WAL entries\n", entriesReplayed)
	return nil
}

// GetWALFiles returns all WAL files in chronological order
func (w *WALManager) GetWALFiles() ([]string, error) {
	files, err := os.ReadDir(w.walDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read WAL directory: %v", err)
	}

	var walFiles []string
	for _, file := range files {
		if !file.IsDir() && len(file.Name()) > 4 && file.Name()[len(file.Name())-4:] == ".log" {
			walFiles = append(walFiles, fmt.Sprintf("%s/%s", w.walDir, file.Name()))
		}
	}

	return walFiles, nil
}

// RotateWAL creates a new WAL file and closes the current one
func (w *WALManager) RotateWAL() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Close current WAL file
	if err := w.walWriter.Flush(); err != nil {
		return err
	}
	if err := w.walFile.Close(); err != nil {
		return err
	}

	// Create new WAL file
	walPath := fmt.Sprintf("%s/wal-%d.log", w.walDir, time.Now().Unix())
	walFile, err := os.OpenFile(walPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to create new WAL file: %v", err)
	}

	w.walFile = walFile
	w.walWriter = bufio.NewWriter(walFile)
	w.entryCount = 0

	return nil
}
