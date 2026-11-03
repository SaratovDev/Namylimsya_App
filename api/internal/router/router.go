package router

import (
	"net/http"

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
	TM     *auth.TokenManager
}

func New(deps Deps) http.Handler {
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

		api.Route("/auth", func(r chi.Router) {
			r.Post("/register", deps.Auth.Register)
			r.Post("/login", deps.Auth.Login)
			// api.Post("/refresh", deps.Auth.Refresh) // Пока пропустим
			// api.Post("/logout", deps.Auth.Logout)   // Пока пропустим
		})

		// Защищённые роуты
		api.Group(func(r chi.Router) {
			r.Use(custommw.AuthRequired(deps.TM))
			r.Get("/users/me", deps.Auth.Me)
		})
	})

	return r
}
