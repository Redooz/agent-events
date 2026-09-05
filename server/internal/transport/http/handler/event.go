package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"agent-events/server/internal/core/port"
	"agent-events/server/internal/core/usecase"
	"agent-events/server/internal/transport/http/authctx"
	"agent-events/server/internal/transport/http/dto"
	"agent-events/server/pkg/apperr"
)

type EventHandler struct {
	svc      *usecase.EventService
	log      port.Logger
	validate *validator.Validate
}

func NewEventHandler(svc *usecase.EventService, log port.Logger) *EventHandler {
	return &EventHandler{
		svc:      svc,
		log:      log,
		validate: validator.New(validator.WithRequiredStructEnabled()),
	}
}

func (h *EventHandler) Register(r chi.Router) {
	r.Post("/", h.create)
	r.Get("/", h.list)
	r.Get("/{id}", h.get)
	r.Put("/{id}", h.update)
	r.Delete("/{id}", h.delete)
}

func (h *EventHandler) create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateEventRequest
	if !DecodeJSON(w, r, h.log, &req) {
		return
	}

	if err := h.validate.Struct(req); err != nil {
		WriteError(w, h.log, validationError(err))
		return
	}

	actor, ok := authctx.AgentFrom(r.Context())
	if !ok {
		WriteError(w, h.log, apperr.Unauthorized("authentication required"))
		return
	}

	event, err := h.svc.Create(r.Context(), usecase.Actor{OwnerID: actor.Owner.ID, AgentID: actor.Agent.ID}, req.ToInput())
	if err != nil {
		WriteError(w, h.log, err)
		return
	}

	WriteJSON(w, http.StatusCreated, dto.NewEventResponse(event))
}

func (h *EventHandler) get(w http.ResponseWriter, r *http.Request) {
	id, err := eventURLID(r)
	if err != nil {
		WriteError(w, h.log, err)
		return
	}

	event, err := h.svc.Get(r.Context(), id)
	if err != nil {
		WriteError(w, h.log, err)
		return
	}

	WriteJSON(w, http.StatusOK, dto.NewEventResponse(event))
}

func (h *EventHandler) list(w http.ResponseWriter, r *http.Request) {
	events, err := h.svc.List(r.Context())
	if err != nil {
		WriteError(w, h.log, err)
		return
	}

	WriteJSON(w, http.StatusOK, dto.NewEventResponses(events))
}

func (h *EventHandler) update(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateEventRequest
	if !DecodeJSON(w, r, h.log, &req) {
		return
	}

	if err := h.validate.Struct(req); err != nil {
		WriteError(w, h.log, validationError(err))
		return
	}

	actor, ok := authctx.AgentFrom(r.Context())
	if !ok {
		WriteError(w, h.log, apperr.Unauthorized("authentication required"))
		return
	}

	id, err := eventURLID(r)
	if err != nil {
		WriteError(w, h.log, err)
		return
	}

	event, err := h.svc.Update(r.Context(), usecase.Actor{OwnerID: actor.Owner.ID, AgentID: actor.Agent.ID}, id, req.ToInput())
	if err != nil {
		WriteError(w, h.log, err)
		return
	}

	WriteJSON(w, http.StatusOK, dto.NewEventResponse(event))
}

func (h *EventHandler) delete(w http.ResponseWriter, r *http.Request) {
	actor, ok := authctx.AgentFrom(r.Context())
	if !ok {
		WriteError(w, h.log, apperr.Unauthorized("authentication required"))
		return
	}

	id, err := eventURLID(r)
	if err != nil {
		WriteError(w, h.log, err)
		return
	}

	if err := h.svc.Delete(r.Context(), usecase.Actor{OwnerID: actor.Owner.ID, AgentID: actor.Agent.ID}, id); err != nil {
		WriteError(w, h.log, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func eventURLID(r *http.Request) (string, error) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		return "", apperr.NotFound("event not found")
	}

	return id, nil
}

func validationError(err error) error {
	var invalid validator.ValidationErrors
	if !errors.As(err, &invalid) {
		return apperr.Invalid(err.Error())
	}

	details := make(map[string]string, len(invalid))
	for _, field := range invalid {
		details[jsonName(field.Field())] = messageFor(field)
	}

	return apperr.Invalid("request validation failed").WithDetails(details)
}

func messageFor(field validator.FieldError) string {
	switch field.Tag() {
	case "required":
		return "is required"
	case "max":
		return fmt.Sprintf("must be at most %s characters", field.Param())
	case "min":
		return fmt.Sprintf("must be at least %s characters", field.Param())
	case "oneof":
		return fmt.Sprintf("must be one of %s", field.Param())
	default:
		return fmt.Sprintf("is invalid (%s)", field.Tag())
	}
}

func jsonName(field string) string {
	if field == "" {
		return field
	}

	return strings.ToLower(field[:1]) + field[1:]
}
