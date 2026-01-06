package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"mini-bank/internal/core"
	"mini-bank/internal/storage"
	"mini-bank/pkg/metrics"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const (
	ContextKeyIP        contextKey = "ip"
	ContextKeyUserAgent contextKey = "user_agent"
)

type Service interface {
	CreateAccount(ctx context.Context, userID int, balance int64) (*core.Account, error)
	GetAccount(ctx context.Context, id int) (*core.Account, error)
	ListAccounts(ctx context.Context) ([]*core.Account, error)
	ListUserAccounts(ctx context.Context, userID int) ([]*core.Account, error)
	UpdateOverdraftLimit(ctx context.Context, accountID int, newLimit int64) (*core.Account, error)
	Transfer(ctx context.Context, fromID, toID int, amount int64, reference string) (*core.Account, *core.Account, error)
	Payment(ctx context.Context, accountID int, amount int64, pType storage.PaymentType, reference string) (*core.Account, error)
	ListTransactions(ctx context.Context, accountID int) ([]*core.Transaction, error)
	GetTransaction(ctx context.Context, reference string) (*core.Transaction, error)
	CreateUser(ctx context.Context, firstName string, lastName string, email string, password string) (*core.User, error)
	GetUsers(ctx context.Context) ([]*core.User, error)
	GetUser(ctx context.Context, id int) (*core.User, error)
	UpdateUser(ctx context.Context, id int, firstName string, lastName string, email string) (*core.User, error)
	DeleteUser(ctx context.Context, id int) error
	UpdateUserPermissions(ctx context.Context, userID int, permissions []string) error
	Login(ctx context.Context, email string, password string) (*core.User, error)
	RequestPasswordReset(ctx context.Context, email string) (token string, err error)
	ResetPassword(ctx context.Context, token string, newPassword string) (*core.User, error)
	Withdraw(ctx context.Context, accountID int, amount int64, reference string) (*core.Account, error)
}

type service struct {
	store       storage.Storage
	emailSender EmailSender
	logger      *slog.Logger
}

func New(store storage.Storage, emailSender EmailSender, logger *slog.Logger) Service {
	return &service{
		store:       store,
		emailSender: emailSender,
		logger:      logger,
	}
}

func (s *service) logActivity(ctx context.Context, userID int, action string, metadata map[string]any) error {
	ip, _ := ctx.Value(ContextKeyIP).(string)
	ua, _ := ctx.Value(ContextKeyUserAgent).(string)

	var metaJSON string
	if metadata != nil {
		b, err := json.Marshal(metadata)
		if err != nil {
			s.logger.Error("failed to marshal audit metadata", "err", err)
			// We continue with empty metadata rather than failing
		} else {
			metaJSON = string(b)
		}
	}

	log := &core.AuditLog{
		UserID:    userID,
		Action:    action,
		Metadata:  metaJSON,
		IP:        ip,
		UserAgent: ua,
		CreatedAt: time.Now().UTC(),
	}

	if err := s.store.CreateAuditLog(ctx, log); err != nil {
		// Log the error - audit log failures are a security concern
		s.logger.Error("failed to create audit log",
			"error", err,
			"user_id", userID,
			"action", action,
			"ip", ip,
		)
		// Increment failure metric for monitoring/alerting
		metrics.AuditLogFailures.WithLabelValues(action).Inc()
		return err
	}
	return nil
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

func (s *service) ListUserAccounts(ctx context.Context, userID int) ([]*core.Account, error) {
	all, err := s.store.ListAccounts(ctx)
	if err != nil {
		return nil, err
	}
	var filtered []*core.Account
	for _, acc := range all {
		if acc.UserID == userID {
			filtered = append(filtered, acc)
		}
	}
	return filtered, nil
}

func (s *service) UpdateOverdraftLimit(ctx context.Context, accountID int, newLimit int64) (*core.Account, error) {
	return s.store.UpdateOverdraftLimit(ctx, accountID, newLimit)
}

func (s *service) Transfer(ctx context.Context, fromID, toID int, amount int64, reference string) (*core.Account, *core.Account, error) {
	from, to, err := s.store.Transfer(ctx, fromID, toID, amount, reference)
	status := "success"
	if err != nil {
		status = "failure"
	}
	metrics.TransactionTotal.WithLabelValues("transfer", status).Inc()
	if err == nil {
		metrics.TransactionAmount.WithLabelValues("transfer").Add(float64(amount))
	}
	return from, to, err
}

func (s *service) Payment(ctx context.Context, accountID int, amount int64, pType storage.PaymentType, reference string) (*core.Account, error) {
	acc, err := s.store.Payment(ctx, accountID, amount, pType, reference)
	status := "success"
	if err != nil {
		status = "failure"
	}
	metrics.TransactionTotal.WithLabelValues(string(pType), status).Inc()
	if err == nil {
		metrics.TransactionAmount.WithLabelValues(string(pType)).Add(float64(amount))
	}
	return acc, err
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
	res, err := s.store.CreateUserWithAccount(ctx, firstName, lastName, email, hashedPassword, 0)
	if err == nil && res != nil {
		if logErr := s.logActivity(ctx, res.ID, "user_created", map[string]any{"email": email}); logErr != nil {
			// User is created but audit failed. We return the error but also the user.
			// The caller can decide how to handle this partial success.
			return res, fmt.Errorf("user created but audit log failed: %w", logErr)
		}
	}
	return res, err
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

func (s *service) UpdateUserPermissions(ctx context.Context, userID int, permissions []string) error {
	// Validate that all permissions are valid
	for _, perm := range permissions {
		if !core.IsValidPermission(perm) {
			return fmt.Errorf("invalid permission: %s", perm)
		}
	}

	if err := s.store.UpdateUserPermissions(ctx, userID, permissions); err != nil {
		return err
	}

	// Log the permission change
	_ = s.logActivity(ctx, userID, "permissions_updated", map[string]any{"permissions": permissions})

	return nil
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
		_ = s.logActivity(ctx, user.ID, "login_failed", map[string]any{"reason": "invalid_password"})
		return nil, storage.ErrInvalidCredentials
	}

	if err := s.logActivity(ctx, user.ID, "login_success", nil); err != nil {
		return nil, fmt.Errorf("audit log failed: %w", err)
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

	if err := s.logActivity(ctx, user.ID, "password_reset_requested", nil); err != nil {
		// If audit fails, we should probably fail the request to ensure traceability
		// However, email is already sent. This is tricky.
		// For fail-closed, we return error.
		return "", fmt.Errorf("audit log failed: %w", err)
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

	if err := s.logActivity(ctx, user.ID, "password_reset_success", nil); err != nil {
		return user, fmt.Errorf("password reset successful but audit log failed: %w", err)
	}
	return user, nil
}

func (s *service) Withdraw(ctx context.Context, accountID int, amount int64, reference string) (*core.Account, error) {
	acc, err := s.store.Withdraw(ctx, accountID, amount, reference)
	return acc, err
}
