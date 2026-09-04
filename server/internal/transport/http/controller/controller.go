package controller

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"agent-events/server/internal/core/port"
	"agent-events/server/internal/transport/http/handler"
	"agent-events/server/internal/transport/http/middleware"
)

const requestTimeout = 30 * time.Second

func NewRouter(
	eventHandler *handler.EventHandler,
	authHandler *handler.AuthHandler,
	agentHandler *handler.AgentHandler,
	auth *middleware.Auth,
	log port.Logger,
) *chi.Mux {
	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(requestTimeout))
	r.Use(RequestLogger(log))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		handler.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/exchange", authHandler.Exchange)

		r.Group(func(r chi.Router) {
			r.Use(auth.RequireOwner)
			r.Route("/agents", agentHandler.Register)
		})

		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAgent)
			r.Get("/auth/whoami", authHandler.Whoami)
			r.Route("/events", eventHandler.Register)
		})
	})

	return r
}

func RequestLogger(log port.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			wrapped := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()

			defer func() {
				log.Info("http request",
					port.Str("method", r.Method),
					port.Str("path", r.URL.Path),
					port.Int("status", wrapped.Status()),
					port.Duration("duration", time.Since(start)),
					port.Str("request_id", chimw.GetReqID(r.Context())),
				)
			}()

			next.ServeHTTP(wrapped, r)
		})
	}
}
