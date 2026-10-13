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

type QRHandler struct {
	svc      *usecase.QRService
	log      port.Logger
	validate *validator.Validate
}

func NewQRHandler(svc *usecase.QRService, log port.Logger) *QRHandler {
	return &QRHandler{
		svc:      svc,
		log:      log,
		validate: validator.New(validator.WithRequiredStructEnabled()),
	}
}

func (h *QRHandler) Register(r chi.Router) {
	r.Post("/{id}/qr", h.generate)
	r.Post("/{id}/qr/scan", h.scan)
}

func (h *QRHandler) generate(w http.ResponseWriter, r *http.Request) {
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

	code, err := h.svc.Generate(r.Context(), identity.User.ID, id)
	if err != nil {
		WriteError(w, h.log, err)
		return
	}

	WriteJSON(w, http.StatusCreated, dto.NewGenerateQRResponse(code))
}

func (h *QRHandler) scan(w http.ResponseWriter, r *http.Request) {
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

	var req dto.ConsumeQRRequest
	if !DecodeJSON(w, r, h.log, &req) {
		return
	}

	if err := h.validate.Struct(req); err != nil {
		WriteError(w, h.log, validationError(err))
		return
	}

	result, err := h.svc.Consume(r.Context(), identity.User.ID, id, req.ToPayload())
	if err != nil {
		WriteError(w, h.log, err)
		return
	}

	WriteJSON(w, http.StatusOK, dto.NewScanResponse(result))
}
