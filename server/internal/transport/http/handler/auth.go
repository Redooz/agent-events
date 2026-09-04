package handler

import (
	"net"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"

	"agent-events/server/internal/core/port"
	"agent-events/server/internal/core/usecase"
	"agent-events/server/internal/transport/http/authctx"
	"agent-events/server/internal/transport/http/dto"
	"agent-events/server/pkg/apperr"
)

type AuthHandler struct {
	svc      *usecase.AuthService
	log      port.Logger
	validate *validator.Validate
}

func NewAuthHandler(svc *usecase.AuthService, log port.Logger) *AuthHandler {
	return &AuthHandler{
		svc:      svc,
		log:      log,
		validate: validator.New(validator.WithRequiredStructEnabled()),
	}
}

func (h *AuthHandler) Exchange(w http.ResponseWriter, r *http.Request) {
	var req dto.ExchangeRequest
	if !DecodeJSON(w, r, h.log, &req) {
		return
	}

	if err := h.validate.Struct(req); err != nil {
		WriteError(w, h.log, validationError(err))
		return
	}

	result, err := h.svc.Exchange(r.Context(), req.ToInput(clientIP(r)))
	if err != nil {
		WriteError(w, h.log, err)
		return
	}

	WriteJSON(w, http.StatusOK, dto.ExchangeResponse{
		Token:     result.Token,
		ExpiresAt: result.ExpiresAt,
		Owner:     dto.NewOwnerResponse(result.Owner),
	})
}

func (h *AuthHandler) Whoami(w http.ResponseWriter, r *http.Request) {
	identity, ok := authctx.AgentFrom(r.Context())
	if !ok {
		WriteError(w, h.log, apperr.Unauthorized("authentication required"))
		return
	}

	WriteJSON(w, http.StatusOK, dto.WhoamiResponse{
		Agent: dto.NewAgentResponse(identity.Agent),
		Owner: dto.NewOwnerResponse(identity.Owner),
	})
}

func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		first, _, _ := strings.Cut(forwarded, ",")
		if ip := strings.TrimSpace(first); ip != "" {
			return ip
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}
