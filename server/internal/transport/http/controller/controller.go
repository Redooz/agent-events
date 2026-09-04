package controller

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"agent-events/server/internal/core/port"
	"agent-events/server/internal/transport/http/handler"
)

const requestTimeout = 30 * time.Second

func NewRouter(eventHandler *handler.EventHandler, log port.Logger) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(requestTimeout))
	r.Use(RequestLogger(log))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		handler.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/events", eventHandler.Register)
	})

	return r
}

func RequestLogger(log port.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			wrapped := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()

			defer func() {
				log.Info("http request",
					port.Str("method", r.Method),
					port.Str("path", r.URL.Path),
					port.Int("status", wrapped.Status()),
					port.Duration("duration", time.Since(start)),
					port.Str("request_id", middleware.GetReqID(r.Context())),
				)
			}()

			next.ServeHTTP(wrapped, r)
		})
	}
}
