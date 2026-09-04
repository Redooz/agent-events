package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"agent-events/server/internal/core/domain"
	"agent-events/server/internal/core/port"
	"agent-events/server/pkg/apperr"
)

type CreateEventInput struct {
	Name        string
	Description string
}

type UpdateEventInput struct {
	Name        string
	Description string
}

type EventService struct {
	repo   port.EventRepository
	logger port.Logger
}

func NewEventService(repo port.EventRepository, logger port.Logger) *EventService {
	return &EventService{repo: repo, logger: logger}
}

func (s *EventService) Create(ctx context.Context, in CreateEventInput) (domain.Event, error) {
	now := time.Now().UTC()
	event := domain.Event{
		ID:          newID(),
		Name:        in.Name,
		Description: in.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := event.Validate(); err != nil {
		s.logger.Warn("event rejected", port.Err(err))
		return domain.Event{}, apperr.Invalid(err.Error())
	}

	saved, err := s.repo.Save(ctx, event)
	if err != nil {
		s.logger.Error("failed to save event", port.Err(err), port.Str("event_id", event.ID))
		return domain.Event{}, apperr.Wrap(err, "could not create event")
	}

	s.logger.Info("event created", port.Str("event_id", saved.ID), port.Str("name", saved.Name))

	return saved, nil
}

func (s *EventService) Get(ctx context.Context, id string) (domain.Event, error) {
	event, err := s.repo.Get(ctx, id)
	switch {
	case errors.Is(err, domain.ErrEventNotFound):
		s.logger.Debug("event not found", port.Str("event_id", id))
		return domain.Event{}, apperr.NotFound("event not found")
	case err != nil:
		s.logger.Error("failed to fetch event", port.Err(err), port.Str("event_id", id))
		return domain.Event{}, apperr.Wrap(err, "could not fetch event")
	}

	return event, nil
}

func (s *EventService) List(ctx context.Context) ([]domain.Event, error) {
	events, err := s.repo.List(ctx)
	if err != nil {
		s.logger.Error("failed to list events", port.Err(err))
		return nil, apperr.Wrap(err, "could not list events")
	}

	s.logger.Debug("events listed", port.Int("count", len(events)))

	return events, nil
}

func (s *EventService) Update(ctx context.Context, id string, in UpdateEventInput) (domain.Event, error) {
	event, err := s.repo.Get(ctx, id)
	switch {
	case errors.Is(err, domain.ErrEventNotFound):
		s.logger.Debug("event to update not found", port.Str("event_id", id))
		return domain.Event{}, apperr.NotFound("event not found")
	case err != nil:
		s.logger.Error("failed to fetch event for update", port.Err(err), port.Str("event_id", id))
		return domain.Event{}, apperr.Wrap(err, "could not update event")
	}

	event.Name = in.Name
	event.Description = in.Description
	event.UpdatedAt = time.Now().UTC()

	if err := event.Validate(); err != nil {
		s.logger.Warn("event update rejected", port.Err(err), port.Str("event_id", id))
		return domain.Event{}, apperr.Invalid(err.Error())
	}

	updated, err := s.repo.Update(ctx, event)
	if err != nil {
		s.logger.Error("failed to update event", port.Err(err), port.Str("event_id", id))
		return domain.Event{}, apperr.Wrap(err, "could not update event")
	}

	s.logger.Info("event updated", port.Str("event_id", updated.ID))

	return updated, nil
}

func (s *EventService) Delete(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		if errors.Is(err, domain.ErrEventNotFound) {
			s.logger.Debug("event to delete not found", port.Str("event_id", id))
			return apperr.NotFound("event not found")
		}

		s.logger.Error("failed to delete event", port.Err(err), port.Str("event_id", id))
		return apperr.Wrap(err, "could not delete event")
	}

	s.logger.Info("event deleted", port.Str("event_id", id))

	return nil
}

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}

	return hex.EncodeToString(b[:])
}
