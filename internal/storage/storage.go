package core

import (
	"context"
	"mini-bank/internal/core"
)

type Storage interface {
	CreateAccount(ctx context.Context, name string, initialBalance float64) (*core.Account, error)
	GetAccount(ctx context.Context, id int) (*core.Account, error)
	ListAccounts(ctx context.Context) ([]*core.Account, error)
	UpdateBalance(ctx context.Context, id int, newBalance float64) error
	RecordTransaction(ctx context.Context, tx *core.Transaction) error
	ListTransactions(ctx context.Context, accountID int) ([]*core.Transaction, error)
}
