package service

import (
	"log/slog"
)

type EmailSender interface {
	SendPasswordResetEmail(email, token string) error
	SendPasswordChangedEmail(email string) error
}

type LogEmailSender struct {
	logger   *slog.Logger
	logToken bool // If true, logs full token (development only)
}

func NewLogEmailSender(logger *slog.Logger, logToken bool) *LogEmailSender {
	return &LogEmailSender{logger: logger, logToken: logToken}
}

func (s *LogEmailSender) SendPasswordResetEmail(email, token string) error {
	// In a real application, this would send an actual email.
	tokenValue := token[:8] + "..." + token[len(token)-4:]
	if s.logToken {
		// WARNING: Only enable in development! Never log tokens in production.
		tokenValue = token
	}

	s.logger.Info("sending password reset email",
		"to", email,
		"token", tokenValue,
		"subject", "Password Reset Request",
	)
	return nil
}

func (s *LogEmailSender) SendPasswordChangedEmail(email string) error {
	s.logger.Info("sending password changed email",
		"to", email,
		"subject", "Your password has been changed",
	)
	return nil
}
