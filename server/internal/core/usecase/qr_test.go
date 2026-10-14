package usecase_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"agent-events/server/internal/core/domain"
	"agent-events/server/internal/core/usecase"
	"agent-events/server/pkg/apperr"
)

type qrStack struct {
	events      *fakeRepo
	attendees   *fakeAttendeeRepo
	tokens      *fakeQRTokenRepo
	scans       *fakeScanRepo
	connections *fakeConnectionRepo
	svc         *usecase.QRService
}

func newQRStack() *qrStack {
	s := &qrStack{
		events:      newFakeRepo(),
		attendees:   newFakeAttendeeRepo(),
		tokens:      newFakeQRTokenRepo(),
		scans:       newFakeScanRepo(),
		connections: newFakeConnectionRepo(),
	}
	connectionSvc := usecase.NewConnectionService(s.events, s.attendees, newFakeRequestRepo(), s.connections, &fakeLimiter{allow: true}, noopLogger{})
	s.svc = usecase.NewQRService(s.events, s.attendees, s.tokens, s.scans, connectionSvc, &fakeLimiter{allow: true}, noopLogger{}, usecase.QRConfig{TokenTTL: time.Minute})

	return s
}

func (s *qrStack) onsiteEventToday(id string, attendeeIDs ...string) {
	seedEvent(s.events, id, domain.EventModeOnsite, time.Now().UTC())
	s.attendees.attendees[id] = map[string]bool{}
	for _, attendee := range attendeeIDs {
		s.attendees.attendees[id][attendee] = true
	}
}

func TestGenerateQRRequiresOnsiteEvent(t *testing.T) {
	t.Parallel()

	s := newQRStack()
	seedEvent(s.events, "ev-1", domain.EventModeOnline, time.Now().UTC())
	s.attendees.attendees["ev-1"] = map[string]bool{"alice": true}

	_, err := s.svc.Generate(context.Background(), "alice", "ev-1")
	if apperr.KindOf(err) != apperr.KindInvalid {
		t.Fatalf("kind = %v, want %v", apperr.KindOf(err), apperr.KindInvalid)
	}
}

func TestGenerateQRRequiresEventDay(t *testing.T) {
	t.Parallel()

	s := newQRStack()
	seedEvent(s.events, "ev-1", domain.EventModeOnsite, time.Now().UTC().Add(24*time.Hour))
	s.attendees.attendees["ev-1"] = map[string]bool{"alice": true}

	_, err := s.svc.Generate(context.Background(), "alice", "ev-1")
	if apperr.KindOf(err) != apperr.KindInvalid {
		t.Fatalf("kind = %v, want %v", apperr.KindOf(err), apperr.KindInvalid)
	}

	if !strings.Contains(err.Error(), "event day") {
		t.Errorf("error = %v, want event day message", err)
	}
}

func TestGenerateQRRequiresAttendance(t *testing.T) {
	t.Parallel()

	s := newQRStack()
	s.onsiteEventToday("ev-1", "bob")

	_, err := s.svc.Generate(context.Background(), "alice", "ev-1")
	if apperr.KindOf(err) != apperr.KindForbidden {
		t.Fatalf("kind = %v, want %v", apperr.KindOf(err), apperr.KindForbidden)
	}
}

func TestGenerateStoresOnlyHash(t *testing.T) {
	t.Parallel()

	s := newQRStack()
	s.onsiteEventToday("ev-1", "alice")

	code, err := s.svc.Generate(context.Background(), "alice", "ev-1")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if !strings.HasPrefix(code.Payload, "aeq_") {
		t.Errorf("payload = %q, want aeq_ prefix", code.Payload)
	}

	if code.ExpiresAt.Before(time.Now().UTC()) {
		t.Error("expiry is in the past")
	}

	if len(s.tokens.tokens) != 1 {
		t.Fatalf("stored tokens = %d, want 1", len(s.tokens.tokens))
	}

	for hash := range s.tokens.tokens {
		if hash == code.Payload {
			t.Error("plaintext payload was stored, want hash")
		}
	}
}

func TestConsumeRequiresScannerAttendance(t *testing.T) {
	t.Parallel()

	s := newQRStack()
	s.onsiteEventToday("ev-1", "alice", "bob")
	ctx := context.Background()

	code, err := s.svc.Generate(ctx, "alice", "ev-1")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if _, err := s.svc.Consume(ctx, "mallory", "ev-1", code.Payload); apperr.KindOf(err) != apperr.KindForbidden {
		t.Fatalf("Consume() kind = %v, want %v", apperr.KindOf(err), apperr.KindForbidden)
	}

	result, err := s.svc.Consume(ctx, "bob", "ev-1", code.Payload)
	if err != nil {
		t.Fatalf("Consume() after rejected scan error = %v", err)
	}

	if result.Status != usecase.ScanStatusPending {
		t.Errorf("status = %q, want %q", result.Status, usecase.ScanStatusPending)
	}
}

func TestConsumeMutualScansCreateConnection(t *testing.T) {
	t.Parallel()

	s := newQRStack()
	s.onsiteEventToday("ev-1", "alice", "bob")
	ctx := context.Background()

	aliceCode, err := s.svc.Generate(ctx, "alice", "ev-1")
	if err != nil {
		t.Fatalf("Generate(alice) error = %v", err)
	}

	first, err := s.svc.Consume(ctx, "bob", "ev-1", aliceCode.Payload)
	if err != nil {
		t.Fatalf("Consume(alice's code) error = %v", err)
	}

	if first.Status != usecase.ScanStatusPending {
		t.Fatalf("first scan status = %q, want %q", first.Status, usecase.ScanStatusPending)
	}

	bobCode, err := s.svc.Generate(ctx, "bob", "ev-1")
	if err != nil {
		t.Fatalf("Generate(bob) error = %v", err)
	}

	second, err := s.svc.Consume(ctx, "alice", "ev-1", bobCode.Payload)
	if err != nil {
		t.Fatalf("Consume(bob's code) error = %v", err)
	}

	if second.Status != usecase.ScanStatusConnected {
		t.Fatalf("second scan status = %q, want %q", second.Status, usecase.ScanStatusConnected)
	}

	if second.Connection.Method != domain.ConnectionMethodQR {
		t.Errorf("method = %q, want %q", second.Connection.Method, domain.ConnectionMethodQR)
	}

	a, b := domain.ConnectionPair("alice", "bob")
	if second.Connection.UserAID != a || second.Connection.UserBID != b {
		t.Errorf("pair = %q/%q, want %q/%q", second.Connection.UserAID, second.Connection.UserBID, a, b)
	}
}

func seedQRToken(s *qrStack, raw, eventID, userID string, expiresAt, usedAt time.Time) {
	s.tokens.tokens[usecase.HashToken(raw)] = domain.QRToken{
		TokenHash: usecase.HashToken(raw),
		EventID:   eventID,
		UserID:    userID,
		ExpiresAt: expiresAt,
		UsedAt:    usedAt,
		CreatedAt: time.Now().UTC(),
	}
}

func TestConsumeRejectsUnknownExpiredAndUsedTokens(t *testing.T) {
	t.Parallel()

	s := newQRStack()
	s.onsiteEventToday("ev-1", "alice", "bob")
	ctx := context.Background()
	now := time.Now().UTC()

	if _, err := s.svc.Consume(ctx, "bob", "ev-1", "aeq_unknown"); apperr.KindOf(err) != apperr.KindInvalid {
		t.Fatalf("unknown token kind = %v, want %v", apperr.KindOf(err), apperr.KindInvalid)
	}

	seedQRToken(s, "aeq_expired", "ev-1", "alice", now.Add(-time.Minute), time.Time{})
	if _, err := s.svc.Consume(ctx, "bob", "ev-1", "aeq_expired"); apperr.KindOf(err) != apperr.KindInvalid {
		t.Fatalf("expired token kind = %v, want %v", apperr.KindOf(err), apperr.KindInvalid)
	}

	seedQRToken(s, "aeq_used", "ev-1", "alice", now.Add(time.Minute), now)
	if _, err := s.svc.Consume(ctx, "bob", "ev-1", "aeq_used"); apperr.KindOf(err) != apperr.KindInvalid {
		t.Fatalf("used token kind = %v, want %v", apperr.KindOf(err), apperr.KindInvalid)
	}
}

func TestConsumeRejectsForeignEventAndSelfScan(t *testing.T) {
	t.Parallel()

	s := newQRStack()
	s.onsiteEventToday("ev-1", "alice", "bob")
	s.onsiteEventToday("ev-2", "alice")
	ctx := context.Background()
	now := time.Now().UTC()

	seedQRToken(s, "aeq_other_event", "ev-2", "alice", now.Add(time.Minute), time.Time{})
	if _, err := s.svc.Consume(ctx, "bob", "ev-1", "aeq_other_event"); apperr.KindOf(err) != apperr.KindInvalid {
		t.Fatalf("foreign event kind = %v, want %v", apperr.KindOf(err), apperr.KindInvalid)
	}

	seedQRToken(s, "aeq_own", "ev-1", "bob", now.Add(time.Minute), time.Time{})
	if _, err := s.svc.Consume(ctx, "bob", "ev-1", "aeq_own"); apperr.KindOf(err) != apperr.KindInvalid {
		t.Fatalf("self scan kind = %v, want %v", apperr.KindOf(err), apperr.KindInvalid)
	}
}

func TestConsumeRejectsOffEventDay(t *testing.T) {
	t.Parallel()

	s := newQRStack()
	seedEvent(s.events, "ev-1", domain.EventModeOnsite, time.Now().UTC().Add(48*time.Hour))
	s.attendees.attendees["ev-1"] = map[string]bool{"alice": true, "bob": true}
	seedQRToken(s, "aeq_valid", "ev-1", "alice", time.Now().UTC().Add(time.Minute), time.Time{})

	_, err := s.svc.Consume(context.Background(), "bob", "ev-1", "aeq_valid")
	if apperr.KindOf(err) != apperr.KindInvalid {
		t.Fatalf("kind = %v, want %v", apperr.KindOf(err), apperr.KindInvalid)
	}
}

func TestConsumeRescanIsIdempotentAndStaysConnected(t *testing.T) {
	t.Parallel()

	s := newQRStack()
	s.onsiteEventToday("ev-1", "alice", "bob")
	ctx := context.Background()
	now := time.Now().UTC()

	seedQRToken(s, "aeq_a1", "ev-1", "alice", now.Add(time.Minute), time.Time{})
	seedQRToken(s, "aeq_a2", "ev-1", "alice", now.Add(time.Minute), time.Time{})
	seedQRToken(s, "aeq_b1", "ev-1", "bob", now.Add(time.Minute), time.Time{})

	if _, err := s.svc.Consume(ctx, "bob", "ev-1", "aeq_a1"); err != nil {
		t.Fatalf("first scan error = %v", err)
	}

	if _, err := s.svc.Consume(ctx, "alice", "ev-1", "aeq_b1"); err != nil {
		t.Fatalf("pairing scan error = %v", err)
	}

	again, err := s.svc.Consume(ctx, "bob", "ev-1", "aeq_a2")
	if err != nil {
		t.Fatalf("rescan error = %v", err)
	}

	if again.Status != usecase.ScanStatusConnected {
		t.Errorf("rescan status = %q, want %q", again.Status, usecase.ScanStatusConnected)
	}

	if len(s.connections.conns) != 1 {
		t.Errorf("connections stored = %d, want 1", len(s.connections.conns))
	}
}
