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
func (s *FileStore) CreateAccount(ctx context.Context, name string, initialBalance float64) (*core.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextID++
	acc := &core.Account{ID: s.nextID, Name: name, Balance: initialBalance}
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
func (s *FileStore) UpdateBalance(ctx context.Context, id int, newBalance float64) error {
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

// Transfer performs a money transfer between two accounts.
func (s *FileStore) Transfer(ctx context.Context, fromID, toID int, amount float64) (*core.Account, *core.Account, error) {
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

	if fromAcc.Balance < amount {
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
		FromAccountID: fromID,
		ToAccountID:   toID,
	}
	tx2 := &core.Transaction{
		AccountID:     toID,
		Type:          "deposit",
		Amount:        amount,
		Timestamp:     time.Now().UTC(),
		FromAccountID: fromID,
		ToAccountID:   toID,
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
