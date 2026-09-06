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
	svc            *usecase.AuthService
	log            port.Logger
	validate       *validator.Validate
	trustedProxies []*net.IPNet
}

func NewAuthHandler(svc *usecase.AuthService, log port.Logger, trustedProxies []*net.IPNet) *AuthHandler {
	return &AuthHandler{
		svc:            svc,
		log:            log,
		validate:       validator.New(validator.WithRequiredStructEnabled()),
		trustedProxies: trustedProxies,
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

	result, err := h.svc.Exchange(r.Context(), req.ToInput(clientIP(r, h.trustedProxies)))
	if err != nil {
		WriteError(w, h.log, err)
		return
	}

	WriteJSON(w, http.StatusOK, dto.ExchangeResponse{
		Token:     result.Token,
		ExpiresAt: result.ExpiresAt,
		User:      dto.NewUserResponse(result.User),
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	identity, ok := authctx.UserFrom(r.Context())
	if !ok {
		WriteError(w, h.log, apperr.Unauthorized("authentication required"))
		return
	}

	if err := h.svc.RevokeUserToken(r.Context(), identity.TokenHash); err != nil {
		WriteError(w, h.log, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) Whoami(w http.ResponseWriter, r *http.Request) {
	identity, ok := authctx.AgentFrom(r.Context())
	if !ok {
		WriteError(w, h.log, apperr.Unauthorized("authentication required"))
		return
	}

	WriteJSON(w, http.StatusOK, dto.WhoamiResponse{
		Agent: dto.NewAgentResponse(identity.Agent),
		User:  dto.NewUserResponse(identity.User),
	})
}

func clientIP(r *http.Request, trusted []*net.IPNet) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}

	peer := net.ParseIP(host)
	if peer == nil || !isTrustedProxy(peer, trusted) {
		return host
	}

	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded == "" {
		return host
	}

	parts := strings.Split(forwarded, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		entry := strings.TrimSpace(parts[i])
		if entry == "" {
			continue
		}

		if ip := net.ParseIP(entry); ip != nil && isTrustedProxy(ip, trusted) {
			continue
		}

		return entry
	}

	return host
}

func isTrustedProxy(ip net.IP, trusted []*net.IPNet) bool {
	for _, network := range trusted {
		if network.Contains(ip) {
			return true
		}
	}

	return false
}
