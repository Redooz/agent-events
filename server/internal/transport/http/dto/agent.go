package dto

import (
	"time"

	"agent-events/server/internal/core/domain"
)

type CreateAgentRequest struct {
	Name string `json:"name" validate:"required,max=128"`
}

type AgentResponse struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

type CreateAgentResponse struct {
	Agent AgentResponse `json:"agent"`
	Key   string        `json:"key"`
}

type WhoamiResponse struct {
	Agent AgentResponse `json:"agent"`
	Owner OwnerResponse `json:"owner"`
}

func NewAgentResponse(agent domain.Agent) AgentResponse {
	return AgentResponse{
		ID:         agent.ID,
		Name:       agent.Name,
		CreatedAt:  agent.CreatedAt,
		RevokedAt:  timePtr(agent.RevokedAt),
		LastUsedAt: timePtr(agent.LastUsedAt),
	}
}

func NewAgentResponses(agents []domain.Agent) []AgentResponse {
	out := make([]AgentResponse, 0, len(agents))
	for _, agent := range agents {
		out = append(out, NewAgentResponse(agent))
	}

	return out
}

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}

	copied := t

	return &copied
}
