package main

import (
	"context"
	"fmt"

	"mini-bank/internal/core"
	"mini-bank/internal/storage/memory"
)

func main() {
	ctx := context.Background()
	store := memory.NewStore()

	// Create sample accounts
	acc1, _ := store.CreateAccount(ctx, "Alice", 1000)
	acc2, _ := store.CreateAccount(ctx, "Bob", 500)

	fmt.Println("✅ Accounts created:")
	fmt.Printf(" - %s (Balance: %.2f)\n", acc1.Name, acc1.Balance)
	fmt.Printf(" - %s (Balance: %.2f)\n\n", acc2.Name, acc2.Balance)

	// Update balance
	_ = store.UpdateBalance(ctx, acc1.ID, 1200)

	// List accounts
	accounts, _ := store.ListAccounts(ctx)
	fmt.Println("💰 All accounts:")
	for _, a := range accounts {
		fmt.Printf("[%d] %s — %.2f\n", a.ID, a.Name, a.Balance)
	}

	// Record a transaction
	tx := &core.Transaction{
		AccountID: acc1.ID,
		Type:      "deposit",
		Amount:    200,
	}
	_ = store.RecordTransaction(ctx, tx)

	// List transactions
	fmt.Println("\n📜 Transactions for Alice:")
	txs, _ := store.ListTransactions(ctx, acc1.ID)
	for _, t := range txs {
		fmt.Printf("- %s of %.2f\n", t.Type, t.Amount)
	}
}
