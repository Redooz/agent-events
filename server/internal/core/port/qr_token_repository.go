package port

import (
	"context"
	"time"

	"agent-events/server/internal/core/domain"
)

type QRTokenRepository interface {
	Save(ctx context.Context, token domain.QRToken) error
	GetByHash(ctx context.Context, tokenHash string) (domain.QRToken, error)
	MarkUsed(ctx context.Context, tokenHash string, at time.Time) (bool, error)
	DeleteExpired(ctx context.Context, before time.Time) error
}
