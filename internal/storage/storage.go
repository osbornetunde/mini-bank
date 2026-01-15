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
	ErrAccountLocked       = errors.New("account temporarily locked due to multiple failed login attempts")
	ErrFeeRuleNotFound     = errors.New("fee rule not found")
)

type PaymentType string

const (
	Deposit  PaymentType = "deposit"
	Withdraw PaymentType = "withdraw"
)

// TransactionFilters contains optional filters for listing transactions
type TransactionFilters struct {
	Status    string     // Filter by transaction type: "deposit", "withdraw", "transfer"
	Reference string     // Filter by exact transaction reference
	DateFrom  *time.Time // Transactions from this date (inclusive)
	DateTo    *time.Time // Transactions until this date (inclusive)
}

// PaginationParams contains pagination parameters
type PaginationParams struct {
	Limit  int // Number of items per page
	Offset int // Number of items to skip
}

// PaginatedResult contains paginated transaction results
type PaginatedResult struct {
	Transactions []*core.Transaction
	TotalCount   int64
}

// UsersPaginatedResult contains paginated user results
type UsersPaginatedResult struct {
	Users      []*core.User
	TotalCount int64
}

// Storage defines how accounts and transactions are persisted.
type Storage interface {
	CreateAccount(ctx context.Context, userID int, initialBalance int64) (*core.Account, error)
	GetAccount(ctx context.Context, id int) (*core.Account, error)
	ListAccounts(ctx context.Context) ([]*core.Account, error)
	UpdateBalance(ctx context.Context, id int, newBalance int64) error
	UpdateOverdraftLimit(ctx context.Context, accountID int, newLimit int64) (*core.Account, error)

	RecordTransaction(ctx context.Context, tx *core.Transaction) error
	ListTransactions(ctx context.Context, accountID int) ([]*core.Transaction, error)
	ListTransactionsPaginated(ctx context.Context, accountID int, filters TransactionFilters, pagination PaginationParams) (*PaginatedResult, error)
	GetTransaction(ctx context.Context, ref string) (*core.Transaction, error)

	Transfer(ctx context.Context, fromID, toID int, amount int64, reference string, feeAmount int64) (*core.Account, *core.Account, error)
	Payment(ctx context.Context, accountID int, amount int64, paymentType PaymentType, reference string) (*core.Account, error)
	CreateUser(ctx context.Context, firstName string, lastName string, email string, password string) (*core.User, error)
	CreateUserWithAccount(ctx context.Context, firstName string, lastName string, email string, password string, initialBalance int64) (*core.User, error)
	GetUsers(ctx context.Context, pagination PaginationParams) (*UsersPaginatedResult, error)
	GetUser(ctx context.Context, id int) (*core.User, error)
	UpdateUser(ctx context.Context, id int, firstName string, lastName string, email string) (*core.User, error)
	DeleteUser(ctx context.Context, id int) error
	GetUserByEmail(ctx context.Context, email string) (*core.User, error)
	UpdateUserPermissions(ctx context.Context, userID int, permissions []string) error

	CreatePasswordResetToken(ctx context.Context, userID int, tokenHash string, expiresAt time.Time) error
	GetPasswordResetToken(ctx context.Context, tokenHash string) (userID int, expiresAt time.Time, usedAt *time.Time, err error)
	MarkPasswordResetTokenAsUsed(ctx context.Context, tokenHash string) error
	InvalidateUserPasswordResetTokens(ctx context.Context, userID int) error
	UpdateUserPassword(ctx context.Context, userID int, hashedPassword string) error
	CleanupExpiredPasswordResetTokens(ctx context.Context) (int64, error)
	ResetPasswordTx(ctx context.Context, tokenHash string, hashedPassword string) (userID int, err error)

	CreateAuditLog(ctx context.Context, log *core.AuditLog) error
	Withdraw(ctx context.Context, accountID int, amount int64, reference string, feeAmount int64) (*core.Account, error)

	// Fee tier management
	CreateFeeTier(ctx context.Context, tier *core.FeeTier) (*core.FeeTier, error)
	GetApplicableFeeTier(ctx context.Context, transactionType string, amount int64) (*core.FeeTier, error)
	ListFeeTiers(ctx context.Context, transactionType *string, activeOnly bool) ([]*core.FeeTier, error)
	UpdateFeeTier(ctx context.Context, tier *core.FeeTier) error
	DeleteFeeTier(ctx context.Context, id int) error
	ListUserAccounts(ctx context.Context, userID int) ([]*core.Account, error)
}
