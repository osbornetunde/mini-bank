package core

import "errors"

type Account struct {
	ID      string
	UserID  string
	Balance float64
}

func (a *Account) Deposit(amount float64) error {
	if amount <= 0 {
		return errors.New("invalid deposit amount")
	}
	a.Balance += amount
	return nil
}

func (a *Account) Withdraw(amount float64) error {
	if amount > a.Balance {
		return errors.New("insufficient funds")
	}
	a.Balance -= amount
	return nil
}
