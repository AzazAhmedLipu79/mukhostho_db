package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"github.com/dark-elixir/mukhostho-db/internal/core"
	"github.com/dark-elixir/mukhostho-db/internal/memory"
	"github.com/dark-elixir/mukhostho-db/internal/persistence"
	"github.com/dark-elixir/mukhostho-db/internal/transaction"
)

// APIServer provides HTTP REST API for Mukhostho DB
type APIServer struct {
	store       *core.Store
	txnManager  *transaction.TransactionManager
	memManager  *memory.MemoryManager
	snapManager *persistence.SnapshotManager
	walManager  *persistence.WALManager
	router      *mux.Router
	logger      *logrus.Logger
	host        string
	port        int
}

// Request/Response structures

type SetRequest struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
	TTL   *int64      `json:"ttl,omitempty"` // TTL in seconds
}

type GetResponse struct {
	Key    string      `json:"key"`
	Value  interface{} `json:"value"`
	Exists bool        `json:"exists"`
}

type DeleteResponse struct {
	Key     string `json:"key"`
	Deleted bool   `json:"deleted"`
}

type ListKeysResponse struct {
	Keys  []string `json:"keys"`
	Count int      `json:"count"`
}

type ListItemsResponse struct {
	Items map[string]interface{} `json:"items"`
	Count int                    `json:"count"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type TransactionRequest struct {
	Operations []TransactionOperation `json:"operations"`
}

type TransactionOperation struct {
	Type  string      `json:"type"` // "SET" or "DELETE"
	Key   string      `json:"key"`
	Value interface{} `json:"value,omitempty"`
	TTL   *int64      `json:"ttl,omitempty"`
}

type TransactionResponse struct {
	TransactionID string `json:"transaction_id"`
	Status        string `json:"status"`
	Message       string `json:"message"`
}

// NewAPIServer creates a new API server
func NewAPIServer(
	host string,
	port int,
	store *core.Store,
	txnManager *transaction.TransactionManager,
	memManager *memory.MemoryManager,
	snapManager *persistence.SnapshotManager,
	walManager *persistence.WALManager,
) *APIServer {
	server := &APIServer{
		store:       store,
		txnManager:  txnManager,
		memManager:  memManager,
		snapManager: snapManager,
		walManager:  walManager,
		router:      mux.NewRouter(),
		logger:      logrus.New(),
		host:        host,
		port:        port,
	}

	server.setupRoutes()
	return server
}

// setupRoutes configures all API routes
func (s *APIServer) setupRoutes() {
	api := s.router.PathPrefix("/api/v1").Subrouter()

	// Basic CRUD operations
	api.HandleFunc("/keys/{key}", s.handleGet).Methods("GET")
	api.HandleFunc("/keys/{key}", s.handleSet).Methods("PUT")
	api.HandleFunc("/keys/{key}", s.handleDelete).Methods("DELETE")
	api.HandleFunc("/keys", s.handleListKeys).Methods("GET")
	api.HandleFunc("/items", s.handleListItems).Methods("GET")

	// Transaction operations
	api.HandleFunc("/transactions", s.handleBeginTransaction).Methods("POST")
	api.HandleFunc("/transactions/{txnId}/commit", s.handleCommitTransaction).Methods("POST")
	api.HandleFunc("/transactions/{txnId}/rollback", s.handleRollbackTransaction).Methods("POST")
	api.HandleFunc("/transactions/{txnId}/set", s.handleTransactionSet).Methods("PUT")
	api.HandleFunc("/transactions/{txnId}/delete", s.handleTransactionDelete).Methods("DELETE")
	api.HandleFunc("/transactions/{txnId}", s.handleGetTransaction).Methods("GET")
	api.HandleFunc("/transactions", s.handleListTransactions).Methods("GET")

	// Administrative operations
	api.HandleFunc("/admin/backup", s.handleBackup).Methods("POST")
	api.HandleFunc("/admin/restore", s.handleRestore).Methods("POST")
	api.HandleFunc("/admin/snapshot", s.handleCreateSnapshot).Methods("POST")
	api.HandleFunc("/admin/stats", s.handleGetStats).Methods("GET")
	api.HandleFunc("/admin/memory", s.handleGetMemoryStats).Methods("GET")
	api.HandleFunc("/admin/health", s.handleHealthCheck).Methods("GET")

	// Add middleware
	s.router.Use(s.loggingMiddleware)
	s.router.Use(s.corsMiddleware)
}

// Start starts the HTTP server
func (s *APIServer) Start() error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	s.logger.Infof("Starting Mukhostho DB API server on %s", addr)
	return http.ListenAndServe(addr, s.router)
}

// CRUD Operation Handlers

func (s *APIServer) handleGet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["key"]

	value, exists, err := s.store.Get(key)
	if err != nil {
		s.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get value", err)
		return
	}

	response := GetResponse{
		Key:    key,
		Value:  value,
		Exists: exists,
	}

	s.writeJSONResponse(w, http.StatusOK, response)
}

func (s *APIServer) handleSet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["key"]

	var req SetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON", err)
		return
	}

	// Convert TTL from seconds to duration
	var ttl *time.Duration
	if req.TTL != nil {
		duration := time.Duration(*req.TTL) * time.Second
		ttl = &duration
	}

	if err := s.store.Set(key, req.Value, ttl); err != nil {
		s.writeErrorResponse(w, http.StatusInternalServerError, "Failed to set value", err)
		return
	}

	// Log to WAL
	if s.walManager != nil {
		var ttlTime *time.Time
		if ttl != nil {
			t := time.Now().Add(*ttl)
			ttlTime = &t
		}
		if err := s.walManager.LogSet(key, req.Value, ttlTime); err != nil {
			s.logger.Errorf("Failed to log SET operation: %v", err)
		}
	}

	s.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"key":     key,
		"success": true,
	})
}

func (s *APIServer) handleDelete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["key"]

	deleted := s.store.Delete(key)

	// Log to WAL
	if s.walManager != nil && deleted {
		if err := s.walManager.LogDelete(key); err != nil {
			s.logger.Errorf("Failed to log DELETE operation: %v", err)
		}
	}

	response := DeleteResponse{
		Key:     key,
		Deleted: deleted,
	}

	s.writeJSONResponse(w, http.StatusOK, response)
}

func (s *APIServer) handleListKeys(w http.ResponseWriter, r *http.Request) {
	keys := s.store.ListKeys()

	response := ListKeysResponse{
		Keys:  keys,
		Count: len(keys),
	}

	s.writeJSONResponse(w, http.StatusOK, response)
}

func (s *APIServer) handleListItems(w http.ResponseWriter, r *http.Request) {
	items := s.store.ListItems()

	response := ListItemsResponse{
		Items: items,
		Count: len(items),
	}

	s.writeJSONResponse(w, http.StatusOK, response)
}

// Transaction Handlers

func (s *APIServer) handleBeginTransaction(w http.ResponseWriter, r *http.Request) {
	txn, err := s.txnManager.BeginTransaction()
	if err != nil {
		s.writeErrorResponse(w, http.StatusInternalServerError, "Failed to begin transaction", err)
		return
	}

	response := TransactionResponse{
		TransactionID: txn.ID,
		Status:        "active",
		Message:       "Transaction started successfully",
	}

	s.writeJSONResponse(w, http.StatusOK, response)
}

func (s *APIServer) handleCommitTransaction(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	txnID := vars["txnId"]

	if err := s.txnManager.CommitTransaction(txnID); err != nil {
		s.writeErrorResponse(w, http.StatusBadRequest, "Failed to commit transaction", err)
		return
	}

	response := TransactionResponse{
		TransactionID: txnID,
		Status:        "committed",
		Message:       "Transaction committed successfully",
	}

	s.writeJSONResponse(w, http.StatusOK, response)
}

func (s *APIServer) handleRollbackTransaction(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	txnID := vars["txnId"]

	if err := s.txnManager.RollbackTransaction(txnID); err != nil {
		s.writeErrorResponse(w, http.StatusBadRequest, "Failed to rollback transaction", err)
		return
	}

	response := TransactionResponse{
		TransactionID: txnID,
		Status:        "aborted",
		Message:       "Transaction rolled back successfully",
	}

	s.writeJSONResponse(w, http.StatusOK, response)
}

func (s *APIServer) handleTransactionSet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	txnID := vars["txnId"]

	var req SetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON", err)
		return
	}

	var ttl *time.Duration
	if req.TTL != nil {
		duration := time.Duration(*req.TTL) * time.Second
		ttl = &duration
	}

	if err := s.txnManager.Set(txnID, req.Key, req.Value, ttl); err != nil {
		s.writeErrorResponse(w, http.StatusBadRequest, "Failed to add SET operation to transaction", err)
		return
	}

	s.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"transaction_id": txnID,
		"operation":      "SET",
		"key":            req.Key,
		"success":        true,
	})
}

func (s *APIServer) handleTransactionDelete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	txnID := vars["txnId"]

	var req struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON", err)
		return
	}

	if err := s.txnManager.Delete(txnID, req.Key); err != nil {
		s.writeErrorResponse(w, http.StatusBadRequest, "Failed to add DELETE operation to transaction", err)
		return
	}

	s.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"transaction_id": txnID,
		"operation":      "DELETE",
		"key":            req.Key,
		"success":        true,
	})
}

func (s *APIServer) handleGetTransaction(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	txnID := vars["txnId"]

	txn, err := s.txnManager.GetTransaction(txnID)
	if err != nil {
		s.writeErrorResponse(w, http.StatusNotFound, "Transaction not found", err)
		return
	}

	s.writeJSONResponse(w, http.StatusOK, txn)
}

func (s *APIServer) handleListTransactions(w http.ResponseWriter, r *http.Request) {
	transactions := s.txnManager.ListActiveTransactions()
	s.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"transactions": transactions,
		"count":        len(transactions),
	})
}

// Administrative Handlers

func (s *APIServer) handleCreateSnapshot(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Description string `json:"description"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if s.snapManager == nil {
		s.writeErrorResponse(w, http.StatusServiceUnavailable, "Snapshot manager not available", nil)
		return
	}

	path, err := s.snapManager.CreateSnapshot(s.store, req.Description)
	if err != nil {
		s.writeErrorResponse(w, http.StatusInternalServerError, "Failed to create snapshot", err)
		return
	}

	s.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"snapshot_path": path,
		"success":       true,
	})
}

func (s *APIServer) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := map[string]interface{}{
		"key_count":    s.store.GetKeyCount(),
		"memory_usage": s.store.GetMemoryUsage(),
		"timestamp":    time.Now(),
	}

	if s.memManager != nil {
		stats["memory_stats"] = s.memManager.GetMemoryStats()
		stats["system_memory"] = s.memManager.GetSystemMemoryInfo()
	}

	s.writeJSONResponse(w, http.StatusOK, stats)
}

func (s *APIServer) handleGetMemoryStats(w http.ResponseWriter, r *http.Request) {
	if s.memManager == nil {
		s.writeErrorResponse(w, http.StatusServiceUnavailable, "Memory manager not available", nil)
		return
	}

	stats := s.memManager.GetMemoryStats()
	systemInfo := s.memManager.GetSystemMemoryInfo()

	response := map[string]interface{}{
		"memory_stats":    stats,
		"system_memory":   systemInfo,
		"eviction_policy": s.memManager.GetEvictionPolicy(),
	}

	s.writeJSONResponse(w, http.StatusOK, response)
}

func (s *APIServer) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
		"uptime":    time.Since(time.Now()), // This should track actual uptime
		"version":   "1.0.0",
	}

	s.writeJSONResponse(w, http.StatusOK, health)
}

func (s *APIServer) handleBackup(w http.ResponseWriter, r *http.Request) {
	// Implementation would depend on backup strategy
	s.writeJSONResponse(w, http.StatusNotImplemented, map[string]string{
		"message": "Backup functionality not yet implemented",
	})
}

func (s *APIServer) handleRestore(w http.ResponseWriter, r *http.Request) {
	// Implementation would depend on restore strategy
	s.writeJSONResponse(w, http.StatusNotImplemented, map[string]string{
		"message": "Restore functionality not yet implemented",
	})
}

// Utility methods

func (s *APIServer) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func (s *APIServer) writeErrorResponse(w http.ResponseWriter, statusCode int, message string, err error) {
	errorMsg := message
	if err != nil {
		errorMsg = fmt.Sprintf("%s: %v", message, err)
		s.logger.Error(errorMsg)
	}

	response := ErrorResponse{
		Error:   errorMsg,
		Code:    statusCode,
		Message: message,
	}

	s.writeJSONResponse(w, statusCode, response)
}

// Middleware

func (s *APIServer) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		duration := time.Since(start)

		s.logger.WithFields(logrus.Fields{
			"method":   r.Method,
			"path":     r.URL.Path,
			"duration": duration,
			"ip":       r.RemoteAddr,
		}).Info("HTTP request")
	})
}

func (s *APIServer) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
