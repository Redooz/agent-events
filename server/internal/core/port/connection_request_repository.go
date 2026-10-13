package port

import (
	"context"
	"time"

	"agent-events/server/internal/core/domain"
)

type ConnectionRequestRepository interface {
	Save(ctx context.Context, request domain.ConnectionRequest) error
	Get(ctx context.Context, id string) (domain.ConnectionRequest, error)
	FindPair(ctx context.Context, eventID, userA, userB string) ([]domain.ConnectionRequest, error)
	ListIncoming(ctx context.Context, userID string) ([]domain.ConnectionRequestView, error)
	MarkAccepted(ctx context.Context, id string, at time.Time) (bool, error)
	MarkDeclined(ctx context.Context, id string, at time.Time) (bool, error)
}
