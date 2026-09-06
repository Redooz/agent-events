package handler_test

import (
	"context"
	"sync"
	"time"

	"agent-events/server/internal/core/domain"
	"agent-events/server/internal/core/port"
)

type fakeEventRepo struct {
	mu     sync.RWMutex
	events map[string]domain.Event
}

var _ port.EventRepository = (*fakeEventRepo)(nil)

func newFakeEventRepo() *fakeEventRepo {
	return &fakeEventRepo{events: make(map[string]domain.Event)}
}

func (r *fakeEventRepo) Save(_ context.Context, event domain.Event) (domain.Event, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.events[event.ID] = event

	return event, nil
}

func (r *fakeEventRepo) Get(_ context.Context, id string) (domain.Event, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	event, ok := r.events[id]
	if !ok {
		return domain.Event{}, domain.ErrEventNotFound
	}

	return event, nil
}

func (r *fakeEventRepo) List(_ context.Context) ([]domain.Event, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	events := make([]domain.Event, 0, len(r.events))
	for _, event := range r.events {
		events = append(events, event)
	}

	return events, nil
}

func (r *fakeEventRepo) Update(_ context.Context, event domain.Event) (domain.Event, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.events[event.ID]; !ok {
		return domain.Event{}, domain.ErrEventNotFound
	}

	r.events[event.ID] = event

	return event, nil
}

func (r *fakeEventRepo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.events[id]; !ok {
		return domain.ErrEventNotFound
	}

	delete(r.events, id)

	return nil
}

type fakeUserRepo struct {
	mu    sync.RWMutex
	users map[string]domain.User
}

var _ port.UserRepository = (*fakeUserRepo)(nil)

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{users: make(map[string]domain.User)}
}

func (r *fakeUserRepo) UpsertByIdentity(_ context.Context, provider, subject, email string) (domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, user := range r.users {
		if user.Provider == provider && user.Subject == subject {
			user.Email = email
			r.users[user.ID] = user

			return user, nil
		}
	}

	user := domain.User{
		ID:        "user-" + subject,
		Provider:  provider,
		Subject:   subject,
		Email:     email,
		CreatedAt: time.Now().UTC(),
	}
	r.users[user.ID] = user

	return user, nil
}

func (r *fakeUserRepo) Get(_ context.Context, id string) (domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, ok := r.users[id]
	if !ok {
		return domain.User{}, domain.ErrUserNotFound
	}

	return user, nil
}

type fakeUserTokenRepo struct {
	mu     sync.RWMutex
	tokens map[string]domain.UserToken
}

var _ port.UserTokenRepository = (*fakeUserTokenRepo)(nil)

func newFakeUserTokenRepo() *fakeUserTokenRepo {
	return &fakeUserTokenRepo{tokens: make(map[string]domain.UserToken)}
}

func (r *fakeUserTokenRepo) Create(_ context.Context, token domain.UserToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tokens[token.TokenHash] = token

	return nil
}

func (r *fakeUserTokenRepo) GetByHash(_ context.Context, tokenHash string) (domain.UserToken, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	token, ok := r.tokens[tokenHash]
	if !ok {
		return domain.UserToken{}, domain.ErrUserTokenNotFound
	}

	return token, nil
}

func (r *fakeUserTokenRepo) DeleteByHash(_ context.Context, tokenHash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.tokens, tokenHash)

	return nil
}

func (r *fakeUserTokenRepo) DeleteExpired(_ context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	for hash, token := range r.tokens {
		if token.ExpiresAt.Before(now) {
			delete(r.tokens, hash)
		}
	}

	return nil
}

type fakeAgentRepo struct {
	mu     sync.RWMutex
	agents map[string]domain.Agent
}

var _ port.AgentRepository = (*fakeAgentRepo)(nil)

func newFakeAgentRepo() *fakeAgentRepo {
	return &fakeAgentRepo{agents: make(map[string]domain.Agent)}
}

func (r *fakeAgentRepo) CreateIfUnderLimit(_ context.Context, agent domain.Agent, max int) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	count := 0
	for _, existing := range r.agents {
		if existing.UserID == agent.UserID && existing.RevokedAt.IsZero() {
			count++
		}
	}

	if count >= max {
		return false, nil
	}

	r.agents[agent.ID] = agent

	return true, nil
}

func (r *fakeAgentRepo) Get(_ context.Context, id string) (domain.Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agent, ok := r.agents[id]
	if !ok {
		return domain.Agent{}, domain.ErrAgentNotFound
	}

	return agent, nil
}

func (r *fakeAgentRepo) GetByKeyHash(_ context.Context, keyHash string) (domain.Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, agent := range r.agents {
		if agent.KeyHash == keyHash {
			return agent, nil
		}
	}

	return domain.Agent{}, domain.ErrAgentNotFound
}

func (r *fakeAgentRepo) ListByUser(_ context.Context, userID string) ([]domain.Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agents := make([]domain.Agent, 0)
	for _, agent := range r.agents {
		if agent.UserID == userID {
			agents = append(agents, agent)
		}
	}

	return agents, nil
}

func (r *fakeAgentRepo) Revoke(_ context.Context, id string, revokedAt time.Time) error {
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

func (r *fakeAgentRepo) TouchLastUsed(_ context.Context, id string, at time.Time) error {
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
