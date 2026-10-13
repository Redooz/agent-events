package handler

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"agent-events/server/internal/core/port"
	"agent-events/server/internal/core/usecase"
	"agent-events/server/internal/transport/http/authctx"
	"agent-events/server/internal/transport/http/dto"
	"agent-events/server/pkg/apperr"
)

type AttendanceHandler struct {
	svc *usecase.AttendanceService
	log port.Logger
}

func NewAttendanceHandler(svc *usecase.AttendanceService, log port.Logger) *AttendanceHandler {
	return &AttendanceHandler{svc: svc, log: log}
}

func (h *AttendanceHandler) RegisterUser(r chi.Router) {
	r.Get("/{id}/attendees", h.attendees)
}

func (h *AttendanceHandler) RegisterEither(r chi.Router) {
	r.Post("/{id}/attend", h.attend)
}

func (h *AttendanceHandler) attend(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFrom(r.Context())
	if !ok {
		WriteError(w, h.log, apperr.Unauthorized("authentication required"))
		return
	}

	id, err := urlParamID(r, "event not found")
	if err != nil {
		WriteError(w, h.log, err)
		return
	}

	if err := h.svc.Attend(r.Context(), actor, id); err != nil {
		WriteError(w, h.log, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *AttendanceHandler) attendees(w http.ResponseWriter, r *http.Request) {
	identity, ok := authctx.UserFrom(r.Context())
	if !ok {
		WriteError(w, h.log, apperr.Unauthorized("authentication required"))
		return
	}

	id, err := urlParamID(r, "event not found")
	if err != nil {
		WriteError(w, h.log, err)
		return
	}

	attendees, err := h.svc.List(r.Context(), identity.User.ID, id)
	if err != nil {
		WriteError(w, h.log, err)
		return
	}

	WriteJSON(w, http.StatusOK, dto.NewAttendeeResponses(attendees))
}

func actorFrom(ctx context.Context) (usecase.Actor, bool) {
	if agent, ok := authctx.AgentFrom(ctx); ok {
		return usecase.Actor{UserID: agent.User.ID, AgentID: agent.Agent.ID}, true
	}

	if user, ok := authctx.UserFrom(ctx); ok {
		return usecase.Actor{UserID: user.User.ID}, true
	}

	return usecase.Actor{}, false
}

func urlParamID(r *http.Request, notFound string) (string, error) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		return "", apperr.NotFound(notFound)
	}

	return id, nil
}
