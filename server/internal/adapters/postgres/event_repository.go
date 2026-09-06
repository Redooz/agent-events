package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"agent-events/server/internal/adapters/postgres/gen"
	"agent-events/server/internal/core/domain"
)

type EventRepository struct {
	q *gen.Queries
}

func NewEventRepository(pool *pgxpool.Pool) *EventRepository {
	return &EventRepository{q: gen.New(pool)}
}

func (r *EventRepository) Save(ctx context.Context, event domain.Event) (domain.Event, error) {
	err := r.q.InsertEvent(ctx, gen.InsertEventParams{
		ID:          event.ID,
		UserID:      event.UserID,
		Name:        event.Name,
		Description: event.Description,
		CreatedAt:   timestamptz(event.CreatedAt),
		UpdatedAt:   timestamptz(event.UpdatedAt),
	})
	if err != nil {
		return domain.Event{}, err
	}

	return event, nil
}

func (r *EventRepository) Get(ctx context.Context, id string) (domain.Event, error) {
	row, err := r.q.GetEventByID(ctx, id)
	if err != nil {
		return domain.Event{}, asNotFound(err, domain.ErrEventNotFound)
	}

	return eventFromRow(row), nil
}

func (r *EventRepository) List(ctx context.Context) ([]domain.Event, error) {
	rows, err := r.q.ListEvents(ctx)
	if err != nil {
		return nil, err
	}

	events := make([]domain.Event, 0, len(rows))
	for _, row := range rows {
		events = append(events, eventFromRow(row))
	}

	return events, nil
}

func (r *EventRepository) Update(ctx context.Context, event domain.Event) (domain.Event, error) {
	affected, err := r.q.UpdateEvent(ctx, gen.UpdateEventParams{
		ID:          event.ID,
		Name:        event.Name,
		Description: event.Description,
		UpdatedAt:   timestamptz(event.UpdatedAt),
	})
	if err != nil {
		return domain.Event{}, err
	}

	if affected == 0 {
		return domain.Event{}, domain.ErrEventNotFound
	}

	return event, nil
}

func (r *EventRepository) Delete(ctx context.Context, id string) error {
	affected, err := r.q.DeleteEvent(ctx, id)
	if err != nil {
		return err
	}

	if affected == 0 {
		return domain.ErrEventNotFound
	}

	return nil
}

func eventFromRow(row gen.Event) domain.Event {
	return domain.Event{
		ID:          row.ID,
		UserID:      row.UserID,
		Name:        row.Name,
		Description: row.Description,
		CreatedAt:   timeFromTimestamptz(row.CreatedAt),
		UpdatedAt:   timeFromTimestamptz(row.UpdatedAt),
	}
}
