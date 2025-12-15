package mailer

import (
	"time"

	gomail "gopkg.in/mail.v2"
)

type Mailer struct {
	dialer *gomail.Dialer
	sender string
}

type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	Sender   string
	Timeout  time.Duration
}

func New(cfg Config) Mailer {
	dialer := gomail.NewDialer(cfg.Host, cfg.Port, cfg.Username, cfg.Password)
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	dialer.Timeout = timeout

	return Mailer{
		dialer: dialer,
		sender: cfg.Sender,
	}
}

func (m Mailer) Send(subject, body string, to []string) error {
	msg := gomail.NewMessage()
	msg.SetHeader("From", m.sender)
	msg.SetHeader("To", to...)
	msg.SetHeader("Subject", subject)
	msg.SetBody("text/html", body)

	return m.dialer.DialAndSend(msg)
}
