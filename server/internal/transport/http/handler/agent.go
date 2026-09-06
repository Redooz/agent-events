package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"agent-events/server/internal/core/port"
	"agent-events/server/internal/core/usecase"
	"agent-events/server/internal/transport/http/authctx"
	"agent-events/server/internal/transport/http/dto"
	"agent-events/server/pkg/apperr"
)

type AgentHandler struct {
	svc      *usecase.AuthService
	log      port.Logger
	validate *validator.Validate
}

func NewAgentHandler(svc *usecase.AuthService, log port.Logger) *AgentHandler {
	return &AgentHandler{
		svc:      svc,
		log:      log,
		validate: validator.New(validator.WithRequiredStructEnabled()),
	}
}

func (h *AgentHandler) Register(r chi.Router) {
	r.Post("/", h.create)
	r.Get("/", h.list)
	r.Delete("/{id}", h.revoke)
}

func (h *AgentHandler) create(w http.ResponseWriter, r *http.Request) {
	identity, ok := authctx.UserFrom(r.Context())
	if !ok {
		WriteError(w, h.log, apperr.Unauthorized("authentication required"))
		return
	}

	var req dto.CreateAgentRequest
	if !DecodeJSON(w, r, h.log, &req) {
		return
	}

	if err := h.validate.Struct(req); err != nil {
		WriteError(w, h.log, validationError(err))
		return
	}

	result, err := h.svc.CreateAgent(r.Context(), identity.User.ID, req.Name)
	if err != nil {
		WriteError(w, h.log, err)
		return
	}

	WriteJSON(w, http.StatusCreated, dto.CreateAgentResponse{
		Agent: dto.NewAgentResponse(result.Agent),
		Key:   result.Key,
	})
}

func (h *AgentHandler) list(w http.ResponseWriter, r *http.Request) {
	identity, ok := authctx.UserFrom(r.Context())
	if !ok {
		WriteError(w, h.log, apperr.Unauthorized("authentication required"))
		return
	}

	agents, err := h.svc.ListAgents(r.Context(), identity.User.ID)
	if err != nil {
		WriteError(w, h.log, err)
		return
	}

	WriteJSON(w, http.StatusOK, dto.NewAgentResponses(agents))
}

func (h *AgentHandler) revoke(w http.ResponseWriter, r *http.Request) {
	identity, ok := authctx.UserFrom(r.Context())
	if !ok {
		WriteError(w, h.log, apperr.Unauthorized("authentication required"))
		return
	}

	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		WriteError(w, h.log, apperr.NotFound("agent not found"))
		return
	}

	if err := h.svc.RevokeAgent(r.Context(), identity.User.ID, id); err != nil {
		WriteError(w, h.log, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
