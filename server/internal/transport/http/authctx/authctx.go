package authctx

import (
	"context"

	"agent-events/server/internal/core/domain"
)

type agentKey struct{}

type ownerKey struct{}

type AgentIdentity struct {
	Agent domain.Agent
	Owner domain.Owner
}

func WithAgent(ctx context.Context, identity AgentIdentity) context.Context {
	return context.WithValue(ctx, agentKey{}, identity)
}

func AgentFrom(ctx context.Context) (AgentIdentity, bool) {
	identity, ok := ctx.Value(agentKey{}).(AgentIdentity)

	return identity, ok
}

type OwnerIdentity struct {
	Owner     domain.Owner
	TokenHash string
}

func WithOwner(ctx context.Context, identity OwnerIdentity) context.Context {
	return context.WithValue(ctx, ownerKey{}, identity)
}

func OwnerFrom(ctx context.Context) (OwnerIdentity, bool) {
	identity, ok := ctx.Value(ownerKey{}).(OwnerIdentity)

	return identity, ok
}
