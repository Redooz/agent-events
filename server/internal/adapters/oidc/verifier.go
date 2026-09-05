package oidc

import (
	"context"
	"errors"
	"fmt"

	"agent-events/server/internal/core/domain"
	"agent-events/server/internal/core/port"
)

const (
	googleIssuer     = "https://accounts.google.com"
	appleIssuer      = "https://appleid.apple.com"
	microsoftJWKSURL = "https://login.microsoftonline.com/common/discovery/v2.0/keys"
)

type verifyFunc func(ctx context.Context, idToken string) (domain.Identity, error)

type Verifier struct {
	byProvider map[string]verifyFunc
}

func New(ctx context.Context, allowDevVerifier bool, googleClientID, appleClientID, microsoftClientID, microsoftTenant string) (*Verifier, error) {
	return newWithOptions(ctx, providerOptions{
		allowDevVerifier:  allowDevVerifier,
		googleClientID:    googleClientID,
		googleIssuer:      googleIssuer,
		appleClientID:     appleClientID,
		appleIssuer:       appleIssuer,
		microsoftClientID: microsoftClientID,
		microsoftTenant:   microsoftTenant,
		microsoftJWKSURL:  microsoftJWKSURL,
	})
}

type providerOptions struct {
	allowDevVerifier  bool
	googleClientID    string
	googleIssuer      string
	appleClientID     string
	appleIssuer       string
	microsoftClientID string
	microsoftTenant   string
	microsoftJWKSURL  string
}

func newWithOptions(ctx context.Context, opts providerOptions) (*Verifier, error) {
	byProvider := make(map[string]verifyFunc, len(domain.Providers))

	if opts.googleClientID != "" {
		google, err := NewGoogle(ctx, opts.googleClientID, opts.googleIssuer)
		if err != nil {
			return nil, err
		}

		byProvider[domain.ProviderGoogle] = google.Verify
	}

	if opts.appleClientID != "" {
		apple, err := NewApple(ctx, opts.appleClientID, opts.appleIssuer)
		if err != nil {
			return nil, err
		}

		byProvider[domain.ProviderApple] = apple.Verify
	}

	if opts.microsoftClientID != "" {
		microsoft, err := NewMicrosoft(ctx, opts.microsoftClientID, opts.microsoftTenant, opts.microsoftJWKSURL)
		if err != nil {
			return nil, err
		}

		byProvider[domain.ProviderMicrosoft] = microsoft.Verify
	}

	if len(byProvider) == 0 {
		if !opts.allowDevVerifier {
			return nil, errors.New("no identity provider client id is configured and the dev verifier is not allowed in this environment")
		}

		dev := devVerifier{}
		for _, provider := range domain.Providers {
			p := provider
			byProvider[p] = func(ctx context.Context, idToken string) (domain.Identity, error) {
				return dev.verify(ctx, p, idToken)
			}
		}
	}

	return &Verifier{byProvider: byProvider}, nil
}

func (v *Verifier) Verify(ctx context.Context, provider, idToken string) (domain.Identity, error) {
	verify, ok := v.byProvider[provider]
	if !ok {
		return domain.Identity{}, fmt.Errorf("%w: %q", port.ErrProviderUnavailable, provider)
	}

	return verify(ctx, idToken)
}
