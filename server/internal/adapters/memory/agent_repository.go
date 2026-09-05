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

func (r *AgentRepository) CreateIfUnderLimit(_ context.Context, agent domain.Agent, max int) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	count := 0
	for _, existing := range r.agents {
		if existing.OwnerID == agent.OwnerID && existing.RevokedAt.IsZero() {
			count++
		}
	}

	if count >= max {
		return false, nil
	}

	r.agents[agent.ID] = agent

	return true, nil
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
