package port

import (
	"context"

	"agent-events/server/internal/core/domain"
)

type UserTokenRepository interface {
	Create(ctx context.Context, token domain.UserToken) error
	GetByHash(ctx context.Context, tokenHash string) (domain.UserToken, error)
	DeleteByHash(ctx context.Context, tokenHash string) error
	DeleteExpired(ctx context.Context) error
}
