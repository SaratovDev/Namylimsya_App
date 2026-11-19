package services

import (
	"context"
	"html"
	"log"
	"sync"
	"time"

	"github.com/SaratovDev/Namylimsya_App/api/internal/db/dbgen"
	"github.com/SaratovDev/Namylimsya_App/api/internal/email"
	"github.com/google/uuid"
)

type queuedEmail struct {
	userID uuid.UUID
	email  string
	items  []emailItem
}

type emailItem struct {
	Type    string
	Payload map[string]any
}

type NotificationWorker struct {
	db      *dbgen.Queries
	mailer  *email.Sender
	queue   chan *queuedEmail
	wg      sync.WaitGroup
	baseURL string
}

// maxEmailAttempts — сколько раз retry одно письмо в flush() перед тем,
// как признать его потерянным.
const maxEmailAttempts = 3

func NewNotificationWorker(db *dbgen.Queries, mailer *email.Sender, baseURL string) *NotificationWorker {
	return &NotificationWorker{
		db:      db,
		mailer:  mailer,
		queue:   make(chan *queuedEmail, 100),
		baseURL: baseURL,
	}
}

func (w *NotificationWorker) Start(ctx context.Context) {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		batch := make(map[uuid.UUID]*queuedEmail)

		for {
			select {
			case <-ctx.Done():
				// Graceful shutdown: ctx уже отменён, SMTP с ним упадёт
				// мгновенно. Даём flush'у независимый контекст с таймаутом
				// 10 сек — письма успевают уйти, если SMTP отвечает быстро.
				flushCtx, flushCancel := context.WithTimeout(
					context.Background(), 10*time.Second,
				)
				w.flush(flushCtx, batch)
				flushCancel()
				return
			case q := <-w.queue:
				if existing, ok := batch[q.userID]; ok {
					existing.items = append(existing.items, q.items...)
				} else {
					batch[q.userID] = q
				}
			case <-ticker.C:
				w.flush(ctx, batch)
				batch = make(map[uuid.UUID]*queuedEmail)
			}
		}
	}()
}

// Enqueue не блокируется. Если канал queue переполнен (SMTP завис,
// flush не успевает, или просто наплыв), письмо дропается с логом.
// In-app уведомление уже создано в БД до этого вызова — теряется только
// email-дайджест, не критично. Без этого Apply/Approve/Cancel висели бы,
// ожидая освобождения слота в канале.
func (w *NotificationWorker) Enqueue(userID uuid.UUID, userEmail string, typ string, payload map[string]any) {
	q := &queuedEmail{
		userID: userID,
		email:  userEmail,
		items:  []emailItem{{Type: typ, Payload: payload}},
	}
	select {
	case w.queue <- q:
		// Ок.
	default:
		log.Printf("notification worker: queue full, dropping email user=%s type=%s", userID, typ)
	}
}

func (w *NotificationWorker) flush(ctx context.Context, batch map[uuid.UUID]*queuedEmail) {
	if w.mailer == nil {
		return
	}
	for _, q := range batch {
		if q.email == "" {
			continue
		}
		msg := email.Message{
			To:      q.email,
			Subject: "Намылимся: новые уведомления",
			Body:    renderEmailBody(q.items, w.baseURL),
		}

		// Retry: до 3 попыток с экспоненциальным backoff — SMTP часто мигает,
		// а терять письмо из-за одной ошибки нельзя (сброс пароля, дайджесты).
		var sendErr error
	retryLoop:
		for attempt := 0; attempt < maxEmailAttempts; attempt++ {
			if attempt > 0 {
				select {
				case <-ctx.Done():
					log.Printf("send email to %s: ctx cancelled before retry %d", q.email, attempt+1)
					break retryLoop
				case <-time.After(time.Duration(attempt) * 2 * time.Second):
				}
			}
			if sendErr = w.mailer.Send(ctx, msg); sendErr == nil {
				break retryLoop
			}
			log.Printf("send email to %s (attempt %d/%d): %v", q.email, attempt+1, maxEmailAttempts, sendErr)
		}
		if sendErr != nil {
			log.Printf("send email to %s: failed after %d attempts: %v", q.email, maxEmailAttempts, sendErr)
		}
	}
}

func renderEmailBody(items []emailItem, baseURL string) string {
	body := "<h2>Новые уведомления на Намылимся</h2><ul>"
	for _, it := range items {
		switch it.Type {
		case "application_new":
			title, _ := it.Payload["event_title"].(string)
			// event_title — пользовательский ввод (организатор придумывает
			// название). Без экранирования можно вставить <a href="evil">
			// или <img onerror=...> в письмо → фишинг/XSS в webmail.
			body += "<li>Новая заявка на событие «" + html.EscapeString(title) + "»</li>"
		case "application_approved":
			body += "<li>Ваша заявка одобрена</li>"
		case "application_rejected":
			body += "<li>Ваша заявка отклонена</li>"
		case "event_cancelled":
			body += "<li>Событие отменено</li>"
		case "event_updated":
			title, _ := it.Payload["event_title"].(string)
			body += "<li>Событие «" + html.EscapeString(title) + "» изменено — проверьте время и место</li>"
		case "application_cancelled":
			body += "<li>Участник снял заявку</li>"
		}
	}
	if baseURL == "" {
		baseURL = "http://localhost:3000"
	}
	body += "</ul><p>Откройте <a href=\"" + baseURL + "\">Намылимся</a>, чтобы посмотреть детали.</p>"
	return body
}

func (w *NotificationWorker) Stop() {
	w.wg.Wait()
}
