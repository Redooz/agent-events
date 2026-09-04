package port

import (
	"context"
	"time"

	"agent-events/server/internal/core/domain"
)

type AgentRepository interface {
	Create(ctx context.Context, agent domain.Agent) error
	Get(ctx context.Context, id string) (domain.Agent, error)
	GetByKeyHash(ctx context.Context, keyHash string) (domain.Agent, error)
	ListByOwner(ctx context.Context, ownerID string) ([]domain.Agent, error)
	CountByOwner(ctx context.Context, ownerID string) (int, error)
	Revoke(ctx context.Context, id string, revokedAt time.Time) error
	TouchLastUsed(ctx context.Context, id string, at time.Time) error
}
