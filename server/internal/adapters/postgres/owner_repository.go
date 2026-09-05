package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"agent-events/server/internal/core/domain"
)

type OwnerRepository struct {
	db *sql.DB
}

func NewOwnerRepository(db *sql.DB) *OwnerRepository {
	return &OwnerRepository{db: db}
}

func (r *OwnerRepository) UpsertByIdentity(ctx context.Context, provider, subject, email string) (domain.Owner, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO owners (id, provider, subject, email, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (provider, subject) DO UPDATE SET email = EXCLUDED.email
		RETURNING id, provider, subject, email, created_at`,
		uuid.NewString(), provider, subject, email, time.Now().UTC(),
	)

	var owner domain.Owner
	if err := row.Scan(&owner.ID, &owner.Provider, &owner.Subject, &owner.Email, &owner.CreatedAt); err != nil {
		return domain.Owner{}, err
	}

	return owner, nil
}

func (r *OwnerRepository) Get(ctx context.Context, id string) (domain.Owner, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, provider, subject, email, created_at
		FROM owners
		WHERE id = $1`,
		id,
	)

	var owner domain.Owner
	if err := row.Scan(&owner.ID, &owner.Provider, &owner.Subject, &owner.Email, &owner.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Owner{}, domain.ErrOwnerNotFound
		}

		return domain.Owner{}, err
	}

	return owner, nil
}
