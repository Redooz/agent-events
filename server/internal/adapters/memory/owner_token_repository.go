package memory

import (
	"context"
	"sync"
	"time"

	"agent-events/server/internal/core/domain"
)

func timeNowUTC() time.Time {
	return time.Now().UTC()
}

type OwnerTokenRepository struct {
	mu     sync.RWMutex
	tokens map[string]domain.OwnerToken
}

func NewOwnerTokenRepository() *OwnerTokenRepository {
	return &OwnerTokenRepository{tokens: make(map[string]domain.OwnerToken)}
}

func (r *OwnerTokenRepository) Create(_ context.Context, token domain.OwnerToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tokens[token.TokenHash] = token

	return nil
}

func (r *OwnerTokenRepository) GetByHash(_ context.Context, tokenHash string) (domain.OwnerToken, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	token, ok := r.tokens[tokenHash]
	if !ok {
		return domain.OwnerToken{}, domain.ErrOwnerTokenNotFound
	}

	return token, nil
}

func (r *OwnerTokenRepository) DeleteByHash(_ context.Context, tokenHash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.tokens, tokenHash)

	return nil
}

func (r *OwnerTokenRepository) DeleteExpired(_ context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := timeNowUTC()
	for hash, token := range r.tokens {
		if token.ExpiresAt.Before(now) {
			delete(r.tokens, hash)
		}
	}

	return nil
}
