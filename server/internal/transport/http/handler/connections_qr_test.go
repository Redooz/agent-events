package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"agent-events/server/internal/transport/http/dto"
)

type userCreds struct {
	token string
	id    string
}

func (s *testStack) exchangeAs(t *testing.T, subject string) userCreds {
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

	return userCreds{token: payload.Token, id: payload.User.ID}
}

func (s *testStack) attend(t *testing.T, eventID, bearer string) {
	t.Helper()

	rec := s.do(t, http.MethodPost, "/api/v1/events/"+eventID+"/attend", "", bearer)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("attend = %d, want %d (%s)", rec.Code, http.StatusNoContent, rec.Body.String())
	}
}

func today() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func TestMutualQRScansConnectUsers(t *testing.T) {
	t.Parallel()

	stack := newTestStack(t, defaultLimits())
	aliceKey := stack.agentKeyFor(t, "alice")
	event := stack.createEventMode(t, aliceKey, "conf", "onsite", today())

	alice := stack.exchangeAs(t, "alice")
	bob := stack.exchangeAs(t, "bob")

	stack.attend(t, event.ID, alice.token)
	stack.attend(t, event.ID, bob.token)

	attendeesRec := stack.do(t, http.MethodGet, "/api/v1/events/"+event.ID+"/attendees", "", alice.token)
	if attendeesRec.Code != http.StatusOK {
		t.Fatalf("attendees = %d, want %d", attendeesRec.Code, http.StatusOK)
	}

	var roster []dto.AttendeeResponse
	if err := json.Unmarshal(attendeesRec.Body.Bytes(), &roster); err != nil {
		t.Fatalf("decode roster: %v", err)
	}

	if len(roster) != 2 {
		t.Fatalf("roster = %d, want 2", len(roster))
	}

	aliceQR := stack.do(t, http.MethodPost, "/api/v1/events/"+event.ID+"/qr", "", alice.token)
	if aliceQR.Code != http.StatusCreated {
		t.Fatalf("generate qr = %d, want %d (%s)", aliceQR.Code, http.StatusCreated, aliceQR.Body.String())
	}

	var code dto.GenerateQRResponse
	if err := json.Unmarshal(aliceQR.Body.Bytes(), &code); err != nil {
		t.Fatalf("decode qr: %v", err)
	}

	scanBody := `{"payload":` + fmt.Sprintf("%q", code.Payload) + `}`
	bobScan := stack.do(t, http.MethodPost, "/api/v1/events/"+event.ID+"/qr/scan", scanBody, bob.token)
	if bobScan.Code != http.StatusOK {
		t.Fatalf("bob scan = %d, want %d (%s)", bobScan.Code, http.StatusOK, bobScan.Body.String())
	}

	var first dto.ScanResponse
	if err := json.Unmarshal(bobScan.Body.Bytes(), &first); err != nil {
		t.Fatalf("decode scan: %v", err)
	}

	if first.Status != "pending" {
		t.Fatalf("first scan status = %q, want pending", first.Status)
	}

	bobQR := stack.do(t, http.MethodPost, "/api/v1/events/"+event.ID+"/qr", "", bob.token)
	var bobCode dto.GenerateQRResponse
	if err := json.Unmarshal(bobQR.Body.Bytes(), &bobCode); err != nil {
		t.Fatalf("decode qr: %v", err)
	}

	aliceScan := stack.do(t, http.MethodPost, "/api/v1/events/"+event.ID+"/qr/scan", `{"payload":"`+bobCode.Payload+`"}`, alice.token)
	if aliceScan.Code != http.StatusOK {
		t.Fatalf("alice scan = %d, want %d (%s)", aliceScan.Code, http.StatusOK, aliceScan.Body.String())
	}

	var second dto.ScanResponse
	if err := json.Unmarshal(aliceScan.Body.Bytes(), &second); err != nil {
		t.Fatalf("decode scan: %v", err)
	}

	if second.Status != "connected" || second.Connection == nil {
		t.Fatalf("second scan = %+v, want connected with connection", second)
	}

	connectionsRec := stack.do(t, http.MethodGet, "/api/v1/connections", "", alice.token)
	var connections []dto.ConnectionResponse
	if err := json.Unmarshal(connectionsRec.Body.Bytes(), &connections); err != nil {
		t.Fatalf("decode connections: %v", err)
	}

	if len(connections) != 1 || connections[0].UserID != bob.id {
		t.Errorf("alice connections = %+v, want one with bob (%s)", connections, bob.id)
	}
}

func TestQRRejectedForOnlineEvent(t *testing.T) {
	t.Parallel()

	stack := newTestStack(t, defaultLimits())
	aliceKey := stack.agentKeyFor(t, "alice")
	event := stack.createEvent(t, aliceKey, "virtual")

	alice := stack.exchangeAs(t, "alice")
	stack.attend(t, event.ID, alice.token)

	rec := stack.do(t, http.MethodPost, "/api/v1/events/"+event.ID+"/qr", "", alice.token)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("qr on online event = %d, want %d (%s)", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestQRRequiresAttendance(t *testing.T) {
	t.Parallel()

	stack := newTestStack(t, defaultLimits())
	aliceKey := stack.agentKeyFor(t, "alice")
	event := stack.createEventMode(t, aliceKey, "conf", "onsite", today())

	stranger := stack.exchangeAs(t, "mallory")

	rec := stack.do(t, http.MethodPost, "/api/v1/events/"+event.ID+"/qr", "", stranger.token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("qr for non-attendee = %d, want %d", rec.Code, http.StatusForbidden)
	}

	rec = stack.do(t, http.MethodGet, "/api/v1/events/"+event.ID+"/attendees", "", stranger.token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("attendees for non-attendee = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestAgentCanAttendViaAgentKey(t *testing.T) {
	t.Parallel()

	stack := newTestStack(t, defaultLimits())
	aliceKey := stack.agentKeyFor(t, "alice")
	event := stack.createEventMode(t, aliceKey, "conf", "onsite", today())

	stack.attend(t, event.ID, aliceKey)

	userToken := stack.exchange(t, "alice")
	rec := stack.do(t, http.MethodGet, "/api/v1/events/"+event.ID+"/attendees", "", userToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("attendees = %d, want %d (%s)", rec.Code, http.StatusOK, rec.Body.String())
	}

	var roster []dto.AttendeeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &roster); err != nil {
		t.Fatalf("decode roster: %v", err)
	}

	if len(roster) != 1 {
		t.Errorf("roster = %d, want 1", len(roster))
	}
}

func TestOnlineConnectionRequestFlow(t *testing.T) {
	t.Parallel()

	stack := newTestStack(t, defaultLimits())
	aliceKey := stack.agentKeyFor(t, "alice")
	event := stack.createEvent(t, aliceKey, "virtual")

	alice := stack.exchangeAs(t, "alice")
	bob := stack.exchangeAs(t, "bob")

	stack.attend(t, event.ID, alice.token)
	stack.attend(t, event.ID, bob.token)

	reqRec := stack.do(t, http.MethodPost, "/api/v1/connection-requests",
		`{"event_id":"`+event.ID+`","to_user_id":"`+bob.id+`"}`, alice.token)
	if reqRec.Code != http.StatusCreated {
		t.Fatalf("request = %d, want %d (%s)", reqRec.Code, http.StatusCreated, reqRec.Body.String())
	}

	var created dto.RequestConnectionResultResponse
	if err := json.Unmarshal(reqRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode request: %v", err)
	}

	if created.Status != "pending" {
		t.Fatalf("status = %q, want pending", created.Status)
	}

	listRec := stack.do(t, http.MethodGet, "/api/v1/connection-requests", "", bob.token)
	var incoming []dto.ConnectionRequestResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &incoming); err != nil {
		t.Fatalf("decode incoming: %v", err)
	}

	if len(incoming) != 1 || incoming[0].FromUserID != alice.id {
		t.Fatalf("bob incoming = %+v, want one from alice", incoming)
	}

	acceptRec := stack.do(t, http.MethodPost, "/api/v1/connection-requests/"+incoming[0].ID+"/accept", "", bob.token)
	if acceptRec.Code != http.StatusOK {
		t.Fatalf("accept = %d, want %d (%s)", acceptRec.Code, http.StatusOK, acceptRec.Body.String())
	}

	connectionsRec := stack.do(t, http.MethodGet, "/api/v1/connections", "", bob.token)
	var connections []dto.ConnectionResponse
	if err := json.Unmarshal(connectionsRec.Body.Bytes(), &connections); err != nil {
		t.Fatalf("decode connections: %v", err)
	}

	if len(connections) != 1 || connections[0].UserID != alice.id {
		t.Errorf("bob connections = %+v, want one with alice", connections)
	}

	reRec := stack.do(t, http.MethodPost, "/api/v1/connection-requests",
		`{"event_id":"`+event.ID+`","to_user_id":"`+alice.id+`"}`, bob.token)
	if reRec.Code != http.StatusBadRequest {
		t.Fatalf("request after connect = %d, want %d", reRec.Code, http.StatusBadRequest)
	}
}

func TestConnectionRequestsRequireUserToken(t *testing.T) {
	t.Parallel()

	stack := newTestStack(t, defaultLimits())
	agentKey := stack.agentKeyFor(t, "alice")

	rec := stack.do(t, http.MethodGet, "/api/v1/connection-requests", "", agentKey)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("agent on connection-requests = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	rec = stack.do(t, http.MethodGet, "/api/v1/connections", "", agentKey)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("agent on connections = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
