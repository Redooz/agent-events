package port

import (
	"context"

	"agent-events/server/internal/core/domain"
)

type ConnectionRepository interface {
	Save(ctx context.Context, connection domain.Connection) error
	Between(ctx context.Context, userA, userB string) (domain.Connection, error)
	ListForUser(ctx context.Context, userID string) ([]domain.ConnectionView, error)
}
