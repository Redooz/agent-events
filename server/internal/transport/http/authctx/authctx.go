package authctx

import (
	"context"

	"agent-events/server/internal/core/domain"
)

type agentKey struct{}

type userKey struct{}

type AgentIdentity struct {
	Agent domain.Agent
	User  domain.User
}

func WithAgent(ctx context.Context, identity AgentIdentity) context.Context {
	return context.WithValue(ctx, agentKey{}, identity)
}

func AgentFrom(ctx context.Context) (AgentIdentity, bool) {
	identity, ok := ctx.Value(agentKey{}).(AgentIdentity)

	return identity, ok
}

type UserIdentity struct {
	User      domain.User
	TokenHash string
}

func WithUser(ctx context.Context, identity UserIdentity) context.Context {
	return context.WithValue(ctx, userKey{}, identity)
}

func UserFrom(ctx context.Context) (UserIdentity, bool) {
	identity, ok := ctx.Value(userKey{}).(UserIdentity)

	return identity, ok
}
