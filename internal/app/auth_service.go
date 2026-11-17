package app

import (
	"context"

	"github.com/junglecola001/LD-Dt/internal/domain"
	"github.com/junglecola001/LD-Dt/internal/network"
	"github.com/junglecola001/LD-Dt/internal/session"
	"github.com/junglecola001/LD-Dt/internal/storage"
)

// AuthService manages site session state, account namespaces, and safe logout boundaries.
type AuthService struct {
	sessions *session.Manager
	coord    *network.RequestCoordinator
	store    *storage.MemoryStore
	events   domain.EventEmitter
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
