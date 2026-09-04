package oidc

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"

	"agent-events/server/internal/core/domain"
)

type GoogleVerifier struct {
	verifier *oidc.IDTokenVerifier
}

func NewGoogle(ctx context.Context, clientID, issuer string) (*GoogleVerifier, error) {
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("discover google oidc provider: %w", err)
	}

	return &GoogleVerifier{verifier: provider.Verifier(&oidc.Config{ClientID: clientID})}, nil
}

func (v *GoogleVerifier) Verify(ctx context.Context, idToken string) (domain.Identity, error) {
	token, err := v.verifier.Verify(ctx, idToken)
	if err != nil {
		return domain.Identity{}, err
	}

	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	}
	if err := token.Claims(&claims); err != nil {
		return domain.Identity{}, fmt.Errorf("parse google id token claims: %w", err)
	}

	if claims.Email == "" {
		return domain.Identity{}, fmt.Errorf("google id token has no email claim")
	}

	if !claims.EmailVerified {
		return domain.Identity{}, fmt.Errorf("google email is not verified")
	}

	return domain.Identity{
		Provider: domain.ProviderGoogle,
		Subject:  token.Subject,
		Email:    claims.Email,
	}, nil
}
