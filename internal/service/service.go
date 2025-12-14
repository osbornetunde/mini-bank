package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"mini-bank/internal/core"
	"mini-bank/internal/storage"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Service interface {
	CreateAccount(ctx context.Context, userID int, balance int64) (*core.Account, error)
	GetAccount(ctx context.Context, id int) (*core.Account, error)
	ListAccounts(ctx context.Context) ([]*core.Account, error)
	Transfer(ctx context.Context, fromID, toID int, amount int64, reference string) (*core.Account, *core.Account, error)
	Payment(ctx context.Context, accountID int, amount int64, pType storage.PaymentType, reference string) (*core.Account, error)
	ListTransactions(ctx context.Context, accountID int) ([]*core.Transaction, error)
	GetTransaction(ctx context.Context, reference string) (*core.Transaction, error)
	CreateUser(ctx context.Context, firstName string, lastName string, email string, password string) (*core.User, error)
	GetUsers(ctx context.Context) ([]*core.User, error)
	GetUser(ctx context.Context, id int) (*core.User, error)
	UpdateUser(ctx context.Context, id int, firstName string, lastName string, email string) (*core.User, error)
	DeleteUser(ctx context.Context, id int) error
	Login(ctx context.Context, email string, password string) (*core.User, error)
	RequestPasswordReset(ctx context.Context, email string) (token string, err error)
	ResetPassword(ctx context.Context, token string, newPassword string) (*core.User, error)
}

type service struct {
	store       storage.Storage
	emailSender EmailSender
}

func New(store storage.Storage, emailSender EmailSender) Service {
	return &service{store: store, emailSender: emailSender}
}

func (s *service) CreateAccount(ctx context.Context, userID int, balance int64) (*core.Account, error) {
	return s.store.CreateAccount(ctx, userID, balance)
}

func (s *service) GetAccount(ctx context.Context, id int) (*core.Account, error) {
	return s.store.GetAccount(ctx, id)
}

func (s *service) ListAccounts(ctx context.Context) ([]*core.Account, error) {
	return s.store.ListAccounts(ctx)
}

func (s *service) Transfer(ctx context.Context, fromID, toID int, amount int64, reference string) (*core.Account, *core.Account, error) {
	return s.store.Transfer(ctx, fromID, toID, amount, reference)
}

func (s *service) Payment(ctx context.Context, accountID int, amount int64, pType storage.PaymentType, reference string) (*core.Account, error) {
	return s.store.Payment(ctx, accountID, amount, pType, reference)
}

func (s *service) ListTransactions(ctx context.Context, accountID int) ([]*core.Transaction, error) {
	return s.store.ListTransactions(ctx, accountID)
}

func (s *service) GetTransaction(ctx context.Context, reference string) (*core.Transaction, error) {
	return s.store.GetTransaction(ctx, reference)
}

func (s *service) CreateUser(ctx context.Context, firstName string, lastName string, email string, password string) (*core.User, error) {
	hashedPassword, err := hashPassword(password)
	if err != nil {
		return nil, err
	}
	res, err := s.store.CreateUser(ctx, firstName, lastName, email, hashedPassword)
	if err != nil {
		return nil, err
	}
	if _, err := s.store.CreateAccount(ctx, res.ID, 0); err != nil {
		return nil, err
	}
	return res, nil
}

func (s *service) GetUsers(ctx context.Context) ([]*core.User, error) {
	return s.store.GetUsers(ctx)
}

func (s *service) GetUser(ctx context.Context, id int) (*core.User, error) {
	return s.store.GetUser(ctx, id)
}

func (s *service) UpdateUser(ctx context.Context, id int, firstName string, lastName string, email string) (*core.User, error) {
	return s.store.UpdateUser(ctx, id, firstName, lastName, email)
}

func (s *service) DeleteUser(ctx context.Context, id int) error {
	return s.store.DeleteUser(ctx, id)
}

func (s *service) Login(ctx context.Context, email string, password string) (*core.User, error) {
	user, err := s.store.GetUserByEmail(ctx, email)
	if err != nil {
		// If user not found, we return InvalidCredentials to avoid enumeration
		if errors.Is(err, storage.ErrUserNotFound) {
			return nil, storage.ErrInvalidCredentials
		}
		return nil, err
	}

	if err := verifyPassword(*user.Password, password); err != nil {
		return nil, storage.ErrInvalidCredentials
	}

	return user, nil
}

func hashPassword(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedPassword), nil
}

func verifyPassword(hashedPassword string, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func (s *service) RequestPasswordReset(ctx context.Context, email string) (string, error) {
	user, err := s.store.GetUserByEmail(ctx, email)
	if err != nil {
		// Don't reveal whether the user exists or not for security reasons
		if errors.Is(err, storage.ErrUserNotFound) {
			// Simulate DB work to mitigate timing attacks
			// In a real production system, you'd want to tune this to match the "found" path
			// which includes a token generation and DB insert.
			time.Sleep(50 * time.Millisecond)

			// Still return success to avoid user enumeration
			return "", nil
		}
		return "", err
	}

	if err := s.store.InvalidateUserPasswordResetTokens(ctx, user.ID); err != nil {
		return "", err
	}

	token := uuid.NewString()

	tokenHash := hashToken(token)

	expiresAt := time.Now().Add(1 * time.Hour)

	if err := s.store.CreatePasswordResetToken(ctx, user.ID, tokenHash, expiresAt); err != nil {
		return "", err
	}

	// Send email with token
	if err := s.emailSender.SendPasswordResetEmail(email, token); err != nil {
		// Clean up the orphaned token since user won't receive it
		_ = s.store.InvalidateUserPasswordResetTokens(ctx, user.ID)
		return "", err
	}

	return token, nil
}

func (s *service) ResetPassword(ctx context.Context, token string, newPassword string) (*core.User, error) {
	tokenHash := hashToken(token)

	hashedPassword, err := hashPassword(newPassword)
	if err != nil {
		return nil, err
	}

	userID, err := s.store.ResetPasswordTx(ctx, tokenHash, hashedPassword)
	if err != nil {
		return nil, err
	}

	user, err := s.store.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Send confirmation email
	if err := s.emailSender.SendPasswordChangedEmail(user.Email); err != nil {
		// Log error but don't fail the request
		// We can't log here directly as we don't have a logger, but in a real system we would.
		// For now we just ignore it as it's non-critical path
	}

	return user, nil
}
