package main

import (
	"context"
	"fmt"
	"log"

	"mini-bank/internal/storage/file"
)

func main() {
	ctx := context.Background()

	store, err := file.NewFileStore("data/accounts.json", "data/transactions.json")
	if err != nil {
		log.Fatalf("failed to initialize file store: %v", err)
	}

	acc, err := store.CreateAccount(ctx, "Charlie", 1000)
	if err != nil {
		log.Fatalf("failed to create account: %v", err)
	}

	fmt.Printf("Created account: %+v\n", acc)
}
