package usecase_test

import (
	"context"
	"testing"
	"time"

	"agent-events/server/internal/core/domain"
	"agent-events/server/internal/core/usecase"
	"agent-events/server/pkg/apperr"
)

func attendanceStack() (*fakeRepo, *fakeAttendeeRepo, *usecase.AttendanceService) {
	events := newFakeRepo()
	attendees := newFakeAttendeeRepo()
	svc := usecase.NewAttendanceService(events, attendees, noopLogger{})

	return events, attendees, svc
}

func seedEvent(events *fakeRepo, id, mode string, date time.Time) {
	events.events[id] = domain.Event{
		ID:     id,
		UserID: "owner",
		Name:   "event-" + id,
		Mode:   mode,
		Date:   date,
	}
}

func TestAttendRecordsIdempotentAttendance(t *testing.T) {
	t.Parallel()

	events, attendees, svc := attendanceStack()
	seedEvent(events, "ev-1", domain.EventModeOnsite, time.Now().UTC())

	actor := usecase.Actor{UserID: "alice", AgentID: "agent-1"}
	if err := svc.Attend(context.Background(), actor, "ev-1"); err != nil {
		t.Fatalf("Attend() error = %v", err)
	}

	if err := svc.Attend(context.Background(), actor, "ev-1"); err != nil {
		t.Fatalf("second Attend() error = %v", err)
	}

	if !attendees.attendees["ev-1"]["alice"] {
		t.Error("Attend() did not record attendance")
	}
}

func TestAttendMissingEventIsNotFound(t *testing.T) {
	t.Parallel()

	_, _, svc := attendanceStack()

	err := svc.Attend(context.Background(), usecase.Actor{UserID: "alice"}, "missing")
	if apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("Attend() kind = %v, want %v", apperr.KindOf(err), apperr.KindNotFound)
	}
}

func TestAttendWithoutUserIsInvalid(t *testing.T) {
	t.Parallel()

	events, _, svc := attendanceStack()
	seedEvent(events, "ev-1", domain.EventModeOnsite, time.Now().UTC())

	err := svc.Attend(context.Background(), usecase.Actor{}, "ev-1")
	if apperr.KindOf(err) != apperr.KindInvalid {
		t.Fatalf("Attend() kind = %v, want %v", apperr.KindOf(err), apperr.KindInvalid)
	}
}

func TestListAttendeesRequiresAttendance(t *testing.T) {
	t.Parallel()

	events, _, svc := attendanceStack()
	seedEvent(events, "ev-1", domain.EventModeOnsite, time.Now().UTC())

	_, err := svc.List(context.Background(), "stranger", "ev-1")
	if apperr.KindOf(err) != apperr.KindForbidden {
		t.Fatalf("List() kind = %v, want %v", apperr.KindOf(err), apperr.KindForbidden)
	}
}

func TestListAttendeesReturnsRosterForAttendee(t *testing.T) {
	t.Parallel()

	events, attendees, svc := attendanceStack()
	seedEvent(events, "ev-1", domain.EventModeOnsite, time.Now().UTC())
	attendees.attendees["ev-1"] = map[string]bool{"alice": true, "bob": true}

	list, err := svc.List(context.Background(), "alice", "ev-1")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(list) != 2 {
		t.Errorf("List() returned %d attendees, want 2", len(list))
	}
}
