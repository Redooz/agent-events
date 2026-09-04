package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"agent-events/server/internal/adapters/memory"
	"agent-events/server/internal/core/port"
	"agent-events/server/internal/core/usecase"
	"agent-events/server/internal/transport/http/controller"
	"agent-events/server/internal/transport/http/dto"
	"agent-events/server/internal/transport/http/handler"
)

type noopLogger struct{}

func (noopLogger) Debug(string, ...port.Field) {}
func (noopLogger) Info(string, ...port.Field)  {}
func (noopLogger) Warn(string, ...port.Field)  {}
func (noopLogger) Error(string, ...port.Field) {}

func newRouter(t *testing.T) http.Handler {
	t.Helper()

	svc := usecase.NewEventService(memory.NewEventRepository(), noopLogger{})

	return controller.NewRouter(handler.NewEventHandler(svc, noopLogger{}), noopLogger{})
}

func do(t *testing.T, router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(method, path, reader))

	return rec
}

func decodeEvent(t *testing.T, rec *httptest.ResponseRecorder) dto.EventResponse {
	t.Helper()

	var event dto.EventResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &event); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}

	return event
}

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) handler.ErrorResponse {
	t.Helper()

	var payload handler.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}

	return payload
}

func createEvent(t *testing.T, router http.Handler, name string) string {
	t.Helper()

	rec := do(t, router, http.MethodPost, "/api/v1/events", `{"name":"`+name+`"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /events = %d, want %d (%s)", rec.Code, http.StatusCreated, rec.Body.String())
	}

	return decodeEvent(t, rec).ID
}

func TestCreateReturnsCreatedEvent(t *testing.T) {
	t.Parallel()

	rec := do(t, newRouter(t), http.MethodPost, "/api/v1/events",
		`{"name":"deploy","description":"shipped v2"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusCreated, rec.Body.String())
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	event := decodeEvent(t, rec)
	if event.ID == "" {
		t.Error("response is missing an id")
	}

	if event.Name != "deploy" || event.Description != "shipped v2" {
		t.Errorf("response = %+v, want the created event", event)
	}
}

func TestCreateRejectsMissingName(t *testing.T) {
	t.Parallel()

	rec := do(t, newRouter(t), http.MethodPost, "/api/v1/events", `{"description":"no name"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	payload := decodeError(t, rec)
	if payload.Error.Code != "invalid" {
		t.Errorf("error code = %q, want invalid", payload.Error.Code)
	}

	if payload.Error.Details["name"] != "is required" {
		t.Errorf("details = %v, want name/is required", payload.Error.Details)
	}
}

func TestCreateRejectsMalformedJSON(t *testing.T) {
	t.Parallel()

	rec := do(t, newRouter(t), http.MethodPost, "/api/v1/events", `{"name":`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestGetReturnsStoredEvent(t *testing.T) {
	t.Parallel()

	router := newRouter(t)
	id := createEvent(t, router, "deploy")

	rec := do(t, router, http.MethodGet, "/api/v1/events/"+id, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusOK, rec.Body.String())
	}

	if got := decodeEvent(t, rec).Name; got != "deploy" {
		t.Errorf("name = %q, want deploy", got)
	}
}

func TestGetUnknownEventReturnsNotFound(t *testing.T) {
	t.Parallel()

	rec := do(t, newRouter(t), http.MethodGet, "/api/v1/events/missing", "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	if got := decodeError(t, rec).Error.Code; got != "not_found" {
		t.Errorf("error code = %q, want not_found", got)
	}
}

func TestListReturnsCreatedEvents(t *testing.T) {
	t.Parallel()

	router := newRouter(t)
	createEvent(t, router, "one")
	createEvent(t, router, "two")

	rec := do(t, router, http.MethodGet, "/api/v1/events", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var events []dto.EventResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &events); err != nil {
		t.Fatalf("decode list: %v", err)
	}

	if len(events) != 2 {
		t.Errorf("list returned %d events, want 2", len(events))
	}
}

func TestUpdateModifiesEvent(t *testing.T) {
	t.Parallel()

	router := newRouter(t)
	id := createEvent(t, router, "deploy")

	rec := do(t, router, http.MethodPut, "/api/v1/events/"+id, `{"name":"rollback"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusOK, rec.Body.String())
	}

	if got := decodeEvent(t, rec).Name; got != "rollback" {
		t.Errorf("name = %q, want rollback", got)
	}
}

func TestUpdateUnknownEventReturnsNotFound(t *testing.T) {
	t.Parallel()

	rec := do(t, newRouter(t), http.MethodPut, "/api/v1/events/missing", `{"name":"rollback"}`)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestDeleteRemovesEvent(t *testing.T) {
	t.Parallel()

	router := newRouter(t)
	id := createEvent(t, router, "deploy")

	rec := do(t, router, http.MethodDelete, "/api/v1/events/"+id, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusNoContent, rec.Body.String())
	}

	rec = do(t, router, http.MethodGet, "/api/v1/events/"+id, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status after delete = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHealthzReportsOK(t *testing.T) {
	t.Parallel()

	rec := do(t, newRouter(t), http.MethodGet, "/healthz", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
