package oidc

import (
	"context"

	"agent-events/server/internal/core/domain"
)

type devVerifier struct{}

func (devVerifier) verify(_ context.Context, provider, idToken string) (domain.Identity, error) {
	return domain.Identity{Provider: provider, Subject: idToken}, nil
}
