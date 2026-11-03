package email

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"
)

type Sender struct {
	host     string
	port     string
	user     string
	pass     string
	from     string
	disabled bool
}

func NewSender(host, port, user, pass, from string, disabled bool) *Sender {
	return &Sender{host: host, port: port, user: user, pass: pass, from: from, disabled: disabled}
}

type Message struct {
	To      string
	Subject string
	Body    string // HTML
}

func (s *Sender) Send(ctx context.Context, msg Message) error {
	if s.disabled {
		return nil
	}

	headers := []string{
		"From: " + s.from,
		"To: " + msg.To,
		"Subject: " + msg.Subject,
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=UTF-8",
	}
	full := strings.Join(headers, "\r\n") + "\r\n\r\n" + msg.Body

	addr := fmt.Sprintf("%s:%s", s.host, s.port)
	var auth smtp.Auth
	if s.user != "" {
		auth = smtp.PlainAuth("", s.user, s.pass, s.host)
	}

	return smtp.SendMail(addr, auth, s.from, []string{msg.To}, []byte(full))
}
