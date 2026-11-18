package app

import (
	"context"
	"net/url"
	"strings"

	"github.com/junglecola001/LD-Dt/internal/domain"
	"github.com/junglecola001/LD-Dt/internal/network"
	"github.com/junglecola001/LD-Dt/internal/session"
	"github.com/junglecola001/LD-Dt/internal/storage"
)

// AuthBridgeHooks connects AuthService to desktop window/browser helpers and Discourse probes
// without coupling domain services to Wails packages.
type AuthBridgeHooks struct {
	Transport            *session.BrowserLikeTransport
	OpenShieldWindow     func(targetURL string) error
	SyncFromShieldWindow func(ctx context.Context) error
	OpenExternalURL      func(targetURL string) error
	ProbeCurrentUser     func(ctx context.Context) (accountID, username, displayName string, err error)
	ProbeSiteMetadata    func(ctx context.Context) error
}

// AuthService manages site session state, account namespaces, Cloudflare shield windows, and safe logout boundaries.
type AuthService struct {
	sessions *session.Manager
	coord    *network.RequestCoordinator
	store    *storage.MemoryStore
	events   domain.EventEmitter
	hooks    AuthBridgeHooks
}

// NewAuthService creates an AuthService backed by the session manager and shared coordinator.
func NewAuthService(sessions *session.Manager, coord *network.RequestCoordinator, store *storage.MemoryStore, events domain.EventEmitter) *AuthService {
	return &AuthService{
		sessions: sessions,
		coord:    coord,
		store:    store,
		events:   events,
	}
}

// SetHooks configures desktop-specific window, credential, and probe callbacks.
func (s *AuthService) SetHooks(hooks AuthBridgeHooks) {
	s.hooks = hooks
}

// GetSession returns the current server-verified session snapshot.
func (s *AuthService) GetSession(_ context.Context) domain.SiteSession {
	return s.sessions.Current()
}

// GetNetworkStatus returns the shared site coordinator state.
func (s *AuthService) GetNetworkStatus(_ context.Context) domain.NetworkStatusSnapshot {
	if s.coord == nil {
		return domain.NetworkStatusSnapshot{
			SiteID: s.sessions.Current().SiteID,
			Phase:  domain.NetworkReady,
		}
	}
	return s.coord.Status()
}

// OpenShieldWindow opens an isolated Wails WebView2 window for Cloudflare challenge or Linux.do login (plan.md 7.3).
func (s *AuthService) OpenShieldWindow(_ context.Context, mode string) error {
	target := "https://linux.do/challenge"
	if strings.EqualFold(mode, "login") {
		target = "https://linux.do/login"
	}
	if s.hooks.OpenShieldWindow == nil {
		return domain.NewError(domain.ErrCodeChallengeRequired, "当前环境不支持内置验证窗口，请在系统浏览器完成验证", true, nil)
	}
	return s.hooks.OpenShieldWindow(target)
}

// OpenExternalURL safely opens an HTTPS URL in the user's default system browser.
func (s *AuthService) OpenExternalURL(_ context.Context, rawURL string) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || !strings.EqualFold(parsed.Scheme, "https") {
		return domain.NewError(domain.ErrCodeValidationFailed, "仅允许打开 HTTPS 链接", false, err)
	}
	if s.hooks.OpenExternalURL == nil {
		return nil
	}
	return s.hooks.OpenExternalURL(parsed.String())
}

// ApplyCredentials stores browser Cookie / User-Api-Key / User-Agent and probes Linux.do once.
func (s *AuthService) ApplyCredentials(ctx context.Context, input domain.CredentialsInput) (domain.AuthProbeResult, error) {
	if s.hooks.Transport != nil {
		if err := s.hooks.Transport.ApplyManualCredentials(input); err != nil {
			return domain.AuthProbeResult{
				Session: s.sessions.Current(),
				Network: s.GetNetworkStatus(ctx),
			}, err
		}
	}
	return s.ProbeRecovery(ctx)
}

// ProbeRecovery runs a single safe GET probe to check whether Cloudflare/429 blocks have cleared.
func (s *AuthService) ProbeRecovery(ctx context.Context) (domain.AuthProbeResult, error) {
	if s.hooks.SyncFromShieldWindow != nil {
		_ = s.hooks.SyncFromShieldWindow(ctx)
	}
	if s.coord != nil {
		s.coord.ResetChallengeCooldown()
	}
	if s.hooks.ProbeCurrentUser != nil {
		acctID, username, displayName, err := s.hooks.ProbeCurrentUser(ctx)
		if err == nil && acctID != "" {
			cur := s.sessions.ConfirmAccount(acctID, username, displayName)
			if s.events != nil {
				s.events.Emit("SessionChanged", cur)
				s.events.Emit("NetworkStateChanged", s.GetNetworkStatus(ctx))
			}
			return domain.AuthProbeResult{
				Session: cur,
				Network: s.GetNetworkStatus(ctx),
			}, nil
		}
		if err != nil && s.hooks.ProbeSiteMetadata == nil {
			return domain.AuthProbeResult{
				Session: s.sessions.Current(),
				Network: s.GetNetworkStatus(ctx),
			}, err
		}
	}
	if s.hooks.ProbeSiteMetadata != nil {
		err := s.hooks.ProbeSiteMetadata(ctx)
		return domain.AuthProbeResult{
			Session: s.sessions.Current(),
			Network: s.GetNetworkStatus(ctx),
		}, err
	}
	return domain.AuthProbeResult{
		Session: s.sessions.Current(),
		Network: s.GetNetworkStatus(ctx),
	}, nil
}

// SyncAndProbeFromBrowser persists cookies synced from the isolated Edge CDP session and refreshes session state.
func (s *AuthService) SyncAndProbeFromBrowser(ctx context.Context, cookieHeader, userAgent string) {
	if s.hooks.Transport != nil && strings.TrimSpace(cookieHeader) != "" {
		_ = s.hooks.Transport.ApplyManualCredentials(domain.CredentialsInput{
			CookieHeader: cookieHeader,
			UserAgent:    userAgent,
		})
	}
	_, _ = s.ProbeRecovery(ctx)
}

// ConfirmAccount switches to an authenticated namespace and emits SessionChanged.
func (s *AuthService) ConfirmAccount(_ context.Context, accountID, username, displayName string) domain.SiteSession {
	cur := s.sessions.ConfirmAccount(accountID, username, displayName)
	if s.events != nil {
		s.events.Emit("SessionChanged", cur)
	}
	return cur
}

// MarkReauthNeeded suspends writes while preserving drafts and non-sensitive caches.
func (s *AuthService) MarkReauthNeeded(_ context.Context) domain.SiteSession {
	cur := s.sessions.MarkReauthNeeded()
	if s.events != nil {
		s.events.Emit("SessionChanged", cur)
	}
	return cur
}

// Logout clears credentials and active cookies while preserving isolated drafts unless clearAllData is true.
func (s *AuthService) Logout(ctx context.Context, clearAllData bool) (domain.SiteSession, error) {
	prevScope := s.sessions.Current().AccountScope()
	if s.hooks.Transport != nil {
		_ = s.hooks.Transport.Clear()
	}
	if clearAllData && s.store != nil {
		s.store.ClearAllAccountData(prevScope)
	} else if s.store != nil {
		// Clear private content caches on logout while preserving isolated drafts (plan.md 3.5).
		_ = s.store.ClearContentCache(ctx, prevScope)
	}
	cur := s.sessions.Logout()
	if s.events != nil {
		s.events.Emit("SessionChanged", cur)
	}
	return cur, nil
}
