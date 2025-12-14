package api

import (
	"errors"
	"net/mail"

	"mini-bank/internal/storage"
)

// Validator is a simple interface for validatable structs
type Validator interface {
	Validate() error
}

// Common validation errors
var (
	ErrInvalidEmail     = errors.New("invalid email format")
	ErrPasswordTooShort = errors.New("password must be at least 8 characters")
	ErrInvalidAmount    = errors.New("amount must be greater than zero")
	ErrInvalidID        = errors.New("invalid ID")
	ErrSameAccount      = errors.New("source and destination accounts cannot be the same")
	ErrMissingField     = errors.New("missing required field")
	ErrInvalidPayment   = errors.New("invalid payment type")
)

func validateEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

// Implement Validator interface for request structs

func (r createAccountRequest) Validate() error {
	if r.UserID <= 0 {
		return ErrInvalidID
	}
	if r.InitialBalance < 0 {
		return errors.New("initial balance cannot be negative")
	}
	return nil
}

func (r transferRequest) Validate() error {
	if r.FromID <= 0 || r.ToID <= 0 {
		return ErrInvalidID
	}
	if r.FromID == r.ToID {
		return ErrSameAccount
	}
	if r.Amount <= 0 {
		return ErrInvalidAmount
	}
	return nil
}

func (r paymentRequest) Validate() error {
	if r.AccountID <= 0 {
		return ErrInvalidID
	}
	if r.Amount <= 0 {
		return ErrInvalidAmount
	}
	if r.Type != storage.Deposit && r.Type != storage.Withdraw {
		return ErrInvalidPayment
	}
	return nil
}

func (r createUserRequest) Validate() error {
	if r.FirstName == "" || r.LastName == "" {
		return ErrMissingField
	}
	if !validateEmail(r.Email) {
		return ErrInvalidEmail
	}
	if len(r.Password) < 8 {
		return ErrPasswordTooShort
	}
	return nil
}
