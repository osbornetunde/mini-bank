package storage

import (
	"context"
	"errors"
	"time"

	"mini-bank/internal/core"
)

var (
	ErrAccountNotFound     = errors.New("account not found")
	ErrInsufficientFunds   = errors.New("insufficient funds")
	ErrTransactionNotFound = errors.New("transaction not found")
	ErrUserNotFound        = errors.New("user not found")
	ErrDuplicateEmail      = errors.New("duplicate email")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrInvalidResetToken   = errors.New("invalid or expired reset token")
)

type PaymentType string

const (
	Deposit  PaymentType = "deposit"
	Withdraw PaymentType = "withdraw"
)

// Storage defines how accounts and transactions are persisted.
type Storage interface {
	CreateAccount(ctx context.Context, userID int, initialBalance int64) (*core.Account, error)
	GetAccount(ctx context.Context, id int) (*core.Account, error)
	ListAccounts(ctx context.Context) ([]*core.Account, error)
	UpdateBalance(ctx context.Context, id int, newBalance int64) error

	RecordTransaction(ctx context.Context, tx *core.Transaction) error
	ListTransactions(ctx context.Context, accountID int) ([]*core.Transaction, error)
	GetTransaction(ctx context.Context, ref string) (*core.Transaction, error)

	Transfer(ctx context.Context, fromID, toID int, amount int64, reference string) (*core.Account, *core.Account, error)
	Payment(ctx context.Context, accountID int, amount int64, paymentType PaymentType, reference string) (*core.Account, error)
	CreateUser(ctx context.Context, firstName string, lastName string, email string, password string) (*core.User, error)
	GetUsers(ctx context.Context) ([]*core.User, error)
	GetUser(ctx context.Context, id int) (*core.User, error)
	UpdateUser(ctx context.Context, id int, firstName string, lastName string, email string) (*core.User, error)
	DeleteUser(ctx context.Context, id int) error
	GetUserByEmail(ctx context.Context, email string) (*core.User, error)

	CreatePasswordResetToken(ctx context.Context, userID int, tokenHash string, expiresAt time.Time) error
	GetPasswordResetToken(ctx context.Context, tokenHash string) (userID int, expiresAt time.Time, usedAt *time.Time, err error)
	MarkPasswordResetTokenAsUsed(ctx context.Context, tokenHash string) error
	InvalidateUserPasswordResetTokens(ctx context.Context, userID int) error
	UpdateUserPassword(ctx context.Context, userID int, hashedPassword string) error
	CleanupExpiredPasswordResetTokens(ctx context.Context) (int64, error)
	ResetPasswordTx(ctx context.Context, tokenHash string, hashedPassword string) (userID int, err error)
}
