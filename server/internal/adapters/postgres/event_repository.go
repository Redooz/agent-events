package postgres

import (
	"context"
	"database/sql"
	"errors"

	"agent-events/server/internal/core/domain"
)

type EventRepository struct {
	db *sql.DB
}

func NewEventRepository(db *sql.DB) *EventRepository {
	return &EventRepository{db: db}
}

func (r *EventRepository) Save(ctx context.Context, event domain.Event) (domain.Event, error) {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO events (id, owner_id, name, description, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		event.ID, event.OwnerID, event.Name, event.Description, event.CreatedAt, event.UpdatedAt,
	)
	if err != nil {
		return domain.Event{}, err
	}

	return event, nil
}

func (r *EventRepository) Get(ctx context.Context, id string) (domain.Event, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, owner_id, name, description, created_at, updated_at
		FROM events
		WHERE id = $1`,
		id,
	)

	return scanEvent(row)
}

func (r *EventRepository) List(ctx context.Context) ([]domain.Event, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, owner_id, name, description, created_at, updated_at
		FROM events
		ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	events := make([]domain.Event, 0)
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}

		events = append(events, event)
	}

	return events, rows.Err()
}

func (r *EventRepository) Update(ctx context.Context, event domain.Event) (domain.Event, error) {
	result, err := r.db.ExecContext(ctx, `
		UPDATE events
		SET name = $2, description = $3, updated_at = $4
		WHERE id = $1`,
		event.ID, event.Name, event.Description, event.UpdatedAt,
	)
	if err != nil {
		return domain.Event{}, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return domain.Event{}, err
	}

	if affected == 0 {
		return domain.Event{}, domain.ErrEventNotFound
	}

	return event, nil
}

func (r *EventRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM events WHERE id = $1`, id)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if affected == 0 {
		return domain.ErrEventNotFound
	}

	return nil
}

func scanEvent(row interface{ Scan(dest ...any) error }) (domain.Event, error) {
	var event domain.Event

	if err := row.Scan(&event.ID, &event.OwnerID, &event.Name, &event.Description, &event.CreatedAt, &event.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) || isInvalidInputSyntax(err) {
			return domain.Event{}, domain.ErrEventNotFound
		}

		return domain.Event{}, err
	}

	return event, nil
}
