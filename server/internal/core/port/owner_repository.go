package port

import (
	"context"

	"agent-events/server/internal/core/domain"
)

type OwnerRepository interface {
	UpsertByIdentity(ctx context.Context, provider, subject, email string) (domain.Owner, error)
	Get(ctx context.Context, id string) (domain.Owner, error)
}
