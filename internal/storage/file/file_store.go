package file

import (
	"context"
	"encoding/json"
	"fmt"
	"mini-bank/internal/core"
	"mini-bank/internal/storage"
	"os"
	"sync"
	"time"
)

type FileStore struct {
	accountsFile     string
	transactionsFile string

	mu           sync.RWMutex
	accounts     map[int]*core.Account
	transactions []*core.Transaction
	nextID       int
}

// NewFileStore creates a new file-based store with given JSON file paths.
func NewFileStore(accountsFile, transactionsFile string) (*FileStore, error) {
	store := &FileStore{
		accountsFile:     accountsFile,
		transactionsFile: transactionsFile,
		accounts:         make(map[int]*core.Account),
	}

	if err := store.loadAccounts(); err != nil {
		return nil, err
	}
	if err := store.loadTransactions(); err != nil {
		return nil, err
	}

	return store, nil
}

// loadAccounts reads accounts from JSON file.
func (s *FileStore) loadAccounts() error {
	file, err := os.Open(s.accountsFile)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, start fresh
			return nil
		}
		return err
	}
	defer file.Close()

	var accounts []*core.Account
	if err := json.NewDecoder(file).Decode(&accounts); err != nil {
		return err
	}

	maxID := 0
	for _, acc := range accounts {
		s.accounts[acc.ID] = acc
		if acc.ID > maxID {
			maxID = acc.ID
		}
	}
	s.nextID = maxID
	return nil
}

// loadTransactions reads transactions from JSON file.
func (s *FileStore) loadTransactions() error {
	file, err := os.Open(s.transactionsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	var transactions []*core.Transaction
	if err := json.NewDecoder(file).Decode(&transactions); err != nil {
		return err
	}

	s.transactions = transactions
	return nil
}

// saveAccounts writes accounts to JSON file.
func (s *FileStore) saveAccounts() error {

	accountsSlice := make([]*core.Account, 0, len(s.accounts))
	for _, acc := range s.accounts {
		accountsSlice = append(accountsSlice, acc)
	}

	data, err := json.MarshalIndent(accountsSlice, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.accountsFile, data, 0644)
}

// saveTransactions writes transactions to JSON file.
func (s *FileStore) saveTransactions() error {

	data, err := json.MarshalIndent(s.transactions, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.transactionsFile, data, 0644)
}

// CreateAccount implements Storage interface.
func (s *FileStore) CreateAccount(ctx context.Context, userID int, initialBalance int64) (*core.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextID++
	acc := &core.Account{ID: s.nextID, UserID: userID, Balance: initialBalance, OverdraftLimit: 0}
	s.accounts[acc.ID] = acc

	if err := s.saveAccounts(); err != nil {
		return nil, err
	}
	return acc, nil
}

// GetAccount retrieves an account by ID.
func (s *FileStore) GetAccount(ctx context.Context, id int) (*core.Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	acc, ok := s.accounts[id]
	if !ok {
		return nil, fmt.Errorf("account not found")
	}
	return acc, nil
}

// ListAccounts returns all accounts.
func (s *FileStore) ListAccounts(ctx context.Context) ([]*core.Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	accounts := make([]*core.Account, 0, len(s.accounts))
	for _, acc := range s.accounts {
		accounts = append(accounts, acc)
	}
	return accounts, nil
}

// UpdateBalance updates account balance.
func (s *FileStore) UpdateBalance(ctx context.Context, id int, newBalance int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	acc, ok := s.accounts[id]
	if !ok {
		return fmt.Errorf("account not found")
	}
	acc.Balance = newBalance

	return s.saveAccounts()
}

// RecordTransaction saves a new transaction.
func (s *FileStore) RecordTransaction(ctx context.Context, tx *core.Transaction) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.transactions = append(s.transactions, tx)
	return s.saveTransactions()
}

// ListTransactions returns all transactions for an account.
func (s *FileStore) ListTransactions(ctx context.Context, accountID int) ([]*core.Transaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*core.Transaction
	for _, t := range s.transactions {
		if t.AccountID == accountID {
			result = append(result, t)
		}
	}
	return result, nil
}

// ListTransactionsPaginated returns paginated transactions for an account with optional filters
func (s *FileStore) ListTransactionsPaginated(ctx context.Context, accountID int, filters storage.TransactionFilters, pagination storage.PaginationParams) (*storage.PaginatedResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// First, filter transactions by account ID and optional filters
	var filtered []*core.Transaction
	for _, t := range s.transactions {
		if t.AccountID != accountID {
			continue
		}

		// Apply status filter
		if filters.Status != "" && t.Type != filters.Status {
			continue
		}

		// Apply reference filter
		if filters.Reference != "" && t.Reference != filters.Reference {
			continue
		}

		// Apply date range filters
		if filters.DateFrom != nil && t.Timestamp.Before(*filters.DateFrom) {
			continue
		}
		if filters.DateTo != nil && t.Timestamp.After(*filters.DateTo) {
			continue
		}

		filtered = append(filtered, t)
	}

	totalCount := int64(len(filtered))

	// Apply pagination
	start := pagination.Offset
	end := pagination.Offset + pagination.Limit

	// Handle bounds
	if start > len(filtered) {
		start = len(filtered)
	}
	if end > len(filtered) {
		end = len(filtered)
	}

	// Slice for pagination
	var paginated []*core.Transaction
	if start < end {
		paginated = filtered[start:end]
	}

	return &storage.PaginatedResult{
		Transactions: paginated,
		TotalCount:   totalCount,
	}, nil
}

func (s *FileStore) GetTransaction(ctx context.Context, ref string) (*core.Transaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, t := range s.transactions {
		if t.Reference == ref {
			return t, nil
		}
	}
	return nil, storage.ErrTransactionNotFound
}

// Transfer performs a money transfer between two accounts.
func (s *FileStore) Transfer(ctx context.Context, fromID, toID int, amount int64, reference string) (*core.Account, *core.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if fromID == toID {
		return nil, nil, fmt.Errorf("cannot transfer to same account")
	}

	fromAcc, ok1 := s.accounts[fromID]
	toAcc, ok2 := s.accounts[toID]

	if !ok1 || !ok2 {
		return nil, nil, storage.ErrAccountNotFound
	}

	if fromAcc.Balance+fromAcc.OverdraftLimit < amount {
		return nil, nil, storage.ErrInsufficientFunds
	}

	fromAcc.Balance -= amount
	toAcc.Balance += amount

	// Record transactions
	tx1 := &core.Transaction{
		AccountID:     fromID,
		Type:          "transfer",
		Amount:        amount,
		Timestamp:     time.Now().UTC(),
		FromAccountID: &fromID,
		ToAccountID:   &toID,
		Reference:     reference,
	}
	tx2 := &core.Transaction{
		AccountID:     toID,
		Type:          "deposit",
		Amount:        amount,
		Timestamp:     time.Now().UTC(),
		FromAccountID: &fromID,
		ToAccountID:   &toID,
		Reference:     reference,
	}
	s.transactions = append(s.transactions, tx1, tx2)

	// Persist changes
	if err := s.saveAccounts(); err != nil {
		// Attempt to rollback in-memory change, then return error.
		fromAcc.Balance += amount
		toAcc.Balance -= amount
		return nil, nil, err
	}

	if err := s.saveTransactions(); err != nil {
		// This is harder to roll back as accounts are already saved.
		// For this simple store, we accept the inconsistency.
		return nil, nil, err
	}

	fromCopy := *fromAcc
	toCopy := *toAcc

	return &fromCopy, &toCopy, nil
}

// Payment performs a deposit or withdrawal on an account.
func (s *FileStore) Payment(ctx context.Context, accountID int, amount int64, paymentType storage.PaymentType, reference string) (*core.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	account, ok := s.accounts[accountID]
	if !ok {
		return nil, storage.ErrAccountNotFound
	}

	if paymentType == storage.Withdraw && account.Balance+account.OverdraftLimit < amount {
		return nil, storage.ErrInsufficientFunds
	}

	originalBalance := account.Balance
	switch paymentType {
	case storage.Deposit:
		account.Balance += amount
	case storage.Withdraw:
		account.Balance -= amount
	default:
		return nil, fmt.Errorf("unknown payment type: %s", paymentType)
	}

	transaction := &core.Transaction{
		AccountID: accountID,
		Type:      string(paymentType),
		Amount:    amount,
		Timestamp: time.Now().UTC(),
		Reference: reference,
	}
	s.transactions = append(s.transactions, transaction)

	if err := s.saveAccounts(); err != nil {
		account.Balance = originalBalance // Rollback in-memory change
		return nil, err
	}

	if err := s.saveTransactions(); err != nil {
		// NOTE: This is harder to roll back as accounts are already saved.
		// For this simple store, we accept the potential inconsistency.
		return nil, err
	}

	accountCopy := *account
	return &accountCopy, nil
}

func (s *FileStore) Withdraw(ctx context.Context, accountID int, amount int64, reference string) (*core.Account, error) {
	return s.Payment(ctx, accountID, amount, storage.Withdraw, reference)
}

// User and Password related methods (Stubs for now to satisfy interface)

func (s *FileStore) CreateUser(ctx context.Context, firstName string, lastName string, email string, password string) (*core.User, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *FileStore) CreateUserWithAccount(ctx context.Context, firstName string, lastName string, email string, password string, initialBalance int64) (*core.User, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *FileStore) GetUsers(ctx context.Context) ([]*core.User, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *FileStore) GetUser(ctx context.Context, id int) (*core.User, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *FileStore) UpdateUser(ctx context.Context, id int, firstName string, lastName string, email string) (*core.User, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *FileStore) DeleteUser(ctx context.Context, id int) error {
	return fmt.Errorf("not implemented")
}

func (s *FileStore) GetUserByEmail(ctx context.Context, email string) (*core.User, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *FileStore) UpdateUserPermissions(ctx context.Context, userID int, permissions []string) error {
	return fmt.Errorf("not implemented")
}

func (s *FileStore) CreatePasswordResetToken(ctx context.Context, userID int, tokenHash string, expiresAt time.Time) error {
	return fmt.Errorf("not implemented")
}

func (s *FileStore) GetPasswordResetToken(ctx context.Context, tokenHash string) (userID int, expiresAt time.Time, usedAt *time.Time, err error) {
	return 0, time.Time{}, nil, fmt.Errorf("not implemented")
}

func (s *FileStore) MarkPasswordResetTokenAsUsed(ctx context.Context, tokenHash string) error {
	return fmt.Errorf("not implemented")
}

func (s *FileStore) InvalidateUserPasswordResetTokens(ctx context.Context, userID int) error {
	return fmt.Errorf("not implemented")
}

func (s *FileStore) UpdateUserPassword(ctx context.Context, userID int, hashedPassword string) error {
	return fmt.Errorf("not implemented")
}

func (s *FileStore) CleanupExpiredPasswordResetTokens(ctx context.Context) (int64, error) {
	return 0, fmt.Errorf("not implemented")
}

func (s *FileStore) ResetPasswordTx(ctx context.Context, tokenHash string, hashedPassword string) (userID int, err error) {
	return 0, fmt.Errorf("not implemented")
}

func (s *FileStore) CreateAuditLog(ctx context.Context, log *core.AuditLog) error {
	return fmt.Errorf("not implemented")
}

func (s *FileStore) UpdateOverdraftLimit(ctx context.Context, accountID int, newLimit int64) (*core.Account, error) {
	return nil, fmt.Errorf("not implemented")
}
