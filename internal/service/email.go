package service

import (
	"fmt"
	"log/slog"
)

// MailSender defines the interface for sending emails
type MailSender interface {
	Send(subject, body string, to []string) error
}

type EmailSender interface {
	SendPasswordResetEmail(email, token string) error
	SendPasswordChangedEmail(email string) error
}

type LogEmailSender struct {
	logger   *slog.Logger
	logToken bool // If true, logs full token (development only)
	mailer   MailSender
}

func NewLogEmailSender(logger *slog.Logger, logToken bool, mailer MailSender) *LogEmailSender {
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
	body := fmt.Sprintf(`<html>
            <body>
                <h1>Password Reset Request</h1>
                <p><b>Hello!</b> This is your password reset token: <code>%s</code>.</p>
                <p>Thanks,<br>Minibank</p>
            </body>
        </html>`, token)
	return s.mailer.Send("Password Reset Request", body, []string{email})
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
	return s.mailer.Send("Your password has been changed", body, []string{email})
}
