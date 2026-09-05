package port

import (
	"context"

	"agent-events/server/internal/core/domain"
)

type OwnerTokenRepository interface {
	Create(ctx context.Context, token domain.OwnerToken) error
	GetByHash(ctx context.Context, tokenHash string) (domain.OwnerToken, error)
	DeleteByHash(ctx context.Context, tokenHash string) error
	DeleteExpired(ctx context.Context) error
}
