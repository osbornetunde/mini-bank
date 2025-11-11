package memory

import (
	"context"
	"fmt"
	"sync"

	"mini-bank/internal/core"
)

// Store provides in-memory persistence for accounts and transactions.
type Store struct {
	mu           sync.RWMutex
	accounts     map[int]*core.Account
	transactions []*core.Transaction
	nextID       int

	locksMu   sync.Mutex
	acctLocks map[int]*sync.Mutex
}

// NewStore creates a new in-memory data store.
func NewStore() *Store {
	return &Store{
		accounts:  make(map[int]*core.Account),
		acctLocks: make(map[int]*sync.Mutex),
	}
}

func (s *Store) getAccountLock(id int) *sync.Mutex {
	s.locksMu.Lock()
	l, ok := s.acctLocks[id]
	if !ok {
		l = &sync.Mutex{}
		s.acctLocks[id] = l
	}
	s.locksMu.Unlock()
	return l
}

// CreateAccount adds a new account to memory.
func (s *Store) CreateAccount(ctx context.Context, name string, initialBalance float64) (*core.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextID++
	acc := &core.Account{ID: s.nextID, Name: name, Balance: initialBalance}
	s.accounts[acc.ID] = acc

	s.locksMu.Lock()
	if _, ok := s.acctLocks[acc.ID]; !ok {
		s.acctLocks[acc.ID] = &sync.Mutex{}
	}
	s.locksMu.Unlock()
	return acc, nil
}

// GetAccount retrieves an account by ID.
func (s *Store) GetAccount(ctx context.Context, id int) (*core.Account, error) {
	acc, ok := s.accounts[id]
	if !ok {
		return nil, fmt.Errorf("account not found")
	}
	return acc, nil
}

// ListAccounts returns all accounts in memory.
func (s *Store) ListAccounts(ctx context.Context) ([]*core.Account, error) {
	var list []*core.Account
	for _, acc := range s.accounts {
		list = append(list, acc)
	}
	return list, nil
}

// UpdateBalance modifies an account's balance.
func (s *Store) UpdateBalance(ctx context.Context, id int, newBalance float64) error {
	acc, ok := s.accounts[id]
	if !ok {
		return fmt.Errorf("account not found")
	}
	acc.Balance = newBalance
	return nil
}

// RecordTransaction stores a transaction in memory.
func (s *Store) RecordTransaction(ctx context.Context, tx *core.Transaction) error {
	s.transactions = append(s.transactions, tx)
	return nil
}

// ListTransactions lists all transactions for an account.
func (s *Store) ListTransactions(ctx context.Context, accountID int) ([]*core.Transaction, error) {
	var list []*core.Transaction
	for _, t := range s.transactions {
		if t.AccountID == accountID {
			list = append(list, t)
		}
	}
	return list, nil
}
