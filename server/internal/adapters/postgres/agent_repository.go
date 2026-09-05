package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"agent-events/server/internal/core/domain"
)

type AgentRepository struct {
	db *sql.DB
}

func NewAgentRepository(db *sql.DB) *AgentRepository {
	return &AgentRepository{db: db}
}

func (r *AgentRepository) CreateIfUnderLimit(ctx context.Context, agent domain.Agent, max int) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	var ownerID string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM owners WHERE id = $1 FOR UPDATE`, agent.OwnerID).Scan(&ownerID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, domain.ErrOwnerNotFound
		}

		return false, err
	}

	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM agents WHERE owner_id = $1 AND revoked_at IS NULL`, agent.OwnerID).Scan(&count); err != nil {
		return false, err
	}

	if count >= max {
		return false, nil
	}

	var revokedAt sql.NullTime
	if !agent.RevokedAt.IsZero() {
		revokedAt = sql.NullTime{Time: agent.RevokedAt, Valid: true}
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO agents (id, owner_id, name, key_hash, created_at, revoked_at, last_used_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		agent.ID, agent.OwnerID, agent.Name, agent.KeyHash, agent.CreatedAt, revokedAt, agent.LastUsedAt,
	); err != nil {
		return false, err
	}

	return true, tx.Commit()
}

func (r *AgentRepository) Get(ctx context.Context, id string) (domain.Agent, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, owner_id, name, key_hash, created_at, revoked_at, last_used_at
		FROM agents
		WHERE id = $1`,
		id,
	)

	return scanAgent(row)
}

func (r *AgentRepository) GetByKeyHash(ctx context.Context, keyHash string) (domain.Agent, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, owner_id, name, key_hash, created_at, revoked_at, last_used_at
		FROM agents
		WHERE key_hash = $1`,
		keyHash,
	)

	return scanAgent(row)
}

func (r *AgentRepository) ListByOwner(ctx context.Context, ownerID string) ([]domain.Agent, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, owner_id, name, key_hash, created_at, revoked_at, last_used_at
		FROM agents
		WHERE owner_id = $1
		ORDER BY created_at`,
		ownerID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	agents := make([]domain.Agent, 0)
	for rows.Next() {
		agent, err := scanAgent(rows)
		if err != nil {
			return nil, err
		}

		agents = append(agents, agent)
	}

	return agents, rows.Err()
}

func (r *AgentRepository) Revoke(ctx context.Context, id string, revokedAt time.Time) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE agents
		SET revoked_at = $2
		WHERE id = $1 AND revoked_at IS NULL`,
		id, revokedAt,
	)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if affected == 0 {
		if _, err := r.Get(ctx, id); err != nil {
			return domain.ErrAgentNotFound
		}
	}

	return nil
}

func (r *AgentRepository) TouchLastUsed(ctx context.Context, id string, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE agents SET last_used_at = $2 WHERE id = $1`, id, at)

	return err
}

func scanAgent(row interface{ Scan(dest ...any) error }) (domain.Agent, error) {
	var (
		agent      domain.Agent
		revokedAt  sql.NullTime
		lastUsedAt sql.NullTime
	)

	if err := row.Scan(&agent.ID, &agent.OwnerID, &agent.Name, &agent.KeyHash, &agent.CreatedAt, &revokedAt, &lastUsedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) || isInvalidInputSyntax(err) {
			return domain.Agent{}, domain.ErrAgentNotFound
		}

		return domain.Agent{}, err
	}

	if revokedAt.Valid {
		agent.RevokedAt = revokedAt.Time
	}

	if lastUsedAt.Valid {
		agent.LastUsedAt = lastUsedAt.Time
	}

	return agent, nil
}
