package shield

import (
	"testing"
)

func TestValidateShieldURL(t *testing.T) {
	valid := []string{
		"https://linux.do/challenge",
		"https://linux.do/login",
		"https://connect.linux.do/oauth2/authorize",
	}
	for _, u := range valid {
		if _, err := validateShieldURL(u); err != nil {
			t.Fatalf("expected valid URL %q, got error: %v", u, err)
		}
	}

	invalid := []string{
		"",
		"http://linux.do/login",
		"file:///C:/Windows/win.ini",
		"javascript:alert(1)",
		"https://evil.example.com/challenge",
	}
	for _, u := range invalid {
		if _, err := validateShieldURL(u); err == nil {
			t.Fatalf("expected invalid URL %q to be rejected", u)
		}
	}
}

func TestLaunchWindowRejectsDisallowedOriginBeforeFallback(t *testing.T) {
	called := false
	err := LaunchWindow("https://evil.example.com", t.TempDir(), func(string) error {
		called = true
		return nil
	})
	if err == nil {
		t.Fatal("expected error for non-linux.do host")
	}
	if called {
		t.Fatal("fallback must not run for rejected origin")
	}
}
