package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client provides a Go client for Mukhostho DB
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new Mukhostho DB client
func NewClient(host string, port int) *Client {
	return &Client{
		baseURL: fmt.Sprintf("http://%s:%d/api/v1", host, port),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetTimeout sets the HTTP client timeout
func (c *Client) SetTimeout(timeout time.Duration) {
	c.httpClient.Timeout = timeout
}

// Set stores a key-value pair
func (c *Client) Set(key string, value interface{}, ttl *int64) error {
	req := map[string]interface{}{
		"value": value,
	}
	if ttl != nil {
		req["ttl"] = *ttl
	}

	_, err := c.makeRequest("PUT", fmt.Sprintf("/keys/%s", key), req)
	return err
}

// Get retrieves a value by key
func (c *Client) Get(key string) (interface{}, bool, error) {
	resp, err := c.makeRequest("GET", fmt.Sprintf("/keys/%s", key), nil)
	if err != nil {
		return nil, false, err
	}

	var response struct {
		Key    string      `json:"key"`
		Value  interface{} `json:"value"`
		Exists bool        `json:"exists"`
	}

	if err := json.Unmarshal(resp, &response); err != nil {
		return nil, false, err
	}

	return response.Value, response.Exists, nil
}

// Delete removes a key
func (c *Client) Delete(key string) (bool, error) {
	resp, err := c.makeRequest("DELETE", fmt.Sprintf("/keys/%s", key), nil)
	if err != nil {
		return false, err
	}

	var response struct {
		Key     string `json:"key"`
		Deleted bool   `json:"deleted"`
	}

	if err := json.Unmarshal(resp, &response); err != nil {
		return false, err
	}

	return response.Deleted, nil
}

// ListKeys returns all keys
func (c *Client) ListKeys() ([]string, error) {
	resp, err := c.makeRequest("GET", "/keys", nil)
	if err != nil {
		return nil, err
	}

	var response struct {
		Keys  []string `json:"keys"`
		Count int      `json:"count"`
	}

	if err := json.Unmarshal(resp, &response); err != nil {
		return nil, err
	}

	return response.Keys, nil
}

// ListItems returns all key-value pairs
func (c *Client) ListItems() (map[string]interface{}, error) {
	resp, err := c.makeRequest("GET", "/items", nil)
	if err != nil {
		return nil, err
	}

	var response struct {
		Items map[string]interface{} `json:"items"`
		Count int                    `json:"count"`
	}

	if err := json.Unmarshal(resp, &response); err != nil {
		return nil, err
	}

	return response.Items, nil
}

// BeginTransaction starts a new transaction
func (c *Client) BeginTransaction() (string, error) {
	resp, err := c.makeRequest("POST", "/transactions", nil)
	if err != nil {
		return "", err
	}

	var response struct {
		TransactionID string `json:"transaction_id"`
		Status        string `json:"status"`
		Message       string `json:"message"`
	}

	if err := json.Unmarshal(resp, &response); err != nil {
		return "", err
	}

	return response.TransactionID, nil
}

// CommitTransaction commits a transaction
func (c *Client) CommitTransaction(txnID string) error {
	_, err := c.makeRequest("POST", fmt.Sprintf("/transactions/%s/commit", txnID), nil)
	return err
}

// RollbackTransaction rolls back a transaction
func (c *Client) RollbackTransaction(txnID string) error {
	_, err := c.makeRequest("POST", fmt.Sprintf("/transactions/%s/rollback", txnID), nil)
	return err
}

// TransactionSet adds a SET operation to a transaction
func (c *Client) TransactionSet(txnID, key string, value interface{}, ttl *int64) error {
	req := map[string]interface{}{
		"key":   key,
		"value": value,
	}
	if ttl != nil {
		req["ttl"] = *ttl
	}

	_, err := c.makeRequest("PUT", fmt.Sprintf("/transactions/%s/set", txnID), req)
	return err
}

// TransactionDelete adds a DELETE operation to a transaction
func (c *Client) TransactionDelete(txnID, key string) error {
	req := map[string]interface{}{
		"key": key,
	}

	_, err := c.makeRequest("DELETE", fmt.Sprintf("/transactions/%s/delete", txnID), req)
	return err
}

// CreateSnapshot creates a manual snapshot
func (c *Client) CreateSnapshot(description string) (string, error) {
	req := map[string]interface{}{
		"description": description,
	}

	resp, err := c.makeRequest("POST", "/admin/snapshot", req)
	if err != nil {
		return "", err
	}

	var response struct {
		SnapshotPath string `json:"snapshot_path"`
		Success      bool   `json:"success"`
	}

	if err := json.Unmarshal(resp, &response); err != nil {
		return "", err
	}

	return response.SnapshotPath, nil
}

// GetStats returns database statistics
func (c *Client) GetStats() (map[string]interface{}, error) {
	resp, err := c.makeRequest("GET", "/admin/stats", nil)
	if err != nil {
		return nil, err
	}

	var stats map[string]interface{}
	if err := json.Unmarshal(resp, &stats); err != nil {
		return nil, err
	}

	return stats, nil
}

// GetMemoryStats returns memory statistics
func (c *Client) GetMemoryStats() (map[string]interface{}, error) {
	resp, err := c.makeRequest("GET", "/admin/memory", nil)
	if err != nil {
		return nil, err
	}

	var stats map[string]interface{}
	if err := json.Unmarshal(resp, &stats); err != nil {
		return nil, err
	}

	return stats, nil
}

// HealthCheck performs a health check
func (c *Client) HealthCheck() (map[string]interface{}, error) {
	resp, err := c.makeRequest("GET", "/admin/health", nil)
	if err != nil {
		return nil, err
	}

	var health map[string]interface{}
	if err := json.Unmarshal(resp, &health); err != nil {
		return nil, err
	}

	return health, nil
}

// makeRequest makes an HTTP request to the server
func (c *Client) makeRequest(method, path string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		var errorResp struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		json.Unmarshal(respBody, &errorResp)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, errorResp.Error)
	}

	return respBody, nil
}
