package windows

import (
	"context"
	"net/url"
	"strings"

	"github.com/junglecola001/LD-Dt/internal/domain"
)

// AuthHostPolicy defines the strict origin boundary for any remote authentication/challenge window (plan.md 7.3).
type AuthHostPolicy struct {
	AllowedHosts []string
}

// IsAllowedNavigation enforces HTTPS and allowlisted Linux.do auth/challenge origins only.
func (p AuthHostPolicy) IsAllowedNavigation(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, allowed := range p.AllowedHosts {
		a := strings.ToLower(strings.TrimSpace(allowed))
		if host == a || strings.HasSuffix(host, "."+a) {
			return true
		}
	}
	return false
}

// PlatformAdapter isolates Windows-specific integrations (Credential Manager/DPAPI,
// WebView2 isolated auth profile, Toast notifications, deep links) behind clean interfaces.
type PlatformAdapter interface {
	OpenIsolatedAuthHost(ctx context.Context, targetURL string) ([]domain.CookieRecord, error)
	StoreSecret(key string, value []byte) error
	ReadSecret(key string) ([]byte, error)
	DeleteSecret(key string) error
	OpenExternalURL(rawURL string) error
}

// NoopPlatformAdapter provides a safe non-privileged fallback for tests and Phase 0 skeleton builds.
type NoopPlatformAdapter struct{}

func (NoopPlatformAdapter) OpenIsolatedAuthHost(context.Context, string) ([]domain.CookieRecord, error) {
	return nil, domain.NewError(domain.ErrCodeChallengeRequired, "请使用桌面环境中的独立认证窗口完成验证", true, nil)
}

func (NoopPlatformAdapter) StoreSecret(string, []byte) error { return nil }
func (NoopPlatformAdapter) ReadSecret(string) ([]byte, error) { return nil, nil }
func (NoopPlatformAdapter) DeleteSecret(string) error         { return nil }
func (NoopPlatformAdapter) OpenExternalURL(string) error      { return nil }
