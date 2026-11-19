package router

import (
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/SaratovDev/Namylimsya_App/api/internal/auth"
	"github.com/SaratovDev/Namylimsya_App/api/internal/handlers"
	custommw "github.com/SaratovDev/Namylimsya_App/api/internal/middleware"
	"github.com/SaratovDev/Namylimsya_App/api/internal/ratelimit"
)

type Deps struct {
	Health         *handlers.HealthHandler
	Auth           *handlers.AuthHandler
	User           *handlers.UserHandler
	Event          *handlers.EventHandler
	Application    *handlers.ApplicationHandler
	Chat           *handlers.ChatHandler
	WS             *handlers.WSHandler
	Notification   *handlers.NotificationHandler
	WSNotification *handlers.WSNotificationHandler
	TM             *auth.TokenManager
	SM             *auth.SessionManager
	Admin          *handlers.AdminHandler
	Support        *handlers.SupportHandler
	CORSOrigins    []string
	RL             *ratelimit.Limiter
	StoragePath    string // корень /static; монтируется внутри New
	TrustedProxy   bool   // доверять XFF только от private peer'ов; см. middleware.RealIP
}

func New(deps Deps) chi.Router {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	// Свой RealIP вместо chi's middleware.RealIP: доверяем XFF только
	// если TRUSTED_PROXY=true И peer private (Docker bridge / Caddy).
	// chi's RealIP безусловно верит XFF → bypass rate-limit при прямом доступе.
	r.Use(custommw.RealIP(deps.TrustedProxy))
	// Кастомный logger скрабит `?token=<jwt>` в WS-handshake —
	// иначе access-токен попадает в stdout/Docker logs/proxy logs.
	r.Use(middleware.RequestLogger(&custommw.ScrubLogFormatter{
		Logger: log.New(os.Stdout, "", log.LstdFlags),
	}))
	r.Use(middleware.Recoverer)

	allowedOrigins := deps.CORSOrigins
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"http://localhost:3000"}
	}

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-Id"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Раздача загруженных файлов — ПОСЛЕ всех r.Use().
	// chi требует: middlewares до первого route/mount.
	// Кастомный handler вместо http.FileServer:
	//   - запрет листинга директорий;
	//   - Content-Type по серверному снифу;
	//   - X-Content-Type-Options: nosniff;
	//   - CSP default-src 'none' — на случай, если сниф промахнулся;
	//   - только image/*, остальное 404.
	if deps.StoragePath != "" {
		r.Mount("/static", http.StripPrefix("/static", SafeStaticHandler(deps.StoragePath)))
	}

	r.Route("/api/v1", func(api chi.Router) {
		// --- Публичные ---
		api.Use(middleware.Timeout(30 * time.Second))
		api.Get("/health", deps.Health.Get)

		// Лента и просмотр события — публичные.
		// ВАЖНО: /events/archive регистрируем ДО /events/{id}, иначе chi
		// может матчить "archive" как {id} (в chi v5 литералы приоритетнее
		// параметров, но порядок регистрации читается лучше).
		api.With(custommw.OptionalAuth(deps.TM)).Get("/events", deps.Event.GetFeed)
		api.With(custommw.OptionalAuth(deps.TM)).Get("/events/archive", deps.Event.GetArchive)
		api.With(custommw.OptionalAuth(deps.TM)).Get("/events/{id}", deps.Event.GetByID)
		api.With(custommw.OptionalAuth(deps.TM)).Get("/events/{id}/participants", deps.Event.ListParticipants)
		api.Get("/users/{id}", deps.User.GetPublic)
		api.Get("/users/{id}/username-history", deps.User.GetUsernameHistory)

		// Поддержка — публичная (для апелляции забаненных), но с IP rate-limit:
		// аноним без ограничений зальёт БД тикетами.
		api.Group(func(r chi.Router) {
			r.Use(custommw.OptionalAuth(deps.TM))
			if deps.RL != nil {
				r.Use(custommw.IPRateLimit(deps.RL, "support_ticket:", 5, 10*time.Minute, true))
			}
			r.Post("/support/tickets", deps.Support.CreateTicket)
		})

		// Auth — защищаем rate-limit'ом по IP (10 запросов / минуту).
		// Без RL (nil) — работаем без ограничения (в тестах, если не прокинут).
		api.Route("/auth", func(r chi.Router) {
			if deps.RL != nil {
				r.Use(custommw.IPRateLimit(deps.RL, "auth_ip:", 10, time.Minute, false))
			}
			r.Post("/register", deps.Auth.Register)
			r.Post("/login", deps.Auth.Login)
			r.Post("/refresh", deps.Auth.Refresh)
			r.Post("/logout", deps.Auth.Logout)
			r.Post("/password-reset-request", deps.Auth.RequestPasswordReset)
			r.Post("/password-reset", deps.Auth.ResetPassword)
		})

		// --- Защищённые (AuthRequired) ---
		api.Group(func(r chi.Router) {
			r.Use(custommw.AuthRequired(deps.TM, deps.SM))
			// Users
			r.Get("/users/me", deps.Auth.Me)
			r.Patch("/users/me", deps.User.UpdateMe)
			r.Post("/users/me/avatar", deps.User.UploadAvatar)
			r.Get("/users/me/events", deps.Event.ListMine)
			r.Get("/users/me/applications", deps.Application.ListMine)

			// Events — создание и отмена (только для авторизованных)
			r.Post("/events", deps.Event.Create)
			r.Patch("/events/{id}", deps.Event.Update)
			r.Post("/events/{id}/cancel", deps.Event.Cancel)

			// Applications
			r.Post("/events/{id}/applications", deps.Application.Apply)
			r.Get("/events/{id}/applications", deps.Application.ListForEvent)
			r.Patch("/applications/{id}/approve", deps.Application.Approve)
			r.Patch("/applications/{id}/reject", deps.Application.Reject)
			r.Post("/applications/{id}/cancel", deps.Application.Cancel)

			// Chat
			r.Get("/events/{id}/messages", deps.Chat.History)
			r.Post("/events/{id}/messages", deps.Chat.SendHTTP)
			r.Post("/events/{id}/messages/read", deps.Chat.MarkRead)

			// Notifications
			r.Get("/notifications", deps.Notification.List)
			r.Patch("/notifications/{id}/read", deps.Notification.MarkRead)
			r.Patch("/notifications/read-all", deps.Notification.MarkAllRead)

			// Support (жалобы) — с rate-limit по IP, иначе авторизованный
			// юзер зальёт БД жалобами.
			r.Group(func(r chi.Router) {
				if deps.RL != nil {
					r.Use(custommw.IPRateLimit(deps.RL, "complaints:", 10, 10*time.Minute, true))
				}
				r.Post("/complaints", deps.Support.CreateComplaint)
			})
		})

		// --- Модератор или админ ---
		api.Group(func(r chi.Router) {
			r.Use(custommw.AuthRequired(deps.TM, deps.SM))
			r.Use(custommw.RequireRole("moderator", "admin"))

			// Users (просмотр + бан, но не смена роли)
			r.Get("/admin/users", deps.Admin.ListUsers)
			r.Get("/admin/users/{id}", deps.Admin.GetUser)
			r.Patch("/admin/users/{id}/ban", deps.Admin.BanUser)

			// Events (просмотр + отмена + скрытие)
			r.Get("/admin/events", deps.Admin.ListEvents)
			r.Get("/admin/events/{id}", deps.Admin.GetEvent)
			r.Post("/admin/events/{id}/cancel", deps.Admin.CancelEvent)
			r.Post("/admin/events/{id}/hide", deps.Admin.HideEvent)
			r.Post("/admin/events/{id}/unhide", deps.Admin.UnhideEvent)

			// Support / complaints
			r.Get("/admin/support/tickets", deps.Support.ListTickets)
			r.Get("/admin/support/tickets/{id}", deps.Support.GetTicket)
			r.Patch("/admin/support/tickets/{id}/resolve", deps.Support.ResolveTicket)
			r.Patch("/admin/support/tickets/{id}/reject", deps.Support.RejectTicket)
			r.Post("/admin/support/tickets/{id}/reply", deps.Support.ReplyToTicket)

			r.Get("/admin/complaints", deps.Support.ListComplaints)
			r.Get("/admin/complaints/{id}", deps.Support.GetComplaint)
			r.Patch("/admin/complaints/{id}/resolve", deps.Support.ResolveComplaint)
			r.Patch("/admin/complaints/{id}/reject", deps.Support.RejectComplaint)

			r.Delete("/admin/messages/{id}", deps.Support.DeleteMessage)
		})

		// --- Только для админа ---
		api.Group(func(r chi.Router) {
			r.Use(custommw.AuthRequired(deps.TM, deps.SM))
			r.Use(custommw.RequireRole("admin"))

			r.Get("/admin/dashboard", deps.Admin.Dashboard)
			r.Get("/admin/audit-log", deps.Admin.ListAuditLog)
			r.Patch("/admin/users/{id}/role", deps.Admin.UpdateUserRole)
			r.Delete("/admin/events/{id}", deps.Admin.DeleteEvent)
		})
	})

	// WebSocket-каналы (вне /api/v1)
	r.Get("/ws/events/{id}", deps.WS.Handle)
	r.Get("/ws/notifications", deps.WSNotification.Handle)

	return r
}

// SafeStaticHandler — безопасная раздача статики (аватары).
// Гарантирует: только image/*, без листинга директорий, с nosniff и CSP.
func SafeStaticHandler(dir string) http.Handler {
	fs := http.Dir(dir)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Запрет листинга директорий.
		if strings.HasSuffix(r.URL.Path, "/") || r.URL.Path == "" {
			http.NotFound(w, r)
			return
		}

		f, err := fs.Open(r.URL.Path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()

		stat, err := f.Stat()
		if err != nil || stat.IsDir() {
			http.NotFound(w, r)
			return
		}

		// Сниф первых 512 байт.
		buf := make([]byte, 512)
		n, _ := f.Read(buf)
		ct := http.DetectContentType(buf[:n])
		if !strings.HasPrefix(ct, "image/") {
			http.NotFound(w, r)
			return
		}

		// Перемотка для ServeContent.
		if seeker, ok := f.(io.Seeker); ok {
			if _, err := seeker.Seek(0, io.SeekStart); err != nil {
				http.Error(w, "internal", http.StatusInternalServerError)
				return
			}
		}

		w.Header().Set("Content-Type", ct)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Disposition", "inline")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src 'self'")

		rs, ok := f.(io.ReadSeeker)
		if !ok {
			http.Error(w, "internal", http.StatusInternalServerError)
			return
		}
		http.ServeContent(w, r, stat.Name(), stat.ModTime(), rs)
	})
}
