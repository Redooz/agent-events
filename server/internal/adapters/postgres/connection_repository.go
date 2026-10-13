package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"agent-events/server/internal/adapters/postgres/gen"
	"agent-events/server/internal/core/domain"
)

type ConnectionRepository struct {
	q *gen.Queries
}

func NewConnectionRepository(pool *pgxpool.Pool) *ConnectionRepository {
	return &ConnectionRepository{q: gen.New(pool)}
}

func (r *ConnectionRepository) Save(ctx context.Context, connection domain.Connection) error {
	err := r.q.InsertConnection(ctx, gen.InsertConnectionParams{
		ID:        connection.ID,
		EventID:   connection.EventID,
		UserAID:   connection.UserAID,
		UserBID:   connection.UserBID,
		Method:    connection.Method,
		CreatedAt: timestamptz(connection.CreatedAt),
	})

	return asUniqueViolation(err, domain.ErrAlreadyConnected)
}

func (r *ConnectionRepository) Between(ctx context.Context, userA, userB string) (domain.Connection, error) {
	a, b := domain.ConnectionPair(userA, userB)

	row, err := r.q.GetConnectionBetween(ctx, gen.GetConnectionBetweenParams{UserAID: a, UserBID: b})
	if err != nil {
		return domain.Connection{}, asNotFound(err, domain.ErrConnectionNotFound)
	}

	return domain.Connection{
		ID:        row.ID,
		EventID:   row.EventID,
		UserAID:   row.UserAID,
		UserBID:   row.UserBID,
		Method:    row.Method,
		CreatedAt: timeFromTimestamptz(row.CreatedAt),
	}, nil
}

func (r *ConnectionRepository) ListForUser(ctx context.Context, userID string) ([]domain.ConnectionView, error) {
	rows, err := r.q.ListConnectionsForUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	views := make([]domain.ConnectionView, 0, len(rows))
	for _, row := range rows {
		views = append(views, domain.ConnectionView{
			Connection: domain.Connection{
				ID:        row.ID,
				EventID:   row.EventID,
				UserAID:   row.UserAID,
				UserBID:   row.UserBID,
				Method:    row.Method,
				CreatedAt: timeFromTimestamptz(row.CreatedAt),
			},
			EventName:      row.EventName,
			OtherUserID:    row.OtherUserID,
			OtherUserEmail: row.OtherUserEmail,
		})
	}

	return views, nil
}
