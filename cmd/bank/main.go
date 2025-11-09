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

	fmt.Println("Enter the following details to create a new account:")
	fmt.Print("Name: ")
	var name string
	fmt.Scanln(&name)

	fmt.Print("Initial balance: ")
	var balance float64
	fmt.Scanln(&balance)

	acc, err := store.CreateAccount(ctx, name, balance)
	if err != nil {
		log.Fatalf("failed to create account: %v", err)
	}

	fmt.Printf("Created account: %+v\n", acc)
}
