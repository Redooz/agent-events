package oidc

import (
	"context"
	"fmt"
	"regexp"

	"github.com/coreos/go-oidc/v3/oidc"

	"agent-events/server/internal/core/domain"
)

const microsoftIssuerTemplate = "https://login.microsoftonline.com/common/v2.0"

var microsoftIssuerPattern = regexp.MustCompile(`^https://login\.microsoftonline\.com/([^/]+)/v2\.0$`)

type MicrosoftVerifier struct {
	verifier   *oidc.IDTokenVerifier
	tenant     string
	restricted bool
}

func NewMicrosoft(ctx context.Context, clientID, tenant, jwksURL string) (*MicrosoftVerifier, error) {
	keySet := oidc.NewRemoteKeySet(ctx, jwksURL)

	verifier := oidc.NewVerifier(microsoftIssuerTemplate, keySet, &oidc.Config{
		ClientID:        clientID,
		SkipIssuerCheck: true,
	})

	switch tenant {
	case "", "common", "consumers", "organizations":
		return &MicrosoftVerifier{verifier: verifier}, nil
	}

	return &MicrosoftVerifier{verifier: verifier, tenant: tenant, restricted: true}, nil
}

func (v *MicrosoftVerifier) Verify(ctx context.Context, idToken string) (domain.Identity, error) {
	token, err := v.verifier.Verify(ctx, idToken)
	if err != nil {
		return domain.Identity{}, err
	}

	match := microsoftIssuerPattern.FindStringSubmatch(token.Issuer)
	if match == nil {
		return domain.Identity{}, fmt.Errorf("microsoft id token issuer %q is not a tenant issuer", token.Issuer)
	}

	if v.restricted && match[1] != v.tenant {
		return domain.Identity{}, fmt.Errorf("microsoft id token tenant %q is not allowed", match[1])
	}

	var claims struct {
		Email string `json:"email"`
	}
	if err := token.Claims(&claims); err != nil {
		return domain.Identity{}, fmt.Errorf("parse microsoft id token claims: %w", err)
	}

	return domain.Identity{
		Provider: domain.ProviderMicrosoft,
		Subject:  token.Subject,
		Email:    claims.Email,
	}, nil
}
