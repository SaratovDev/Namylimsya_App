package httpx

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/rs/zerolog/log"
)

type ErrorBody struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

func JSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Error().Err(err).Msg("encode response")
	}
}

func Error(w http.ResponseWriter, status int, code, message string) {
	JSON(w, status, ErrorBody{Error: code, Message: message})
}

func OK(w http.ResponseWriter, payload any) { JSON(w, http.StatusOK, payload) }

// InternalError — ответ 500 без утечки внутренних деталей (SQL, драйвер,
// имена таблиц/констрейнтов, пути файлов) в публичный API. err логируется
// полностью, клиенту отдаётся generic "internal server error".
// В APP_ENV != "production" отдаётся err.Error() — для отладки в dev/тестах.
func InternalError(w http.ResponseWriter, err error) {
	log.Error().Err(err).Msg("internal error")
	msg := "internal server error"
	if os.Getenv("APP_ENV") != "production" {
		msg = err.Error()
	}
	Error(w, http.StatusInternalServerError, "internal", msg)
}
