package oidc

import (
	"context"
	"errors"
	"testing"

	"agent-events/server/internal/core/domain"
	"agent-events/server/internal/core/port"
)

func TestMultiVerifierRoutesAndReportsUnavailable(t *testing.T) {
	t.Parallel()

	idp := newFakeIDP(t)

	verifier, err := newWithOptions(context.Background(), providerOptions{
		googleClientID:    testClientID,
		googleIssuer:      idp.server.URL,
		microsoftClientID: testClientID,
		microsoftTenant:   "common",
		microsoftJWKSURL:  idp.server.URL + "/keys",
	})
	if err != nil {
		t.Fatalf("newWithOptions() error = %v, want nil", err)
	}

	raw := idp.sign(t, "RS256", idp.kid, nil, map[string]any{
		"email":          testEmail,
		"email_verified": true,
	})

	identity, err := verifier.Verify(context.Background(), domain.ProviderGoogle, raw)
	if err != nil {
		t.Fatalf("Verify() google error = %v, want nil", err)
	}

	if identity.Provider != domain.ProviderGoogle || identity.Subject != testSubject {
		t.Errorf("Verify() = %+v, want google/%s", identity, testSubject)
	}

	if _, err := verifier.Verify(context.Background(), domain.ProviderApple, raw); !errors.Is(err, port.ErrProviderUnavailable) {
		t.Errorf("Verify() apple error = %v, want ErrProviderUnavailable", err)
	}

	if _, err := verifier.Verify(context.Background(), "discord", raw); !errors.Is(err, port.ErrProviderUnavailable) {
		t.Errorf("Verify() discord error = %v, want ErrProviderUnavailable", err)
	}
}

func TestMultiVerifierDevModeWhenUnconfigured(t *testing.T) {
	t.Parallel()

	verifier, err := newWithOptions(context.Background(), providerOptions{})
	if err != nil {
		t.Fatalf("newWithOptions() error = %v, want nil", err)
	}

	for _, provider := range domain.Providers {
		identity, err := verifier.Verify(context.Background(), provider, "dev-token")
		if err != nil {
			t.Fatalf("Verify(%q) error = %v, want nil", provider, err)
		}

		if identity.Provider != provider || identity.Subject != "dev-token" {
			t.Errorf("Verify(%q) = %+v, want %q/dev-token", provider, identity, provider)
		}
	}
}
