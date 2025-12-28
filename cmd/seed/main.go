package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"os"

	"mini-bank/internal/core"
	"mini-bank/internal/storage/postgres"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	db, err := postgres.NewDB(dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	repo := postgres.NewRepo(db)
	ctx := context.Background()

	// Check if cleanup is requested
	if os.Getenv("SEED_CLEAN") == "true" {
		fmt.Println("Cleaning database...")
		if err := cleanDatabase(ctx, db.DB); err != nil {
			log.Fatalf("Failed to clean database: %v", err)
		}
		fmt.Println("Database cleaned successfully.")
	}

	// 1. Create Users
	// Get password from environment or use default (dev/test only!)
	password := os.Getenv("SEED_PASSWORD")
	if password == "" {
		password = "password123"
		log.Println("Using default password. Set SEED_PASSWORD env var to customize.")
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}
	hash := string(hashedPassword)

	users := []struct {
		First   string
		Last    string
		Email   string
		Balance int64 // Balance in cents (100 = $1.00)
	}{
		{"Alice", "Smith", "alice@example.com", 100000},    // $1000.00
		{"Bob", "Jones", "bob@example.com", 50000},         // $500.00
		{"Charlie", "Brown", "charlie@example.com", 75000}, // $750.00
	}

	var createdAccounts []*core.Account

	fmt.Println("Seeding users...")
	for _, u := range users {
		// Check if user exists first to avoid duplicates on re-runs
		existing, err := repo.GetUserByEmail(ctx, u.Email)
		if err == nil && existing != nil {
			fmt.Printf("User %s already exists, skipping creation.\n", u.Email)
			// Find their account
			if acc := findUserAccount(ctx, repo, existing.ID); acc != nil {
				createdAccounts = append(createdAccounts, acc)
			}
			continue
		}

		user, err := repo.CreateUserWithAccount(ctx, u.First, u.Last, u.Email, hash, u.Balance)
		if err != nil {
			log.Printf("Failed to create user %s: %v", u.Email, err)
			continue
		}
		fmt.Printf("Created user: %s (ID: %d)\n", u.Email, user.ID)

		// Find the account we just created
		if acc := findUserAccount(ctx, repo, user.ID); acc != nil {
			createdAccounts = append(createdAccounts, acc)
		} else {
			log.Printf("Warning: Could not find account for user %d", user.ID)
		}
	}

	if len(createdAccounts) < 2 {
		fmt.Println("Not enough accounts to simulate transfers.")
		return
	}

	// 2. Simulate Transactions
	fmt.Println("Simulating transactions...")

	for i := range 5 {
		fromIdx := rand.Intn(len(createdAccounts))
		toIdx := rand.Intn(len(createdAccounts))
		if fromIdx == toIdx {
			toIdx = (fromIdx + 1) % len(createdAccounts)
		}

		from := createdAccounts[fromIdx]
		to := createdAccounts[toIdx]
		amount := int64(rand.Intn(5000) + 100) // Random amount between 1.00 and 51.00

		fmt.Printf("Transferring %d from Account %d to Account %d...\n", amount, from.ID, to.ID)
		_, _, err := repo.Transfer(ctx, from.ID, to.ID, amount, fmt.Sprintf("seed-transfer-%d", i))
		if err != nil {
			log.Printf("Transfer failed: %v", err)
		} else {
			fmt.Println("Transfer successful.")
		}
	}

	// 3. Simulate Withdrawals
	fmt.Println("Simulating withdrawals...")
	for _, acc := range createdAccounts {
		amount := int64(rand.Intn(2000) + 100)
		fmt.Printf("Withdrawing %d from Account %d...\n", amount, acc.ID)
		_, err := repo.Withdraw(ctx, acc.ID, amount, fmt.Sprintf("seed-withdraw-%d", acc.ID))
		if err != nil {
			log.Printf("Withdrawal failed: %v", err)
		} else {
			fmt.Println("Withdrawal successful.")
		}
	}

	fmt.Println("Seeding complete.")
}

// findUserAccount finds the first account belonging to a user
// This is more efficient than loading all accounts and searching
func findUserAccount(ctx context.Context, repo *postgres.Repo, userID int) *core.Account {
	allAccounts, err := repo.ListAccounts(ctx)
	if err != nil {
		log.Printf("Failed to list accounts: %v", err)
		return nil
	}
	for _, acc := range allAccounts {
		if acc.UserID == userID {
			return acc
		}
	}
	return nil
}

// cleanDatabase truncates all tables to start fresh
// WARNING: This deletes ALL data! Use only in development/testing.
func cleanDatabase(ctx context.Context, db *sql.DB) error {
	// Order matters due to foreign key constraints
	tables := []string{
		"audit_logs",
		"transactions",
		"accounts",
		"password_reset_tokens",
		"users",
	}

	for _, table := range tables {
		query := fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", table)
		if _, err := db.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("failed to truncate %s: %w", table, err)
		}
		fmt.Printf("Truncated table: %s\n", table)
	}

	return nil
}
