package main

import (
	"fmt"

	"github.com/dark-elixir/mukhostho-db/internal/core"
)

func main() {
	store := core.NewStore()
	err := store.Set("test", "value", nil)
	if err != nil {
		fmt.Printf("Error setting value: %v\n", err)
		return
	}

	if value, exists, err := store.Get("test"); err != nil {
		fmt.Printf("Error getting value: %v\n", err)
	} else if exists {
		fmt.Printf("Test passed: %v\n", value)
	} else {
		fmt.Println("Test failed")
	}
}
