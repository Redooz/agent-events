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

func WithOwner(ctx context.Context, owner domain.Owner) context.Context {
	return context.WithValue(ctx, ownerKey{}, owner)
}

func OwnerFrom(ctx context.Context) (domain.Owner, bool) {
	owner, ok := ctx.Value(ownerKey{}).(domain.Owner)

	return owner, ok
}
