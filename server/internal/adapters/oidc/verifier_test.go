package oidc

import (
	"context"
	"crypto"
	"testing"
	"time"

	"agent-events/server/internal/core/domain"
)

const cryptoSHA256 = crypto.SHA256

const (
	testClientID = "client-id"
	testSubject  = "subject-1"
	testEmail    = "user@example.com"
)

func TestGoogleVerifierAcceptsValidToken(t *testing.T) {
	t.Parallel()

	verifier, idp := newGoogleStack(t)

	raw := idp.sign(t, "RS256", idp.kid, nil, map[string]any{
		"email":          testEmail,
		"email_verified": true,
	})

	identity, err := verifier.Verify(context.Background(), raw)
	if err != nil {
		t.Fatalf("Verify() error = %v, want nil", err)
	}

	if identity.Provider != domain.ProviderGoogle || identity.Subject != testSubject || identity.Email != testEmail {
		t.Errorf("Verify() = %+v, want google/%s/%s", identity, testSubject, testEmail)
	}
}

func TestGoogleVerifierRejectsUnverifiedEmail(t *testing.T) {
	t.Parallel()

	verifier, idp := newGoogleStack(t)

	raw := idp.sign(t, "RS256", idp.kid, nil, map[string]any{
		"email":          testEmail,
		"email_verified": false,
	})

	if _, err := verifier.Verify(context.Background(), raw); err == nil {
		t.Error("Verify() accepted token with unverified email")
	}
}

func TestGoogleVerifierRejectsMissingEmail(t *testing.T) {
	t.Parallel()

	verifier, idp := newGoogleStack(t)

	raw := idp.sign(t, "RS256", idp.kid, nil, map[string]any{
		"email_verified": true,
	})

	if _, err := verifier.Verify(context.Background(), raw); err == nil {
		t.Error("Verify() accepted token without email")
	}
}

func TestGoogleVerifierRejectsWrongAudience(t *testing.T) {
	t.Parallel()

	verifier, idp := newGoogleStack(t)

	raw := idp.sign(t, "RS256", idp.kid, nil, map[string]any{
		"aud":            "another-app",
		"email":          testEmail,
		"email_verified": true,
	})

	if _, err := verifier.Verify(context.Background(), raw); err == nil {
		t.Error("Verify() accepted token minted for another app")
	}
}

func TestGoogleVerifierRejectsWrongIssuer(t *testing.T) {
	t.Parallel()

	verifier, idp := newGoogleStack(t)

	raw := idp.sign(t, "RS256", idp.kid, nil, map[string]any{
		"iss":            "https://evil.example.com",
		"email":          testEmail,
		"email_verified": true,
	})

	if _, err := verifier.Verify(context.Background(), raw); err == nil {
		t.Error("Verify() accepted token from wrong issuer")
	}
}

func TestGoogleVerifierRejectsExpiredToken(t *testing.T) {
	t.Parallel()

	verifier, idp := newGoogleStack(t)

	raw := idp.sign(t, "RS256", idp.kid, nil, map[string]any{
		"exp":            time.Now().Unix() - 60,
		"email":          testEmail,
		"email_verified": true,
	})

	if _, err := verifier.Verify(context.Background(), raw); err == nil {
		t.Error("Verify() accepted expired token")
	}
}

func TestGoogleVerifierRejectsAlgNone(t *testing.T) {
	t.Parallel()

	verifier, idp := newGoogleStack(t)

	raw := idp.sign(t, "none", "", nil, map[string]any{
		"email":          testEmail,
		"email_verified": true,
	})

	if _, err := verifier.Verify(context.Background(), raw); err == nil {
		t.Error("Verify() accepted alg=none token")
	}
}

func TestGoogleVerifierRejectsUnknownKey(t *testing.T) {
	t.Parallel()

	verifier, idp := newGoogleStack(t)

	raw := idp.sign(t, "RS256", "unknown-kid", nil, map[string]any{
		"email":          testEmail,
		"email_verified": true,
	})

	if _, err := verifier.Verify(context.Background(), raw); err == nil {
		t.Error("Verify() accepted token signed with unknown key")
	}
}

func TestAppleVerifierAcceptsStringEmailVerified(t *testing.T) {
	t.Parallel()

	verifier, idp := newAppleStack(t)

	raw := idp.sign(t, "RS256", idp.kid, nil, map[string]any{
		"email":          testEmail,
		"email_verified": "true",
	})

	identity, err := verifier.Verify(context.Background(), raw)
	if err != nil {
		t.Fatalf("Verify() error = %v, want nil", err)
	}

	if identity.Provider != domain.ProviderApple || identity.Subject != testSubject {
		t.Errorf("Verify() = %+v, want apple/%s", identity, testSubject)
	}
}

func TestAppleVerifierAcceptsBoolEmailVerified(t *testing.T) {
	t.Parallel()

	verifier, idp := newAppleStack(t)

	raw := idp.sign(t, "RS256", idp.kid, nil, map[string]any{
		"email":          testEmail,
		"email_verified": true,
	})

	if _, err := verifier.Verify(context.Background(), raw); err != nil {
		t.Fatalf("Verify() error = %v, want nil", err)
	}
}

func TestAppleVerifierRejectsUnverifiedEmail(t *testing.T) {
	t.Parallel()

	verifier, idp := newAppleStack(t)

	raw := idp.sign(t, "RS256", idp.kid, nil, map[string]any{
		"email":          testEmail,
		"email_verified": "false",
	})

	if _, err := verifier.Verify(context.Background(), raw); err == nil {
		t.Error("Verify() accepted unverified apple email")
	}
}

func TestAppleVerifierRejectsGarbageEmailVerified(t *testing.T) {
	t.Parallel()

	verifier, idp := newAppleStack(t)

	raw := idp.sign(t, "RS256", idp.kid, nil, map[string]any{
		"email":          testEmail,
		"email_verified": []string{"true"},
	})

	if _, err := verifier.Verify(context.Background(), raw); err == nil {
		t.Error("Verify() accepted malformed email_verified")
	}
}

func TestAppleVerifierRejectsMissingEmail(t *testing.T) {
	t.Parallel()

	verifier, idp := newAppleStack(t)

	raw := idp.sign(t, "RS256", idp.kid, nil, map[string]any{})

	if _, err := verifier.Verify(context.Background(), raw); err == nil {
		t.Error("Verify() accepted token without email")
	}
}

func TestMicrosoftVerifierAcceptsAnyTenantByDefault(t *testing.T) {
	t.Parallel()

	verifier, idp := newMicrosoftStack(t, "common")

	raw := idp.signMicrosoftToken(t, "11111111-2222-3333-4444-555555555555", map[string]any{
		"email": testEmail,
	})

	identity, err := verifier.Verify(context.Background(), raw)
	if err != nil {
		t.Fatalf("Verify() error = %v, want nil", err)
	}

	if identity.Provider != domain.ProviderMicrosoft || identity.Subject != testSubject {
		t.Errorf("Verify() = %+v, want microsoft/%s", identity, testSubject)
	}
}

func TestMicrosoftVerifierAcceptsTokenWithoutEmail(t *testing.T) {
	t.Parallel()

	verifier, idp := newMicrosoftStack(t, "common")

	raw := idp.signMicrosoftToken(t, "11111111-2222-3333-4444-555555555555", nil)

	identity, err := verifier.Verify(context.Background(), raw)
	if err != nil {
		t.Fatalf("Verify() error = %v, want nil", err)
	}

	if identity.Email != "" {
		t.Errorf("Verify() email = %q, want empty", identity.Email)
	}
}

func TestMicrosoftVerifierRejectsUnexpectedIssuer(t *testing.T) {
	t.Parallel()

	verifier, idp := newMicrosoftStack(t, "common")

	raw := idp.sign(t, "RS256", idp.kid, nil, map[string]any{
		"iss": "https://evil.example.com/v2.0",
	})

	if _, err := verifier.Verify(context.Background(), raw); err == nil {
		t.Error("Verify() accepted token with non-microsoft issuer")
	}
}

func TestMicrosoftVerifierRejectsIssuerWithoutTenant(t *testing.T) {
	t.Parallel()

	verifier, idp := newMicrosoftStack(t, "common")

	raw := idp.sign(t, "RS256", idp.kid, nil, map[string]any{
		"iss": "https://login.microsoftonline.com/v2.0",
	})

	if _, err := verifier.Verify(context.Background(), raw); err == nil {
		t.Error("Verify() accepted issuer without tenant id")
	}
}

func TestMicrosoftVerifierEnforcesTenantRestriction(t *testing.T) {
	t.Parallel()

	verifier, idp := newMicrosoftStack(t, "99999999-8888-7777-6666-555555555555")

	untrusted := idp.signMicrosoftToken(t, "11111111-2222-3333-4444-555555555555", nil)
	if _, err := verifier.Verify(context.Background(), untrusted); err == nil {
		t.Error("Verify() accepted token from untrusted tenant")
	}

	trusted := idp.signMicrosoftToken(t, "99999999-8888-7777-6666-555555555555", nil)
	if _, err := verifier.Verify(context.Background(), trusted); err != nil {
		t.Errorf("Verify() trusted tenant error = %v, want nil", err)
	}
}

func TestMicrosoftVerifierRejectsWrongAudience(t *testing.T) {
	t.Parallel()

	verifier, idp := newMicrosoftStack(t, "common")

	raw := idp.signMicrosoftToken(t, "11111111-2222-3333-4444-555555555555", map[string]any{
		"aud": "another-app",
	})

	if _, err := verifier.Verify(context.Background(), raw); err == nil {
		t.Error("Verify() accepted token minted for another app")
	}
}

func newGoogleStack(t *testing.T) (*GoogleVerifier, *fakeIDP) {
	t.Helper()

	idp := newFakeIDP(t)
	verifier, err := NewGoogle(context.Background(), testClientID, idp.server.URL)
	if err != nil {
		t.Fatalf("NewGoogle() error = %v, want nil", err)
	}

	return verifier, idp
}

func newAppleStack(t *testing.T) (*AppleVerifier, *fakeIDP) {
	t.Helper()

	idp := newFakeIDP(t)
	verifier, err := NewApple(context.Background(), testClientID, idp.server.URL)
	if err != nil {
		t.Fatalf("NewApple() error = %v, want nil", err)
	}

	return verifier, idp
}

func newMicrosoftStack(t *testing.T, tenant string) (*MicrosoftVerifier, *fakeIDP) {
	t.Helper()

	idp := newFakeIDP(t)
	verifier, err := NewMicrosoft(context.Background(), testClientID, tenant, idp.server.URL+"/keys")
	if err != nil {
		t.Fatalf("NewMicrosoft() error = %v, want nil", err)
	}

	return verifier, idp
}
