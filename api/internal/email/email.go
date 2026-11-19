package email

import (
	"context"
	"crypto/tls"
	"fmt"

	"net"
	"net/smtp"
	"strings"
	"time"
)

// defaultSendTimeout — общий лимит на одну SMTP-операцию (dial + handshake +
// auth + send). Без него smtp.SendMail может висеть минуты на мёртвом SMTP,
// а NotificationWorker flush — блокировать канал queue, что докатывается до
// Apply/Approve/Cancel (они вызывают notif.Create → Enqueue).
const defaultSendTimeout = 10 * time.Second

type Sender struct {
	host     string
	port     string
	user     string
	pass     string
	from     string
	disabled bool
	timeout  time.Duration
}

func NewSender(host, port, user, pass, from string, disabled bool) *Sender {
	return &Sender{
		host:     host,
		port:     port,
		user:     user,
		pass:     pass,
		from:     from,
		disabled: disabled,
		timeout:  defaultSendTimeout,
	}
}

type Message struct {
	To      string
	Subject string
	Body    string // HTML
}

// Send отправляет письмо по SMTP с жёстким таймаутом.
// Если ctx истекает раньше timeout — уважаем ctx.
// smtp.SendMail не поддерживает ни context, ни timeout, поэтому
// используем ручной smtp.Client поверх net.Dialer.
func (s *Sender) Send(ctx context.Context, msg Message) error {
	if s.disabled {
		return nil
	}

	deadline := time.Now().Add(s.timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}

	// Санитизация заголовков: пользовательский ввод (To из ответа поддержки,
	// Subject из тикета) мог содержать \r\n → header injection:
	//   Subject: hi\r\nBcc: attacker@evil.com
	// После SanitizeHeader все CR/LF вырезаны, инъекция невозможна.
	headers := []string{
		"From: " + SanitizeHeader(s.from),
		"To: " + SanitizeHeader(msg.To),
		"Subject: " + SanitizeHeader(msg.Subject),
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=UTF-8",
	}
	full := strings.Join(headers, "\r\n") + "\r\n\r\n" + msg.Body

	addr := fmt.Sprintf("%s:%s", s.host, s.port)

	dialer := &net.Dialer{Timeout: s.timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("smtp dial %s: %w", addr, err)
	}
	// Deadline на всё соединение — гарантирует, что даже если сервер
	// отвечает по одному байту в час, мы отвалимся вовремя.
	_ = conn.SetDeadline(deadline)
	defer conn.Close()

	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return fmt.Errorf("smtp new client: %w", err)
	}
	defer client.Close()

	// STARTTLS, если сервер поддерживает (пропускаем для Mailpit/localhost,
	// там STARTTLS нет и это нормально).
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: s.host}); err != nil {
			return fmt.Errorf("smtp starttls: %w", err)
		}
	}

	if s.user != "" {
		auth := smtp.PlainAuth("", s.user, s.pass, s.host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}

	if err := client.Mail(s.from); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := client.Rcpt(msg.To); err != nil {
		return fmt.Errorf("smtp rcpt to: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write([]byte(full)); err != nil {
		_ = w.Close()
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close data: %w", err)
	}
	if err := client.Quit(); err != nil {
		// Quit может не дойти, если сервер уже закрыл — не критично.
		return nil
	}
	return nil
}

// SanitizeHeader убирает CR и LF из значения заголовка, предотвращая
// SMTP header injection: пользовательский ввод вида
//
//	"hi\r\nBcc: attacker@evil.com"
//
// больше не разорвёт заголовок на две строки.
// Также обрезает по 1000 рун — защита от гигантских значений.
func SanitizeHeader(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	const maxLen = 1000
	if len(s) > maxLen {
		s = s[:maxLen]
	}
	return s
}
