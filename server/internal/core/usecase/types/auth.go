package types

import (
	"time"

	"agent-events/server/internal/core/domain"
)

type AuthConfig struct {
	UserTokenTTL     time.Duration
	MaxAgentsPerUser int
}

type ExchangeInput struct {
	Provider string
	IDToken  string
	ClientIP string
}

type ExchangeResult struct {
	User      domain.User
	Token     string
	ExpiresAt time.Time
}

type AgentWithKey struct {
	Agent domain.Agent
	Key   string
}

type AgentCredentials struct {
	Agent domain.Agent
	User  domain.User
}

type Actor struct {
	UserID  string
	AgentID string
}
