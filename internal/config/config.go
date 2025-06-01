package config

import (
	"encoding/json"
	"os"
	"time"
)

// Config holds all configuration for Mukhostho DB
type Config struct {
	Server      ServerConfig      `json:"server"`
	Memory      MemoryConfig      `json:"memory"`
	Persistence PersistenceConfig `json:"persistence"`
	Security    SecurityConfig    `json:"security"`
	Logging     LoggingConfig     `json:"logging"`
}

type ServerConfig struct {
	Host         string        `json:"host"`
	Port         int           `json:"port"`
	ReadTimeout  time.Duration `json:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout"`
	MaxClients   int           `json:"max_clients"`
}

type MemoryConfig struct {
	MaxMemoryMB     int64         `json:"max_memory_mb"`
	EvictionPolicy  string        `json:"eviction_policy"` // "lru", "lfu", "random"
	TTLCleanupDelay time.Duration `json:"ttl_cleanup_delay"`
	GCInterval      time.Duration `json:"gc_interval"`
}

type PersistenceConfig struct {
	DataDir          string        `json:"data_dir"`
	WALDir           string        `json:"wal_dir"`
	SnapshotDir      string        `json:"snapshot_dir"`
	BackupDir        string        `json:"backup_dir"`
	SnapshotInterval time.Duration `json:"snapshot_interval"`
	WALSyncPolicy    string        `json:"wal_sync_policy"`  // "always", "periodic", "never"
	CompressionType  string        `json:"compression_type"` // "none", "gzip", "lz4"
}

type SecurityConfig struct {
	EnableAuth       bool   `json:"enable_auth"`
	EnableTLS        bool   `json:"enable_tls"`
	CertFile         string `json:"cert_file"`
	KeyFile          string `json:"key_file"`
	EnableEncryption bool   `json:"enable_encryption"`
	EncryptionKey    string `json:"encryption_key"`
}

type LoggingConfig struct {
	Level      string `json:"level"`  // "debug", "info", "warn", "error"
	Format     string `json:"format"` // "text", "json"
	Output     string `json:"output"` // "stdout", "file"
	LogFile    string `json:"log_file"`
	MaxSizeMB  int    `json:"max_size_mb"`
	MaxBackups int    `json:"max_backups"`
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:         "localhost",
			Port:         8080,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			MaxClients:   1000,
		},
		Memory: MemoryConfig{
			MaxMemoryMB:     1024, // 1GB
			EvictionPolicy:  "lru",
			TTLCleanupDelay: 1 * time.Minute,
			GCInterval:      5 * time.Minute,
		},
		Persistence: PersistenceConfig{
			DataDir:          "./data",
			WALDir:           "./data/wal",
			SnapshotDir:      "./data/snapshots",
			BackupDir:        "./data/backups",
			SnapshotInterval: 10 * time.Minute,
			WALSyncPolicy:    "always",
			CompressionType:  "gzip",
		},
		Security: SecurityConfig{
			EnableAuth:       false,
			EnableTLS:        false,
			EnableEncryption: false,
		},
		Logging: LoggingConfig{
			Level:      "info",
			Format:     "text",
			Output:     "stdout",
			MaxSizeMB:  100,
			MaxBackups: 3,
		},
	}
}

// LoadConfig loads configuration from a JSON file
func LoadConfig(filename string) (*Config, error) {
	config := DefaultConfig()

	if filename == "" {
		return config, nil
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil // Use defaults if file doesn't exist
		}
		return nil, err
	}

	if err := json.Unmarshal(data, config); err != nil {
		return nil, err
	}

	return config, nil
}

// SaveConfig saves configuration to a JSON file
func (c *Config) SaveConfig(filename string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}
