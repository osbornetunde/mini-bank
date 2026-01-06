package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
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
		First       string
		Last        string
		Email       string
		Balance     int64 // Balance in cents (100 = $1.00)
		Permissions []string
		Role        string
	}{
		{
			"Alice", "Admin", "alice@example.com", 100000,
			[]string{
				core.PermAccountsRead,
				core.PermAccountsWrite,
				core.PermAccountsUpdate,
				core.PermTransactionsRead,
				core.PermTransactionsProcess,
				core.PermUsersRead,
				core.PermUsersWrite,
				core.PermUsersUpdate,
				core.PermPermissionsManage,
			},
			"Admin",
		},
		{
			"Bob", "Manager", "bob@example.com", 50000,
			[]string{
				core.PermAccountsRead,
				core.PermAccountsWrite,
				core.PermAccountsUpdate,
				core.PermTransactionsRead,
				core.PermTransactionsProcess,
				core.PermUsersRead,
			},
			"Manager",
		},
		{
			"Charlie", "User", "charlie@example.com", 75000,
			[]string{
				core.PermAccountsRead,
				core.PermTransactionsRead,
			},
			"User",
		},
	}

	var createdAccounts []*core.Account

	fmt.Println("Seeding users...")
	for _, u := range users {
		// Check if user exists first to avoid duplicates on re-runs
		existing, err := repo.GetUserByEmail(ctx, u.Email)
		if err == nil && existing != nil {
			fmt.Printf("User %s already exists, updating permissions.\n", u.Email)

			// Update permissions for existing user
			if err := repo.UpdateUserPermissions(ctx, existing.ID, u.Permissions); err != nil {
				log.Printf("Failed to update permissions for %s: %v", u.Email, err)
			} else {
				fmt.Printf("Updated permissions for %s (%s): %v\n", u.Email, u.Role, u.Permissions)
			}

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
		fmt.Printf("Created user: %s (ID: %d, Role: %s)\n", u.Email, user.ID, u.Role)

		// Set permissions for the new user
		if err := repo.UpdateUserPermissions(ctx, user.ID, u.Permissions); err != nil {
			log.Printf("Failed to set permissions for %s: %v", u.Email, err)
		} else {
			fmt.Printf("Assigned permissions to %s: %v\n", u.Email, u.Permissions)
		}

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

	// 2. Set Overdraft Limits
	fmt.Println("Setting overdraft limits...")
	overdraftLimits := []struct {
		AccountID int
		Limit     int64
		Type      string
	}{
		{createdAccounts[0].ID, 50000, "Premium Account ($500 overdraft)"},  // Admin gets premium account
		{createdAccounts[1].ID, 20000, "Standard Account ($200 overdraft)"}, // Manager gets standard
		{createdAccounts[2].ID, 0, "Basic Account (no overdraft)"},          // User has no overdraft
	}

	for _, od := range overdraftLimits {
		_, err := repo.UpdateOverdraftLimit(ctx, od.AccountID, od.Limit)
		if err != nil {
			log.Printf("Failed to set overdraft for account %d: %v", od.AccountID, err)
		} else {
			fmt.Printf("Set overdraft for Account %d: %s\n", od.AccountID, od.Type)
		}
	}

	// 3. Simulate Deposits
	fmt.Println("Simulating deposits...")
	deposits := []struct {
		AccountIndex int
		Amount       int64
		Reference    string
	}{
		{0, 25000, "Salary deposit"},
		{1, 15000, "Freelance payment"},
		{2, 10000, "Cash deposit"},
		{0, 5000, "Refund"},
		{1, 8000, "Bonus"},
	}

	for i, dep := range deposits {
		if dep.AccountIndex < len(createdAccounts) {
			acc := createdAccounts[dep.AccountIndex]
			fmt.Printf("Depositing %d cents to Account %d (%s)...\n", dep.Amount, acc.ID, dep.Reference)
			_, err := repo.Deposit(ctx, acc.ID, dep.Amount, fmt.Sprintf("seed-deposit-%d-%s", i, dep.Reference))
			if err != nil {
				log.Printf("Deposit failed: %v", err)
			} else {
				fmt.Printf("Deposit successful: %s\n", dep.Reference)
			}
		}
	}

	// 4. Simulate Transfers
	fmt.Println("Simulating transfers...")
	transfers := []struct {
		FromIndex int
		ToIndex   int
		Amount    int64
		Reference string
	}{
		{0, 1, 5000, "Loan payment"},
		{1, 2, 3000, "Gift"},
		{0, 2, 10000, "Invoice payment"},
		{2, 0, 2000, "Repayment"},
		{1, 0, 4000, "Shared expense"},
	}

	for i, tr := range transfers {
		if tr.FromIndex < len(createdAccounts) && tr.ToIndex < len(createdAccounts) {
			from := createdAccounts[tr.FromIndex]
			to := createdAccounts[tr.ToIndex]
			fmt.Printf("Transferring %d cents from Account %d to Account %d (%s)...\n", tr.Amount, from.ID, to.ID, tr.Reference)
			_, _, err := repo.Transfer(ctx, from.ID, to.ID, tr.Amount, fmt.Sprintf("seed-transfer-%d-%s", i, tr.Reference))
			if err != nil {
				log.Printf("Transfer failed: %v", err)
			} else {
				fmt.Printf("Transfer successful: %s\n", tr.Reference)
			}
		}
	}

	// 5. Simulate Withdrawals
	fmt.Println("Simulating withdrawals...")
	withdrawals := []struct {
		AccountIndex int
		Amount       int64
		Reference    string
	}{
		{0, 10000, "ATM withdrawal"},
		{1, 5000, "Cash withdrawal"},
		{2, 3000, "ATM withdrawal"},
		{0, 2000, "Bank teller"},
		{1, 1500, "ATM withdrawal"},
	}

	for i, wd := range withdrawals {
		if wd.AccountIndex < len(createdAccounts) {
			acc := createdAccounts[wd.AccountIndex]
			fmt.Printf("Withdrawing %d cents from Account %d (%s)...\n", wd.Amount, acc.ID, wd.Reference)
			_, err := repo.Withdraw(ctx, acc.ID, wd.Amount, fmt.Sprintf("seed-withdraw-%d-%s", i, wd.Reference))
			if err != nil {
				log.Printf("Withdrawal failed: %v", err)
			} else {
				fmt.Printf("Withdrawal successful: %s\n", wd.Reference)
			}
		}
	}

	// 6. Create Additional Accounts (multiple accounts per user + edge cases)
	fmt.Println("Creating additional accounts...")
	additionalAccounts := []struct {
		UserIndex int
		Balance   int64
		Type      string
	}{
		{0, 200000, "Savings Account"},  // Admin gets a savings account
		{1, 100000, "Business Account"}, // Manager gets a business account
		{2, 0, "Zero Balance Account"},  // Edge case: zero balance account
		{0, 1, "Micro Balance Account"}, // Edge case: $0.01 balance
	}

	for _, addAcc := range additionalAccounts {
		if addAcc.UserIndex < len(createdAccounts) {
			userID := createdAccounts[addAcc.UserIndex].UserID
			acc, err := repo.CreateAccount(ctx, userID, addAcc.Balance)
			if err != nil {
				log.Printf("Failed to create %s for user %d: %v", addAcc.Type, userID, err)
			} else {
				fmt.Printf("Created %s (ID: %d) for user %d with balance $%.2f\n",
					addAcc.Type, acc.ID, userID, float64(addAcc.Balance)/100)
			}
		}
	}

	// 7. Simulate Transaction Failures (Edge Cases)
	fmt.Println("\nSimulating transaction failures (expected errors)...")

	// Test insufficient funds
	fmt.Println("Testing insufficient funds...")
	if len(createdAccounts) > 2 {
		charlieAccount := createdAccounts[2] // Charlie (User) has ~$750 balance
		_, err := repo.Withdraw(ctx, charlieAccount.ID, 100000000, "seed-failure-insufficient-funds")
		if err != nil {
			fmt.Printf("✓ Insufficient funds error caught (expected): %v\n", err)
		} else {
			log.Printf("⚠ WARNING: Withdrawal should have failed due to insufficient funds!")
		}
	}

	// Test overdraft limit exceeded
	fmt.Println("Testing overdraft limit...")
	if len(createdAccounts) > 2 {
		charlieAccount := createdAccounts[2] // Charlie has $0 overdraft limit
		currentBalance := charlieAccount.Balance
		_, err := repo.Withdraw(ctx, charlieAccount.ID, currentBalance+1000, "seed-failure-overdraft-exceeded")
		if err != nil {
			fmt.Printf("✓ Overdraft limit error caught (expected): %v\n", err)
		} else {
			log.Printf("⚠ WARNING: Withdrawal should have failed due to overdraft limit!")
		}
	}

	// Test transfer to non-existent account
	fmt.Println("Testing transfer to invalid account...")
	if len(createdAccounts) > 0 {
		aliceAccount := createdAccounts[0]
		_, _, err := repo.Transfer(ctx, aliceAccount.ID, 999999, 100, "seed-failure-invalid-account")
		if err != nil {
			fmt.Printf("✓ Invalid account error caught (expected): %v\n", err)
		} else {
			log.Printf("⚠ WARNING: Transfer should have failed due to invalid account!")
		}
	}

	// Test transfer from account with insufficient balance
	fmt.Println("Testing transfer with insufficient balance...")
	if len(createdAccounts) > 1 {
		charlieAccount := createdAccounts[2]
		aliceAccount := createdAccounts[0]
		_, _, err := repo.Transfer(ctx, charlieAccount.ID, aliceAccount.ID, 50000000, "seed-failure-insufficient-for-transfer")
		if err != nil {
			fmt.Printf("✓ Insufficient balance error caught (expected): %v\n", err)
		} else {
			log.Printf("⚠ WARNING: Transfer should have failed due to insufficient balance!")
		}
	}

	// Test negative amount (should be prevented)
	fmt.Println("Testing negative amount withdrawal...")
	if len(createdAccounts) > 0 {
		aliceAccount := createdAccounts[0]
		_, err := repo.Withdraw(ctx, aliceAccount.ID, -1000, "seed-failure-negative-amount")
		if err != nil {
			fmt.Printf("✓ Negative amount error caught (expected): %v\n", err)
		} else {
			log.Printf("⚠ WARNING: Negative amount withdrawal should have been prevented!")
		}
	}

	fmt.Println("\n✓ Seeding complete with edge case validation.")
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
