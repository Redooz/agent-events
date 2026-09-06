package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"agent-events/server/internal/adapters/postgres/gen"
	"agent-events/server/internal/core/domain"
)

type AgentRepository struct {
	pool *pgxpool.Pool
	q    *gen.Queries
}

func NewAgentRepository(pool *pgxpool.Pool) *AgentRepository {
	return &AgentRepository{pool: pool, q: gen.New(pool)}
}

func (r *AgentRepository) CreateIfUnderLimit(ctx context.Context, agent domain.Agent, max int) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := r.q.WithTx(tx)

	if _, err := qtx.LockUserForUpdate(ctx, agent.UserID); err != nil {
		return false, asNotFound(err, domain.ErrUserNotFound)
	}

	count, err := qtx.CountActiveAgentsByUser(ctx, agent.UserID)
	if err != nil {
		return false, err
	}

	if int(count) >= max {
		return false, nil
	}

	if err := qtx.InsertAgent(ctx, insertAgentParams(agent)); err != nil {
		return false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return false, err
	}

	return true, nil
}

func (r *AgentRepository) Get(ctx context.Context, id string) (domain.Agent, error) {
	row, err := r.q.GetAgentByID(ctx, id)
	if err != nil {
		return domain.Agent{}, asNotFound(err, domain.ErrAgentNotFound)
	}

	return agentFromRow(row), nil
}

func (r *AgentRepository) GetByKeyHash(ctx context.Context, keyHash string) (domain.Agent, error) {
	row, err := r.q.GetAgentByKeyHash(ctx, keyHash)
	if err != nil {
		return domain.Agent{}, asNotFound(err, domain.ErrAgentNotFound)
	}

	return agentFromRow(row), nil
}

func (r *AgentRepository) ListByUser(ctx context.Context, userID string) ([]domain.Agent, error) {
	rows, err := r.q.ListAgentsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	agents := make([]domain.Agent, 0, len(rows))
	for _, row := range rows {
		agents = append(agents, agentFromRow(row))
	}

	return agents, nil
}

func (r *AgentRepository) Revoke(ctx context.Context, id string, revokedAt time.Time) error {
	affected, err := r.q.RevokeAgent(ctx, gen.RevokeAgentParams{ID: id, RevokedAt: timestamptz(revokedAt)})
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
	return r.q.TouchAgentLastUsed(ctx, gen.TouchAgentLastUsedParams{ID: id, LastUsedAt: timestamptz(at)})
}

func insertAgentParams(agent domain.Agent) gen.InsertAgentParams {
	return gen.InsertAgentParams{
		ID:         agent.ID,
		UserID:     agent.UserID,
		Name:       agent.Name,
		KeyHash:    agent.KeyHash,
		CreatedAt:  timestamptz(agent.CreatedAt),
		RevokedAt:  timestamptz(agent.RevokedAt),
		LastUsedAt: timestamptz(agent.LastUsedAt),
	}
}

func agentFromRow(row gen.Agent) domain.Agent {
	return domain.Agent{
		ID:         row.ID,
		UserID:     row.UserID,
		Name:       row.Name,
		KeyHash:    row.KeyHash,
		CreatedAt:  timeFromTimestamptz(row.CreatedAt),
		RevokedAt:  timeFromTimestamptz(row.RevokedAt),
		LastUsedAt: timeFromTimestamptz(row.LastUsedAt),
	}
}
