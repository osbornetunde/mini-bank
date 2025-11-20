package storage

import (
	"context"
	"errors"

	"mini-bank/internal/core"
)

var (
	ErrAccountNotFound     = errors.New("account not found")
	ErrInsufficientFunds   = errors.New("insufficient funds")
	ErrTransactionNotFound = errors.New("transaction not found")
)

type PaymentType string

const (
	Deposit  PaymentType = "deposit"
	Withdraw PaymentType = "withdraw"
)

// Storage defines how accounts and transactions are persisted.
type Storage interface {
	CreateAccount(ctx context.Context, name string, initialBalance int64) (*core.Account, error)
	GetAccount(ctx context.Context, id int) (*core.Account, error)
	ListAccounts(ctx context.Context) ([]*core.Account, error)
	UpdateBalance(ctx context.Context, id int, newBalance int64) error

	RecordTransaction(ctx context.Context, tx *core.Transaction) error
	ListTransactions(ctx context.Context, accountID int) ([]*core.Transaction, error)
	GetTransaction(ctx context.Context, ref string) (*core.Transaction, error)

	Transfer(ctx context.Context, fromID, toID int, amount int64, reference string) (*core.Account, *core.Account, error)
	Payment(ctx context.Context, accountID int, amount int64, paymentType PaymentType, reference string) (*core.Account, error)
}
