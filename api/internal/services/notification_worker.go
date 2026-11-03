package services

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/SaratovDev/Namylimsya_App/api/internal/db/dbgen"
	"github.com/SaratovDev/Namylimsya_App/api/internal/email"
	"github.com/google/uuid"
)

type NotificationWorker struct {
	db     *dbgen.Queries
	mailer *email.Sender
	queue  chan *queuedEmail
	wg     sync.WaitGroup
}

type queuedEmail struct {
	userID uuid.UUID
	email  string
	items  []emailItem
}

type emailItem struct {
	Type    string
	Payload map[string]any
}

func NewNotificationWorker(db *dbgen.Queries, mailer *email.Sender) *NotificationWorker {
	return &NotificationWorker{
		db:     db,
		mailer: mailer,
		queue:  make(chan *queuedEmail, 100),
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
				// flush
				w.flush(ctx, batch)
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

func (w *NotificationWorker) Enqueue(userID uuid.UUID, userEmail string, typ string, payload map[string]any) {
	w.queue <- &queuedEmail{
		userID: userID,
		email:  userEmail,
		items:  []emailItem{{Type: typ, Payload: payload}},
	}
}

func (w *NotificationWorker) flush(ctx context.Context, batch map[uuid.UUID]*queuedEmail) {
	for _, q := range batch {
		if q.email == "" {
			continue
		}
		if err := w.mailer.Send(ctx, email.Message{
			To:      q.email,
			Subject: "Намылимся: новые уведомления",
			Body:    renderEmailBody(q.items),
		}); err != nil {
			log.Printf("send email to %s: %v", q.email, err)
		}
	}
}

func renderEmailBody(items []emailItem) string {
	body := "<h2>Новые уведомления на Намылимся</h2><ul>"
	for _, it := range items {
		switch it.Type {
		case "application_new":
			title, _ := it.Payload["event_title"].(string)
			body += "<li>Новая заявка на событие «" + title + "»</li>"
		case "application_approved":
			body += "<li>Ваша заявка одобрена</li>"
		case "application_rejected":
			body += "<li>Ваша заявка отклонена</li>"
		case "event_cancelled":
			body += "<li>Событие отменено</li>"
		case "application_cancelled":
			body += "<li>Участник снял заявку</li>"
		}
	}
	body += "</ul><p>Откройте <a href=\"http://localhost:3000\">Намылимся</a>, чтобы посмотреть детали.</p>"
	return body
}

func (w *NotificationWorker) Stop() {
	w.wg.Wait()
}
