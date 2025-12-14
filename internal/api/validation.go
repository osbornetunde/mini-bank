package api

import (
	"errors"
	"net/mail"
	"unicode"

	"github.com/google/uuid"
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
	ErrPasswordTooLong  = errors.New("password must not exceed 128 characters")
	ErrPasswordTooWeak  = errors.New("password must contain at least one uppercase letter, one lowercase letter, one digit, and one special character")
	ErrInvalidAmount    = errors.New("amount must be greater than zero")
	ErrInvalidID        = errors.New("invalid ID")
	ErrSameAccount      = errors.New("source and destination accounts cannot be the same")
	ErrMissingField     = errors.New("missing required field")
	ErrInvalidPayment   = errors.New("invalid payment type")
	ErrInvalidToken     = errors.New("invalid token format")
)

// validatePasswordStrength checks if a password meets complexity requirements:
// - At least 8 characters
// - At most 128 characters (bcrypt has a 72-byte limit, but we allow more for future flexibility)
// - At least one uppercase letter
// - At least one lowercase letter
// - At least one digit
// - At least one special character
func validatePasswordStrength(password string) error {
	if len(password) < 8 {
		return ErrPasswordTooShort
	}

	if len(password) > 128 {
		return ErrPasswordTooLong
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool

	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsDigit(char):
			hasDigit = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	if !hasUpper || !hasLower || !hasDigit || !hasSpecial {
		return ErrPasswordTooWeak
	}

	return nil
}

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
	if err := validatePasswordStrength(r.Password); err != nil {
		return err
	}
	return nil
}

func (r RequestPasswordResetRequest) Validate() error {
	if r.Email == "" {
		return ErrMissingField
	}
	if !validateEmail(r.Email) {
		return ErrInvalidEmail
	}
	return nil
}

func (r ResetPasswordRequest) Validate() error {
	if r.Token == "" {
		return ErrInvalidToken
	}
	// Validate token is a valid UUID format before hitting the database
	if _, err := uuid.Parse(r.Token); err != nil {
		return ErrInvalidToken
	}
	if r.NewPassword == "" {
		return ErrMissingField
	}
	if err := validatePasswordStrength(r.NewPassword); err != nil {
		return err
	}
	return nil
}
