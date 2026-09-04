package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"

	"agent-events/server/internal/core/port"
	"agent-events/server/internal/core/usecase"
	"agent-events/server/internal/transport/http/dto"
	"agent-events/server/pkg/apperr"
)

const maxBodySize = 1 << 20

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
	if !h.decode(w, r, &req) {
		return
	}

	if err := h.validate.Struct(req); err != nil {
		WriteError(w, h.log, validationError(err))
		return
	}

	event, err := h.svc.Create(r.Context(), req.ToInput())
	if err != nil {
		WriteError(w, h.log, err)
		return
	}

	WriteJSON(w, http.StatusCreated, dto.NewEventResponse(event))
}

func (h *EventHandler) get(w http.ResponseWriter, r *http.Request) {
	event, err := h.svc.Get(r.Context(), chi.URLParam(r, "id"))
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
	if !h.decode(w, r, &req) {
		return
	}

	if err := h.validate.Struct(req); err != nil {
		WriteError(w, h.log, validationError(err))
		return
	}

	event, err := h.svc.Update(r.Context(), chi.URLParam(r, "id"), req.ToInput())
	if err != nil {
		WriteError(w, h.log, err)
		return
	}

	WriteJSON(w, http.StatusOK, dto.NewEventResponse(event))
}

func (h *EventHandler) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		WriteError(w, h.log, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *EventHandler) decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		WriteError(w, h.log, apperr.Invalid("request body must be valid JSON"))
		return false
	}

	return true
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
