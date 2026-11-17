package app

import (
	"context"
	"sync"
	"time"

	"github.com/junglecola001/LD-Dt/internal/domain"
	"github.com/junglecola001/LD-Dt/internal/session"
)

// NotificationBookmarkGateway abstracts server notification & bookmark APIs.
type NotificationBookmarkGateway interface {
	MarkNotificationRead(ctx context.Context, notificationID int64) error
	MarkAllNotificationsRead(ctx context.Context) error
	FetchBookmarks(ctx context.Context, page int) ([]domain.Bookmark, error)
	DeleteBookmark(ctx context.Context, bookmarkID int64) error
}

// NotificationService separates live unread counts from stale bootstrap snapshots
// and prevents failed notification navigation from advancing topic read cursors (B11, B12).
type NotificationService struct {
	mu           sync.Mutex
	sessions     *session.Manager
	gateway      NotificationBookmarkGateway
	clock        domain.Clock
	snapshots    map[string]domain.NotificationSnapshot
	bookmarks    map[string][]domain.Bookmark
	bookmarkSync map[string]domain.BookmarkSyncState
}

// NewNotificationService constructs NotificationService.
func NewNotificationService(sessions *session.Manager, gateway NotificationBookmarkGateway, clock domain.Clock) *NotificationService {
	if clock == nil {
		clock = domain.SystemClock{}
	}
	return &NotificationService{
		sessions:     sessions,
		gateway:      gateway,
		clock:        clock,
		snapshots:    make(map[string]domain.NotificationSnapshot),
		bookmarks:    make(map[string][]domain.Bookmark),
		bookmarkSync: make(map[string]domain.BookmarkSyncState),
	}
}

// ApplyBootstrapCount never overwrites a newer live unread count (B11).
func (s *NotificationService) ApplyBootstrapCount(count int, version int64) domain.NotificationSnapshot {
	scope := s.sessions.Current().AccountScope()
	s.mu.Lock()
	defer s.mu.Unlock()
	snap := s.snapshots[scope]
	if version <= snap.CountVersion {
		return snap
	}
	snap.UnreadCount = count
	snap.CountVersion = version
	snap.LastReconciledAt = s.clock.Now()
	s.snapshots[scope] = snap
	return snap
}

// UpdateLiveCount applies real-time unread increments/updates.
func (s *NotificationService) UpdateLiveCount(count int, version int64) domain.NotificationSnapshot {
	return s.ApplyBootstrapCount(count, version)
}

// SetNotifications stores the current reconciled list without regressing live unread count.
func (s *NotificationService) SetNotifications(items []domain.Notification) domain.NotificationSnapshot {
	scope := s.sessions.Current().AccountScope()
	s.mu.Lock()
	defer s.mu.Unlock()
	snap := s.snapshots[scope]
	snap.Notifications = append([]domain.Notification(nil), items...)
	snap.LastReconciledAt = s.clock.Now()
	s.snapshots[scope] = snap
	return snap
}

// ConfirmNotificationVisited marks a notification read ONLY after target topic navigation succeeds (B11).
func (s *NotificationService) ConfirmNotificationVisited(ctx context.Context, notificationID int64, navigationSucceeded bool) (domain.NotificationSnapshot, error) {
	scope := s.sessions.Current().AccountScope()
	if !navigationSucceeded {
		s.mu.Lock()
		snap := s.snapshots[scope]
		s.mu.Unlock()
		return snap, domain.NewError(domain.ErrCodeForbidden, "无法打开目标楼层，保留未读通知以便重试", true, nil)
	}
	if s.gateway != nil {
		if err := s.gateway.MarkNotificationRead(ctx, notificationID); err != nil {
			s.mu.Lock()
			snap := s.snapshots[scope]
			s.mu.Unlock()
			return snap, err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	snap := s.snapshots[scope]
	for i := range snap.Notifications {
		if snap.Notifications[i].ID == notificationID && !snap.Notifications[i].Read {
			snap.Notifications[i].Read = true
			if snap.UnreadCount > 0 {
				snap.UnreadCount--
			}
			snap.CountVersion++
		}
	}
	s.snapshots[scope] = snap
	return snap, nil
}

// SyncBookmarks separates sync status from empty list state and preserves existing bookmarks on error (B12).
func (s *NotificationService) SyncBookmarks(ctx context.Context) ([]domain.Bookmark, domain.BookmarkSyncState, error) {
	scope := s.sessions.Current().AccountScope()
	s.mu.Lock()
	state := s.bookmarkSync[scope]
	state.Syncing = true
	state.LastError = ""
	s.bookmarkSync[scope] = state
	existing := append([]domain.Bookmark(nil), s.bookmarks[scope]...)
	s.mu.Unlock()

	if s.gateway == nil {
		return existing, state, nil
	}

	items, err := s.gateway.FetchBookmarks(ctx, 0)
	s.mu.Lock()
	defer s.mu.Unlock()
	state = s.bookmarkSync[scope]
	state.Syncing = false
	if err != nil {
		state.LastError = err.Error()
		s.bookmarkSync[scope] = state
		// Never wipe existing bookmarks on sync failure (B12).
		return append([]domain.Bookmark(nil), s.bookmarks[scope]...), state, err
	}
	state.HasSynced = true
	state.LastSyncedAt = s.clock.Now()
	s.bookmarkSync[scope] = state
	s.bookmarks[scope] = append([]domain.Bookmark(nil), items...)
	return append([]domain.Bookmark(nil), items...), state, nil
}

// DeleteBookmark removes a bookmark locally only after server confirmation (plan.md 3.10, B12).
func (s *NotificationService) DeleteBookmark(ctx context.Context, bookmarkID int64) ([]domain.Bookmark, error) {
	scope := s.sessions.Current().AccountScope()
	if s.gateway != nil {
		if err := s.gateway.DeleteBookmark(ctx, bookmarkID); err != nil {
			s.mu.Lock()
			existing := append([]domain.Bookmark(nil), s.bookmarks[scope]...)
			s.mu.Unlock()
			return existing, err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cur := s.bookmarks[scope]
	filtered := make([]domain.Bookmark, 0, len(cur))
	for _, b := range cur {
		if b.ID != bookmarkID {
			filtered = append(filtered, b)
		}
	}
	s.bookmarks[scope] = filtered
	return append([]domain.Bookmark(nil), filtered...), nil
}

// BookmarkState returns current cached bookmarks and sync metadata.
func (s *NotificationService) BookmarkState() ([]domain.Bookmark, domain.BookmarkSyncState) {
	scope := s.sessions.Current().AccountScope()
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]domain.Bookmark(nil), s.bookmarks[scope]...), s.bookmarkSync[scope]
}

// Touch ensures time import remains intentional if clock snapshots expand.
var _ = time.Time{}
