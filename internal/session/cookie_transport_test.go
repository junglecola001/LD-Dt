package session

import (
	"path/filepath"
	"testing"

	"github.com/junglecola001/LD-Dt/internal/domain"
)

func TestBrowserLikeTransportParsesAndPersistsCredentials(t *testing.T) {
	persist := filepath.Join(t.TempDir(), "creds.json")
	tr, err := NewBrowserLikeTransport("https://linux.do", persist)
	if err != nil {
		t.Fatalf("NewBrowserLikeTransport failed: %v", err)
	}

	err = tr.ApplyManualCredentials(domain.CredentialsInput{
		CookieHeader: "Cookie: cf_clearance=clear123; _t=tok456; _forum_session=sess789",
		UserAPIKey:   "api-key-abc",
		UserAgent:    "CustomUA/1.0",
	})
	if err != nil {
		t.Fatalf("ApplyManualCredentials failed: %v", err)
	}
	if !tr.HasAuthMaterial() {
		t.Fatalf("expected HasAuthMaterial true")
	}

	// Reload from disk into a fresh transport instance.
	tr2, err := NewBrowserLikeTransport("https://linux.do", persist)
	if err != nil {
		t.Fatalf("reload NewBrowserLikeTransport failed: %v", err)
	}
	if !tr2.HasAuthMaterial() || tr2.userAgent != "CustomUA/1.0" || tr2.userAPIKey != "api-key-abc" {
		t.Fatalf("expected persisted credentials reloaded, got ua=%q key=%q", tr2.userAgent, tr2.userAPIKey)
	}
	cookies := tr2.jar.Cookies(tr2.baseURL)
	if len(cookies) != 3 {
		t.Fatalf("expected 3 host-only cookies loaded, got %d", len(cookies))
	}
}
