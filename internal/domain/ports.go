package domain

import (
	"context"
	"net/http"
	"time"
)

// Clock abstracts time so request budgets, rate-limit waits, and read timers are deterministic in tests.
type Clock interface {
	Now() time.Time
	After(d time.Duration) <-chan time.Time
}

// SystemClock uses the standard wall clock in production.
type SystemClock struct{}

func (SystemClock) Now() time.Time                         { return time.Now() }
func (SystemClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

// HTTPTransport executes raw HTTP requests for a specific site host.
type HTTPTransport interface {
	Do(ctx context.Context, req *http.Request) (*http.Response, error)
}

// CookieRecord preserves browser cookie attributes across the Go <-> WebView2 boundary.
type CookieRecord struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Domain   string    `json:"domain"`
	Path     string    `json:"path"`
	Expires  time.Time `json:"expires"`
	Secure   bool      `json:"secure"`
	HTTPOnly bool      `json:"httpOnly"`
	SameSite string    `json:"sameSite"` // Lax, Strict, None, Default
}

// SessionCookieStore manages site-isolated cookies with monotonic session generations.
type SessionCookieStore interface {
	Generation() int64
	ApplySyncedCookies(siteID string, generation int64, cookies []CookieRecord) error
	ExportCookies(siteID string) []CookieRecord
	ClearSite(siteID string) error
}

// ChallengeHost opens an isolated, business-binding-free WebView/browser flow for site verification.
type ChallengeHost interface {
	RunChallenge(ctx context.Context, siteID string, generation int64) error
}

// EventEmitter publishes sanitized state deltas to the UI layer.
type EventEmitter interface {
	Emit(eventName string, payload any)
}

// DraftRepository persists all composer drafts and submission receipts independently from layouts.
type DraftRepository interface {
	SaveDraft(ctx context.Context, draft Draft) (Draft, error)
	GetDraft(ctx context.Context, accountScope, draftID string) (Draft, bool, error)
	ListDrafts(ctx context.Context, accountScope string) ([]Draft, error)
	DeleteDraft(ctx context.Context, accountScope, draftID string) error
	SaveSubmitReceipt(ctx context.Context, op SubmitOperation) error
	GetSubmitReceipt(ctx context.Context, accountScope, operationID string) (SubmitOperation, bool, error)
	ListPendingReceipts(ctx context.Context, accountScope string) ([]SubmitOperation, error)
}

// WorkspaceRepository persists named workspace layouts without storing credentials or draft text.
type WorkspaceRepository interface {
	SaveLayout(ctx context.Context, layout WorkspaceLayout) error
	GetLayout(ctx context.Context, accountScope, layoutID string) (WorkspaceLayout, bool, error)
	ListLayouts(ctx context.Context, accountScope string) ([]WorkspaceLayout, error)
	DeleteLayout(ctx context.Context, accountScope, layoutID string) error
}

// CacheRepository manages topic/post/notification/bookmark snapshots per account scope.
type CacheRepository interface {
	SaveTopicDetail(ctx context.Context, accountScope string, detail TopicDetail) error
	GetTopicDetail(ctx context.Context, accountScope string, topicID int64) (TopicDetail, bool, error)
	SaveReadAnchor(ctx context.Context, anchor ReadAnchor) error
	GetReadAnchor(ctx context.Context, accountScope, instanceID string, topicID int64) (ReadAnchor, bool, error)
	ClearContentCache(ctx context.Context, accountScope string) error
}
