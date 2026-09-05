package usecase_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"agent-events/server/internal/core/domain"
	"agent-events/server/internal/core/port"
	"agent-events/server/internal/core/usecase"
	"agent-events/server/pkg/apperr"
)

type fakeVerifier struct {
	err error
}

var _ port.IdentityVerifier = (*fakeVerifier)(nil)

func (f *fakeVerifier) Verify(_ context.Context, provider, idToken string) (domain.Identity, error) {
	if f.err != nil {
		return domain.Identity{}, f.err
	}

	return domain.Identity{Provider: provider, Subject: idToken, Email: idToken + "@example.com"}, nil
}

type fakeOwnerStore struct {
	owners  map[string]domain.Owner
	failErr error
}

var _ port.OwnerRepository = (*fakeOwnerStore)(nil)

func newFakeOwnerStore() *fakeOwnerStore {
	return &fakeOwnerStore{owners: make(map[string]domain.Owner)}
}

func (f *fakeOwnerStore) UpsertByIdentity(_ context.Context, provider, subject, email string) (domain.Owner, error) {
	if f.failErr != nil {
		return domain.Owner{}, f.failErr
	}

	for _, owner := range f.owners {
		if owner.Provider == provider && owner.Subject == subject {
			owner.Email = email
			f.owners[owner.ID] = owner
			return owner, nil
		}
	}

	owner := domain.Owner{
		ID:        "owner-" + subject,
		Provider:  provider,
		Subject:   subject,
		Email:     email,
		CreatedAt: time.Now().UTC(),
	}
	f.owners[owner.ID] = owner

	return owner, nil
}

func (f *fakeOwnerStore) Get(_ context.Context, id string) (domain.Owner, error) {
	if f.failErr != nil {
		return domain.Owner{}, f.failErr
	}

	owner, ok := f.owners[id]
	if !ok {
		return domain.Owner{}, domain.ErrOwnerNotFound
	}

	return owner, nil
}

type fakeTokenStore struct {
	tokens  map[string]domain.OwnerToken
	failErr error
}

var _ port.OwnerTokenRepository = (*fakeTokenStore)(nil)

func newFakeTokenStore() *fakeTokenStore {
	return &fakeTokenStore{tokens: make(map[string]domain.OwnerToken)}
}

func (f *fakeTokenStore) Create(_ context.Context, token domain.OwnerToken) error {
	if f.failErr != nil {
		return f.failErr
	}

	f.tokens[token.TokenHash] = token

	return nil
}

func (f *fakeTokenStore) GetByHash(_ context.Context, tokenHash string) (domain.OwnerToken, error) {
	if f.failErr != nil {
		return domain.OwnerToken{}, f.failErr
	}

	token, ok := f.tokens[tokenHash]
	if !ok {
		return domain.OwnerToken{}, domain.ErrOwnerTokenNotFound
	}

	return token, nil
}

func (f *fakeTokenStore) DeleteByHash(_ context.Context, tokenHash string) error {
	if f.failErr != nil {
		return f.failErr
	}

	delete(f.tokens, tokenHash)

	return nil
}

func (f *fakeTokenStore) DeleteExpired(_ context.Context) error {
	if f.failErr != nil {
		return f.failErr
	}

	now := time.Now().UTC()
	for hash, token := range f.tokens {
		if token.ExpiresAt.Before(now) {
			delete(f.tokens, hash)
		}
	}

	return nil
}

type fakeAgentStore struct {
	agents     map[string]domain.Agent
	failErr    error
	touchCount int
}

var _ port.AgentRepository = (*fakeAgentStore)(nil)

func newFakeAgentStore() *fakeAgentStore {
	return &fakeAgentStore{agents: make(map[string]domain.Agent)}
}

func (f *fakeAgentStore) CreateIfUnderLimit(_ context.Context, agent domain.Agent, max int) (bool, error) {
	if f.failErr != nil {
		return false, f.failErr
	}

	count := 0
	for _, existing := range f.agents {
		if existing.OwnerID == agent.OwnerID && existing.RevokedAt.IsZero() {
			count++
		}
	}

	if count >= max {
		return false, nil
	}

	f.agents[agent.ID] = agent

	return true, nil
}

func (f *fakeAgentStore) Get(_ context.Context, id string) (domain.Agent, error) {
	if f.failErr != nil {
		return domain.Agent{}, f.failErr
	}

	agent, ok := f.agents[id]
	if !ok {
		return domain.Agent{}, domain.ErrAgentNotFound
	}

	return agent, nil
}

func (f *fakeAgentStore) GetByKeyHash(_ context.Context, keyHash string) (domain.Agent, error) {
	if f.failErr != nil {
		return domain.Agent{}, f.failErr
	}

	for _, agent := range f.agents {
		if agent.KeyHash == keyHash {
			return agent, nil
		}
	}

	return domain.Agent{}, domain.ErrAgentNotFound
}

func (f *fakeAgentStore) ListByOwner(_ context.Context, ownerID string) ([]domain.Agent, error) {
	if f.failErr != nil {
		return nil, f.failErr
	}

	agents := make([]domain.Agent, 0)
	for _, agent := range f.agents {
		if agent.OwnerID == ownerID {
			agents = append(agents, agent)
		}
	}

	return agents, nil
}

func (f *fakeAgentStore) Revoke(_ context.Context, id string, revokedAt time.Time) error {
	if f.failErr != nil {
		return f.failErr
	}

	agent, ok := f.agents[id]
	if !ok {
		return domain.ErrAgentNotFound
	}

	if agent.RevokedAt.IsZero() {
		agent.RevokedAt = revokedAt
		f.agents[id] = agent
	}

	return nil
}

func (f *fakeAgentStore) TouchLastUsed(_ context.Context, id string, at time.Time) error {
	f.touchCount++

	if f.failErr != nil {
		return f.failErr
	}

	agent, ok := f.agents[id]
	if !ok {
		return domain.ErrAgentNotFound
	}

	agent.LastUsedAt = at
	f.agents[id] = agent

	return nil
}

type fakeAuthLimiter struct {
	allow bool
	err   error
}

var _ port.RateLimiter = (*fakeAuthLimiter)(nil)

func (f *fakeAuthLimiter) Allow(_ context.Context, _, _ string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}

	return f.allow, nil
}

func newAuthService(
	owners *fakeOwnerStore,
	tokens *fakeTokenStore,
	agents *fakeAgentStore,
	verifier port.IdentityVerifier,
	limiter port.RateLimiter,
) *usecase.AuthService {
	if owners == nil {
		owners = newFakeOwnerStore()
	}
	if tokens == nil {
		tokens = newFakeTokenStore()
	}
	if agents == nil {
		agents = newFakeAgentStore()
	}
	if verifier == nil {
		verifier = &fakeVerifier{}
	}
	if limiter == nil {
		limiter = &fakeAuthLimiter{allow: true}
	}

	return usecase.NewAuthService(verifier, owners, tokens, agents, limiter, noopLogger{}, usecase.AuthConfig{
		OwnerTokenTTL:     24 * time.Hour,
		MaxAgentsPerOwner: 2,
	})
}

func TestExchangeIssuesTokenAndUpsertsOwner(t *testing.T) {
	t.Parallel()

	owners := newFakeOwnerStore()
	svc := newAuthService(owners, nil, nil, nil, nil)

	result, err := svc.Exchange(context.Background(), usecase.ExchangeInput{
		Provider: domain.ProviderGoogle,
		IDToken:  "google-sub-1",
		ClientIP: "192.0.2.1",
	})
	if err != nil {
		t.Fatalf("Exchange() error = %v, want nil", err)
	}

	if !strings.HasPrefix(result.Token, "aeo_") {
		t.Errorf("Exchange() token = %q, want aeo_ prefix", result.Token)
	}

	if result.ExpiresAt.Before(time.Now().UTC()) {
		t.Error("Exchange() token already expired")
	}

	if result.Owner.Provider != domain.ProviderGoogle || result.Owner.Subject != "google-sub-1" {
		t.Errorf("Exchange() owner = %+v, want google identity", result.Owner)
	}

	if len(owners.owners) != 1 {
		t.Errorf("Exchange() created %d owners, want 1", len(owners.owners))
	}

	again, err := svc.Exchange(context.Background(), usecase.ExchangeInput{
		Provider: domain.ProviderGoogle,
		IDToken:  "google-sub-1",
		ClientIP: "192.0.2.1",
	})
	if err != nil {
		t.Fatalf("Exchange() second call error = %v, want nil", err)
	}

	if again.Owner.ID != result.Owner.ID {
		t.Errorf("Exchange() second call created new owner %q, want same %q", again.Owner.ID, result.Owner.ID)
	}

	if again.Token == result.Token {
		t.Error("Exchange() reused the same token")
	}
}

func TestExchangeRejectsUnknownProvider(t *testing.T) {
	t.Parallel()

	svc := newAuthService(nil, nil, nil, nil, nil)

	_, err := svc.Exchange(context.Background(), usecase.ExchangeInput{
		Provider: "discord",
		IDToken:  "tok",
		ClientIP: "192.0.2.1",
	})
	if apperr.KindOf(err) != apperr.KindInvalid {
		t.Fatalf("Exchange() kind = %v, want %v", apperr.KindOf(err), apperr.KindInvalid)
	}
}

func TestExchangeRejectsBadTokenAsUnauthorized(t *testing.T) {
	t.Parallel()

	svc := newAuthService(nil, nil, nil, &fakeVerifier{err: errors.New("bad signature")}, nil)

	_, err := svc.Exchange(context.Background(), usecase.ExchangeInput{
		Provider: domain.ProviderGoogle,
		IDToken:  "forged",
		ClientIP: "192.0.2.1",
	})
	if apperr.KindOf(err) != apperr.KindUnauthorized {
		t.Fatalf("Exchange() kind = %v, want %v", apperr.KindOf(err), apperr.KindUnauthorized)
	}
}

func TestExchangeReportsRateLimit(t *testing.T) {
	t.Parallel()

	svc := newAuthService(nil, nil, nil, nil, &fakeAuthLimiter{allow: false})

	_, err := svc.Exchange(context.Background(), usecase.ExchangeInput{
		Provider: domain.ProviderGoogle,
		IDToken:  "tok",
		ClientIP: "192.0.2.1",
	})
	if apperr.KindOf(err) != apperr.KindTooMany {
		t.Fatalf("Exchange() kind = %v, want %v", apperr.KindOf(err), apperr.KindTooMany)
	}
}

func TestExchangeReportsProviderUnavailable(t *testing.T) {
	t.Parallel()

	svc := newAuthService(nil, nil, nil, &fakeVerifier{err: port.ErrProviderUnavailable}, nil)

	_, err := svc.Exchange(context.Background(), usecase.ExchangeInput{
		Provider: domain.ProviderApple,
		IDToken:  "tok",
		ClientIP: "192.0.2.1",
	})
	if apperr.KindOf(err) != apperr.KindInvalid {
		t.Fatalf("Exchange() kind = %v, want %v", apperr.KindOf(err), apperr.KindInvalid)
	}
}

func TestAuthenticateOwnerRoundTripsExchange(t *testing.T) {
	t.Parallel()

	svc := newAuthService(nil, nil, nil, nil, nil)

	result, err := svc.Exchange(context.Background(), usecase.ExchangeInput{
		Provider: domain.ProviderGoogle,
		IDToken:  "sub-1",
		ClientIP: "192.0.2.1",
	})
	if err != nil {
		t.Fatalf("Exchange() error = %v, want nil", err)
	}

	owner, err := svc.AuthenticateOwner(context.Background(), result.Token)
	if err != nil {
		t.Fatalf("AuthenticateOwner() error = %v, want nil", err)
	}

	if owner.ID != result.Owner.ID {
		t.Errorf("AuthenticateOwner() owner = %q, want %q", owner.ID, result.Owner.ID)
	}
}

func TestAuthenticateOwnerRejectsUnknownAndExpiredTokens(t *testing.T) {
	t.Parallel()

	svc := newAuthService(nil, nil, nil, nil, nil)

	if _, err := svc.AuthenticateOwner(context.Background(), "aeo_unknown"); apperr.KindOf(err) != apperr.KindUnauthorized {
		t.Fatalf("AuthenticateOwner() kind = %v, want %v", apperr.KindOf(err), apperr.KindUnauthorized)
	}

	owners := newFakeOwnerStore()
	tokens := newFakeTokenStore()
	svc = newAuthService(owners, tokens, nil, nil, nil)

	result, err := svc.Exchange(context.Background(), usecase.ExchangeInput{
		Provider: domain.ProviderGoogle,
		IDToken:  "sub-1",
		ClientIP: "192.0.2.1",
	})
	if err != nil {
		t.Fatalf("Exchange() error = %v, want nil", err)
	}

	for hash, token := range tokens.tokens {
		token.ExpiresAt = time.Now().UTC().Add(-time.Minute)
		tokens.tokens[hash] = token
	}

	if _, err := svc.AuthenticateOwner(context.Background(), result.Token); apperr.KindOf(err) != apperr.KindUnauthorized {
		t.Fatalf("AuthenticateOwner() kind = %v, want %v", apperr.KindOf(err), apperr.KindUnauthorized)
	}
}

func TestAgentKeyLifecycle(t *testing.T) {
	t.Parallel()

	svc := newAuthService(nil, nil, nil, nil, nil)

	result, err := svc.Exchange(context.Background(), usecase.ExchangeInput{
		Provider: domain.ProviderGoogle,
		IDToken:  "sub-1",
		ClientIP: "192.0.2.1",
	})
	if err != nil {
		t.Fatalf("Exchange() error = %v, want nil", err)
	}

	created, err := svc.CreateAgent(context.Background(), result.Owner.ID, "scout")
	if err != nil {
		t.Fatalf("CreateAgent() error = %v, want nil", err)
	}

	if !strings.HasPrefix(created.Key, "aea_") {
		t.Errorf("CreateAgent() key = %q, want aea_ prefix", created.Key)
	}

	if created.Agent.OwnerID != result.Owner.ID {
		t.Errorf("CreateAgent() agent owner = %q, want %q", created.Agent.OwnerID, result.Owner.ID)
	}

	creds, err := svc.AuthenticateAgent(context.Background(), created.Key)
	if err != nil {
		t.Fatalf("AuthenticateAgent() error = %v, want nil", err)
	}

	if creds.Agent.ID != created.Agent.ID || creds.Owner.ID != result.Owner.ID {
		t.Errorf("AuthenticateAgent() = %+v, want agent %q / owner %q", creds, created.Agent.ID, result.Owner.ID)
	}

	listed, err := svc.ListAgents(context.Background(), result.Owner.ID)
	if err != nil {
		t.Fatalf("ListAgents() error = %v, want nil", err)
	}

	if len(listed) != 1 || listed[0].ID != created.Agent.ID {
		t.Errorf("ListAgents() = %+v, want the created agent", listed)
	}

	if err := svc.RevokeAgent(context.Background(), result.Owner.ID, created.Agent.ID); err != nil {
		t.Fatalf("RevokeAgent() error = %v, want nil", err)
	}

	if err := svc.RevokeAgent(context.Background(), result.Owner.ID, created.Agent.ID); err != nil {
		t.Errorf("RevokeAgent() on revoked agent error = %v, want nil (idempotent)", err)
	}

	if _, err := svc.AuthenticateAgent(context.Background(), created.Key); apperr.KindOf(err) != apperr.KindUnauthorized {
		t.Fatalf("AuthenticateAgent() kind = %v, want %v", apperr.KindOf(err), apperr.KindUnauthorized)
	}
}

func TestCreateAgentEnforcesLimit(t *testing.T) {
	t.Parallel()

	svc := newAuthService(nil, nil, nil, nil, nil)

	result, err := svc.Exchange(context.Background(), usecase.ExchangeInput{
		Provider: domain.ProviderGoogle,
		IDToken:  "sub-1",
		ClientIP: "192.0.2.1",
	})
	if err != nil {
		t.Fatalf("Exchange() error = %v, want nil", err)
	}

	for i := 0; i < 2; i++ {
		if _, err := svc.CreateAgent(context.Background(), result.Owner.ID, "agent"); err != nil {
			t.Fatalf("CreateAgent() #%d error = %v, want nil", i+1, err)
		}
	}

	_, err = svc.CreateAgent(context.Background(), result.Owner.ID, "one too many")
	if apperr.KindOf(err) != apperr.KindForbidden {
		t.Fatalf("CreateAgent() kind = %v, want %v", apperr.KindOf(err), apperr.KindForbidden)
	}
}

func TestCreateAgentFreesSlotAfterRevoke(t *testing.T) {
	t.Parallel()

	svc := newAuthService(nil, nil, nil, nil, nil)

	result, err := svc.Exchange(context.Background(), usecase.ExchangeInput{
		Provider: domain.ProviderGoogle,
		IDToken:  "sub-1",
		ClientIP: "192.0.2.1",
	})
	if err != nil {
		t.Fatalf("Exchange() error = %v, want nil", err)
	}

	first, err := svc.CreateAgent(context.Background(), result.Owner.ID, "one")
	if err != nil {
		t.Fatalf("CreateAgent() #1 error = %v, want nil", err)
	}

	if _, err := svc.CreateAgent(context.Background(), result.Owner.ID, "two"); err != nil {
		t.Fatalf("CreateAgent() #2 error = %v, want nil", err)
	}

	if _, err := svc.CreateAgent(context.Background(), result.Owner.ID, "three"); apperr.KindOf(err) != apperr.KindForbidden {
		t.Fatalf("CreateAgent() #3 kind = %v, want %v", apperr.KindOf(err), apperr.KindForbidden)
	}

	if err := svc.RevokeAgent(context.Background(), result.Owner.ID, first.Agent.ID); err != nil {
		t.Fatalf("RevokeAgent() error = %v, want nil", err)
	}

	if _, err := svc.CreateAgent(context.Background(), result.Owner.ID, "again"); err != nil {
		t.Fatalf("CreateAgent() after revoke error = %v, want nil", err)
	}
}

func TestAuthenticateAgentThrottlesTouch(t *testing.T) {
	t.Parallel()

	agents := newFakeAgentStore()
	svc := newAuthService(nil, nil, agents, nil, nil)

	result, err := svc.Exchange(context.Background(), usecase.ExchangeInput{
		Provider: domain.ProviderGoogle,
		IDToken:  "sub-1",
		ClientIP: "192.0.2.1",
	})
	if err != nil {
		t.Fatalf("Exchange() error = %v, want nil", err)
	}

	created, err := svc.CreateAgent(context.Background(), result.Owner.ID, "scout")
	if err != nil {
		t.Fatalf("CreateAgent() error = %v, want nil", err)
	}

	if _, err := svc.AuthenticateAgent(context.Background(), created.Key); err != nil {
		t.Fatalf("AuthenticateAgent() #1 error = %v, want nil", err)
	}

	if _, err := svc.AuthenticateAgent(context.Background(), created.Key); err != nil {
		t.Fatalf("AuthenticateAgent() #2 error = %v, want nil", err)
	}

	if agents.touchCount != 1 {
		t.Errorf("TouchLastUsed calls = %d, want 1", agents.touchCount)
	}
}

func TestCreateAgentRejectsBlankName(t *testing.T) {
	t.Parallel()

	svc := newAuthService(nil, nil, nil, nil, nil)

	result, err := svc.Exchange(context.Background(), usecase.ExchangeInput{
		Provider: domain.ProviderGoogle,
		IDToken:  "sub-1",
		ClientIP: "192.0.2.1",
	})
	if err != nil {
		t.Fatalf("Exchange() error = %v, want nil", err)
	}

	_, err = svc.CreateAgent(context.Background(), result.Owner.ID, "   ")
	if apperr.KindOf(err) != apperr.KindInvalid {
		t.Fatalf("CreateAgent() kind = %v, want %v", apperr.KindOf(err), apperr.KindInvalid)
	}
}

func TestRevokeAgentDeniesForeignOwner(t *testing.T) {
	t.Parallel()

	owners := newFakeOwnerStore()
	svc := newAuthService(owners, nil, nil, nil, nil)

	ownerA, err := svc.Exchange(context.Background(), usecase.ExchangeInput{
		Provider: domain.ProviderGoogle,
		IDToken:  "sub-a",
		ClientIP: "192.0.2.1",
	})
	if err != nil {
		t.Fatalf("Exchange() error = %v, want nil", err)
	}

	ownerB, err := svc.Exchange(context.Background(), usecase.ExchangeInput{
		Provider: domain.ProviderGoogle,
		IDToken:  "sub-b",
		ClientIP: "192.0.2.1",
	})
	if err != nil {
		t.Fatalf("Exchange() error = %v, want nil", err)
	}

	agent, err := svc.CreateAgent(context.Background(), ownerA.Owner.ID, "scout")
	if err != nil {
		t.Fatalf("CreateAgent() error = %v, want nil", err)
	}

	if err := svc.RevokeAgent(context.Background(), ownerB.Owner.ID, agent.Agent.ID); apperr.KindOf(err) != apperr.KindForbidden {
		t.Fatalf("RevokeAgent() kind = %v, want %v", apperr.KindOf(err), apperr.KindForbidden)
	}

	if err := svc.RevokeAgent(context.Background(), ownerA.Owner.ID, agent.Agent.ID); err != nil {
		t.Errorf("RevokeAgent() by real owner error = %v, want nil", err)
	}
}

func TestRevokeAgentNotFound(t *testing.T) {
	t.Parallel()

	svc := newAuthService(nil, nil, nil, nil, nil)

	if err := svc.RevokeAgent(context.Background(), "owner-1", "missing"); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("RevokeAgent() kind = %v, want %v", apperr.KindOf(err), apperr.KindNotFound)
	}
}
