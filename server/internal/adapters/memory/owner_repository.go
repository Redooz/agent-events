package memory

import (
	"context"
	"sync"

	"github.com/google/uuid"

	"agent-events/server/internal/core/domain"
)

type OwnerRepository struct {
	mu     sync.RWMutex
	owners map[string]domain.Owner
}

func NewOwnerRepository() *OwnerRepository {
	return &OwnerRepository{owners: make(map[string]domain.Owner)}
}

func (r *OwnerRepository) UpsertByIdentity(_ context.Context, provider, subject, email string) (domain.Owner, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, owner := range r.owners {
		if owner.Provider == provider && owner.Subject == subject {
			owner.Email = email
			r.owners[owner.ID] = owner

			return owner, nil
		}
	}

	owner := domain.Owner{
		ID:        uuid.NewString(),
		Provider:  provider,
		Subject:   subject,
		Email:     email,
		CreatedAt: timeNowUTC(),
	}
	r.owners[owner.ID] = owner

	return owner, nil
}

func (r *OwnerRepository) Get(_ context.Context, id string) (domain.Owner, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	owner, ok := r.owners[id]
	if !ok {
		return domain.Owner{}, domain.ErrOwnerNotFound
	}

	return owner, nil
}
