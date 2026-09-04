package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/coreos/go-oidc/v3/oidc"

	"agent-events/server/internal/core/domain"
)

type flexibleBool bool

func (b *flexibleBool) UnmarshalJSON(data []byte) error {
	var asBool bool
	if err := json.Unmarshal(data, &asBool); err == nil {
		*b = flexibleBool(asBool)

		return nil
	}

	var asString string
	if err := json.Unmarshal(data, &asString); err != nil {
		return fmt.Errorf("value is neither bool nor string: %w", err)
	}

	parsed, err := strconv.ParseBool(asString)
	if err != nil {
		return fmt.Errorf("string value is not a bool: %w", err)
	}

	*b = flexibleBool(parsed)

	return nil
}

type AppleVerifier struct {
	verifier *oidc.IDTokenVerifier
}

func NewApple(ctx context.Context, clientID, issuer string) (*AppleVerifier, error) {
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("discover apple oidc provider: %w", err)
	}

	return &AppleVerifier{verifier: provider.Verifier(&oidc.Config{ClientID: clientID})}, nil
}

func (v *AppleVerifier) Verify(ctx context.Context, idToken string) (domain.Identity, error) {
	token, err := v.verifier.Verify(ctx, idToken)
	if err != nil {
		return domain.Identity{}, err
	}

	var claims struct {
		Email         string       `json:"email"`
		EmailVerified flexibleBool `json:"email_verified"`
	}
	if err := token.Claims(&claims); err != nil {
		return domain.Identity{}, fmt.Errorf("parse apple id token claims: %w", err)
	}

	if claims.Email == "" {
		return domain.Identity{}, fmt.Errorf("apple id token has no email claim")
	}

	if !claims.EmailVerified {
		return domain.Identity{}, fmt.Errorf("apple email is not verified")
	}

	return domain.Identity{
		Provider: domain.ProviderApple,
		Subject:  token.Subject,
		Email:    claims.Email,
	}, nil
}
