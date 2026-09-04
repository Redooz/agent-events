package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"agent-events/server/internal/core/domain"
	"agent-events/server/internal/core/port"
	"agent-events/server/pkg/apperr"
)

const (
	ActionAuthExchange = "auth.exchange"

	tokenPrefixOwner = "aeo_"
	tokenPrefixAgent = "aea_"
)

type AuthConfig struct {
	OwnerTokenTTL     time.Duration
	MaxAgentsPerOwner int
}

type ExchangeInput struct {
	Provider string
	IDToken  string
	ClientIP string
}

type ExchangeResult struct {
	Owner     domain.Owner
	Token     string
	ExpiresAt time.Time
}

type AgentWithKey struct {
	Agent domain.Agent
	Key   string
}

type AgentCredentials struct {
	Agent domain.Agent
	Owner domain.Owner
}

type AuthService struct {
	verifier port.IdentityVerifier
	owners   port.OwnerRepository
	tokens   port.OwnerTokenRepository
	agents   port.AgentRepository
	limiter  port.RateLimiter
	logger   port.Logger
	config   AuthConfig
}

func NewAuthService(
	verifier port.IdentityVerifier,
	owners port.OwnerRepository,
	tokens port.OwnerTokenRepository,
	agents port.AgentRepository,
	limiter port.RateLimiter,
	logger port.Logger,
	config AuthConfig,
) *AuthService {
	return &AuthService{
		verifier: verifier,
		owners:   owners,
		tokens:   tokens,
		agents:   agents,
		limiter:  limiter,
		logger:   logger,
		config:   config,
	}
}

func (s *AuthService) Exchange(ctx context.Context, in ExchangeInput) (ExchangeResult, error) {
	if !domain.ValidProvider(in.Provider) {
		return ExchangeResult{}, apperr.Invalid("unknown identity provider")
	}

	allowed, err := s.limiter.Allow(ctx, "ip:"+in.ClientIP, ActionAuthExchange)
	if err != nil {
		s.logger.Error("exchange rate limit check failed", port.Err(err))
		return ExchangeResult{}, apperr.Wrap(err, "could not check rate limit")
	}

	if !allowed {
		s.logger.Warn("exchange rate limited", port.Str("client_ip", in.ClientIP))
		return ExchangeResult{}, apperr.TooMany("too many sign-in attempts, try again later")
	}

	identity, err := s.verifier.Verify(ctx, in.Provider, in.IDToken)
	switch {
	case errors.Is(err, port.ErrProviderUnavailable):
		s.logger.Warn("exchange for unconfigured provider", port.Str("provider", in.Provider))
		return ExchangeResult{}, apperr.Invalid("identity provider is not configured")
	case err != nil:
		s.logger.Warn("identity token rejected", port.Str("provider", in.Provider), port.Err(err))
		return ExchangeResult{}, apperr.Unauthorized("invalid or expired identity token")
	}

	owner, err := s.owners.UpsertByIdentity(ctx, identity.Provider, identity.Subject, identity.Email)
	if err != nil {
		s.logger.Error("failed to upsert owner", port.Err(err), port.Str("provider", identity.Provider))
		return ExchangeResult{}, apperr.Wrap(err, "could not sign in")
	}

	rawToken, hash, err := newToken(tokenPrefixOwner)
	if err != nil {
		s.logger.Error("failed to generate owner token", port.Err(err))
		return ExchangeResult{}, apperr.Wrap(err, "could not sign in")
	}

	now := time.Now().UTC()
	expiresAt := now.Add(s.config.OwnerTokenTTL)

	if err := s.tokens.Create(ctx, domain.OwnerToken{
		TokenHash: hash,
		OwnerID:   owner.ID,
		ExpiresAt: expiresAt,
		CreatedAt: now,
	}); err != nil {
		s.logger.Error("failed to store owner token", port.Err(err), port.Str("owner_id", owner.ID))
		return ExchangeResult{}, apperr.Wrap(err, "could not sign in")
	}

	s.logger.Info("owner token issued",
		port.Str("owner_id", owner.ID),
		port.Str("provider", owner.Provider),
		port.Str("client_ip", in.ClientIP),
	)

	return ExchangeResult{Owner: owner, Token: rawToken, ExpiresAt: expiresAt}, nil
}

func (s *AuthService) AuthenticateOwner(ctx context.Context, rawToken string) (domain.Owner, error) {
	token, err := s.tokens.GetByHash(ctx, hashToken(rawToken))
	switch {
	case errors.Is(err, domain.ErrOwnerTokenNotFound):
		return domain.Owner{}, apperr.Unauthorized("invalid or expired owner token")
	case err != nil:
		s.logger.Error("failed to fetch owner token", port.Err(err))
		return domain.Owner{}, apperr.Wrap(err, "could not authenticate")
	}

	if token.Expired(time.Now().UTC()) {
		return domain.Owner{}, apperr.Unauthorized("invalid or expired owner token")
	}

	owner, err := s.owners.Get(ctx, token.OwnerID)
	switch {
	case errors.Is(err, domain.ErrOwnerNotFound):
		return domain.Owner{}, apperr.Unauthorized("invalid or expired owner token")
	case err != nil:
		s.logger.Error("failed to fetch owner for token", port.Err(err), port.Str("owner_id", token.OwnerID))
		return domain.Owner{}, apperr.Wrap(err, "could not authenticate")
	}

	return owner, nil
}

func (s *AuthService) AuthenticateAgent(ctx context.Context, rawKey string) (AgentCredentials, error) {
	agent, err := s.agents.GetByKeyHash(ctx, hashToken(rawKey))
	switch {
	case errors.Is(err, domain.ErrAgentNotFound):
		return AgentCredentials{}, apperr.Unauthorized("invalid agent key")
	case err != nil:
		s.logger.Error("failed to fetch agent by key", port.Err(err))
		return AgentCredentials{}, apperr.Wrap(err, "could not authenticate")
	}

	if agent.Revoked() {
		s.logger.Warn("revoked agent key used", port.Str("agent_id", agent.ID), port.Str("owner_id", agent.OwnerID))
		return AgentCredentials{}, apperr.Unauthorized("agent key has been revoked")
	}

	owner, err := s.owners.Get(ctx, agent.OwnerID)
	switch {
	case errors.Is(err, domain.ErrOwnerNotFound):
		s.logger.Error("agent references missing owner", port.Str("agent_id", agent.ID), port.Str("owner_id", agent.OwnerID))
		return AgentCredentials{}, apperr.Unauthorized("invalid agent key")
	case err != nil:
		s.logger.Error("failed to fetch agent owner", port.Err(err), port.Str("agent_id", agent.ID))
		return AgentCredentials{}, apperr.Wrap(err, "could not authenticate")
	}

	if err := s.agents.TouchLastUsed(ctx, agent.ID, time.Now().UTC()); err != nil {
		s.logger.Warn("failed to touch agent last used", port.Err(err), port.Str("agent_id", agent.ID))
	}

	return AgentCredentials{Agent: agent, Owner: owner}, nil
}

func (s *AuthService) CreateAgent(ctx context.Context, ownerID, name string) (AgentWithKey, error) {
	count, err := s.agents.CountByOwner(ctx, ownerID)
	if err != nil {
		s.logger.Error("failed to count agents", port.Err(err), port.Str("owner_id", ownerID))
		return AgentWithKey{}, apperr.Wrap(err, "could not create agent")
	}

	if count >= s.config.MaxAgentsPerOwner {
		s.logger.Warn("agent limit reached", port.Str("owner_id", ownerID), port.Int("count", count))
		return AgentWithKey{}, apperr.Forbidden("agent limit reached, revoke an agent first")
	}

	agent := domain.Agent{
		ID:        newID(),
		OwnerID:   ownerID,
		Name:      name,
		CreatedAt: time.Now().UTC(),
	}

	if err := agent.Validate(); err != nil {
		s.logger.Warn("agent rejected", port.Err(err), port.Str("owner_id", ownerID))
		return AgentWithKey{}, apperr.Invalid(err.Error())
	}

	rawKey, hash, err := newToken(tokenPrefixAgent)
	if err != nil {
		s.logger.Error("failed to generate agent key", port.Err(err))
		return AgentWithKey{}, apperr.Wrap(err, "could not create agent")
	}

	agent.KeyHash = hash

	if err := s.agents.Create(ctx, agent); err != nil {
		s.logger.Error("failed to store agent", port.Err(err), port.Str("owner_id", ownerID))
		return AgentWithKey{}, apperr.Wrap(err, "could not create agent")
	}

	s.logger.Info("agent created", port.Str("agent_id", agent.ID), port.Str("owner_id", ownerID))

	return AgentWithKey{Agent: agent, Key: rawKey}, nil
}

func (s *AuthService) ListAgents(ctx context.Context, ownerID string) ([]domain.Agent, error) {
	agents, err := s.agents.ListByOwner(ctx, ownerID)
	if err != nil {
		s.logger.Error("failed to list agents", port.Err(err), port.Str("owner_id", ownerID))
		return nil, apperr.Wrap(err, "could not list agents")
	}

	return agents, nil
}

func (s *AuthService) RevokeAgent(ctx context.Context, ownerID, agentID string) error {
	agent, err := s.agents.Get(ctx, agentID)
	switch {
	case errors.Is(err, domain.ErrAgentNotFound):
		return apperr.NotFound("agent not found")
	case err != nil:
		s.logger.Error("failed to fetch agent for revoke", port.Err(err), port.Str("agent_id", agentID))
		return apperr.Wrap(err, "could not revoke agent")
	}

	if agent.OwnerID != ownerID {
		s.logger.Warn("agent revoke denied for non-owner",
			port.Str("agent_id", agentID),
			port.Str("requesting_owner_id", ownerID),
			port.Str("agent_owner_id", agent.OwnerID),
		)
		return apperr.Forbidden("agent does not belong to you")
	}

	if agent.Revoked() {
		return nil
	}

	if err := s.agents.Revoke(ctx, agentID, time.Now().UTC()); err != nil {
		s.logger.Error("failed to revoke agent", port.Err(err), port.Str("agent_id", agentID))
		return apperr.Wrap(err, "could not revoke agent")
	}

	s.logger.Info("agent revoked", port.Str("agent_id", agentID), port.Str("owner_id", ownerID))

	return nil
}

func newToken(prefix string) (string, string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}

	raw := prefix + base64.RawURLEncoding.EncodeToString(b)

	return raw, hashToken(raw), nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))

	return hex.EncodeToString(sum[:])
}
