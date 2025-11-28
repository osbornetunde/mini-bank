package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	// I used "pgx" driver here because "postgres" (lib/pq) wasn't available
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping db: %v", err)
	}

	migration := `
	ALTER TABLE accounts
    ALTER COLUMN balance TYPE BIGINT USING (balance * 100)::BIGINT;

	ALTER TABLE transactions
    ALTER COLUMN amount TYPE BIGINT USING (amount * 100)::BIGINT;
	`

	_, err = db.Exec(migration)
	if err != nil {
		log.Fatalf("failed to execute migration: %v", err)
	}

	fmt.Println("Migration executed successfully")
}