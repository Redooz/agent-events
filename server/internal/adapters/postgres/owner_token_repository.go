package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"agent-events/server/internal/core/domain"
)

type OwnerTokenRepository struct {
	db *sql.DB
}

func NewOwnerTokenRepository(db *sql.DB) *OwnerTokenRepository {
	return &OwnerTokenRepository{db: db}
}

func (r *OwnerTokenRepository) Create(ctx context.Context, token domain.OwnerToken) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM owner_tokens WHERE expires_at < $1`, time.Now().UTC()); err != nil {
		return err
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO owner_tokens (token_hash, owner_id, expires_at, created_at)
		VALUES ($1, $2, $3, $4)`,
		token.TokenHash, token.OwnerID, token.ExpiresAt, token.CreatedAt,
	)

	return err
}

func (r *OwnerTokenRepository) GetByHash(ctx context.Context, tokenHash string) (domain.OwnerToken, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT token_hash, owner_id, expires_at, created_at
		FROM owner_tokens
		WHERE token_hash = $1`,
		tokenHash,
	)

	var token domain.OwnerToken
	if err := row.Scan(&token.TokenHash, &token.OwnerID, &token.ExpiresAt, &token.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.OwnerToken{}, domain.ErrOwnerTokenNotFound
		}

		return domain.OwnerToken{}, err
	}

	return token, nil
}
