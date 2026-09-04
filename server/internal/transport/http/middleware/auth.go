package middleware

import (
	"net/http"
	"strings"

	"agent-events/server/internal/core/port"
	"agent-events/server/internal/core/usecase"
	"agent-events/server/internal/transport/http/authctx"
	"agent-events/server/internal/transport/http/handler"
	"agent-events/server/pkg/apperr"
)

type Auth struct {
	auth *usecase.AuthService
	log  port.Logger
}

func NewAuth(auth *usecase.AuthService, log port.Logger) *Auth {
	return &Auth{auth: auth, log: log}
}

func (a *Auth) RequireAgent(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, ok := bearerToken(r)
		if !ok {
			handler.WriteError(w, a.log, apperr.Unauthorized("missing agent key, use Authorization: Bearer <agent key>"))
			return
		}

		creds, err := a.auth.AuthenticateAgent(r.Context(), raw)
		if err != nil {
			handler.WriteError(w, a.log, err)
			return
		}

		ctx := authctx.WithAgent(r.Context(), authctx.AgentIdentity{Agent: creds.Agent, Owner: creds.Owner})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *Auth) RequireOwner(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, ok := bearerToken(r)
		if !ok {
			handler.WriteError(w, a.log, apperr.Unauthorized("missing owner token, use Authorization: Bearer <owner token>"))
			return
		}

		owner, err := a.auth.AuthenticateOwner(r.Context(), raw)
		if err != nil {
			handler.WriteError(w, a.log, err)
			return
		}

		ctx := authctx.WithOwner(r.Context(), owner)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func bearerToken(r *http.Request) (string, bool) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if header == "" {
		return "", false
	}

	raw, found := strings.CutPrefix(header, "Bearer ")
	if !found {
		return "", false
	}

	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}

	return raw, true
}
