package mailer

import (
	"strconv"
	"time"

	gomail "gopkg.in/mail.v2"
)

type Mailer struct {
	dailer *gomail.Dialer
	sender string
}

func New(host string, port string, username, password, sender string) Mailer {
	intPort, err := strconv.Atoi(port)
	if err != nil {
		panic(err)
	}
	dailer := gomail.NewDialer(host, intPort, username, password)
	dailer.Timeout = time.Second * 10

	return Mailer{
		dailer: dailer,
		sender: sender,
	}
}

func (m Mailer) Send(subject, body string, to []string) error {
	msg := gomail.NewMessage()
	msg.SetHeader("From", m.sender)
	msg.SetHeader("To", to...)
	msg.SetHeader("Subject", subject)
	msg.SetBody("text/html", body)

	return m.dailer.DialAndSend(msg)
}
