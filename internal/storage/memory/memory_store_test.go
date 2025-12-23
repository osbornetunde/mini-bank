package memory

import (
	"context"
	"testing"

	"mini-bank/internal/storage"

	"github.com/stretchr/testify/assert"
)

func TestStore_CreateAccount(t *testing.T) {
	s := NewStore()
	userID := 1
	initialBalance := int64(1000)

	acc, err := s.CreateAccount(context.Background(), userID, initialBalance)

	assert.NoError(t, err)
	assert.NotNil(t, acc)
	assert.Equal(t, userID, acc.UserID)
	assert.Equal(t, initialBalance, acc.Balance)
	assert.Greater(t, acc.ID, 0)
}

func TestStore_Transfer(t *testing.T) {
	s := NewStore()
	acc1, _ := s.CreateAccount(context.Background(), 1, 1000)
	acc2, _ := s.CreateAccount(context.Background(), 2, 500)

	fromAcc, toAcc, err := s.Transfer(context.Background(), acc1.ID, acc2.ID, 200, "transfer-ref")

	assert.NoError(t, err)
	assert.Equal(t, int64(800), fromAcc.Balance)
	assert.Equal(t, int64(700), toAcc.Balance)

	// Verify persistence
	a1, _ := s.GetAccount(context.Background(), acc1.ID)
	a2, _ := s.GetAccount(context.Background(), acc2.ID)
	assert.Equal(t, int64(800), a1.Balance)
	assert.Equal(t, int64(700), a2.Balance)
}

func TestStore_Transfer_InsufficientFunds(t *testing.T) {
	s := NewStore()
	acc1, _ := s.CreateAccount(context.Background(), 1, 100)
	acc2, _ := s.CreateAccount(context.Background(), 2, 500)

	_, _, err := s.Transfer(context.Background(), acc1.ID, acc2.ID, 200, "transfer-ref")

	assert.ErrorIs(t, err, storage.ErrInsufficientFunds)
}

func TestStore_Transfer_Overdraft(t *testing.T) {
	s := NewStore()
	acc1, _ := s.CreateAccount(context.Background(), 1, 100)
	acc1.OverdraftLimit = 200
	acc2, _ := s.CreateAccount(context.Background(), 2, 500)

	// Transfer 200 from 100 balance with 200 overdraft = OK
	fromAcc, toAcc, err := s.Transfer(context.Background(), acc1.ID, acc2.ID, 250, "transfer-ref")

	assert.NoError(t, err)
	assert.Equal(t, int64(-150), fromAcc.Balance)
	assert.Equal(t, int64(750), toAcc.Balance)
}

func TestStore_Payment_Deposit(t *testing.T) {
	s := NewStore()
	acc, _ := s.CreateAccount(context.Background(), 1, 1000)

	res, err := s.Payment(context.Background(), acc.ID, 500, storage.Deposit, "dep-ref")

	assert.NoError(t, err)
	assert.Equal(t, int64(1500), res.Balance)
}

func TestStore_Payment_Withdraw(t *testing.T) {
	s := NewStore()
	acc, _ := s.CreateAccount(context.Background(), 1, 1000)

	res, err := s.Payment(context.Background(), acc.ID, 300, storage.Withdraw, "with-ref")

	assert.NoError(t, err)
	assert.Equal(t, int64(700), res.Balance)
}
