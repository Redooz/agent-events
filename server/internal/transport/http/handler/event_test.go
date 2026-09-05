package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"agent-events/server/internal/adapters/memory"
	"agent-events/server/internal/core/domain"
	"agent-events/server/internal/core/port"
	"agent-events/server/internal/core/usecase"
	"agent-events/server/internal/transport/http/controller"
	"agent-events/server/internal/transport/http/dto"
	"agent-events/server/internal/transport/http/handler"
	"agent-events/server/internal/transport/http/middleware"
	"agent-events/server/pkg/apperr"
)

type noopLogger struct{}

func (noopLogger) Debug(string, ...port.Field) {}
func (noopLogger) Info(string, ...port.Field)  {}
func (noopLogger) Warn(string, ...port.Field)  {}
func (noopLogger) Error(string, ...port.Field) {}

type fakeVerifier struct{}

func (fakeVerifier) Verify(_ context.Context, provider, idToken string) (domain.Identity, error) {
	if idToken == "" {
		return domain.Identity{}, apperr.Unauthorized("empty token")
	}

	return domain.Identity{
		Provider: provider,
		Subject:  idToken,
		Email:    idToken + "@example.com",
	}, nil
}

type stackLimits struct {
	eventsPerDay    int
	exchangePerHour int
	maxAgents       int
}

func defaultLimits() stackLimits {
	return stackLimits{eventsPerDay: 100, exchangePerHour: 100, maxAgents: 10}
}

type testStack struct {
	router http.Handler
	auth   *usecase.AuthService
	events *usecase.EventService
}

func newTestStack(t *testing.T, limits stackLimits) *testStack {
	t.Helper()

	eventsRepo := memory.NewEventRepository()
	owners := memory.NewOwnerRepository()
	tokens := memory.NewOwnerTokenRepository()
	agents := memory.NewAgentRepository()
	limiter := memory.NewRateLimiter(map[string]memory.Limit{
		usecase.ActionCreateEvent:  {Max: limits.eventsPerDay, Window: 24 * time.Hour},
		usecase.ActionAuthExchange: {Max: limits.exchangePerHour, Window: time.Hour},
	})
	authCfg := usecase.AuthConfig{OwnerTokenTTL: time.Hour, MaxAgentsPerOwner: limits.maxAgents}

	authSvc := usecase.NewAuthService(fakeVerifier{}, owners, tokens, agents, limiter, noopLogger{}, authCfg)
	eventSvc := usecase.NewEventService(eventsRepo, limiter, noopLogger{})

	eventHandler := handler.NewEventHandler(eventSvc, noopLogger{})
	authHandler := handler.NewAuthHandler(authSvc, noopLogger{}, nil)
	agentHandler := handler.NewAgentHandler(authSvc, noopLogger{})
	authMW := middleware.NewAuth(authSvc, noopLogger{})

	return &testStack{
		router: controller.NewRouter(eventHandler, authHandler, agentHandler, authMW, noopLogger{}),
		auth:   authSvc,
		events: eventSvc,
	}
}

func (s *testStack) do(t *testing.T, method, path, body, bearer string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}

	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)

	return rec
}

func (s *testStack) exchange(t *testing.T, subject string) string {
	t.Helper()

	body := fmt.Sprintf(`{"provider":"google","id_token":%q}`, subject)
	rec := s.do(t, http.MethodPost, "/api/v1/auth/exchange", body, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("exchange = %d, want %d (%s)", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload dto.ExchangeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode exchange response: %v", err)
	}

	return payload.Token
}

func (s *testStack) createAgent(t *testing.T, ownerToken, name string) (string, string) {
	t.Helper()

	body := fmt.Sprintf(`{"name":%q}`, name)
	rec := s.do(t, http.MethodPost, "/api/v1/agents", body, ownerToken)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create agent = %d, want %d (%s)", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var payload dto.CreateAgentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode create agent response: %v", err)
	}

	return payload.Agent.ID, payload.Key
}

func (s *testStack) agentKeyFor(t *testing.T, subject string) string {
	t.Helper()

	ownerToken := s.exchange(t, subject)
	_, key := s.createAgent(t, ownerToken, "agent-"+subject)

	return key
}

func (s *testStack) createEvent(t *testing.T, agentKey, name string) dto.EventResponse {
	t.Helper()

	rec := s.do(t, http.MethodPost, "/api/v1/events", `{"name":"`+name+`"}`, agentKey)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create event = %d, want %d (%s)", rec.Code, http.StatusCreated, rec.Body.String())
	}

	return decodeEvent(t, rec)
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

func TestCreateReturnsCreatedEventWithOwner(t *testing.T) {
	t.Parallel()

	stack := newTestStack(t, defaultLimits())
	key := stack.agentKeyFor(t, "alice")

	event := stack.createEvent(t, key, "deploy")
	if event.ID == "" || event.OwnerID == "" {
		t.Errorf("event = %+v, want id and owner_id", event)
	}

	if event.Name != "deploy" {
		t.Errorf("name = %q, want deploy", event.Name)
	}
}

func TestEventsRequireAgentKey(t *testing.T) {
	t.Parallel()

	stack := newTestStack(t, defaultLimits())

	rec := stack.do(t, http.MethodPost, "/api/v1/events", `{"name":"x"}`, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	rec = stack.do(t, http.MethodGet, "/api/v1/events", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	rec = stack.do(t, http.MethodPost, "/api/v1/events", `{"name":"x"}`, "not-a-bearer")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	rec = stack.do(t, http.MethodPost, "/api/v1/events", `{"name":"x"}`, "aea_garbage")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestOwnerTokenCannotActAsAgent(t *testing.T) {
	t.Parallel()

	stack := newTestStack(t, defaultLimits())
	ownerToken := stack.exchange(t, "alice")

	rec := stack.do(t, http.MethodPost, "/api/v1/events", `{"name":"x"}`, ownerToken)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAgentsRequireOwnerToken(t *testing.T) {
	t.Parallel()

	stack := newTestStack(t, defaultLimits())
	agentKey := stack.agentKeyFor(t, "alice")

	rec := stack.do(t, http.MethodPost, "/api/v1/agents", `{"name":"x"}`, agentKey)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("agent key on /agents = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	rec = stack.do(t, http.MethodGet, "/api/v1/agents", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestExchangeReturnsUniformResponseForNewAndExistingOwners(t *testing.T) {
	t.Parallel()

	stack := newTestStack(t, defaultLimits())

	first := stack.do(t, http.MethodPost, "/api/v1/auth/exchange", `{"provider":"google","id_token":"alice"}`, "")
	second := stack.do(t, http.MethodPost, "/api/v1/auth/exchange", `{"provider":"google","id_token":"alice"}`, "")

	if first.Code != http.StatusOK || second.Code != http.StatusOK {
		t.Fatalf("exchange codes = %d and %d, want both %d", first.Code, second.Code, http.StatusOK)
	}

	var a, b dto.ExchangeResponse
	_ = json.Unmarshal(first.Body.Bytes(), &a)
	_ = json.Unmarshal(second.Body.Bytes(), &b)

	if a.Owner.ID != b.Owner.ID {
		t.Errorf("owner ids differ: %q vs %q", a.Owner.ID, b.Owner.ID)
	}

	if a.Owner.Email == "" {
		t.Error("owner email is empty")
	}
}

func TestExchangeRejectsUnknownProvider(t *testing.T) {
	t.Parallel()

	stack := newTestStack(t, defaultLimits())

	rec := stack.do(t, http.MethodPost, "/api/v1/auth/exchange", `{"provider":"discord","id_token":"x"}`, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	rec = stack.do(t, http.MethodPost, "/api/v1/auth/exchange", `{"id_token":"x"}`, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestExchangeRateLimitedPerIP(t *testing.T) {
	t.Parallel()

	stack := newTestStack(t, stackLimits{eventsPerDay: 100, exchangePerHour: 1, maxAgents: 10})

	first := stack.do(t, http.MethodPost, "/api/v1/auth/exchange", `{"provider":"google","id_token":"alice"}`, "")
	if first.Code != http.StatusOK {
		t.Fatalf("first exchange = %d, want %d", first.Code, http.StatusOK)
	}

	second := stack.do(t, http.MethodPost, "/api/v1/auth/exchange", `{"provider":"google","id_token":"bob"}`, "")
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second exchange = %d, want %d", second.Code, http.StatusTooManyRequests)
	}
}

func TestCreateEventRateLimitedPerOwner(t *testing.T) {
	t.Parallel()

	stack := newTestStack(t, stackLimits{eventsPerDay: 1, exchangePerHour: 100, maxAgents: 10})
	key := stack.agentKeyFor(t, "alice")

	stack.createEvent(t, key, "one")

	rec := stack.do(t, http.MethodPost, "/api/v1/events", `{"name":"two"}`, key)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusTooManyRequests, rec.Body.String())
	}

	otherKey := stack.agentKeyFor(t, "bob")
	rec = stack.do(t, http.MethodPost, "/api/v1/events", `{"name":"three"}`, otherKey)
	if rec.Code != http.StatusCreated {
		t.Fatalf("other owner status = %d, want %d", rec.Code, http.StatusCreated)
	}
}

func TestAgentLimitPerOwner(t *testing.T) {
	t.Parallel()

	stack := newTestStack(t, stackLimits{eventsPerDay: 100, exchangePerHour: 100, maxAgents: 2})
	ownerToken := stack.exchange(t, "alice")

	stack.createAgent(t, ownerToken, "one")
	stack.createAgent(t, ownerToken, "two")

	rec := stack.do(t, http.MethodPost, "/api/v1/agents", `{"name":"three"}`, ownerToken)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestCrossOwnerEventAccessIsForbidden(t *testing.T) {
	t.Parallel()

	stack := newTestStack(t, defaultLimits())
	aliceKey := stack.agentKeyFor(t, "alice")
	bobKey := stack.agentKeyFor(t, "bob")

	event := stack.createEvent(t, aliceKey, "deploy")

	rec := stack.do(t, http.MethodPut, "/api/v1/events/"+event.ID, `{"name":"hijacked"}`, bobKey)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("update by foreign agent = %d, want %d", rec.Code, http.StatusForbidden)
	}

	rec = stack.do(t, http.MethodDelete, "/api/v1/events/"+event.ID, "", bobKey)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("delete by foreign agent = %d, want %d", rec.Code, http.StatusForbidden)
	}

	rec = stack.do(t, http.MethodPut, "/api/v1/events/"+event.ID, `{"name":"renamed"}`, aliceKey)
	if rec.Code != http.StatusOK {
		t.Fatalf("update by owner = %d, want %d (%s)", rec.Code, http.StatusOK, rec.Body.String())
	}

	rec = stack.do(t, http.MethodDelete, "/api/v1/events/"+event.ID, "", aliceKey)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete by owner = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestCrossOwnerAgentRevokeIsForbidden(t *testing.T) {
	t.Parallel()

	stack := newTestStack(t, defaultLimits())
	aliceToken := stack.exchange(t, "alice")
	bobToken := stack.exchange(t, "bob")

	agentID, agentKey := stack.createAgent(t, aliceToken, "scout")

	rec := stack.do(t, http.MethodDelete, "/api/v1/agents/"+agentID, "", bobToken)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("revoke by foreign owner = %d, want %d", rec.Code, http.StatusForbidden)
	}

	rec = stack.do(t, http.MethodGet, "/api/v1/agents", "", aliceToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("list agents = %d, want %d", rec.Code, http.StatusOK)
	}

	var agents []dto.AgentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &agents); err != nil {
		t.Fatalf("decode agents: %v", err)
	}

	if len(agents) != 1 || agents[0].ID != agentID {
		t.Errorf("agents = %+v, want the single owned agent", agents)
	}

	rec = stack.do(t, http.MethodDelete, "/api/v1/agents/"+agentID, "", aliceToken)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("revoke by owner = %d, want %d", rec.Code, http.StatusNoContent)
	}

	rec = stack.do(t, http.MethodGet, "/api/v1/events", "", agentKey)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("revoked agent key = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestListAgentsOnlyShowsOwn(t *testing.T) {
	t.Parallel()

	stack := newTestStack(t, defaultLimits())
	aliceToken := stack.exchange(t, "alice")
	bobToken := stack.exchange(t, "bob")

	stack.createAgent(t, aliceToken, "scout")
	stack.createAgent(t, bobToken, "worker")

	rec := stack.do(t, http.MethodGet, "/api/v1/agents", "", aliceToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("list agents = %d, want %d", rec.Code, http.StatusOK)
	}

	var agents []dto.AgentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &agents); err != nil {
		t.Fatalf("decode agents: %v", err)
	}

	if len(agents) != 1 || agents[0].Name != "scout" {
		t.Errorf("agents = %+v, want only alice's agent", agents)
	}
}

func TestWhoamiReturnsAgentAndOwner(t *testing.T) {
	t.Parallel()

	stack := newTestStack(t, defaultLimits())
	key := stack.agentKeyFor(t, "alice")

	rec := stack.do(t, http.MethodGet, "/api/v1/auth/whoami", "", key)
	if rec.Code != http.StatusOK {
		t.Fatalf("whoami = %d, want %d (%s)", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload dto.WhoamiResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode whoami: %v", err)
	}

	if payload.Owner.Email != "alice@example.com" {
		t.Errorf("owner email = %q, want alice@example.com", payload.Owner.Email)
	}

	if payload.Agent.Name == "" {
		t.Error("agent name is empty")
	}
}

func TestGetAndListEventsWithAgentKey(t *testing.T) {
	t.Parallel()

	stack := newTestStack(t, defaultLimits())
	aliceKey := stack.agentKeyFor(t, "alice")
	bobKey := stack.agentKeyFor(t, "bob")

	event := stack.createEvent(t, aliceKey, "deploy")
	stack.createEvent(t, bobKey, "meetup")

	rec := stack.do(t, http.MethodGet, "/api/v1/events/"+event.ID, "", aliceKey)
	if rec.Code != http.StatusOK {
		t.Fatalf("get event = %d, want %d", rec.Code, http.StatusOK)
	}

	if got := decodeEvent(t, rec).Name; got != "deploy" {
		t.Errorf("name = %q, want deploy", got)
	}

	rec = stack.do(t, http.MethodGet, "/api/v1/events", "", aliceKey)
	if rec.Code != http.StatusOK {
		t.Fatalf("list events = %d, want %d", rec.Code, http.StatusOK)
	}

	var events []dto.EventResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &events); err != nil {
		t.Fatalf("decode list: %v", err)
	}

	if len(events) != 2 {
		t.Errorf("list returned %d events, want 2 (events are discoverable across owners)", len(events))
	}

	rec = stack.do(t, http.MethodGet, "/api/v1/events/missing", "", aliceKey)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing event = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestMalformedIDsReturnNotFound(t *testing.T) {
	t.Parallel()

	stack := newTestStack(t, defaultLimits())
	key := stack.agentKeyFor(t, "alice")
	ownerToken := stack.exchange(t, "alice")

	rec := stack.do(t, http.MethodGet, "/api/v1/events/not-a-uuid", "", key)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get event with malformed id = %d, want %d", rec.Code, http.StatusNotFound)
	}

	rec = stack.do(t, http.MethodPut, "/api/v1/events/not-a-uuid", `{"name":"x"}`, key)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("update event with malformed id = %d, want %d", rec.Code, http.StatusNotFound)
	}

	rec = stack.do(t, http.MethodDelete, "/api/v1/events/not-a-uuid", "", key)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("delete event with malformed id = %d, want %d", rec.Code, http.StatusNotFound)
	}

	rec = stack.do(t, http.MethodDelete, "/api/v1/agents/not-a-uuid", "", ownerToken)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("revoke agent with malformed id = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestCreateRejectsMalformedJSON(t *testing.T) {
	t.Parallel()

	stack := newTestStack(t, defaultLimits())
	key := stack.agentKeyFor(t, "alice")

	rec := stack.do(t, http.MethodPost, "/api/v1/events", `{"name":`, key)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateRejectsMissingName(t *testing.T) {
	t.Parallel()

	stack := newTestStack(t, defaultLimits())
	key := stack.agentKeyFor(t, "alice")

	rec := stack.do(t, http.MethodPost, "/api/v1/events", `{"description":"no name"}`, key)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	payload := decodeError(t, rec)
	if payload.Error.Code != "invalid" {
		t.Errorf("error code = %q, want invalid", payload.Error.Code)
	}
}

func TestHealthzReportsOK(t *testing.T) {
	t.Parallel()

	stack := newTestStack(t, defaultLimits())

	rec := stack.do(t, http.MethodGet, "/healthz", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
