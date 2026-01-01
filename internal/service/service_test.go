package service

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"mini-bank/internal/core"
	"mini-bank/internal/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockStorage is a mock of the storage.Storage interface
type MockStorage struct {
	mock.Mock
}

func (m *MockStorage) CreateAccount(ctx context.Context, userID int, balance int64) (*core.Account, error) {
	args := m.Called(ctx, userID, balance)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*core.Account), args.Error(1)
}

func (m *MockStorage) GetAccount(ctx context.Context, id int) (*core.Account, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*core.Account), args.Error(1)
}

func (m *MockStorage) ListAccounts(ctx context.Context) ([]*core.Account, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*core.Account), args.Error(1)
}

func (m *MockStorage) UpdateBalance(ctx context.Context, id int, newBalance int64) error {
	args := m.Called(ctx, id, newBalance)
	return args.Error(0)
}

func (m *MockStorage) UpdateOverdraftLimit(ctx context.Context, accountID int, newLimit int64) (*core.Account, error) {
	args := m.Called(ctx, accountID, newLimit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*core.Account), args.Error(1)
}

func (m *MockStorage) RecordTransaction(ctx context.Context, tx *core.Transaction) error {
	args := m.Called(ctx, tx)
	return args.Error(0)
}

func (m *MockStorage) ListTransactions(ctx context.Context, accountID int) ([]*core.Transaction, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*core.Transaction), args.Error(1)
}

func (m *MockStorage) GetTransaction(ctx context.Context, ref string) (*core.Transaction, error) {
	args := m.Called(ctx, ref)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*core.Transaction), args.Error(1)
}

func (m *MockStorage) Transfer(ctx context.Context, fromID, toID int, amount int64, reference string) (*core.Account, *core.Account, error) {
	args := m.Called(ctx, fromID, toID, amount, reference)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	return args.Get(0).(*core.Account), args.Get(1).(*core.Account), args.Error(2)
}

func (m *MockStorage) Payment(ctx context.Context, accountID int, amount int64, paymentType storage.PaymentType, reference string) (*core.Account, error) {
	args := m.Called(ctx, accountID, amount, paymentType, reference)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*core.Account), args.Error(1)
}

func (m *MockStorage) CreateUser(ctx context.Context, firstName, lastName, email, password string) (*core.User, error) {
	args := m.Called(ctx, firstName, lastName, email, password)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*core.User), args.Error(1)
}

func (m *MockStorage) CreateUserWithAccount(ctx context.Context, firstName, lastName, email, password string, initialBalance int64) (*core.User, error) {
	args := m.Called(ctx, firstName, lastName, email, password, initialBalance)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*core.User), args.Error(1)
}

func (m *MockStorage) GetUsers(ctx context.Context) ([]*core.User, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*core.User), args.Error(1)
}

func (m *MockStorage) GetUser(ctx context.Context, id int) (*core.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*core.User), args.Error(1)
}

func (m *MockStorage) UpdateUser(ctx context.Context, id int, firstName, lastName, email string) (*core.User, error) {
	args := m.Called(ctx, id, firstName, lastName, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*core.User), args.Error(1)
}

func (m *MockStorage) DeleteUser(ctx context.Context, id int) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockStorage) GetUserByEmail(ctx context.Context, email string) (*core.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*core.User), args.Error(1)
}

func (m *MockStorage) UpdateUserPermissions(ctx context.Context, userID int, permissions []string) error {
	args := m.Called(ctx, userID, permissions)
	return args.Error(0)
}

func (m *MockStorage) CreatePasswordResetToken(ctx context.Context, userID int, tokenHash string, expiresAt time.Time) error {
	args := m.Called(ctx, userID, tokenHash, expiresAt)
	return args.Error(0)
}

func (m *MockStorage) GetPasswordResetToken(ctx context.Context, tokenHash string) (int, time.Time, *time.Time, error) {
	args := m.Called(ctx, tokenHash)
	return args.Int(0), args.Get(1).(time.Time), args.Get(2).(*time.Time), args.Error(3)
}

func (m *MockStorage) MarkPasswordResetTokenAsUsed(ctx context.Context, tokenHash string) error {
	args := m.Called(ctx, tokenHash)
	return args.Error(0)
}

func (m *MockStorage) InvalidateUserPasswordResetTokens(ctx context.Context, userID int) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockStorage) UpdateUserPassword(ctx context.Context, userID int, hashedPassword string) error {
	args := m.Called(ctx, userID, hashedPassword)
	return args.Error(0)
}

func (m *MockStorage) CleanupExpiredPasswordResetTokens(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockStorage) ResetPasswordTx(ctx context.Context, tokenHash, hashedPassword string) (int, error) {
	args := m.Called(ctx, tokenHash, hashedPassword)
	return args.Int(0), args.Error(1)
}

func (m *MockStorage) CreateAuditLog(ctx context.Context, log *core.AuditLog) error {
	args := m.Called(ctx, log)
	return args.Error(0)
}

func (m *MockStorage) Withdraw(ctx context.Context, accountID int, amount int64, reference string) (*core.Account, error) {
	args := m.Called(ctx, accountID, amount, reference)
	return args.Get(0).(*core.Account), args.Error(1)
}

// MockEmailSender is a mock of the service.EmailSender interface
type MockEmailSender struct {
	mock.Mock
}

func (m *MockEmailSender) SendPasswordResetEmail(email, token string) error {
	args := m.Called(email, token)
	return args.Error(0)
}

func (m *MockEmailSender) SendPasswordChangedEmail(email string) error {
	args := m.Called(email)
	return args.Error(0)
}

func TestService_CreateUser(t *testing.T) {
	mockStore := new(MockStorage)
	mockEmail := new(MockEmailSender)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	s := New(mockStore, mockEmail, logger)

	firstName := "John"
	lastName := "Doe"
	email := "john@example.com"
	password := "Password123!"

	mockStore.On("CreateUserWithAccount", mock.Anything, firstName, lastName, email, mock.Anything, int64(0)).
		Return(&core.User{ID: 1, FirstName: firstName, LastName: lastName, Email: email, Permissions: []string{}}, nil)
	mockStore.On("CreateAuditLog", mock.Anything, mock.Anything).Return(nil)

	user, err := s.CreateUser(context.Background(), firstName, lastName, email, password)

	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, firstName, user.FirstName)
	mockStore.AssertExpectations(t)
}

func TestService_Transfer(t *testing.T) {
	mockStore := new(MockStorage)
	mockEmail := new(MockEmailSender)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	s := New(mockStore, mockEmail, logger)

	fromID := 1
	toID := 2
	amount := int64(100)
	ref := "test-ref"

	fromAcc := &core.Account{ID: fromID, Balance: 900}
	toAcc := &core.Account{ID: toID, Balance: 1100}

	mockStore.On("Transfer", mock.Anything, fromID, toID, amount, ref).
		Return(fromAcc, toAcc, nil)

	resFrom, resTo, err := s.Transfer(context.Background(), fromID, toID, amount, ref)

	assert.NoError(t, err)
	assert.Equal(t, fromAcc, resFrom)
	assert.Equal(t, toAcc, resTo)
	mockStore.AssertExpectations(t)
}
