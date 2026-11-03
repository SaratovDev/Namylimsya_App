package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/SaratovDev/Namylimsya_App/api/internal/auth"
	"github.com/SaratovDev/Namylimsya_App/api/internal/handlers"
	custommw "github.com/SaratovDev/Namylimsya_App/api/internal/middleware"
)

type Deps struct {
	Health *handlers.HealthHandler
	Auth   *handlers.AuthHandler
	User   *handlers.UserHandler
	Event  *handlers.EventHandler
	TM     *auth.TokenManager
}

func New(deps Deps) chi.Router {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * 1e9))

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000"},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-Id"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Route("/api/v1", func(api chi.Router) {
		api.Get("/health", deps.Health.Get)
		api.Get("/events", deps.Event.GetFeed)      // публичный
		api.Get("/events/{id}", deps.Event.GetByID) // публичный
		api.Get("/users/{id}", deps.User.GetPublic) // публичный

		api.Route("/auth", func(r chi.Router) {
			r.Post("/register", deps.Auth.Register)
			r.Post("/login", deps.Auth.Login)
			r.Post("/refresh", deps.Auth.Refresh)
			r.Post("/logout", deps.Auth.Logout)
		})

		// Защищённые роуты
		api.Group(func(r chi.Router) {
			r.Use(custommw.AuthRequired(deps.TM))
			r.Get("/users/me", deps.Auth.Me)
			r.Patch("/users/me", deps.User.UpdateMe)
			r.Post("/users/me/avatar", deps.User.UploadAvatar)
			r.Post("/events", deps.Event.Create)
			r.Post("/events/{id}/cancel", deps.Event.Cancel)
		})

		// Публичные роуты
		api.Get("/users/{id}", deps.User.GetPublic)

	})

	return r
}
