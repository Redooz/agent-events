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

	for hash, stored := range r.tokens {
		if stored.ExpiresAt.Before(timeNowUTC()) {
			delete(r.tokens, hash)
		}
	}

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
