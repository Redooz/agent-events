package port

import (
	"context"

	"agent-events/server/internal/core/domain"
)

type EventRepository interface {
	Save(ctx context.Context, event domain.Event) (domain.Event, error)
	Get(ctx context.Context, id string) (domain.Event, error)
	List(ctx context.Context) ([]domain.Event, error)
	Update(ctx context.Context, event domain.Event) (domain.Event, error)
	Delete(ctx context.Context, id string) error
}
