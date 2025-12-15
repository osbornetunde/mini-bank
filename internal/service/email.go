package service

import (
	"log/slog"

	"mini-bank/internal/mailer"
)

type EmailSender interface {
	SendPasswordResetEmail(email, token string) error
	SendPasswordChangedEmail(email string) error
}

type LogEmailSender struct {
	logger   *slog.Logger
	logToken bool // If true, logs full token (development only)
	mailer   mailer.Mailer
}

func NewLogEmailSender(logger *slog.Logger, logToken bool, mailer mailer.Mailer) *LogEmailSender {
	return &LogEmailSender{logger: logger, logToken: logToken, mailer: mailer}
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
	body := `<html>
            <body>
                <h1>Password Reset Request</h1>
                <p><b>Hello!</b> This is your password reset token: <code>${token}</code>.</p>
                <p>Thanks,<br>Minibank</p>
            </body>
        </html>`
	s.mailer.Send("Password Reset Request", body, []string{email})
	return nil
}

func (s *LogEmailSender) SendPasswordChangedEmail(email string) error {
	s.logger.Info("sending password changed email",
		"to", email,
		"subject", "Your password has been changed",
	)
	body := `<html>
            <body>
                <h1>Password Changed!</h1>
                <p><b>Hello!</b> Your password has been changed.</p>
                <p>Thanks,<br>Minibank</p>
            </body>
        </html>`
	s.mailer.Send("Password Reset Request", body, []string{email})
	return nil
}
