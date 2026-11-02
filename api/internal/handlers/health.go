package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/SaratovDev/Namylimsya_App/api/internal/httpx"
)

type HealthHandler struct {
	DB    *pgxpool.Pool
	Redis *redis.Client
}

type healthResponse struct {
	Status string `json:"status"`
	DB     string `json:"db"`
	Redis  string `json:"redis"`
	Time   string `json:"time"`
}

func (h *HealthHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	resp := healthResponse{
		Status: "ok",
		DB:     "ok",
		Redis:  "ok",
		Time:   time.Now().UTC().Format(time.RFC3339),
	}
	status := http.StatusOK

	if h.DB != nil {
		if err := h.DB.Ping(ctx); err != nil {
			resp.DB = "error"
			resp.Status = "degraded"
			status = http.StatusServiceUnavailable
		}
	} else {
		resp.DB = "not_configured"
	}

	if h.Redis != nil {
		if err := h.Redis.Ping(ctx).Err(); err != nil {
			resp.Redis = "error"
			resp.Status = "degraded"
			status = http.StatusServiceUnavailable
		}
	} else {
		resp.Redis = "not_configured"
	}

	httpx.JSON(w, status, resp)
}