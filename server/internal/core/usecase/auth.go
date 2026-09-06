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
	"agent-events/server/internal/core/usecase/types"
	"agent-events/server/pkg/apperr"
)

const (
	ActionAuthExchange = "auth.exchange"

	tokenPrefixUser  = "aeu_"
	tokenPrefixAgent = "aea_"

	agentTouchInterval = time.Minute
)

type AuthService struct {
	verifier port.IdentityVerifier
	users    port.UserRepository
	tokens   port.UserTokenRepository
	agents   port.AgentRepository
	limiter  port.RateLimiter
	logger   port.Logger
	config   types.AuthConfig
}

func NewAuthService(
	verifier port.IdentityVerifier,
	users port.UserRepository,
	tokens port.UserTokenRepository,
	agents port.AgentRepository,
	limiter port.RateLimiter,
	logger port.Logger,
	config types.AuthConfig,
) *AuthService {
	return &AuthService{
		verifier: verifier,
		users:    users,
		tokens:   tokens,
		agents:   agents,
		limiter:  limiter,
		logger:   logger,
		config:   config,
	}
}

func (s *AuthService) Exchange(ctx context.Context, in types.ExchangeInput) (types.ExchangeResult, error) {
	if !domain.ValidProvider(in.Provider) {
		return types.ExchangeResult{}, apperr.Invalid("unknown identity provider")
	}

	allowed, err := s.limiter.Allow(ctx, "ip:"+in.ClientIP, ActionAuthExchange)
	if err != nil {
		s.logger.Error("exchange rate limit check failed", port.Err(err))
		return types.ExchangeResult{}, apperr.Wrap(err, "could not check rate limit")
	}

	if !allowed {
		s.logger.Warn("exchange rate limited", port.Str("client_ip", in.ClientIP))
		return types.ExchangeResult{}, apperr.TooMany("too many sign-in attempts, try again later")
	}

	identity, err := s.verifier.Verify(ctx, in.Provider, in.IDToken)
	switch {
	case errors.Is(err, port.ErrProviderUnavailable):
		s.logger.Warn("exchange for unconfigured provider", port.Str("provider", in.Provider))
		return types.ExchangeResult{}, apperr.Invalid("identity provider is not configured")
	case err != nil:
		s.logger.Warn("identity token rejected", port.Str("provider", in.Provider), port.Err(err))
		return types.ExchangeResult{}, apperr.Unauthorized("invalid or expired identity token")
	}

	user, err := s.users.UpsertByIdentity(ctx, identity.Provider, identity.Subject, identity.Email)
	if err != nil {
		s.logger.Error("failed to upsert user", port.Err(err), port.Str("provider", identity.Provider))
		return types.ExchangeResult{}, apperr.Wrap(err, "could not sign in")
	}

	rawToken, hash, err := newToken(tokenPrefixUser)
	if err != nil {
		s.logger.Error("failed to generate user token", port.Err(err))
		return types.ExchangeResult{}, apperr.Wrap(err, "could not sign in")
	}

	now := time.Now().UTC()
	expiresAt := now.Add(s.config.UserTokenTTL)

	if err := s.tokens.Create(ctx, domain.UserToken{
		TokenHash: hash,
		UserID:    user.ID,
		ExpiresAt: expiresAt,
		CreatedAt: now,
	}); err != nil {
		s.logger.Error("failed to store user token", port.Err(err), port.Str("user_id", user.ID))
		return types.ExchangeResult{}, apperr.Wrap(err, "could not sign in")
	}

	s.logger.Info("user token issued",
		port.Str("user_id", user.ID),
		port.Str("provider", user.Provider),
		port.Str("client_ip", in.ClientIP),
	)

	return types.ExchangeResult{User: user, Token: rawToken, ExpiresAt: expiresAt}, nil
}

func (s *AuthService) AuthenticateUser(ctx context.Context, rawToken string) (domain.User, error) {
	token, err := s.tokens.GetByHash(ctx, HashToken(rawToken))
	switch {
	case errors.Is(err, domain.ErrUserTokenNotFound):
		return domain.User{}, apperr.Unauthorized("invalid or expired user token")
	case err != nil:
		s.logger.Error("failed to fetch user token", port.Err(err))
		return domain.User{}, apperr.Wrap(err, "could not authenticate")
	}

	if token.Expired(time.Now().UTC()) {
		return domain.User{}, apperr.Unauthorized("invalid or expired user token")
	}

	user, err := s.users.Get(ctx, token.UserID)
	switch {
	case errors.Is(err, domain.ErrUserNotFound):
		return domain.User{}, apperr.Unauthorized("invalid or expired user token")
	case err != nil:
		s.logger.Error("failed to fetch user for token", port.Err(err), port.Str("user_id", token.UserID))
		return domain.User{}, apperr.Wrap(err, "could not authenticate")
	}

	return user, nil
}

func (s *AuthService) RevokeUserToken(ctx context.Context, tokenHash string) error {
	if err := s.tokens.DeleteByHash(ctx, tokenHash); err != nil {
		s.logger.Error("failed to revoke user token", port.Err(err))
		return apperr.Wrap(err, "could not log out")
	}

	return nil
}

func (s *AuthService) AuthenticateAgent(ctx context.Context, rawKey string) (types.AgentCredentials, error) {
	agent, err := s.agents.GetByKeyHash(ctx, HashToken(rawKey))
	switch {
	case errors.Is(err, domain.ErrAgentNotFound):
		return types.AgentCredentials{}, apperr.Unauthorized("invalid agent key")
	case err != nil:
		s.logger.Error("failed to fetch agent by key", port.Err(err))
		return types.AgentCredentials{}, apperr.Wrap(err, "could not authenticate")
	}

	if agent.Revoked() {
		s.logger.Warn("revoked agent key used", port.Str("agent_id", agent.ID), port.Str("user_id", agent.UserID))
		return types.AgentCredentials{}, apperr.Unauthorized("agent key has been revoked")
	}

	user, err := s.users.Get(ctx, agent.UserID)
	switch {
	case errors.Is(err, domain.ErrUserNotFound):
		s.logger.Error("agent references missing user", port.Str("agent_id", agent.ID), port.Str("user_id", agent.UserID))
		return types.AgentCredentials{}, apperr.Unauthorized("invalid agent key")
	case err != nil:
		s.logger.Error("failed to fetch agent user", port.Err(err), port.Str("agent_id", agent.ID))
		return types.AgentCredentials{}, apperr.Wrap(err, "could not authenticate")
	}

	now := time.Now().UTC()
	if agent.LastUsedAt.IsZero() || now.Sub(agent.LastUsedAt) >= agentTouchInterval {
		if err := s.agents.TouchLastUsed(ctx, agent.ID, now); err != nil {
			s.logger.Warn("failed to touch agent last used", port.Err(err), port.Str("agent_id", agent.ID))
		}
	}

	return types.AgentCredentials{Agent: agent, User: user}, nil
}

func (s *AuthService) CreateAgent(ctx context.Context, userID, name string) (types.AgentWithKey, error) {
	agent := domain.Agent{
		ID:        newID(),
		UserID:    userID,
		Name:      name,
		CreatedAt: time.Now().UTC(),
	}

	if err := agent.Validate(); err != nil {
		s.logger.Warn("agent rejected", port.Err(err), port.Str("user_id", userID))
		return types.AgentWithKey{}, apperr.Invalid(err.Error())
	}

	rawKey, hash, err := newToken(tokenPrefixAgent)
	if err != nil {
		s.logger.Error("failed to generate agent key", port.Err(err))
		return types.AgentWithKey{}, apperr.Wrap(err, "could not create agent")
	}

	agent.KeyHash = hash

	created, err := s.agents.CreateIfUnderLimit(ctx, agent, s.config.MaxAgentsPerUser)
	if err != nil {
		s.logger.Error("failed to store agent", port.Err(err), port.Str("user_id", userID))
		return types.AgentWithKey{}, apperr.Wrap(err, "could not create agent")
	}

	if !created {
		s.logger.Warn("agent limit reached", port.Str("user_id", userID), port.Int("max_agents", s.config.MaxAgentsPerUser))
		return types.AgentWithKey{}, apperr.Forbidden("agent limit reached, revoke an agent first")
	}

	s.logger.Info("agent created", port.Str("agent_id", agent.ID), port.Str("user_id", userID))

	return types.AgentWithKey{Agent: agent, Key: rawKey}, nil
}

func (s *AuthService) ListAgents(ctx context.Context, userID string) ([]domain.Agent, error) {
	agents, err := s.agents.ListByUser(ctx, userID)
	if err != nil {
		s.logger.Error("failed to list agents", port.Err(err), port.Str("user_id", userID))
		return nil, apperr.Wrap(err, "could not list agents")
	}

	return agents, nil
}

func (s *AuthService) RevokeAgent(ctx context.Context, userID, agentID string) error {
	agent, err := s.agents.Get(ctx, agentID)
	switch {
	case errors.Is(err, domain.ErrAgentNotFound):
		return apperr.NotFound("agent not found")
	case err != nil:
		s.logger.Error("failed to fetch agent for revoke", port.Err(err), port.Str("agent_id", agentID))
		return apperr.Wrap(err, "could not revoke agent")
	}

	if agent.UserID != userID {
		s.logger.Warn("agent revoke denied for non-user",
			port.Str("agent_id", agentID),
			port.Str("requesting_user_id", userID),
			port.Str("agent_user_id", agent.UserID),
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

	s.logger.Info("agent revoked", port.Str("agent_id", agentID), port.Str("user_id", userID))

	return nil
}

func newToken(prefix string) (string, string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}

	raw := prefix + base64.RawURLEncoding.EncodeToString(b)

	return raw, HashToken(raw), nil
}

func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))

	return hex.EncodeToString(sum[:])
}
