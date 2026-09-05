package port

import (
	"context"
	"errors"

	"agent-events/server/internal/core/domain"
)

var ErrProviderUnavailable = errors.New("identity provider is not configured")

type IdentityVerifier interface {
	Verify(ctx context.Context, provider, idToken string) (domain.Identity, error)
}
