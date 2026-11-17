package domain

import "time"

// SessionStatus represents the verified connection state of a site session.
type SessionStatus string

const (
	SessionAnonymous  SessionStatus = "anonymous"
	SessionVerifying  SessionStatus = "verifying"
	SessionConnected  SessionStatus = "connected"
	SessionReauthNeeded SessionStatus = "reauth_needed"
)

// NetworkPhase represents the shared per-site coordinator state.
type NetworkPhase string

const (
	NetworkReady             NetworkPhase = "ready"
	NetworkCoolingDown       NetworkPhase = "cooling_down"
	NetworkChallengeRequired NetworkPhase = "challenge_required"
	NetworkChallengePaused   NetworkPhase = "challenge_paused"
	NetworkAuthExpired       NetworkPhase = "auth_expired"
	NetworkForbidden         NetworkPhase = "forbidden"
	NetworkOffline           NetworkPhase = "offline"
)

// ComposerMode enumerates all supported write modes (plan.md 3.8).
type ComposerMode string

const (
	ComposerNewTopic   ComposerMode = "newTopic"
	ComposerReplyTopic ComposerMode = "replyTopic"
	ComposerReplyPost  ComposerMode = "replyPost"
	ComposerEditPost   ComposerMode = "editPost"
)

// LocalDraftState tracks local SQLite persistence separate from cloud sync.
type LocalDraftState string

const (
	LocalDraftRestoring  LocalDraftState = "restoring"
	LocalDraftEditing    LocalDraftState = "editing"
	LocalDraftSaving     LocalDraftState = "saving"
	LocalDraftSaved      LocalDraftState = "saved"
	LocalDraftSaveFailed LocalDraftState = "saveFailed"
)

// CloudDraftState tracks optional Discourse server draft synchronization.
type CloudDraftState string

const (
	CloudDraftDisabled CloudDraftState = "disabled"
	CloudDraftPending  CloudDraftState = "pending"
	CloudDraftSyncing  CloudDraftState = "syncing"
	CloudDraftSynced   CloudDraftState = "synced"
	CloudDraftConflict CloudDraftState = "conflict"
	CloudDraftFailed   CloudDraftState = "failed"
)

// SubmitState tracks post/topic submission independently from draft saving.
type SubmitState string

const (
	SubmitIdle           SubmitState = "idle"
	SubmitPreparing      SubmitState = "preparing"
	SubmitSubmitting     SubmitState = "submitting"
	SubmitPublished      SubmitState = "published"
	SubmitEnqueued       SubmitState = "enqueued"
	SubmitRejected       SubmitState = "rejected"
	SubmitOutcomeUnknown SubmitState = "outcomeUnknown"
)

// UploadState tracks upload completion and markdown insertion as separate steps.
type UploadState string

const (
	UploadQueued    UploadState = "queued"
	UploadUploading UploadState = "uploading"
	UploadUploaded  UploadState = "uploaded"
	UploadInserted  UploadState = "inserted"
	UploadFailed    UploadState = "failed"
	UploadCancelled UploadState = "cancelled"
)

// NotificationLevel defines topic subscription levels in Discourse.
type NotificationLevel int

const (
	NotifyMuted    NotificationLevel = 0
	NotifyNormal   NotificationLevel = 1
	NotifyTracking NotificationLevel = 2
	NotifyWatching NotificationLevel = 3
)

// BridgeCallContext carries request traceability and session isolation metadata.
type BridgeCallContext struct {
	RequestID         string `json:"requestId"`
	SiteID            string `json:"siteId"`
	SessionGeneration int64  `json:"sessionGeneration"`
}

// SiteSession identifies the active account namespace and generation.
type SiteSession struct {
	SiteID            string        `json:"siteId"`
	AccountID         string        `json:"accountId"`
	Username          string        `json:"username"`
	DisplayName       string        `json:"displayName"`
	SessionGeneration int64         `json:"sessionGeneration"`
	Status            SessionStatus `json:"status"`
	UpdatedAt         time.Time     `json:"updatedAt"`
}

// AccountScope returns the canonical namespace isolating anonymous vs authenticated caches.
func (s SiteSession) AccountScope() string {
	if s.AccountID == "" {
		return s.SiteID + ":anon"
	}
	return s.SiteID + ":" + s.AccountID
}

// NetworkStatusSnapshot exposes the single shared network state to the UI status bar.
type NetworkStatusSnapshot struct {
	SiteID            string       `json:"siteId"`
	Phase             NetworkPhase `json:"phase"`
	BlockedUntilUnix  int64        `json:"blockedUntilUnix"`
	RetryAfterSeconds int          `json:"retryAfterSeconds"`
	InFlightCount     int          `json:"inFlightCount"`
	QueuedCount       int          `json:"queuedCount"`
	LastErrorCode     ErrCode      `json:"lastErrorCode,omitempty"`
	Message           string       `json:"message"`
}

// FeedFilter describes the complete identity of a topic list query.
type FeedQuery struct {
	SiteID            string `json:"siteId"`
	AccountScope      string `json:"accountScope"`
	Filter            string `json:"filter"` // latest, unread, top, category
	CategorySlug      string `json:"categorySlug,omitempty"`
	CategoryID        int64  `json:"categoryId,omitempty"`
	Tag               string `json:"tag,omitempty"`
	Order             string `json:"order,omitempty"`
	Period            string `json:"period,omitempty"`
	Page              int    `json:"page"`
	Cursor            string `json:"cursor,omitempty"`
	QueryGeneration   int64  `json:"queryGeneration"`
}

// CacheKey returns a deterministic query key so pagination never leaks across filters/accounts.
func (q FeedQuery) CacheKey() string {
	return q.SiteID + "|" + q.AccountScope + "|" + q.Filter + "|" + q.CategorySlug + "|" + q.Tag + "|" + q.Order + "|" + q.Period
}

// TopicSummary represents a single row in topic discovery lists.
type TopicSummary struct {
	ID                  int64     `json:"id"`
	Title               string    `json:"title"`
	Slug                string    `json:"slug"`
	CategoryID          int64     `json:"categoryId"`
	CategoryName        string    `json:"categoryName"`
	AuthorUsername      string    `json:"authorUsername"`
	PostsCount          int       `json:"postsCount"`
	ReplyCount          int       `json:"replyCount"`
	Views               int       `json:"views"`
	LikeCount           int       `json:"likeCount"`
	UnreadPosts         int       `json:"unreadPosts"`
	NewPosts            int       `json:"newPosts"`
	HighestPostNumber   int       `json:"highestPostNumber"`
	LastReadPostNumber  int       `json:"lastReadPostNumber"`
	Pinned              bool      `json:"pinned"`
	Closed              bool      `json:"closed"`
	Archived            bool      `json:"archived"`
	NotificationLevel   NotificationLevel `json:"notificationLevel"`
	LastPostedAt        time.Time `json:"lastPostedAt"`
}

// FeedPage contains deduplicated topics and cursor metadata.
type FeedPage struct {
	QueryKey        string         `json:"queryKey"`
	QueryGeneration int64          `json:"queryGeneration"`
	Page            int            `json:"page"`
	NextCursor      string         `json:"nextCursor,omitempty"`
	HasMore         bool           `json:"hasMore"`
	Topics          []TopicSummary `json:"topics"`
	FetchedAt       time.Time      `json:"fetchedAt"`
	Stale           bool           `json:"stale"`
}

// Post represents a canonical post entity shared across windows.
type Post struct {
	ID              int64     `json:"id"`
	TopicID         int64     `json:"topicId"`
	PostNumber      int       `json:"postNumber"`
	Username        string    `json:"username"`
	DisplayName     string    `json:"displayName"`
	AvatarTemplate  string    `json:"avatarTemplate"`
	CookedHTML      string    `json:"cookedHtml"`
	RawMarkdown     string    `json:"rawMarkdown,omitempty"`
	Version         int       `json:"version"`
	LikeCount       int       `json:"likeCount"`
	LikedByMe       bool      `json:"likedByMe"`
	Bookmarked      bool      `json:"bookmarked"`
	CanEdit         bool      `json:"canEdit"`
	CanDelete       bool      `json:"canDelete"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// TopicDetail represents canonical topic metadata and a post chunk.
type TopicDetail struct {
	Summary          TopicSummary `json:"summary"`
	CanReply         bool         `json:"canReply"`
	CanCreatePost    bool         `json:"canCreatePost"`
	Posts            []Post       `json:"posts"`
	StreamPostIDs    []int64      `json:"streamPostIds"`
	TargetPostNumber int          `json:"targetPostNumber,omitempty"`
	FetchedAt        time.Time    `json:"fetchedAt"`
	Stale            bool         `json:"stale"`
}

// ReadAnchor stores window-local reading position without relying on fragile pixel scrollTop alone.
type ReadAnchor struct {
	InstanceID   string  `json:"instanceId"`
	SiteID       string  `json:"siteId"`
	AccountScope string  `json:"accountScope"`
	TopicID      int64   `json:"topicId"`
	PostNumber   int     `json:"postNumber"`
	PostID       int64   `json:"postId"`
	OffsetRatio  float64 `json:"offsetRatio"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// SearchQuery isolates input/cursor per window instance while sharing results by canonical query.
type SearchQuery struct {
	SiteID          string `json:"siteId"`
	AccountScope    string `json:"accountScope"`
	InstanceID      string `json:"instanceId"`
	TopicID         int64  `json:"topicId,omitempty"`
	Keyword         string `json:"keyword"`
	Page            int    `json:"page"`
	QueryGeneration int64  `json:"queryGeneration"`
}

// SearchHit represents a single search result pointing to a topic and optional post number.
type SearchHit struct {
	TopicID    int64  `json:"topicId"`
	PostID     int64  `json:"postId"`
	PostNumber int    `json:"postNumber"`
	Title      string `json:"title"`
	Excerpt    string `json:"excerpt"`
	Author     string `json:"author"`
	Category   string `json:"category"`
}

// SearchResult holds paged search hits tagged with the winning query generation.
type SearchResult struct {
	InstanceID      string      `json:"instanceId"`
	QueryGeneration int64       `json:"queryGeneration"`
	Page            int         `json:"page"`
	HasMore         bool        `json:"hasMore"`
	Hits            []SearchHit `json:"hits"`
}

// UploadTask separates upload completion from markdown insertion.
type UploadTask struct {
	UploadID       string      `json:"uploadId"`
	DraftID        string      `json:"draftId"`
	AccountScope   string      `json:"accountScope"`
	FileName       string      `json:"fileName"`
	MimeType       string      `json:"mimeType"`
	SizeBytes      int64       `json:"sizeBytes"`
	Generation     int64       `json:"generation"`
	EditorRevision int64       `json:"editorRevision"`
	InsertAnchor   int         `json:"insertAnchor"`
	State          UploadState `json:"state"`
	ShortURL       string      `json:"shortUrl,omitempty"`
	ResolvedURL    string      `json:"resolvedUrl,omitempty"`
	ErrorMessage   string      `json:"errorMessage,omitempty"`
}

// Draft preserves all four composer modes locally and tracks optional server sync.
type Draft struct {
	DraftID         string          `json:"draftId"`
	SiteID          string          `json:"siteId"`
	AccountScope    string          `json:"accountScope"`
	Mode            ComposerMode    `json:"mode"`
	TopicID         int64           `json:"topicId,omitempty"`
	ReplyToPostNum  int             `json:"replyToPostNumber,omitempty"`
	EditPostID      int64           `json:"editPostId,omitempty"`
	InitialRaw      string          `json:"initialRaw,omitempty"`
	EditReason      string          `json:"editReason,omitempty"`
	BasePostVersion int             `json:"basePostVersion,omitempty"`
	Title           string          `json:"title,omitempty"`
	CategoryID      int64           `json:"categoryId,omitempty"`
	Tags            []string        `json:"tags,omitempty"`
	Body            string          `json:"body"`
	Revision        int64           `json:"revision"`
	LocalState      LocalDraftState `json:"localState"`
	CloudState      CloudDraftState `json:"cloudState"`
	ServerDraftKey  string          `json:"serverDraftKey,omitempty"`
	ServerSequence  int64           `json:"serverSequence,omitempty"`
	ConflictBody    string          `json:"conflictBody,omitempty"`
	Uploads         []UploadTask    `json:"uploads,omitempty"`
	UpdatedAt       time.Time       `json:"updatedAt"`
}

// SubmitOperation records an immutable submission snapshot and outcome receipt.
type SubmitOperation struct {
	OperationID      string      `json:"operationId"`
	DraftID          string      `json:"draftId"`
	AccountScope     string      `json:"accountScope"`
	Mode             ComposerMode `json:"mode"`
	SnapshotRevision int64       `json:"snapshotRevision"`
	SnapshotBody     string      `json:"snapshotBody"`
	SnapshotTitle    string      `json:"snapshotTitle,omitempty"`
	State            SubmitState `json:"state"`
	CreatedTopicID   int64       `json:"createdTopicId,omitempty"`
	CreatedPostID    int64       `json:"createdPostId,omitempty"`
	CreatedPostNum   int         `json:"createdPostNumber,omitempty"`
	ErrorCode        ErrCode     `json:"errorCode,omitempty"`
	ErrorMessage     string      `json:"errorMessage,omitempty"`
	UpdatedAt        time.Time   `json:"updatedAt"`
}

// Notification represents a forum notification separated from topic read state.
type Notification struct {
	ID               int64     `json:"id"`
	SiteID           string    `json:"siteId"`
	AccountScope     string    `json:"accountScope"`
	NotificationType int       `json:"notificationType"`
	Read             bool      `json:"read"`
	TopicID          int64     `json:"topicId"`
	PostNumber       int       `json:"postNumber"`
	TopicTitle       string    `json:"topicTitle"`
	ActorUsername    string    `json:"actorUsername"`
	Excerpt          string    `json:"excerpt"`
	CreatedAt        time.Time `json:"createdAt"`
}

// NotificationSnapshot separates live unread counters from list reconciliation.
type NotificationSnapshot struct {
	UnreadCount      int            `json:"unreadCount"`
	CountVersion     int64          `json:"countVersion"`
	Notifications    []Notification `json:"notifications"`
	LastReconciledAt time.Time      `json:"lastReconciledAt"`
}

// Bookmark represents a server-confirmed user bookmark.
type Bookmark struct {
	ID           int64     `json:"id"`
	SiteID       string    `json:"siteId"`
	AccountScope string    `json:"accountScope"`
	TopicID      int64     `json:"topicId"`
	PostID       int64     `json:"postId,omitempty"`
	PostNumber   int       `json:"postNumber,omitempty"`
	Name         string    `json:"name"`
	TopicTitle   string    `json:"topicTitle"`
	ReminderAt   string    `json:"reminderAt,omitempty"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// BookmarkSyncState keeps sync progress separate from empty-list presentation.
type BookmarkSyncState struct {
	Syncing      bool      `json:"syncing"`
	HasSynced    bool      `json:"hasSynced"`
	LastError    string    `json:"lastError,omitempty"`
	LastSyncedAt time.Time `json:"lastSyncedAt"`
}

// WindowBounds stores logical CSS pixel bounds for an internal workspace window.
type WindowBounds struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// WindowState models an internal DAW workspace panel without storing draft bodies or secrets.
type WindowState struct {
	InstanceID    string       `json:"instanceId"`
	PanelType     string       `json:"panelType"`
	Title         string       `json:"title"`
	ResourceKey   string       `json:"resourceKey"`
	DisplayMode   string       `json:"displayMode"` // floating | docked
	ViewState     string       `json:"viewState"`   // normal | minimized | maximized
	DockRegion    string       `json:"dockRegion,omitempty"`
	DockGroupID   string       `json:"dockGroupId,omitempty"`
	Bounds        WindowBounds `json:"bounds"`
	RestoreBounds WindowBounds `json:"restoreBounds"`
	ZIndex        int          `json:"zIndex"`
	Pinned        bool         `json:"pinned"`
	DraftID       string       `json:"draftId,omitempty"`
	TopicID       int64        `json:"topicId,omitempty"`
	UnreadBadge   int          `json:"unreadBadge,omitempty"`
}

// WorkspaceLayout persists a named layout safely without credentials or post bodies.
type WorkspaceLayout struct {
	SchemaVersion  int           `json:"schemaVersion"`
	LayoutID       string        `json:"layoutId"`
	Name           string        `json:"name"`
	PresetID       string        `json:"presetId"`
	AccountScope   string        `json:"accountScope"`
	ActiveInstance string        `json:"activeInstanceId"`
	Windows        []WindowState `json:"windows"`
	UpdatedAt      time.Time     `json:"updatedAt"`
}
