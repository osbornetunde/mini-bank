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

func (m *MockStorage) ListTransactionsPaginated(ctx context.Context, accountID int, filters storage.TransactionFilters, pagination storage.PaginationParams) (*storage.PaginatedResult, error) {
	args := m.Called(ctx, accountID, filters, pagination)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*storage.PaginatedResult), args.Error(1)
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

func (m *MockStorage) GetUsers(ctx context.Context, pagination storage.PaginationParams) (*storage.UsersPaginatedResult, error) {
	args := m.Called(ctx, pagination)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*storage.UsersPaginatedResult), args.Error(1)
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

func TestService_GetUsers_WithZeroAccounts(t *testing.T) {
	mockStore := new(MockStorage)
	mockEmail := new(MockEmailSender)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	s := New(mockStore, mockEmail, logger)

	zeroBalance := int64(0)
	usersData := []*core.User{
		{
			ID:          1,
			FirstName:   "John",
			LastName:    "Doe",
			Email:       "john@example.com",
			Balance:     &zeroBalance,
			Permissions: []string{},
			Accounts:    []*core.Account{}, // No accounts
		},
	}

	mockStore.On("GetUsers", mock.Anything, mock.Anything).Return(&storage.UsersPaginatedResult{Users: usersData, TotalCount: 1}, nil)

	result, err := s.GetUsers(context.Background(), storage.PaginationParams{Limit: 10, Offset: 0})

	assert.NoError(t, err)
	assert.Len(t, result.Users, 1)
	assert.Equal(t, "John", result.Users[0].FirstName)
	assert.Empty(t, result.Users[0].Accounts)
	assert.Equal(t, int64(0), *result.Users[0].Balance)
	mockStore.AssertExpectations(t)
}

func TestService_GetUsers_WithMultipleAccounts(t *testing.T) {
	mockStore := new(MockStorage)
	mockEmail := new(MockEmailSender)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	s := New(mockStore, mockEmail, logger)

	now := time.Now()
	totalBalance := int64(2500) // 1000 + 1500
	usersData := []*core.User{
		{
			ID:          1,
			FirstName:   "Jane",
			LastName:    "Smith",
			Email:       "jane@example.com",
			Balance:     &totalBalance,
			Permissions: []string{"accounts_read"},
			Accounts: []*core.Account{
				{ID: 1, UserID: 1, Balance: 1000, OverdraftLimit: 500, CreatedAt: now},
				{ID: 2, UserID: 1, Balance: 1500, OverdraftLimit: 0, CreatedAt: now},
			},
		},
	}

	mockStore.On("GetUsers", mock.Anything, mock.Anything).Return(&storage.UsersPaginatedResult{Users: usersData, TotalCount: 1}, nil)

	result, err := s.GetUsers(context.Background(), storage.PaginationParams{Limit: 10, Offset: 0})

	assert.NoError(t, err)
	assert.Len(t, result.Users, 1)
	assert.Len(t, result.Users[0].Accounts, 2)
	assert.Equal(t, int64(2500), *result.Users[0].Balance)

	// Verify individual account details
	assert.Equal(t, int64(1000), result.Users[0].Accounts[0].Balance)
	assert.Equal(t, int64(1500), result.Users[0].Accounts[1].Balance)
	mockStore.AssertExpectations(t)
}

func TestService_GetUsers_AggregatedBalanceWithNegative(t *testing.T) {
	mockStore := new(MockStorage)
	mockEmail := new(MockEmailSender)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	s := New(mockStore, mockEmail, logger)

	now := time.Now()
	// One account positive, one negative (overdraft) = net 700
	totalBalance := int64(700) // 1000 + (-300)
	usersData := []*core.User{
		{
			ID:          1,
			FirstName:   "Bob",
			LastName:    "Wilson",
			Email:       "bob@example.com",
			Balance:     &totalBalance,
			Permissions: []string{},
			Accounts: []*core.Account{
				{ID: 1, UserID: 1, Balance: 1000, OverdraftLimit: 0, CreatedAt: now},
				{ID: 2, UserID: 1, Balance: -300, OverdraftLimit: 500, CreatedAt: now}, // Overdraft
			},
		},
	}

	mockStore.On("GetUsers", mock.Anything, mock.Anything).Return(&storage.UsersPaginatedResult{Users: usersData, TotalCount: 1}, nil)

	result, err := s.GetUsers(context.Background(), storage.PaginationParams{Limit: 10, Offset: 0})

	assert.NoError(t, err)
	assert.Len(t, result.Users, 1)
	assert.Equal(t, int64(700), *result.Users[0].Balance)

	// Verify negative balance account is included
	assert.Equal(t, int64(-300), result.Users[0].Accounts[1].Balance)
	mockStore.AssertExpectations(t)
}

func TestService_GetUser_WithZeroAccounts(t *testing.T) {
	mockStore := new(MockStorage)
	mockEmail := new(MockEmailSender)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	s := New(mockStore, mockEmail, logger)

	zeroBalance := int64(0)
	userData := &core.User{
		ID:          1,
		FirstName:   "Alice",
		LastName:    "Brown",
		Email:       "alice@example.com",
		Balance:     &zeroBalance,
		Permissions: []string{},
		Accounts:    nil,
	}

	mockStore.On("GetUser", mock.Anything, 1).Return(userData, nil)

	user, err := s.GetUser(context.Background(), 1)

	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "Alice", user.FirstName)
	assert.Equal(t, int64(0), *user.Balance)
	mockStore.AssertExpectations(t)
}

func TestService_GetUser_NotFound(t *testing.T) {
	mockStore := new(MockStorage)
	mockEmail := new(MockEmailSender)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	s := New(mockStore, mockEmail, logger)

	mockStore.On("GetUser", mock.Anything, 999).Return(nil, storage.ErrUserNotFound)

	user, err := s.GetUser(context.Background(), 999)

	assert.Nil(t, user)
	assert.ErrorIs(t, err, storage.ErrUserNotFound)
	mockStore.AssertExpectations(t)
}
