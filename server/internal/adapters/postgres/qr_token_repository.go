package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"agent-events/server/internal/adapters/postgres/gen"
	"agent-events/server/internal/core/domain"
)

type QRTokenRepository struct {
	q *gen.Queries
}

func NewQRTokenRepository(pool *pgxpool.Pool) *QRTokenRepository {
	return &QRTokenRepository{q: gen.New(pool)}
}

func (r *QRTokenRepository) Save(ctx context.Context, token domain.QRToken) error {
	return r.q.InsertQRToken(ctx, gen.InsertQRTokenParams{
		TokenHash: token.TokenHash,
		EventID:   token.EventID,
		UserID:    token.UserID,
		ExpiresAt: timestamptz(token.ExpiresAt),
		CreatedAt: timestamptz(token.CreatedAt),
	})
}

func (r *QRTokenRepository) GetByHash(ctx context.Context, tokenHash string) (domain.QRToken, error) {
	row, err := r.q.GetQRTokenByHash(ctx, tokenHash)
	if err != nil {
		return domain.QRToken{}, asNotFound(err, domain.ErrQRTokenNotFound)
	}

	return domain.QRToken{
		TokenHash: row.TokenHash,
		EventID:   row.EventID,
		UserID:    row.UserID,
		ExpiresAt: timeFromTimestamptz(row.ExpiresAt),
		UsedAt:    timeFromTimestamptz(row.UsedAt),
		CreatedAt: timeFromTimestamptz(row.CreatedAt),
	}, nil
}

func (r *QRTokenRepository) MarkUsed(ctx context.Context, tokenHash string, at time.Time) (bool, error) {
	affected, err := r.q.MarkQRTokenUsed(ctx, gen.MarkQRTokenUsedParams{
		TokenHash: tokenHash,
		UsedAt:    timestamptz(at),
	})
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

func (r *QRTokenRepository) DeleteExpired(ctx context.Context, before time.Time) error {
	return r.q.DeleteExpiredQRTokens(ctx, timestamptz(before))
}
