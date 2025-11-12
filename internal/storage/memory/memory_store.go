package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

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
	s.mu.RLock()
	defer s.mu.RUnlock()

	acc, ok := s.accounts[id]
	if !ok {
		return nil, fmt.Errorf("account not found")
	}
	copyAcc := *acc
	return &copyAcc, nil
}

// ListAccounts returns all accounts in memory.
func (s *Store) ListAccounts(ctx context.Context) ([]*core.Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var list []*core.Account
	for _, acc := range s.accounts {
		copyAcc := *acc
		list = append(list, &copyAcc)
	}
	return list, nil
}

// UpdateBalance modifies an account's balance.
func (s *Store) UpdateBalance(ctx context.Context, id int, delta float64) error {
	s.mu.RLock()
	acc, ok := s.accounts[id]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("account not found")
	}

	al := s.getAccountLock(id)
	al.Lock()
	defer al.Unlock()

	acc.Balance += delta
	return nil
}

// RecordTransaction stores a transaction in memory.
func (s *Store) RecordTransaction(ctx context.Context, tx *core.Transaction) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if tx.Timestamp.IsZero() {
		tx.Timestamp = time.Now().UTC()
	}
	s.transactions = append(s.transactions, tx)
	return nil
}

// ListTransactions lists all transactions for an account.
func (s *Store) ListTransactions(ctx context.Context, accountID int) ([]*core.Transaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var list []*core.Transaction
	for _, t := range s.transactions {
		if t.AccountID == accountID {
			c := *t
			list = append(list, &c)
		}
	}
	return list, nil
}

func (s *Store) Transfer(ctx context.Context, fromID, toID int, amount float64) error {
	if fromID == toID {
		return fmt.Errorf("cannot transfer to same account")
	}

	// determine lock order to avoid deadlock: lower ID first
	first, second := fromID, toID
	if first > second {
		first, second = second, first
	}

	// Step 1: Lock accounts and perform all checks and preparations.
	firstLock := s.getAccountLock(first)
	secondLock := s.getAccountLock(second)

	firstLock.Lock()
	defer firstLock.Unlock()
	secondLock.Lock()
	defer secondLock.Unlock()

	s.mu.RLock()
	fromAcc, ok1 := s.accounts[fromID]
	toAcc, ok2 := s.accounts[toID]
	s.mu.RUnlock()

	if !ok1 || !ok2 {
		return fmt.Errorf("account not found")
	}

	if fromAcc.Balance < amount {
		return fmt.Errorf("insufficient funds")
	}

	// Step 2: Prepare changes in temporary variables.
	// The "live" data is not touched yet.
	newFromBalance := fromAcc.Balance - amount
	newToBalance := toAcc.Balance + amount

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

	// Step 3: Commit all changes in a single atomic block.
	s.mu.Lock()
	defer s.mu.Unlock()

	fromAcc.Balance = newFromBalance
	toAcc.Balance = newToBalance
	s.transactions = append(s.transactions, tx1, tx2)

	return nil
}
