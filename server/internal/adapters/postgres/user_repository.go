package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"agent-events/server/internal/adapters/postgres/gen"
	"agent-events/server/internal/core/domain"
)

type UserRepository struct {
	q *gen.Queries
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{q: gen.New(pool)}
}

func (r *UserRepository) UpsertByIdentity(ctx context.Context, provider, subject, email string) (domain.User, error) {
	row, err := r.q.UpsertUserIdentity(ctx, gen.UpsertUserIdentityParams{
		ID:        uuid.NewString(),
		Provider:  provider,
		Subject:   subject,
		Email:     email,
		CreatedAt: timestamptz(time.Now().UTC()),
	})
	if err != nil {
		return domain.User{}, err
	}

	return userFromRow(row), nil
}

func (r *UserRepository) Get(ctx context.Context, id string) (domain.User, error) {
	row, err := r.q.GetUserByID(ctx, id)
	if err != nil {
		return domain.User{}, asNotFound(err, domain.ErrUserNotFound)
	}

	return userFromRow(row), nil
}

func userFromRow(row gen.User) domain.User {
	return domain.User{
		ID:        row.ID,
		Provider:  row.Provider,
		Subject:   row.Subject,
		Email:     row.Email,
		CreatedAt: timeFromTimestamptz(row.CreatedAt),
	}
}
