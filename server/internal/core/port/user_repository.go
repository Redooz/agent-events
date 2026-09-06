package port

import (
	"context"

	"agent-events/server/internal/core/domain"
)

type UserRepository interface {
	UpsertByIdentity(ctx context.Context, provider, subject, email string) (domain.User, error)
	Get(ctx context.Context, id string) (domain.User, error)
}
