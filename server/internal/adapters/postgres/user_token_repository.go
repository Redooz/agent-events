package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"agent-events/server/internal/adapters/postgres/gen"
	"agent-events/server/internal/core/domain"
)

type UserTokenRepository struct {
	q *gen.Queries
}

func NewUserTokenRepository(pool *pgxpool.Pool) *UserTokenRepository {
	return &UserTokenRepository{q: gen.New(pool)}
}

func (r *UserTokenRepository) Create(ctx context.Context, token domain.UserToken) error {
	return r.q.InsertUserToken(ctx, gen.InsertUserTokenParams{
		TokenHash: token.TokenHash,
		UserID:    token.UserID,
		ExpiresAt: timestamptz(token.ExpiresAt),
		CreatedAt: timestamptz(token.CreatedAt),
	})
}

func (r *UserTokenRepository) GetByHash(ctx context.Context, tokenHash string) (domain.UserToken, error) {
	row, err := r.q.GetUserTokenByHash(ctx, tokenHash)
	if err != nil {
		return domain.UserToken{}, asNotFound(err, domain.ErrUserTokenNotFound)
	}

	return domain.UserToken{
		TokenHash: row.TokenHash,
		UserID:    row.UserID,
		ExpiresAt: timeFromTimestamptz(row.ExpiresAt),
		CreatedAt: timeFromTimestamptz(row.CreatedAt),
	}, nil
}

func (r *UserTokenRepository) DeleteByHash(ctx context.Context, tokenHash string) error {
	return r.q.DeleteUserTokenByHash(ctx, tokenHash)
}

func (r *UserTokenRepository) DeleteExpired(ctx context.Context) error {
	return r.q.DeleteExpiredUserTokens(ctx, timestamptz(time.Now().UTC()))
}
