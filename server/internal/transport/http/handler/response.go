package handler

import (
	"encoding/json"
	"net/http"

	"agent-events/server/internal/core/port"
	"agent-events/server/pkg/apperr"
)

type ErrorBody struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

func WriteJSON(w http.ResponseWriter, status int, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":"internal","message":"could not encode response"}}`))

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func WriteError(w http.ResponseWriter, log port.Logger, err error) {
	status := StatusCode(err)

	if status >= http.StatusInternalServerError {
		log.Error("request failed", port.Err(err), port.Int("status", status))
	} else {
		log.Warn("request rejected", port.Err(err), port.Int("status", status))
	}

	message := apperr.MessageOf(err)
	if status >= http.StatusInternalServerError {
		message = http.StatusText(status)
	}

	WriteJSON(w, status, ErrorResponse{Error: ErrorBody{
		Code:    string(apperr.KindOf(err)),
		Message: message,
		Details: apperr.DetailsOf(err),
	}})
}

func StatusCode(err error) int {
	switch apperr.KindOf(err) {
	case apperr.KindNotFound:
		return http.StatusNotFound
	case apperr.KindInvalid:
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
