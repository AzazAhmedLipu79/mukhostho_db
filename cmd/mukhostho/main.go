package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/dark-elixir/mukhostho-db/internal/api"
	"github.com/dark-elixir/mukhostho-db/internal/config"
	"github.com/dark-elixir/mukhostho-db/internal/core"
	"github.com/dark-elixir/mukhostho-db/internal/memory"
	"github.com/dark-elixir/mukhostho-db/internal/persistence"
	"github.com/dark-elixir/mukhostho-db/internal/transaction"
)

func main() {
	// Parse command line flags
	configFile := flag.String("config", "", "Path to configuration file")
	dataDir := flag.String("data-dir", "./data", "Data directory")
	port := flag.Int("port", 8080, "Server port")
	host := flag.String("host", "localhost", "Server host")
	memoryLimit := flag.Int64("memory-limit", 1024, "Memory limit in MB")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	flag.Parse()

	// Setup logging
	logger := logrus.New()
	level, err := logrus.ParseLevel(*logLevel)
	if err != nil {
		log.Fatalf("Invalid log level: %v", err)
	}
	logger.SetLevel(level)
	logger.SetFormatter(&logrus.JSONFormatter{})

	logger.Info("Starting Mukhostho DB...")

	// Load configuration
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		logger.Fatalf("Failed to load configuration: %v", err)
	}

	// Override config with command line flags
	if *dataDir != "./data" {
		cfg.Persistence.DataDir = *dataDir
		cfg.Persistence.WALDir = *dataDir + "/wal"
		cfg.Persistence.SnapshotDir = *dataDir + "/snapshots"
		cfg.Persistence.BackupDir = *dataDir + "/backups"
	}
	if *port != 8080 {
		cfg.Server.Port = *port
	}
	if *host != "localhost" {
		cfg.Server.Host = *host
	}
	if *memoryLimit != 1024 {
		cfg.Memory.MaxMemoryMB = *memoryLimit
	}

	// Create directories
	dirs := []string{
		cfg.Persistence.DataDir,
		cfg.Persistence.WALDir,
		cfg.Persistence.SnapshotDir,
		cfg.Persistence.BackupDir,
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			logger.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	logger.WithFields(logrus.Fields{
		"data_dir":     cfg.Persistence.DataDir,
		"host":         cfg.Server.Host,
		"port":         cfg.Server.Port,
		"memory_limit": cfg.Memory.MaxMemoryMB,
	}).Info("Configuration loaded")

	// Initialize core components
	store := core.NewStore()
	logger.Info("Created in-memory store")

	// Initialize WAL manager
	walManager, err := persistence.NewWALManager(cfg.Persistence.WALDir, cfg.Persistence.WALSyncPolicy)
	if err != nil {
		logger.Fatalf("Failed to create WAL manager: %v", err)
	}
	logger.Info("Created WAL manager")

	// Initialize snapshot manager
	snapManager, err := persistence.NewSnapshotManager(cfg.Persistence.SnapshotDir, cfg.Persistence.CompressionType)
	if err != nil {
		logger.Fatalf("Failed to create snapshot manager: %v", err)
	}
	logger.Info("Created snapshot manager")

	// Initialize memory manager
	memManager := memory.NewMemoryManager(store, &cfg.Memory)
	memManager.Start()
	logger.Info("Started memory manager")

	// Initialize transaction manager
	txnManager := transaction.NewTransactionManager(store, walManager)
	logger.Info("Created transaction manager")

	// Attempt to restore from existing data
	if err := restoreFromPersistence(store, walManager, snapManager, logger); err != nil {
		logger.Warnf("Failed to restore from persistence: %v", err)
	}

	// Create API server
	apiServer := api.NewAPIServer(
		cfg.Server.Host,
		cfg.Server.Port,
		store,
		txnManager,
		memManager,
		snapManager,
		walManager,
	)
	logger.Info("Created API server")

	// Start periodic snapshot creation
	go startPeriodicSnapshots(snapManager, store, cfg.Persistence.SnapshotInterval, logger)

	// Handle graceful shutdown
	setupGracefulShutdown(walManager, snapManager, memManager, logger)

	// Start the server
	logger.WithFields(logrus.Fields{
		"host": cfg.Server.Host,
		"port": cfg.Server.Port,
	}).Info("Starting HTTP server")

	if err := apiServer.Start(); err != nil {
		logger.Fatalf("Failed to start server: %v", err)
	}
}

func restoreFromPersistence(store *core.Store, walManager *persistence.WALManager, snapManager *persistence.SnapshotManager, logger *logrus.Logger) error {
	// Try to restore from latest snapshot
	latestSnapshot, err := snapManager.GetLatestSnapshot()
	if err == nil {
		logger.Infof("Restoring from snapshot: %s", latestSnapshot)
		if err := snapManager.RestoreSnapshot(store, latestSnapshot); err != nil {
			return fmt.Errorf("failed to restore snapshot: %v", err)
		}
		logger.Info("Successfully restored from snapshot")
	} else {
		logger.Info("No snapshot found, starting with empty database")
	}

	// Replay WAL entries
	walFiles, err := walManager.GetWALFiles()
	if err != nil {
		return fmt.Errorf("failed to get WAL files: %v", err)
	}

	for _, walFile := range walFiles {
		logger.Infof("Replaying WAL file: %s", walFile)
		if err := walManager.ReplayWAL(store, walFile); err != nil {
			logger.Errorf("Failed to replay WAL file %s: %v", walFile, err)
			continue
		}
	}

	logger.WithFields(logrus.Fields{
		"key_count":    store.GetKeyCount(),
		"memory_usage": store.GetMemoryUsage(),
	}).Info("Database restored successfully")

	return nil
}

func startPeriodicSnapshots(snapManager *persistence.SnapshotManager, store *core.Store, interval time.Duration, logger *logrus.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			logger.Info("Creating periodic snapshot")
			path, err := snapManager.CreateSnapshot(store, "Periodic snapshot")
			if err != nil {
				logger.Errorf("Failed to create periodic snapshot: %v", err)
			} else {
				logger.Infof("Created snapshot: %s", path)
			}
		}
	}
}

func setupGracefulShutdown(walManager *persistence.WALManager, snapManager *persistence.SnapshotManager, memManager *memory.MemoryManager, logger *logrus.Logger) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		logger.Info("Received shutdown signal, gracefully shutting down...")

		// Stop memory manager
		memManager.Stop()
		logger.Info("Stopped memory manager")

		// Flush WAL
		if err := walManager.Flush(); err != nil {
			logger.Errorf("Failed to flush WAL: %v", err)
		} else {
			logger.Info("Flushed WAL")
		}

		// Create final snapshot
		if _, err := snapManager.CreateSnapshot(nil, "Shutdown snapshot"); err != nil {
			logger.Errorf("Failed to create shutdown snapshot: %v", err)
		} else {
			logger.Info("Created shutdown snapshot")
		}

		// Close WAL
		if err := walManager.Close(); err != nil {
			logger.Errorf("Failed to close WAL: %v", err)
		} else {
			logger.Info("Closed WAL")
		}

		logger.Info("Shutdown complete")
		os.Exit(0)
	}()
}
