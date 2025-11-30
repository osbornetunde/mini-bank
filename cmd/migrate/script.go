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

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping db: %v", err)
	}

	if len(os.Args) < 2 {
		log.Fatal("Usage: go run cmd/migrate/script.go <path-to-migration-file>")
	}
	migrationFile := os.Args[1]

	// Read the migration file
	migrationBytes, err := os.ReadFile(migrationFile)
	if err != nil {
		log.Fatalf("failed to read migration file: %v", err)
	}
	migration := string(migrationBytes)

	_, err = db.Exec(migration)
	if err != nil {
		log.Fatalf("failed to execute migration: %v", err)
	}

	fmt.Println("Migration executed successfully")
}