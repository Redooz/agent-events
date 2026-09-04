package memory

import (
	"context"
	"sync"

	"agent-events/server/internal/core/domain"
)

type EventRepository struct {
	mu     sync.RWMutex
	events map[string]domain.Event
}

func NewEventRepository() *EventRepository {
	return &EventRepository{events: make(map[string]domain.Event)}
}

func (r *EventRepository) Save(_ context.Context, event domain.Event) (domain.Event, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.events[event.ID] = event

	return event, nil
}

func (r *EventRepository) Get(_ context.Context, id string) (domain.Event, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	event, ok := r.events[id]
	if !ok {
		return domain.Event{}, domain.ErrEventNotFound
	}

	return event, nil
}

func (r *EventRepository) List(_ context.Context) ([]domain.Event, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	events := make([]domain.Event, 0, len(r.events))
	for _, event := range r.events {
		events = append(events, event)
	}

	return events, nil
}

func (r *EventRepository) Update(_ context.Context, event domain.Event) (domain.Event, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.events[event.ID]; !ok {
		return domain.Event{}, domain.ErrEventNotFound
	}

	r.events[event.ID] = event

	return event, nil
}

func (r *EventRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.events[id]; !ok {
		return domain.ErrEventNotFound
	}

	delete(r.events, id)

	return nil
}
