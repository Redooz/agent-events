package memory

import (
	"context"
	"testing"
	"time"

	"agent-events/server/internal/core/domain"
)

func TestDeleteExpiredRemovesOnlyExpiredTokens(t *testing.T) {
	ctx := context.Background()
	repo := NewOwnerTokenRepository()

	expired := domain.OwnerToken{
		TokenHash: "expired-hash",
		OwnerID:   "owner-1",
		ExpiresAt: timeNowUTC().Add(-time.Hour),
		CreatedAt: timeNowUTC().Add(-2 * time.Hour),
	}
	live := domain.OwnerToken{
		TokenHash: "live-hash",
		OwnerID:   "owner-1",
		ExpiresAt: timeNowUTC().Add(time.Hour),
		CreatedAt: timeNowUTC(),
	}

	if err := repo.Create(ctx, expired); err != nil {
		t.Fatalf("Create() expired error = %v, want nil", err)
	}

	if err := repo.Create(ctx, live); err != nil {
		t.Fatalf("Create() live error = %v, want nil", err)
	}

	if err := repo.DeleteExpired(ctx); err != nil {
		t.Fatalf("DeleteExpired() error = %v, want nil", err)
	}

	if _, err := repo.GetByHash(ctx, expired.TokenHash); err != domain.ErrOwnerTokenNotFound {
		t.Errorf("GetByHash(expired) error = %v, want %v", err, domain.ErrOwnerTokenNotFound)
	}

	if _, err := repo.GetByHash(ctx, live.TokenHash); err != nil {
		t.Errorf("GetByHash(live) error = %v, want nil", err)
	}
}

func TestDeleteByHashRemovesSingleToken(t *testing.T) {
	ctx := context.Background()
	repo := NewOwnerTokenRepository()

	token := domain.OwnerToken{
		TokenHash: "hash-1",
		OwnerID:   "owner-1",
		ExpiresAt: timeNowUTC().Add(time.Hour),
		CreatedAt: timeNowUTC(),
	}

	if err := repo.Create(ctx, token); err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	if err := repo.DeleteByHash(ctx, token.TokenHash); err != nil {
		t.Fatalf("DeleteByHash() error = %v, want nil", err)
	}

	if _, err := repo.GetByHash(ctx, token.TokenHash); err != domain.ErrOwnerTokenNotFound {
		t.Errorf("GetByHash() error = %v, want %v", err, domain.ErrOwnerTokenNotFound)
	}
}
