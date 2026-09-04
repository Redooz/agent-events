package memory

import (
	"context"
	"sync"
	"time"

	"agent-events/server/internal/core/domain"
)

type AgentRepository struct {
	mu     sync.RWMutex
	agents map[string]domain.Agent
}

func NewAgentRepository() *AgentRepository {
	return &AgentRepository{agents: make(map[string]domain.Agent)}
}

func (r *AgentRepository) Create(_ context.Context, agent domain.Agent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.agents[agent.ID] = agent

	return nil
}

func (r *AgentRepository) Get(_ context.Context, id string) (domain.Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agent, ok := r.agents[id]
	if !ok {
		return domain.Agent{}, domain.ErrAgentNotFound
	}

	return agent, nil
}

func (r *AgentRepository) GetByKeyHash(_ context.Context, keyHash string) (domain.Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, agent := range r.agents {
		if agent.KeyHash == keyHash {
			return agent, nil
		}
	}

	return domain.Agent{}, domain.ErrAgentNotFound
}

func (r *AgentRepository) ListByOwner(_ context.Context, ownerID string) ([]domain.Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agents := make([]domain.Agent, 0)
	for _, agent := range r.agents {
		if agent.OwnerID == ownerID {
			agents = append(agents, agent)
		}
	}

	return agents, nil
}

func (r *AgentRepository) CountByOwner(_ context.Context, ownerID string) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, agent := range r.agents {
		if agent.OwnerID == ownerID {
			count++
		}
	}

	return count, nil
}

func (r *AgentRepository) Revoke(_ context.Context, id string, revokedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	agent, ok := r.agents[id]
	if !ok {
		return domain.ErrAgentNotFound
	}

	if agent.RevokedAt.IsZero() {
		agent.RevokedAt = revokedAt
		r.agents[id] = agent
	}

	return nil
}

func (r *AgentRepository) TouchLastUsed(_ context.Context, id string, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	agent, ok := r.agents[id]
	if !ok {
		return domain.ErrAgentNotFound
	}

	agent.LastUsedAt = at
	r.agents[id] = agent

	return nil
}
