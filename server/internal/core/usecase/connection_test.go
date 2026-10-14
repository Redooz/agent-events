package usecase_test

import (
	"context"
	"testing"
	"time"

	"agent-events/server/internal/core/domain"
	"agent-events/server/internal/core/usecase"
	"agent-events/server/pkg/apperr"
)

type connectionStack struct {
	events      *fakeRepo
	attendees   *fakeAttendeeRepo
	requests    *fakeRequestRepo
	connections *fakeConnectionRepo
	svc         *usecase.ConnectionService
}

func newConnectionStack() *connectionStack {
	s := &connectionStack{
		events:      newFakeRepo(),
		attendees:   newFakeAttendeeRepo(),
		requests:    newFakeRequestRepo(),
		connections: newFakeConnectionRepo(),
	}
	s.svc = usecase.NewConnectionService(s.events, s.attendees, s.requests, s.connections, &fakeLimiter{allow: true}, noopLogger{})

	return s
}

func (s *connectionStack) onlineEvent(id string, attendeeIDs ...string) {
	seedEvent(s.events, id, domain.EventModeOnline, time.Now().UTC().Add(24*time.Hour))
	s.attendees.attendees[id] = map[string]bool{}
	for _, attendee := range attendeeIDs {
		s.attendees.attendees[id][attendee] = true
	}
}

func TestRequestConnectionCreatesPending(t *testing.T) {
	t.Parallel()

	s := newConnectionStack()
	s.onlineEvent("ev-1", "alice", "bob")

	result, err := s.svc.Request(context.Background(), "alice", "ev-1", "bob")
	if err != nil {
		t.Fatalf("Request() error = %v", err)
	}

	if result.Request.Status != domain.RequestStatusPending {
		t.Errorf("status = %q, want %q", result.Request.Status, domain.RequestStatusPending)
	}

	if result.Connection != nil {
		t.Error("Request() should not connect immediately")
	}
}

func TestRequestConnectionRejectsSelf(t *testing.T) {
	t.Parallel()

	s := newConnectionStack()
	s.onlineEvent("ev-1", "alice")

	_, err := s.svc.Request(context.Background(), "alice", "ev-1", "alice")
	if apperr.KindOf(err) != apperr.KindInvalid {
		t.Fatalf("kind = %v, want %v", apperr.KindOf(err), apperr.KindInvalid)
	}
}

func TestRequestConnectionRejectsOnsiteEvent(t *testing.T) {
	t.Parallel()

	s := newConnectionStack()
	seedEvent(s.events, "ev-1", domain.EventModeOnsite, time.Now().UTC())

	_, err := s.svc.Request(context.Background(), "alice", "ev-1", "bob")
	if apperr.KindOf(err) != apperr.KindInvalid {
		t.Fatalf("kind = %v, want %v", apperr.KindOf(err), apperr.KindInvalid)
	}
}

func TestRequestConnectionRequiresRequesterAttendance(t *testing.T) {
	t.Parallel()

	s := newConnectionStack()
	s.onlineEvent("ev-1", "bob")

	_, err := s.svc.Request(context.Background(), "alice", "ev-1", "bob")
	if apperr.KindOf(err) != apperr.KindForbidden {
		t.Fatalf("kind = %v, want %v", apperr.KindOf(err), apperr.KindForbidden)
	}
}

func TestRequestConnectionRejectsNonAttendeeTarget(t *testing.T) {
	t.Parallel()

	s := newConnectionStack()
	s.onlineEvent("ev-1", "alice")

	_, err := s.svc.Request(context.Background(), "alice", "ev-1", "bob")
	if apperr.KindOf(err) != apperr.KindInvalid {
		t.Fatalf("kind = %v, want %v", apperr.KindOf(err), apperr.KindInvalid)
	}
}

func TestDuplicateRequestRejected(t *testing.T) {
	t.Parallel()

	s := newConnectionStack()
	s.onlineEvent("ev-1", "alice", "bob")

	if _, err := s.svc.Request(context.Background(), "alice", "ev-1", "bob"); err != nil {
		t.Fatalf("first Request() error = %v", err)
	}

	_, err := s.svc.Request(context.Background(), "alice", "ev-1", "bob")
	if apperr.KindOf(err) != apperr.KindInvalid {
		t.Fatalf("second Request() kind = %v, want %v", apperr.KindOf(err), apperr.KindInvalid)
	}
}

func TestReciprocalRequestAutoAccepts(t *testing.T) {
	t.Parallel()

	s := newConnectionStack()
	s.onlineEvent("ev-1", "alice", "bob")
	ctx := context.Background()

	if _, err := s.svc.Request(ctx, "bob", "ev-1", "alice"); err != nil {
		t.Fatalf("first Request() error = %v", err)
	}

	result, err := s.svc.Request(ctx, "alice", "ev-1", "bob")
	if err != nil {
		t.Fatalf("second Request() error = %v", err)
	}

	if result.Request.Status != domain.RequestStatusAccepted {
		t.Errorf("request status = %q, want %q", result.Request.Status, domain.RequestStatusAccepted)
	}

	if result.Connection == nil {
		t.Fatal("reciprocal request did not produce a connection")
	}

	a, b := domain.ConnectionPair("alice", "bob")
	if result.Connection.UserAID != a || result.Connection.UserBID != b {
		t.Errorf("connection pair = %q/%q, want %q/%q", result.Connection.UserAID, result.Connection.UserBID, a, b)
	}

	if result.Connection.Method != domain.ConnectionMethodOnline {
		t.Errorf("method = %q, want %q", result.Connection.Method, domain.ConnectionMethodOnline)
	}
}

func TestReciprocalRequestAutoAcceptsAfterDeclinedSameDirection(t *testing.T) {
	t.Parallel()

	s := newConnectionStack()
	s.onlineEvent("ev-1", "alice", "bob")
	ctx := context.Background()

	first, err := s.svc.Request(ctx, "alice", "ev-1", "bob")
	if err != nil {
		t.Fatalf("first Request() error = %v", err)
	}

	if err := s.svc.Decline(ctx, "bob", first.Request.ID); err != nil {
		t.Fatalf("Decline() error = %v", err)
	}

	if _, err := s.svc.Request(ctx, "bob", "ev-1", "alice"); err != nil {
		t.Fatalf("reciprocal Request() error = %v", err)
	}

	result, err := s.svc.Request(ctx, "alice", "ev-1", "bob")
	if err != nil {
		t.Fatalf("Request() after declined same-direction error = %v", err)
	}

	if result.Connection == nil {
		t.Fatal("declined same-direction request shadowed the pending reciprocal request")
	}

	if result.Request.Status != domain.RequestStatusAccepted {
		t.Errorf("request status = %q, want %q", result.Request.Status, domain.RequestStatusAccepted)
	}
}

func TestAcceptConnectionCreatesConnection(t *testing.T) {
	t.Parallel()

	s := newConnectionStack()
	s.onlineEvent("ev-1", "alice", "bob")
	ctx := context.Background()

	result, err := s.svc.Request(ctx, "alice", "ev-1", "bob")
	if err != nil {
		t.Fatalf("Request() error = %v", err)
	}

	connection, err := s.svc.Accept(ctx, "bob", result.Request.ID)
	if err != nil {
		t.Fatalf("Accept() error = %v", err)
	}

	if connection.Method != domain.ConnectionMethodOnline {
		t.Errorf("method = %q, want %q", connection.Method, domain.ConnectionMethodOnline)
	}
}

func TestAcceptByNonRecipientIsForbidden(t *testing.T) {
	t.Parallel()

	s := newConnectionStack()
	s.onlineEvent("ev-1", "alice", "bob", "carol")
	ctx := context.Background()

	result, err := s.svc.Request(ctx, "alice", "ev-1", "bob")
	if err != nil {
		t.Fatalf("Request() error = %v", err)
	}

	_, err = s.svc.Accept(ctx, "carol", result.Request.ID)
	if apperr.KindOf(err) != apperr.KindForbidden {
		t.Fatalf("Accept() kind = %v, want %v", apperr.KindOf(err), apperr.KindForbidden)
	}
}

func TestDeclinePreventsDoubleResponse(t *testing.T) {
	t.Parallel()

	s := newConnectionStack()
	s.onlineEvent("ev-1", "alice", "bob")
	ctx := context.Background()

	result, err := s.svc.Request(ctx, "alice", "ev-1", "bob")
	if err != nil {
		t.Fatalf("Request() error = %v", err)
	}

	if err := s.svc.Decline(ctx, "bob", result.Request.ID); err != nil {
		t.Fatalf("Decline() error = %v", err)
	}

	if err := s.svc.Decline(ctx, "bob", result.Request.ID); apperr.KindOf(err) != apperr.KindInvalid {
		t.Fatalf("second Decline() kind = %v, want %v", apperr.KindOf(err), apperr.KindInvalid)
	}

	if _, err := s.svc.Accept(ctx, "bob", result.Request.ID); apperr.KindOf(err) != apperr.KindInvalid {
		t.Fatalf("Accept after decline kind = %v, want %v", apperr.KindOf(err), apperr.KindInvalid)
	}
}

func TestConnectedUsersRequestRejected(t *testing.T) {
	t.Parallel()

	s := newConnectionStack()
	s.onlineEvent("ev-1", "alice", "bob")
	ctx := context.Background()

	first, err := s.svc.Request(ctx, "alice", "ev-1", "bob")
	if err != nil {
		t.Fatalf("Request() error = %v", err)
	}

	if _, err := s.svc.Accept(ctx, "bob", first.Request.ID); err != nil {
		t.Fatalf("Accept() error = %v", err)
	}

	_, err = s.svc.Request(ctx, "alice", "ev-1", "bob")
	if apperr.KindOf(err) != apperr.KindInvalid {
		t.Fatalf("request after connect kind = %v, want %v", apperr.KindOf(err), apperr.KindInvalid)
	}
}
