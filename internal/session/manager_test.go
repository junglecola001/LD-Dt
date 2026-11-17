package session

import (
	"errors"
	"testing"
	"time"

	"github.com/junglecola001/LD-Dt/internal/domain"
)

func TestSessionGenerationRejectsStaleCookieSync(t *testing.T) {
	mgr := NewManager("linux.do", domain.SystemClock{})
	s1 := mgr.ConfirmAccount("acct-a", "alice", "Alice")
	s2 := mgr.ConfirmAccount("acct-b", "bob", "Bob")
	if s2.SessionGeneration <= s1.SessionGeneration {
		t.Fatalf("expected generation to increase on account switch")
	}

	err := mgr.ApplySyncedCookies("linux.do", s1.SessionGeneration, []domain.CookieRecord{
		{Name: "_t", Value: "old-alice", Domain: "linux.do", Secure: true, HTTPOnly: true, SameSite: "Lax"},
	})
	var appErr *domain.AppError
	if !errors.As(err, &appErr) || appErr.Code != domain.ErrCodeStaleSession {
		t.Fatalf("expected ErrCodeStaleSession for stale cookie sync, got %v", err)
	}

	// Valid generation succeeds and drops foreign-domain cookies.
	err = mgr.ApplySyncedCookies("linux.do", s2.SessionGeneration, []domain.CookieRecord{
		{Name: "_t", Value: "bob-token", Domain: "linux.do", Secure: true, HTTPOnly: true, SameSite: "Lax", Expires: time.Now().Add(time.Hour)},
		{Name: "evil", Value: "x", Domain: "evil.example"},
	})
	if err != nil {
		t.Fatalf("unexpected error syncing valid generation cookies: %v", err)
	}

	exported := mgr.ExportCookies("linux.do")
	if len(exported) != 1 || exported[0].Name != "_t" || exported[0].Value != "bob-token" {
		t.Fatalf("expected only linux.do cookie retained, got %+v", exported)
	}
}
