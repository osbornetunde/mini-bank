package service

import (
	"log/slog"
	"mini-bank/internal/templates"
	"mini-bank/internal/worker"
)

// EmailSender interface remains the same
type EmailSender interface {
	SendPasswordResetEmail(email, token string) error
	SendPasswordChangedEmail(email string) error
}

type AsyncEmailSender struct {
	logger      *slog.Logger
	logToken    bool
	distributor worker.TaskDistributor
}

func NewAsyncEmailSender(logger *slog.Logger, logToken bool, distributor worker.TaskDistributor) *AsyncEmailSender {
	return &AsyncEmailSender{
		logger:      logger,
		logToken:    logToken,
		distributor: distributor,
	}
}

func (s *AsyncEmailSender) SendPasswordResetEmail(email, token string) error {
	subject, body, err := templates.GetPasswordResetContent(token)
	if err != nil {
		return err
	}

	tokenValue := token[:8] + "..." + token[len(token)-4:]
	if s.logToken {
		tokenValue = token
	}

	s.logger.Info("enqueueing password reset email",
		"to", email,
		"token", tokenValue,
	)

	return s.distributor.DistributeTaskSendEmail(&worker.PayloadSendEmail{
		Email:   email,
		Subject: subject,
		Body:    body,
	})
}

func (s *AsyncEmailSender) SendPasswordChangedEmail(email string) error {
	subject, body, err := templates.GetPasswordChangedContent()
	if err != nil {
		return err
	}

	s.logger.Info("enqueueing password changed email", "to", email)

	return s.distributor.DistributeTaskSendEmail(&worker.PayloadSendEmail{
		Email:   email,
		Subject: subject,
		Body:    body,
	})
}
