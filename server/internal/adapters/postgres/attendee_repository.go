package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"agent-events/server/internal/adapters/postgres/gen"
	"agent-events/server/internal/core/domain"
)

type AttendeeRepository struct {
	q *gen.Queries
}

func NewAttendeeRepository(pool *pgxpool.Pool) *AttendeeRepository {
	return &AttendeeRepository{q: gen.New(pool)}
}

func (r *AttendeeRepository) Attend(ctx context.Context, eventID, userID string, at time.Time) error {
	return r.q.UpsertAttendee(ctx, gen.UpsertAttendeeParams{
		EventID:   eventID,
		UserID:    userID,
		CreatedAt: timestamptz(at),
	})
}

func (r *AttendeeRepository) IsAttendee(ctx context.Context, eventID, userID string) (bool, error) {
	return r.q.IsEventAttendee(ctx, gen.IsEventAttendeeParams{
		EventID: eventID,
		UserID:  userID,
	})
}

func (r *AttendeeRepository) ListByEvent(ctx context.Context, eventID string) ([]domain.Attendee, error) {
	rows, err := r.q.ListEventAttendees(ctx, eventID)
	if err != nil {
		return nil, err
	}

	attendees := make([]domain.Attendee, 0, len(rows))
	for _, row := range rows {
		attendees = append(attendees, domain.Attendee{
			UserID:    row.UserID,
			Email:     row.Email,
			CreatedAt: timeFromTimestamptz(row.CreatedAt),
		})
	}

	return attendees, nil
}
