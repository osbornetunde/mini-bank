package storage

import (
	"context"
	"errors"
	"mini-bank/internal/core"
)

var (
	ErrAccountNotFound   = errors.New("account not found")
	ErrInsufficientFunds = errors.New("insufficient funds")
)

// Storage defines how accounts and transactions are persisted.
type Storage interface {
	CreateAccount(ctx context.Context, name string, initialBalance float64) (*core.Account, error)
	GetAccount(ctx context.Context, id int) (*core.Account, error)
	ListAccounts(ctx context.Context) ([]*core.Account, error)
	UpdateBalance(ctx context.Context, id int, newBalance float64) error

	RecordTransaction(ctx context.Context, tx *core.Transaction) error
	ListTransactions(ctx context.Context, accountID int) ([]*core.Transaction, error)

	Transfer(ctx context.Context, fromID, toID int, amount float64) (*core.Account, *core.Account, error)
	Deposit(ctx context.Context, accountID int, amount float64) (*core.Account, error)
}
