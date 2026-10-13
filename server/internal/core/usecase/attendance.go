package usecase

import (
	"context"
	"errors"
	"time"

	"agent-events/server/internal/core/domain"
	"agent-events/server/internal/core/port"
	"agent-events/server/pkg/apperr"
)

type AttendanceService struct {
	events    port.EventRepository
	attendees port.AttendeeRepository
	logger    port.Logger
}

func NewAttendanceService(events port.EventRepository, attendees port.AttendeeRepository, logger port.Logger) *AttendanceService {
	return &AttendanceService{events: events, attendees: attendees, logger: logger}
}

func (s *AttendanceService) Attend(ctx context.Context, actor Actor, eventID string) error {
	if actor.UserID == "" {
		return apperr.Invalid("user is required")
	}

	if _, err := s.events.Get(ctx, eventID); err != nil {
		if errors.Is(err, domain.ErrEventNotFound) {
			return apperr.NotFound("event not found")
		}

		s.logger.Error("failed to fetch event for attendance", port.Err(err), port.Str("event_id", eventID))
		return apperr.Wrap(err, "could not attend event")
	}

	now := time.Now().UTC()

	if err := s.attendees.Attend(ctx, eventID, actor.UserID, now); err != nil {
		s.logger.Error("failed to record attendance", port.Err(err), port.Str("event_id", eventID), port.Str("user_id", actor.UserID))
		return apperr.Wrap(err, "could not attend event")
	}

	s.logger.Info("user attended event",
		port.Str("event_id", eventID),
		port.Str("user_id", actor.UserID),
		port.Str("agent_id", actor.AgentID),
	)

	return nil
}

func (s *AttendanceService) List(ctx context.Context, userID, eventID string) ([]domain.Attendee, error) {
	if _, err := s.events.Get(ctx, eventID); err != nil {
		if errors.Is(err, domain.ErrEventNotFound) {
			return nil, apperr.NotFound("event not found")
		}

		s.logger.Error("failed to fetch event for attendee list", port.Err(err), port.Str("event_id", eventID))
		return nil, apperr.Wrap(err, "could not list attendees")
	}

	isAttendee, err := s.attendees.IsAttendee(ctx, eventID, userID)
	if err != nil {
		s.logger.Error("failed to check attendance", port.Err(err), port.Str("event_id", eventID), port.Str("user_id", userID))
		return nil, apperr.Wrap(err, "could not list attendees")
	}

	if !isAttendee {
		return nil, apperr.Forbidden("only attendees can view the roster")
	}

	attendees, err := s.attendees.ListByEvent(ctx, eventID)
	if err != nil {
		s.logger.Error("failed to list attendees", port.Err(err), port.Str("event_id", eventID))
		return nil, apperr.Wrap(err, "could not list attendees")
	}

	return attendees, nil
}
