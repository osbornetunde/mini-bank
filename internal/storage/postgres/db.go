package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"
	
	_"github.com/jackc/pgx/v5/stdlib"
)

type DB struct {
	*sql.DB
}


func NewDB(dsn string) (*DB, error){
	
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	
	db.SetConnMaxIdleTime(10 * time.Minute)
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	
	return &DB{DB: db}, nil
}
