package service

import (
	"context"

	"mini-bank/internal/core"
	"mini-bank/internal/storage"
)

type Service interface {
	CreateAccount(ctx context.Context, userID int, balance int64) (*core.Account, error)
	GetAccount(ctx context.Context, id int) (*core.Account, error)
	ListAccounts(ctx context.Context) ([]*core.Account, error)
	Transfer(ctx context.Context, fromID, toID int, amount int64, reference string) (*core.Account, *core.Account, error)
	Payment(ctx context.Context, accountID int, amount int64, pType storage.PaymentType, reference string) (*core.Account, error)
	ListTransactions(ctx context.Context, accountID int) ([]*core.Transaction, error)
	GetTransaction(ctx context.Context, reference string) (*core.Transaction, error)
	CreateUser(ctx context.Context, firstName string, lastName string, email string, password string) (*core.User, error)
}

type service struct {
	store storage.Storage
}

func New(store storage.Storage) Service {
	return &service{store: store}
}

func (s *service) CreateAccount(ctx context.Context, userID int, balance int64) (*core.Account, error) {
	return s.store.CreateAccount(ctx, userID, balance)
}

func (s *service) GetAccount(ctx context.Context, id int) (*core.Account, error) {
	return s.store.GetAccount(ctx, id)
}

func (s *service) ListAccounts(ctx context.Context) ([]*core.Account, error) {
	return s.store.ListAccounts(ctx)
}

func (s *service) Transfer(ctx context.Context, fromID, toID int, amount int64, reference string) (*core.Account, *core.Account, error) {
	return s.store.Transfer(ctx, fromID, toID, amount, reference)
}

func (s *service) Payment(ctx context.Context, accountID int, amount int64, pType storage.PaymentType, reference string) (*core.Account, error) {
	return s.store.Payment(ctx, accountID, amount, pType, reference)
}

func (s *service) ListTransactions(ctx context.Context, accountID int) ([]*core.Transaction, error) {
	return s.store.ListTransactions(ctx, accountID)
}

func (s *service) GetTransaction(ctx context.Context, reference string) (*core.Transaction, error) {
	return s.store.GetTransaction(ctx, reference)
}

func (s *service) CreateUser(ctx context.Context, firstName string, lastName string, email string, password string) (*core.User, error) {
	return s.store.CreateUser(ctx, firstName, lastName, email, password)
}
