package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/dark-elixir/mukhostho-db/pkg/client"
)

func main() {
	// Create a new client
	c := client.NewClient("http://localhost:8080", 30)

	fmt.Println("Mukhostho DB Client Example")
	fmt.Println("===========================")

	// Basic operations example
	basicOperationsExample(c)

	// TTL example
	ttlExample(c)

	// Transaction example
	transactionExample(c)

	// Batch operations example
	batchOperationsExample(c)

	// Complex data types example
	complexDataExample(c)

	fmt.Println("\nAll examples completed successfully!")
}

// Basic CRUD operations
func basicOperationsExample(c *client.Client) {
	fmt.Println("\n1. Basic Operations Example")
	fmt.Println("---------------------------")

	// SET operation
	fmt.Println("Setting key 'user:1' with value 'Alice'")
	if err := c.Set("user:1", "Alice", nil); err != nil {
		log.Printf("Error setting key: %v", err)
		return
	}

	// GET operation
	value, exists, err := c.Get("user:1")
	if err != nil {
		log.Printf("Error getting key: %v", err)
		return
	}
	if exists {
		fmt.Printf("Retrieved value: %v\n", value)
	} else {
		fmt.Println("Key not found")
	}

	// EXISTS operation - check by trying to get the key
	_, exists, err = c.Get("user:1")
	if err != nil {
		log.Printf("Error checking key existence: %v", err)
		return
	}
	fmt.Printf("Key exists: %v\n", exists)

	// DELETE operation
	fmt.Println("Deleting key 'user:1'")
	if _, err := c.Delete("user:1"); err != nil {
		log.Printf("Error deleting key: %v", err)
		return
	}

	// Verify deletion
	_, exists, _ = c.Get("user:1")
	fmt.Printf("Key exists after deletion: %v\n", exists)
}

// TTL (Time To Live) example
func ttlExample(c *client.Client) {
	fmt.Println("\n2. TTL Example")
	fmt.Println("--------------")

	// Set key with 3 second TTL
	ttl := time.Now().Unix() + 3
	fmt.Printf("Setting key 'temp:session' with 3 second TTL\n")
	if err := c.Set("temp:session", "session_data_123", &ttl); err != nil {
		log.Printf("Error setting key with TTL: %v", err)
		return
	}

	// Check immediately
	value, exists, _ := c.Get("temp:session")
	if exists {
		fmt.Printf("Immediately after setting: %v\n", value)
	}

	// Wait and check again
	fmt.Println("Waiting 4 seconds for TTL to expire...")
	time.Sleep(4 * time.Second)

	_, exists, _ = c.Get("temp:session")
	fmt.Printf("After TTL expiration, key exists: %v\n", exists)
}

// Transaction example
func transactionExample(c *client.Client) {
	fmt.Println("\n3. Transaction Example")
	fmt.Println("----------------------")

	// Begin transaction
	txnID, err := c.BeginTransaction()
	if err != nil {
		log.Printf("Error beginning transaction: %v", err)
		return
	}
	fmt.Printf("Started transaction: %s\n", txnID)

	// Set multiple keys in transaction
	fmt.Println("Setting multiple keys in transaction...")
	if err := c.Set("account:123:balance", "1000.00", nil); err != nil {
		log.Printf("Error setting in transaction: %v", err)
		c.RollbackTransaction(txnID)
		return
	}

	if err := c.Set("account:456:balance", "2000.00", nil); err != nil {
		log.Printf("Error setting in transaction: %v", err)
		c.RollbackTransaction(txnID)
		return
	}

	// Check that keys are not visible outside transaction
	_, exists, _ := c.Get("account:123:balance")
	fmt.Printf("Key visible outside transaction before commit: %v\n", exists)

	// Commit transaction
	fmt.Println("Committing transaction...")
	if err := c.CommitTransaction(txnID); err != nil {
		log.Printf("Error committing transaction: %v", err)
		return
	}

	// Verify keys are now visible
	value, exists, _ := c.Get("account:123:balance")
	if exists {
		fmt.Printf("After commit, account:123:balance = %v\n", value)
	}

	// Cleanup
	c.Delete("account:123:balance")
	c.Delete("account:456:balance")
}

// Batch operations example
func batchOperationsExample(c *client.Client) {
	fmt.Println("\n4. Batch Operations Example")
	fmt.Println("---------------------------")

	// Set multiple keys
	keys := []string{"product:1", "product:2", "product:3", "product:4", "product:5"}
	values := []string{"Laptop", "Mouse", "Keyboard", "Monitor", "Headphones"}

	fmt.Println("Setting multiple product keys...")
	for i, key := range keys {
		if err := c.Set(key, values[i], nil); err != nil {
			log.Printf("Error setting %s: %v", key, err)
		}
	}

	// Get all keys
	fmt.Println("Retrieving all product keys:")
	for _, key := range keys {
		value, exists, err := c.Get(key)
		if err != nil {
			log.Printf("Error getting %s: %v", key, err)
			continue
		}
		if exists {
			fmt.Printf("  %s = %v\n", key, value)
		}
	}

	// Get server stats
	stats, err := c.GetStats()
	if err != nil {
		log.Printf("Error getting stats: %v", err)
	} else {
		fmt.Printf("Server stats: %+v\n", stats)
	}

	// Cleanup
	for _, key := range keys {
		c.Delete(key)
	}
}

// Complex data types example
func complexDataExample(c *client.Client) {
	fmt.Println("\n5. Complex Data Types Example")
	fmt.Println("-----------------------------")

	// Store different data types
	examples := map[string]interface{}{
		"string_key": "Hello, World!",
		"int_key":    42,
		"float_key":  3.14159,
		"bool_key":   true,
		"array_key":  []string{"apple", "banana", "cherry"},
		"object_key": map[string]interface{}{
			"name":  "John Doe",
			"age":   30,
			"email": "john.doe@example.com",
		},
	}

	fmt.Println("Storing different data types:")
	for key, value := range examples {
		// Convert complex types to JSON for storage
		var storeValue interface{}
		switch v := value.(type) {
		case []string, map[string]interface{}:
			jsonData, err := json.Marshal(v)
			if err != nil {
				log.Printf("Error marshaling %s: %v", key, err)
				continue
			}
			storeValue = string(jsonData)
		default:
			storeValue = v
		}

		if err := c.Set(key, storeValue, nil); err != nil {
			log.Printf("Error setting %s: %v", key, err)
		} else {
			fmt.Printf("  Set %s = %v\n", key, value)
		}
	}

	fmt.Println("\nRetrieving and displaying stored values:")
	for key := range examples {
		value, exists, err := c.Get(key)
		if err != nil {
			log.Printf("Error getting %s: %v", key, err)
			continue
		}
		if exists {
			fmt.Printf("  %s = %v (type: %T)\n", key, value, value)
		}
	}

	// Cleanup
	for key := range examples {
		c.Delete(key)
	}
}

// Error handling example
func errorHandlingExample(c *client.Client) {
	fmt.Println("\n6. Error Handling Example")
	fmt.Println("-------------------------")

	// Try to get a non-existent key
	_, exists, err := c.Get("non_existent_key")
	if err != nil {
		fmt.Printf("Error getting non-existent key: %v\n", err)
	} else {
		fmt.Printf("Non-existent key exists: %v\n", exists)
	}

	// Try to commit a non-existent transaction
	err = c.CommitTransaction("invalid_txn_id")
	if err != nil {
		fmt.Printf("Expected error committing invalid transaction: %v\n", err)
	}
}

// Performance measurement example
func performanceExample(c *client.Client) {
	fmt.Println("\n7. Performance Measurement Example")
	fmt.Println("----------------------------------")

	const numOperations = 1000

	// Measure SET operations
	start := time.Now()
	for i := 0; i < numOperations; i++ {
		key := fmt.Sprintf("perf:set:%d", i)
		value := fmt.Sprintf("value_%d", i)
		if err := c.Set(key, value, nil); err != nil {
			log.Printf("Error in SET operation %d: %v", i, err)
		}
	}
	setDuration := time.Since(start)

	// Measure GET operations
	start = time.Now()
	for i := 0; i < numOperations; i++ {
		key := fmt.Sprintf("perf:set:%d", i)
		if _, _, err := c.Get(key); err != nil {
			log.Printf("Error in GET operation %d: %v", i, err)
		}
	}
	getDuration := time.Since(start)

	// Measure DELETE operations
	start = time.Now()
	for i := 0; i < numOperations; i++ {
		key := fmt.Sprintf("perf:set:%d", i)
		if _, err := c.Delete(key); err != nil {
			log.Printf("Error in DELETE operation %d: %v", i, err)
		}
	}
	deleteDuration := time.Since(start)

	fmt.Printf("Performance Results (%d operations):\n", numOperations)
	fmt.Printf("  SET operations: %v (%.2f ops/sec)\n", setDuration, float64(numOperations)/setDuration.Seconds())
	fmt.Printf("  GET operations: %v (%.2f ops/sec)\n", getDuration, float64(numOperations)/getDuration.Seconds())
	fmt.Printf("  DELETE operations: %v (%.2f ops/sec)\n", deleteDuration, float64(numOperations)/deleteDuration.Seconds())
}
