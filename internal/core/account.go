package core

import (
	"errors"
	"fmt"
)

type Account struct {
	ID      int
	Name    string
	Balance float64
}

var accounts = make(map[int]*Account)
var nextID = 1

func Deposit(id int, amount float64) error {
	acc, exists := accounts[id]
	if !exists {
		return fmt.Errorf("account %d not found", id)
	}
	if amount <= 0 {
		return errors.New("invalid deposit amount")
	}
	acc.Balance += amount
	return nil
}

func Withdraw(id int, amount float64) error {
	acc, exists := accounts[id]
	if !exists {
		return fmt.Errorf("account %d not found", id)
	}
	if amount > acc.Balance {
		return errors.New("insufficient funds")
	}
	acc.Balance -= amount
	return nil
}

func CreateAccount(name string, intialBalance float64) *Account {
	acc := &Account{
		ID:      nextID,
		Name:    name,
		Balance: intialBalance,
	}

	accounts[nextID] = acc
	nextID++
	return acc
}

func GetAllAccounts() {
	fmt.Println("ID\tName\tBalance")
	for _, acc := range accounts {
		fmt.Printf("%d\t%s\t%.2f\n", acc.ID, acc.Name, acc.Balance)
	}
}
