package port

import (
	"context"
	"time"

	"agent-events/server/internal/core/domain"
)

type AttendeeRepository interface {
	Attend(ctx context.Context, eventID, userID string, at time.Time) error
	IsAttendee(ctx context.Context, eventID, userID string) (bool, error)
	ListByEvent(ctx context.Context, eventID string) ([]domain.Attendee, error)
}
