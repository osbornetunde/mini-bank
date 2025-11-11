package main

import (
	"context"
	"fmt"
	"log"
	"sync"

	"mini-bank/internal/storage/memory"
)

func main() {
	ctx := context.Background()
	store := memory.NewStore()

	acc1, _ := store.CreateAccount(ctx, "Alice", 1000.0)
	acc2, _ := store.CreateAccount(ctx, "Bob", 5000.0)

	fmt.Printf("Initial: Alice=%.2f, Bob=%.2f\n", acc1.Balance, acc2.Balance)

	const n = 1000
	var wg sync.WaitGroup

	// start n concurrent deposits of +1.0
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			if err := store.Transfer(ctx, acc1.ID, acc2.ID, 1.0); err != nil {
				log.Println("transfer error:", err)
			}
		}()
	}

	// concurrent transfers from Bob to Alice of 0.5
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			if err := store.Transfer(ctx, acc2.ID, acc1.ID, 0.5); err != nil {
				log.Println("transfer error:", err)
			}
		}()
	}

	wg.Wait()

	a1, _ := store.GetAccount(ctx, acc1.ID)
	a2, _ := store.GetAccount(ctx, acc2.ID)
	fmt.Printf("Final: Alice=%.2f, Bob=%.2f\n", a1.Balance, a2.Balance)

	fmt.Printf("Expected: Alice=%.2f, Bob=%.2f\n", 500.00, 5500.00)
}
