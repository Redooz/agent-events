package usecase_test

import (
	"context"
	"errors"
	"testing"

	"agent-events/server/internal/core/domain"
	"agent-events/server/internal/core/port"
	"agent-events/server/internal/core/usecase"
	"agent-events/server/pkg/apperr"
)

var errStoreDown = errors.New("store is down")

type noopLogger struct{}

var _ port.Logger = noopLogger{}

func (noopLogger) Debug(string, ...port.Field) {}
func (noopLogger) Info(string, ...port.Field)  {}
func (noopLogger) Warn(string, ...port.Field)  {}
func (noopLogger) Error(string, ...port.Field) {}

type fakeRepo struct {
	events  map[string]domain.Event
	failErr error
}

var _ port.EventRepository = (*fakeRepo)(nil)

type fakeLimiter struct {
	allow bool
	err   error
	calls int
}

var _ port.RateLimiter = (*fakeLimiter)(nil)

func (f *fakeLimiter) Allow(_ context.Context, _, _ string) (bool, error) {
	f.calls++

	if f.err != nil {
		return false, f.err
	}

	return f.allow, nil
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{events: make(map[string]domain.Event)}
}

func (f *fakeRepo) Save(_ context.Context, event domain.Event) (domain.Event, error) {
	if f.failErr != nil {
		return domain.Event{}, f.failErr
	}

	f.events[event.ID] = event

	return event, nil
}

func (f *fakeRepo) Get(_ context.Context, id string) (domain.Event, error) {
	if f.failErr != nil {
		return domain.Event{}, f.failErr
	}

	event, ok := f.events[id]
	if !ok {
		return domain.Event{}, domain.ErrEventNotFound
	}

	return event, nil
}

func (f *fakeRepo) List(_ context.Context) ([]domain.Event, error) {
	if f.failErr != nil {
		return nil, f.failErr
	}

	events := make([]domain.Event, 0, len(f.events))
	for _, event := range f.events {
		events = append(events, event)
	}

	return events, nil
}

func (f *fakeRepo) Update(_ context.Context, event domain.Event) (domain.Event, error) {
	if f.failErr != nil {
		return domain.Event{}, f.failErr
	}

	if _, ok := f.events[event.ID]; !ok {
		return domain.Event{}, domain.ErrEventNotFound
	}

	f.events[event.ID] = event

	return event, nil
}

func (f *fakeRepo) Delete(_ context.Context, id string) error {
	if f.failErr != nil {
		return f.failErr
	}

	if _, ok := f.events[id]; !ok {
		return domain.ErrEventNotFound
	}

	delete(f.events, id)

	return nil
}

func newService(repo *fakeRepo) *usecase.EventService {
	return usecase.NewEventService(repo, &fakeLimiter{allow: true}, noopLogger{})
}

var actor = usecase.Actor{OwnerID: "owner-1", AgentID: "agent-1"}

func TestCreateAssignsIdentityAndTimestamps(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	svc := newService(repo)

	event, err := svc.Create(context.Background(), actor, usecase.CreateEventInput{Name: "deploy", Description: "shipped v2"})
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	if event.ID == "" {
		t.Error("Create() did not assign an ID")
	}

	if event.OwnerID != actor.OwnerID {
		t.Errorf("Create() owner_id = %q, want %q", event.OwnerID, actor.OwnerID)
	}

	if event.CreatedAt.IsZero() || event.UpdatedAt.IsZero() {
		t.Error("Create() did not set timestamps")
	}

	if _, ok := repo.events[event.ID]; !ok {
		t.Error("Create() did not persist the event")
	}
}

func TestCreateRejectsMissingActor(t *testing.T) {
	t.Parallel()

	svc := newService(newFakeRepo())

	_, err := svc.Create(context.Background(), usecase.Actor{}, usecase.CreateEventInput{Name: "deploy"})
	if apperr.KindOf(err) != apperr.KindInvalid {
		t.Fatalf("Create() kind = %v, want %v", apperr.KindOf(err), apperr.KindInvalid)
	}
}

func TestCreateRateLimited(t *testing.T) {
	t.Parallel()

	svc := usecase.NewEventService(newFakeRepo(), &fakeLimiter{allow: false}, noopLogger{})

	_, err := svc.Create(context.Background(), actor, usecase.CreateEventInput{Name: "deploy"})
	if apperr.KindOf(err) != apperr.KindTooMany {
		t.Fatalf("Create() kind = %v, want %v", apperr.KindOf(err), apperr.KindTooMany)
	}
}

func TestCreateReportsRateLimiterFailureAsInternal(t *testing.T) {
	t.Parallel()

	svc := usecase.NewEventService(newFakeRepo(), &fakeLimiter{err: errStoreDown}, noopLogger{})

	_, err := svc.Create(context.Background(), actor, usecase.CreateEventInput{Name: "deploy"})
	if apperr.KindOf(err) != apperr.KindInternal {
		t.Fatalf("Create() kind = %v, want %v", apperr.KindOf(err), apperr.KindInternal)
	}
}

func TestCreateRejectsBlankName(t *testing.T) {
	t.Parallel()

	svc := newService(newFakeRepo())

	_, err := svc.Create(context.Background(), actor, usecase.CreateEventInput{Name: "   "})
	if apperr.KindOf(err) != apperr.KindInvalid {
		t.Fatalf("Create() kind = %v, want %v", apperr.KindOf(err), apperr.KindInvalid)
	}
}

func TestCreateReportsStoreFailureAsInternal(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	repo.failErr = errStoreDown
	svc := newService(repo)

	_, err := svc.Create(context.Background(), actor, usecase.CreateEventInput{Name: "deploy"})
	if apperr.KindOf(err) != apperr.KindInternal {
		t.Fatalf("Create() kind = %v, want %v", apperr.KindOf(err), apperr.KindInternal)
	}

	if !errors.Is(err, errStoreDown) {
		t.Error("Create() should keep the underlying error for logging")
	}
}

func TestGetMissingEventIsNotFound(t *testing.T) {
	t.Parallel()

	svc := newService(newFakeRepo())

	_, err := svc.Get(context.Background(), "missing")
	if apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("Get() kind = %v, want %v", apperr.KindOf(err), apperr.KindNotFound)
	}
}

func TestGetRoundTripsStoredEvent(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	svc := newService(repo)

	created, err := svc.Create(context.Background(), actor, usecase.CreateEventInput{Name: "deploy"})
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	got, err := svc.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v, want nil", err)
	}

	if got.Name != created.Name {
		t.Errorf("Get() name = %q, want %q", got.Name, created.Name)
	}
}

func TestListReturnsStoredEvents(t *testing.T) {
	t.Parallel()

	svc := newService(newFakeRepo())

	for _, name := range []string{"one", "two"} {
		if _, err := svc.Create(context.Background(), actor, usecase.CreateEventInput{Name: name}); err != nil {
			t.Fatalf("Create(%q) error = %v, want nil", name, err)
		}
	}

	events, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}

	if len(events) != 2 {
		t.Errorf("List() returned %d events, want 2", len(events))
	}
}

func TestListReportsStoreFailureAsInternal(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	repo.failErr = errStoreDown
	svc := newService(repo)

	if _, err := svc.List(context.Background()); apperr.KindOf(err) != apperr.KindInternal {
		t.Fatalf("List() kind = %v, want %v", apperr.KindOf(err), apperr.KindInternal)
	}
}

func TestUpdateReplacesMutableFields(t *testing.T) {
	t.Parallel()

	svc := newService(newFakeRepo())

	created, err := svc.Create(context.Background(), actor, usecase.CreateEventInput{Name: "deploy"})
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	updated, err := svc.Update(context.Background(), actor, created.ID, usecase.UpdateEventInput{
		Name:        "rollback",
		Description: "reverted v2",
	})
	if err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}

	if updated.Name != "rollback" || updated.Description != "reverted v2" {
		t.Errorf("Update() = %+v, want renamed event", updated)
	}

	if !updated.UpdatedAt.After(created.UpdatedAt) {
		t.Error("Update() did not bump UpdatedAt")
	}
}

func TestUpdateMissingEventIsNotFound(t *testing.T) {
	t.Parallel()

	svc := newService(newFakeRepo())

	if _, err := svc.Update(context.Background(), actor, "missing", usecase.UpdateEventInput{Name: "x"}); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("Update() kind = %v, want %v", apperr.KindOf(err), apperr.KindNotFound)
	}
}

func TestUpdateRejectsBlankName(t *testing.T) {
	t.Parallel()

	svc := newService(newFakeRepo())

	created, err := svc.Create(context.Background(), actor, usecase.CreateEventInput{Name: "deploy"})
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	_, err = svc.Update(context.Background(), actor, created.ID, usecase.UpdateEventInput{Name: ""})
	if apperr.KindOf(err) != apperr.KindInvalid {
		t.Fatalf("Update() kind = %v, want %v", apperr.KindOf(err), apperr.KindInvalid)
	}
}

func TestUpdateByNonOwnerIsForbidden(t *testing.T) {
	t.Parallel()

	svc := newService(newFakeRepo())

	created, err := svc.Create(context.Background(), actor, usecase.CreateEventInput{Name: "deploy"})
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	intruder := usecase.Actor{OwnerID: "owner-2", AgentID: "agent-2"}
	_, err = svc.Update(context.Background(), intruder, created.ID, usecase.UpdateEventInput{Name: "hijacked"})
	if apperr.KindOf(err) != apperr.KindForbidden {
		t.Fatalf("Update() kind = %v, want %v", apperr.KindOf(err), apperr.KindForbidden)
	}
}

func TestDeleteRemovesEvent(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	svc := newService(repo)

	created, err := svc.Create(context.Background(), actor, usecase.CreateEventInput{Name: "deploy"})
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	if err := svc.Delete(context.Background(), actor, created.ID); err != nil {
		t.Fatalf("Delete() error = %v, want nil", err)
	}

	if _, ok := repo.events[created.ID]; ok {
		t.Error("Delete() left the event stored")
	}
}

func TestDeleteByNonOwnerIsForbidden(t *testing.T) {
	t.Parallel()

	svc := newService(newFakeRepo())

	created, err := svc.Create(context.Background(), actor, usecase.CreateEventInput{Name: "deploy"})
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	intruder := usecase.Actor{OwnerID: "owner-2", AgentID: "agent-2"}
	if err := svc.Delete(context.Background(), intruder, created.ID); apperr.KindOf(err) != apperr.KindForbidden {
		t.Fatalf("Delete() kind = %v, want %v", apperr.KindOf(err), apperr.KindForbidden)
	}
}

func TestDeleteMissingEventIsNotFound(t *testing.T) {
	t.Parallel()

	svc := newService(newFakeRepo())

	if err := svc.Delete(context.Background(), actor, "missing"); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("Delete() kind = %v, want %v", apperr.KindOf(err), apperr.KindNotFound)
	}
}
