package handler_test

import (
	"net/http"
	"testing"
)

func TestLogoutRevokesUserTokenButKeepsAgents(t *testing.T) {
	t.Parallel()

	stack := newTestStack(t, defaultLimits())
	userToken := stack.exchange(t, "alice")
	_, agentKey := stack.createAgent(t, userToken, "scout")

	rec := stack.do(t, http.MethodDelete, "/api/v1/auth/session", "", userToken)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout = %d, want %d (%s)", rec.Code, http.StatusNoContent, rec.Body.String())
	}

	rec = stack.do(t, http.MethodGet, "/api/v1/agents", "", userToken)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("user token after logout = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	rec = stack.do(t, http.MethodGet, "/api/v1/auth/whoami", "", agentKey)
	if rec.Code != http.StatusOK {
		t.Fatalf("agent key after user logout = %d, want %d (%s)", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestLogoutRequiresUserToken(t *testing.T) {
	t.Parallel()

	stack := newTestStack(t, defaultLimits())
	key := stack.agentKeyFor(t, "alice")

	rec := stack.do(t, http.MethodDelete, "/api/v1/auth/session", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("logout without token = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	rec = stack.do(t, http.MethodDelete, "/api/v1/auth/session", "", key)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("logout with agent key = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
