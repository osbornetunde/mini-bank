package main

import (
	"context"
	"fmt"
	"sync"

	"mini-bank/internal/storage/memory"
)

func main() {
	ctx := context.Background()
	store := memory.NewStore()

	acc, _ := store.CreateAccount(ctx, "Alice", 1000.0)

	const n = 1000
	var wg sync.WaitGroup

	// start n concurrent deposits of +1.0
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			// naive read-update-write
			a, _ := store.GetAccount(ctx, acc.ID)
			// note: this is intentionally naive to reproduce race if store doesn't protect internal operations
			_ = store.UpdateBalance(ctx, a.ID, a.Balance+1.0)
		}()
	}

	wg.Wait()

	final, _ := store.GetAccount(ctx, acc.ID)
	fmt.Printf("Expected balance: %.2f\n", 1000.0+float64(n))
	fmt.Printf("Actual   balance: %.2f\n", final.Balance)
}
