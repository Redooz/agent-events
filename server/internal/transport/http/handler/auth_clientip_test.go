package handler

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func trustedNetworks(t *testing.T, cidrs ...string) []*net.IPNet {
	t.Helper()

	networks := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			t.Fatalf("ParseCIDR(%q) error = %v, want nil", cidr, err)
		}

		networks = append(networks, network)
	}

	return networks
}

func newRequest(remoteAddr string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/exchange", nil)
	req.RemoteAddr = remoteAddr

	return req
}

func TestClientIPUntrustedPeerIgnoresForwardedHeader(t *testing.T) {
	t.Parallel()

	req := newRequest("192.0.2.10:1234")
	req.Header.Set("X-Forwarded-For", "203.0.113.7")

	if got := clientIP(req, nil); got != "192.0.2.10" {
		t.Errorf("clientIP() = %q, want %q (header must be ignored for untrusted peers)", got, "192.0.2.10")
	}
}

func TestClientIPTrustedPeerUsesRightMostUntrustedEntry(t *testing.T) {
	t.Parallel()

	trusted := trustedNetworks(t, "10.0.0.0/8", "172.16.0.0/12")

	req := newRequest("10.0.0.5:1234")
	req.Header.Set("X-Forwarded-For", "203.0.113.7")
	if got := clientIP(req, trusted); got != "203.0.113.7" {
		t.Errorf("clientIP() = %q, want %q", got, "203.0.113.7")
	}

	req = newRequest("10.0.0.5:1234")
	req.Header.Set("X-Forwarded-For", "198.51.100.9, 10.0.0.5")
	if got := clientIP(req, trusted); got != "198.51.100.9" {
		t.Errorf("clientIP() = %q, want %q (proxy entries must be skipped from the right)", got, "198.51.100.9")
	}
}

func TestClientIPTrustedPeerBrokenHeaderValues(t *testing.T) {
	t.Parallel()

	trusted := trustedNetworks(t, "10.0.0.0/8")

	req := newRequest("10.0.0.5:1234")
	req.Header.Set("X-Forwarded-For", "not-an-ip, garbage")
	if got := clientIP(req, trusted); got != "garbage" {
		t.Errorf("clientIP() = %q, want %q (right-most non-proxy entry wins)", got, "garbage")
	}

	req = newRequest("10.0.0.5:1234")
	req.Header.Set("X-Forwarded-For", ", ,")
	if got := clientIP(req, trusted); got != "10.0.0.5" {
		t.Errorf("clientIP() = %q, want %q (empty entries fall back to peer)", got, "10.0.0.5")
	}

	req = newRequest("10.0.0.5:1234")
	if got := clientIP(req, trusted); got != "10.0.0.5" {
		t.Errorf("clientIP() = %q, want %q (missing header falls back to peer)", got, "10.0.0.5")
	}
}

func TestClientIPAllEntriesTrustedFallsBackToPeer(t *testing.T) {
	t.Parallel()

	trusted := trustedNetworks(t, "10.0.0.0/8")

	req := newRequest("10.0.0.5:1234")
	req.Header.Set("X-Forwarded-For", "10.0.0.9, 10.0.0.5")
	if got := clientIP(req, trusted); got != "10.0.0.5" {
		t.Errorf("clientIP() = %q, want %q", got, "10.0.0.5")
	}
}
