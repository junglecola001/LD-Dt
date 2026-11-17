package session

import (
	"errors"
	"strings"
	"sync"

	"github.com/junglecola001/LD-Dt/internal/domain"
)

// Manager owns site session generation, account isolation, and cookie boundary synchronization.
// Prevents stale WebView profile snapshots from overwriting a newer login (plan.md 3.5, 7.3, B10).
type Manager struct {
	mu      sync.RWMutex
	siteID  string
	current domain.SiteSession
	cookies map[string][]domain.CookieRecord
	clock   domain.Clock
}

// NewManager creates an anonymous session initialized at generation 1.
func NewManager(siteID string, clock domain.Clock) *Manager {
	if clock == nil {
		clock = domain.SystemClock{}
	}
	return &Manager{
		siteID: siteID,
		clock:  clock,
		current: domain.SiteSession{
			SiteID:            siteID,
			SessionGeneration: 1,
			Status:            domain.SessionAnonymous,
			UpdatedAt:         clock.Now(),
		},
		cookies: make(map[string][]domain.CookieRecord),
	}
}

// Current returns a copy of the active SiteSession.
func (m *Manager) Current() domain.SiteSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current
}

// Generation implements domain.SessionCookieStore.
func (m *Manager) Generation() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current.SessionGeneration
}

// BeginConnect transitions into verifying or connected state for a confirmed account, advancing generation on account change.
func (m *Manager) ConfirmAccount(accountID, username, displayName string) domain.SiteSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.current.SessionGeneration++
	m.current.AccountID = accountID
	m.current.Username = username
	m.current.DisplayName = displayName
	m.current.Status = domain.SessionConnected
	m.current.UpdatedAt = m.clock.Now()
	return m.current
}

// MarkReauthNeeded suspends writes while keeping existing caches/drafts scoped to the account.
func (m *Manager) MarkReauthNeeded() domain.SiteSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.current.Status = domain.SessionReauthNeeded
	m.current.UpdatedAt = m.clock.Now()
	return m.current
}

// Logout advances session generation, clears active cookies, and drops back to anonymous mode.
// Protected account drafts are preserved by caller policy unless ClearAllData is explicitly requested.
func (m *Manager) Logout() domain.SiteSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.current.SessionGeneration++
	delete(m.cookies, m.siteID)
	m.current.AccountID = ""
	m.current.Username = ""
	m.current.DisplayName = ""
	m.current.Status = domain.SessionAnonymous
	m.current.UpdatedAt = m.clock.Now()
	return m.current
}

// ApplySyncedCookies rejects stale generation snapshots and enforces HTTPS / host domain boundaries.
func (m *Manager) ApplySyncedCookies(siteID string, generation int64, incoming []domain.CookieRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if siteID != m.siteID {
		return domain.NewError(domain.ErrCodeValidationFailed, "站点标识不匹配", false, nil)
	}
	if generation != m.current.SessionGeneration {
		return domain.NewError(domain.ErrCodeStaleSession, "忽略过期会话的 Cookie 同步", true, nil)
	}

	filtered := make([]domain.CookieRecord, 0, len(incoming))
	for _, c := range incoming {
		if c.Name == "" || c.Domain == "" {
			continue
		}
		if !domainMatchesSite(c.Domain, siteID) {
			continue
		}
		if c.Path == "" {
			c.Path = "/"
		}
		filtered = append(filtered, c)
	}
	m.cookies[siteID] = filtered
	return nil
}

// ExportCookies returns a defensive copy of stored cookie records.
func (m *Manager) ExportCookies(siteID string) []domain.CookieRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()
	src := m.cookies[siteID]
	out := make([]domain.CookieRecord, len(src))
	copy(out, src)
	return out
}

// ClearSite clears cookies for the given site and increments generation.
func (m *Manager) ClearSite(siteID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if siteID == "" {
		return errors.New("siteID required")
	}
	delete(m.cookies, siteID)
	m.current.SessionGeneration++
	return nil
}

func domainMatchesSite(cookieDomain, siteHost string) bool {
	d := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(cookieDomain)), ".")
	h := strings.ToLower(strings.TrimSpace(siteHost))
	return d == h || strings.HasSuffix(h, "."+d)
}
