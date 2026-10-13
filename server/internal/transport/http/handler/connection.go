package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"

	"agent-events/server/internal/core/port"
	"agent-events/server/internal/core/usecase"
	"agent-events/server/internal/transport/http/authctx"
	"agent-events/server/internal/transport/http/dto"
	"agent-events/server/pkg/apperr"
)

type ConnectionHandler struct {
	svc      *usecase.ConnectionService
	log      port.Logger
	validate *validator.Validate
}

func NewConnectionHandler(svc *usecase.ConnectionService, log port.Logger) *ConnectionHandler {
	return &ConnectionHandler{
		svc:      svc,
		log:      log,
		validate: validator.New(validator.WithRequiredStructEnabled()),
	}
}

func (h *ConnectionHandler) Register(r chi.Router) {
	r.Post("/", h.request)
	r.Get("/", h.listRequests)
	r.Post("/{id}/accept", h.accept)
	r.Post("/{id}/decline", h.decline)
}

func (h *ConnectionHandler) request(w http.ResponseWriter, r *http.Request) {
	identity, ok := authctx.UserFrom(r.Context())
	if !ok {
		WriteError(w, h.log, apperr.Unauthorized("authentication required"))
		return
	}

	var req dto.CreateConnectionRequest
	if !DecodeJSON(w, r, h.log, &req) {
		return
	}

	if err := h.validate.Struct(req); err != nil {
		WriteError(w, h.log, validationError(err))
		return
	}

	result, err := h.svc.Request(r.Context(), identity.User.ID, req.EventID, req.ToUserID)
	if err != nil {
		WriteError(w, h.log, err)
		return
	}

	WriteJSON(w, http.StatusCreated, dto.NewRequestConnectionResult(result))
}

func (h *ConnectionHandler) listRequests(w http.ResponseWriter, r *http.Request) {
	identity, ok := authctx.UserFrom(r.Context())
	if !ok {
		WriteError(w, h.log, apperr.Unauthorized("authentication required"))
		return
	}

	requests, err := h.svc.ListRequests(r.Context(), identity.User.ID)
	if err != nil {
		WriteError(w, h.log, err)
		return
	}

	WriteJSON(w, http.StatusOK, dto.NewConnectionRequestResponses(requests))
}

func (h *ConnectionHandler) accept(w http.ResponseWriter, r *http.Request) {
	identity, ok := authctx.UserFrom(r.Context())
	if !ok {
		WriteError(w, h.log, apperr.Unauthorized("authentication required"))
		return
	}

	id, err := urlParamID(r, "connection request not found")
	if err != nil {
		WriteError(w, h.log, err)
		return
	}

	connection, err := h.svc.Accept(r.Context(), identity.User.ID, id)
	if err != nil {
		WriteError(w, h.log, err)
		return
	}

	WriteJSON(w, http.StatusOK, dto.ConnectionResponseFrom(connection))
}

func (h *ConnectionHandler) decline(w http.ResponseWriter, r *http.Request) {
	identity, ok := authctx.UserFrom(r.Context())
	if !ok {
		WriteError(w, h.log, apperr.Unauthorized("authentication required"))
		return
	}

	id, err := urlParamID(r, "connection request not found")
	if err != nil {
		WriteError(w, h.log, err)
		return
	}

	if err := h.svc.Decline(r.Context(), identity.User.ID, id); err != nil {
		WriteError(w, h.log, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *ConnectionHandler) ListConnections(w http.ResponseWriter, r *http.Request) {
	identity, ok := authctx.UserFrom(r.Context())
	if !ok {
		WriteError(w, h.log, apperr.Unauthorized("authentication required"))
		return
	}

	connections, err := h.svc.List(r.Context(), identity.User.ID)
	if err != nil {
		WriteError(w, h.log, err)
		return
	}

	WriteJSON(w, http.StatusOK, dto.NewConnectionResponses(connections))
}
