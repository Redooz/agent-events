package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"agent-events/server/internal/core/domain"
	"agent-events/server/internal/core/port"
	"agent-events/server/internal/core/usecase/types"
	"agent-events/server/pkg/apperr"
)

const ActionCreateEvent = "event.create"

type EventService struct {
	repo    port.EventRepository
	limiter port.RateLimiter
	logger  port.Logger
}

func NewEventService(repo port.EventRepository, limiter port.RateLimiter, logger port.Logger) *EventService {
	return &EventService{repo: repo, limiter: limiter, logger: logger}
}

func (s *EventService) Create(ctx context.Context, actor types.Actor, in types.CreateEventInput) (domain.Event, error) {
	if actor.UserID == "" {
		return domain.Event{}, apperr.Invalid("user is required")
	}

	allowed, err := s.limiter.Allow(ctx, "user:"+actor.UserID, ActionCreateEvent)
	if err != nil {
		s.logger.Error("event rate limit check failed", port.Err(err))
		return domain.Event{}, apperr.Wrap(err, "could not check rate limit")
	}

	if !allowed {
		s.logger.Warn("event create rate limited", port.Str("user_id", actor.UserID), port.Str("agent_id", actor.AgentID))
		return domain.Event{}, apperr.TooMany("event creation limit reached, try again later")
	}

	now := time.Now().UTC()
	event := domain.Event{
		ID:          newID(),
		UserID:      actor.UserID,
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

	s.logger.Info("event created",
		port.Str("event_id", saved.ID),
		port.Str("user_id", actor.UserID),
		port.Str("agent_id", actor.AgentID),
	)

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

func (s *EventService) Update(ctx context.Context, actor types.Actor, id string, in types.UpdateEventInput) (domain.Event, error) {
	event, err := s.repo.Get(ctx, id)
	switch {
	case errors.Is(err, domain.ErrEventNotFound):
		s.logger.Debug("event to update not found", port.Str("event_id", id))
		return domain.Event{}, apperr.NotFound("event not found")
	case err != nil:
		s.logger.Error("failed to fetch event for update", port.Err(err), port.Str("event_id", id))
		return domain.Event{}, apperr.Wrap(err, "could not update event")
	}

	if event.UserID != actor.UserID {
		s.logger.Warn("event update denied for non-user",
			port.Str("event_id", id),
			port.Str("requesting_user_id", actor.UserID),
			port.Str("event_user_id", event.UserID),
		)
		return domain.Event{}, apperr.Forbidden("event does not belong to you")
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

	s.logger.Info("event updated", port.Str("event_id", updated.ID), port.Str("user_id", actor.UserID))

	return updated, nil
}

func (s *EventService) Delete(ctx context.Context, actor types.Actor, id string) error {
	event, err := s.repo.Get(ctx, id)
	switch {
	case errors.Is(err, domain.ErrEventNotFound):
		s.logger.Debug("event to delete not found", port.Str("event_id", id))
		return apperr.NotFound("event not found")
	case err != nil:
		s.logger.Error("failed to fetch event for delete", port.Err(err), port.Str("event_id", id))
		return apperr.Wrap(err, "could not delete event")
	}

	if event.UserID != actor.UserID {
		s.logger.Warn("event delete denied for non-user",
			port.Str("event_id", id),
			port.Str("requesting_user_id", actor.UserID),
			port.Str("event_user_id", event.UserID),
		)
		return apperr.Forbidden("event does not belong to you")
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		s.logger.Error("failed to delete event", port.Err(err), port.Str("event_id", id))
		return apperr.Wrap(err, "could not delete event")
	}

	s.logger.Info("event deleted", port.Str("event_id", id), port.Str("user_id", actor.UserID))

	return nil
}

func newID() string {
	return uuid.NewString()
}
