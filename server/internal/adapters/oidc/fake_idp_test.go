package oidc

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fakeIDP struct {
	server *httptest.Server
	key    *rsa.PrivateKey
	kid    string
}

func newFakeIDP(t *testing.T) *fakeIDP {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}

	idp := &fakeIDP{key: key, kid: "test-key"}

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	idp.server = server

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, map[string]string{
			"issuer":   server.URL,
			"jwks_uri": server.URL + "/keys",
		})
	})

	mux.HandleFunc("/keys", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, map[string]any{
			"keys": []map[string]string{{
				"kty": "RSA",
				"use": "sig",
				"kid": idp.kid,
				"alg": "RS256",
				"n":   base64url(key.N.Bytes()),
				"e":   base64url(big.NewInt(int64(key.E)).Bytes()),
			}},
		})
	})

	return idp
}

func (f *fakeIDP) sign(t *testing.T, alg, kid string, headerOverrides, claims map[string]any) string {
	t.Helper()

	header := map[string]any{"typ": "JWT", "kid": kid}
	if alg != "" {
		header["alg"] = alg
	}
	for k, v := range headerOverrides {
		header[k] = v
	}

	now := time.Now().Unix()
	merged := map[string]any{
		"iss": f.server.URL,
		"aud": "client-id",
		"sub": "subject-1",
		"exp": now + 3600,
		"iat": now,
	}
	for k, v := range claims {
		merged[k] = v
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}

	claimsJSON, err := json.Marshal(merged)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}

	signingInput := base64url(headerJSON) + "." + base64url(claimsJSON)

	if alg == "none" {
		return signingInput + "."
	}

	sum := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, f.key, cryptoSHA256, sum[:])
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	return signingInput + "." + base64url(signature)
}

func (f *fakeIDP) signMicrosoftToken(t *testing.T, tenantID string, extraClaims map[string]any) string {
	t.Helper()

	claims := map[string]any{
		"iss": "https://login.microsoftonline.com/" + tenantID + "/v2.0",
	}
	for k, v := range extraClaims {
		claims[k] = v
	}

	return f.sign(t, "RS256", f.kid, nil, claims)
}

func writeJSON(t *testing.T, w http.ResponseWriter, payload any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		t.Fatalf("write json: %v", err)
	}
}

func base64url(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}
