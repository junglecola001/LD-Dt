package app

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/junglecola001/LD-Dt/internal/domain"
	"github.com/junglecola001/LD-Dt/internal/session"
	"github.com/junglecola001/LD-Dt/internal/storage"
)

type stubClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *stubClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *stubClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func (c *stubClock) After(d time.Duration) <-chan time.Time {
	ch := make(chan time.Time, 1)
	c.Advance(d)
	ch <- c.Now()
	return ch
}

type stubFeedFetcher struct {
	fetchFeedFn  func(ctx context.Context, query domain.FeedQuery) (domain.FeedPage, error)
	fetchTopicFn func(ctx context.Context, siteID string, topicID int64, around int) (domain.TopicDetail, error)
}

func (f *stubFeedFetcher) FetchFeed(ctx context.Context, q domain.FeedQuery) (domain.FeedPage, error) {
	return f.fetchFeedFn(ctx, q)
}

func (f *stubFeedFetcher) FetchTopic(ctx context.Context, siteID string, topicID int64, around int) (domain.TopicDetail, error) {
	return f.fetchTopicFn(ctx, siteID, topicID, around)
}

type stubSearchFetcher struct {
	fn func(ctx context.Context, q domain.SearchQuery) (domain.SearchResult, error)
}

func (s *stubSearchFetcher) Search(ctx context.Context, q domain.SearchQuery) (domain.SearchResult, error) {
	return s.fn(ctx, q)
}

type stubComposerGateway struct {
	syncFn   func(ctx context.Context, d domain.Draft) (int64, string, bool, error)
	submitFn func(ctx context.Context, op domain.SubmitOperation, d domain.Draft) (domain.SubmitOperation, error)
	uploadFn func(ctx context.Context, t domain.UploadTask, data []byte) (string, string, error)
}

func (g *stubComposerGateway) SyncServerDraft(ctx context.Context, d domain.Draft) (int64, string, bool, error) {
	return g.syncFn(ctx, d)
}

func (g *stubComposerGateway) SubmitSnapshot(ctx context.Context, op domain.SubmitOperation, d domain.Draft) (domain.SubmitOperation, error) {
	return g.submitFn(ctx, op, d)
}

func (g *stubComposerGateway) UploadFile(ctx context.Context, t domain.UploadTask, data []byte) (string, string, error) {
	return g.uploadFn(ctx, t, data)
}

type stubNotifGateway struct {
	markReadErr  error
	bookmarksErr error
	deleteErr    error
	bookmarks    []domain.Bookmark
}

func (g *stubNotifGateway) MarkNotificationRead(context.Context, int64) error { return g.markReadErr }
func (g *stubNotifGateway) MarkAllNotificationsRead(context.Context) error    { return nil }
func (g *stubNotifGateway) FetchBookmarks(context.Context, int) ([]domain.Bookmark, error) {
	if g.bookmarksErr != nil {
		return nil, g.bookmarksErr
	}
	return g.bookmarks, nil
}
func (g *stubNotifGateway) DeleteBookmark(context.Context, int64) error { return g.deleteErr }

func TestB01FeedGenerationAndPaginationDedup(t *testing.T) {
	clk := &stubClock{now: time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)}
	sess := session.NewManager("linux.do", clk)
	store := storage.NewMemoryStore()

	releaseA := make(chan struct{})
	failBPage1 := true
	fetcher := &stubFeedFetcher{
		fetchFeedFn: func(ctx context.Context, q domain.FeedQuery) (domain.FeedPage, error) {
			if q.Filter == "latest" {
				<-releaseA
				return domain.FeedPage{Topics: []domain.TopicSummary{{ID: 1, Title: "A"}}}, nil
			}
			if q.Page == 0 {
				return domain.FeedPage{Page: 0, HasMore: true, Topics: []domain.TopicSummary{{ID: 10, Title: "B0"}}}, nil
			}
			if failBPage1 {
				failBPage1 = false
				return domain.FeedPage{}, domain.NewError(domain.ErrCodeNetworkOffline, "page 1 failed", true, nil)
			}
			// Includes duplicate ID 10 + new ID 11.
			return domain.FeedPage{Page: 1, Topics: []domain.TopicSummary{{ID: 10, Title: "B0"}, {ID: 11, Title: "B1"}}}, nil
		},
	}

	svc := NewTopicService(sess, store, fetcher, clk, nil)
	errCh := make(chan error, 1)
	go func() {
		_, err := svc.LoadFeed(context.Background(), "win-1", domain.FeedQuery{Filter: "latest", QueryGeneration: 1})
		errCh <- err
	}()

	// Switch to filter B (gen 2) before A returns.
	page0, err := svc.LoadFeed(context.Background(), "win-1", domain.FeedQuery{Filter: "top", Page: 0, QueryGeneration: 2})
	if err != nil || len(page0.Topics) != 1 || page0.Topics[0].ID != 10 {
		t.Fatalf("unexpected page 0: %+v err=%v", page0, err)
	}

	close(releaseA)
	if err := <-errCh; err == nil {
		t.Fatalf("expected slow query A to be discarded")
	}

	// Next page fails once, then retry succeeds without losing B0 or duplicating ID 10.
	if _, err := svc.LoadFeed(context.Background(), "win-1", domain.FeedQuery{Filter: "top", Page: 1, QueryGeneration: 2}); err == nil {
		t.Fatalf("expected initial page 1 failure")
	}
	page1, err := svc.LoadFeed(context.Background(), "win-1", domain.FeedQuery{Filter: "top", Page: 1, QueryGeneration: 2})
	if err != nil || len(page1.Topics) != 2 || page1.Topics[0].ID != 10 || page1.Topics[1].ID != 11 {
		t.Fatalf("expected deduplicated B0+B1 after retry, got %+v err=%v", page1, err)
	}
}

func TestB02AndB03SharedTopicIndependentAnchorsAndReadTracker(t *testing.T) {
	clk := &stubClock{now: time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)}
	sess := session.NewManager("linux.do", clk)
	store := storage.NewMemoryStore()

	fetcher := &stubFeedFetcher{
		fetchTopicFn: func(ctx context.Context, siteID string, topicID int64, around int) (domain.TopicDetail, error) {
			return domain.TopicDetail{
				Summary: domain.TopicSummary{ID: topicID, Title: "Topic"},
				Posts: []domain.Post{
					{ID: 101, TopicID: topicID, PostNumber: 1},
					{ID: 155, TopicID: topicID, PostNumber: 55},
				},
			}, nil
		},
	}

	svc := NewTopicService(sess, store, fetcher, clk, nil)
	_, anchor1, err := svc.OpenTopic(context.Background(), "win-A", 42, 0)
	if err != nil || anchor1.PostNumber != 1 {
		t.Fatalf("unexpected win-A anchor: %+v err=%v", anchor1, err)
	}

	// Notification jumps win-B to uncached post 55; win-A anchor remains untouched.
	_, anchor2, err := svc.OpenTopic(context.Background(), "win-B", 42, 55)
	if err != nil || anchor2.PostNumber != 55 {
		t.Fatalf("unexpected win-B anchor: %+v err=%v", anchor2, err)
	}
	savedA, ok, _ := store.GetReadAnchor(context.Background(), sess.Current().AccountScope(), "win-A", 42)
	if ok && savedA.PostNumber == 55 {
		t.Fatalf("win-B jump must not overwrite win-A anchor")
	}

	// B03: ReadTracker pauses when minimized, modal-blocked, or blurred.
	svc.UpdateReadVisibility("win-B", 42, true, false, false)
	clk.Advance(10 * time.Second)
	svc.UpdateReadVisibility("win-B", 42, true, true, false) // minimized
	clk.Advance(30 * time.Second)
	svc.UpdateReadVisibility("win-B", 42, false, false, false) // blurred
	clk.Advance(20 * time.Second)
	svc.UpdateReadVisibility("win-B", 42, true, false, true) // modal blocked
	clk.Advance(15 * time.Second)

	tracked := svc.FlushTopicReadTime(42)
	if tracked != 10*time.Second {
		t.Fatalf("expected exactly 10s active read time, got %v", tracked)
	}
}

func TestB04SearchGenerationIsolation(t *testing.T) {
	sess := session.NewManager("linux.do", &stubClock{now: time.Now()})
	releaseQ1 := make(chan struct{})
	svc := NewSearchService(sess, &stubSearchFetcher{
		fn: func(ctx context.Context, q domain.SearchQuery) (domain.SearchResult, error) {
			if q.Keyword == "q1" {
				<-releaseQ1
			}
			return domain.SearchResult{Hits: []domain.SearchHit{{TopicID: q.QueryGeneration, Title: q.Keyword}}}, nil
		},
	})

	errCh := make(chan error, 1)
	go func() {
		_, err := svc.Execute(context.Background(), domain.SearchQuery{InstanceID: "search-1", Keyword: "q1", QueryGeneration: 1})
		errCh <- err
	}()

	res2, err := svc.Execute(context.Background(), domain.SearchQuery{InstanceID: "search-1", Keyword: "q2", QueryGeneration: 2})
	if err != nil || len(res2.Hits) != 1 || res2.Hits[0].Title != "q2" {
		t.Fatalf("expected q2 result, got %+v err=%v", res2, err)
	}

	close(releaseQ1)
	if err := <-errCh; err == nil {
		t.Fatalf("expected stale q1 response to be rejected")
	}
}

func TestB05ToB09ComposerDraftConflictSubmitAndUpload(t *testing.T) {
	clk := &stubClock{now: time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)}
	sess := session.NewManager("linux.do", clk)
	sess.ConfirmAccount("acct-1", "alice", "Alice")
	store := storage.NewMemoryStore()

	gw := &stubComposerGateway{
		syncFn: func(ctx context.Context, d domain.Draft) (int64, string, bool, error) {
			return 2, "remote conflict body", true, nil
		},
		uploadFn: func(ctx context.Context, task domain.UploadTask, data []byte) (string, string, error) {
			return "upload://short123", "https://linux.do/uploads/123.png", nil
		},
		submitFn: func(ctx context.Context, op domain.SubmitOperation, d domain.Draft) (domain.SubmitOperation, error) {
			switch d.Body {
			case "enqueue-me":
				op.State = domain.SubmitEnqueued
				return op, nil
			case "timeout-me":
				return op, domain.NewError(domain.ErrCodeOutcomeUnknown, "timeout after write", true, nil)
			default:
				op.State = domain.SubmitPublished
				op.CreatedPostID = 999
				return op, nil
			}
		},
	}

	svc := NewComposerService(sess, store, gw, clk, nil)

	// B05: editPost local recovery + stale revision callback rejected.
	saved2, err := svc.SaveLocalDraft(context.Background(), domain.Draft{
		DraftID:    "edit-77",
		Mode:       domain.ComposerEditPost,
		EditPostID: 77,
		InitialRaw: "original",
		Body:       "newer local edit",
		Revision:   2,
	})
	if err != nil || saved2.Revision != 2 || saved2.LocalState != domain.LocalDraftSaved {
		t.Fatalf("unexpected saved draft: %+v err=%v", saved2, err)
	}
	staleRes, err := svc.SaveLocalDraft(context.Background(), domain.Draft{
		DraftID:  "edit-77",
		Mode:     domain.ComposerEditPost,
		Body:     "older edit",
		Revision: 1,
	})
	if err != nil || staleRes.Body != "newer local edit" || staleRes.Revision != 2 {
		t.Fatalf("stale save overwrote newer revision: %+v", staleRes)
	}

	// B06: Cloud conflict preserves both local body and remote conflictBody; disk error fails cleanly.
	conflicted, err := svc.SyncCloudDraft(context.Background(), "edit-77")
	if err != nil || conflicted.CloudState != domain.CloudDraftConflict || conflicted.Body != "newer local edit" || conflicted.ConflictBody != "remote conflict body" {
		t.Fatalf("expected dual-copy conflict preservation, got %+v err=%v", conflicted, err)
	}

	store.FailNextDraftSave = true
	failedSave, err := svc.SaveLocalDraft(context.Background(), domain.Draft{
		DraftID:  "edit-77",
		Mode:     domain.ComposerEditPost,
		Body:     "unsaved change",
		Revision: 3,
	})
	if err == nil || failedSave.LocalState != domain.LocalDraftSaveFailed {
		t.Fatalf("expected save failure state, got %+v err=%v", failedSave, err)
	}

	// B07: Confirm revision mismatch blocks stale snapshot submit.
	if _, err := svc.SubmitDraft(context.Background(), "edit-77", 99); err == nil {
		t.Fatalf("expected revision mismatch conflict error")
	}

	// B09: Upload completed but not yet inserted blocks submit until ConfirmUploadInserted.
	upTask, err := svc.StartUpload(context.Background(), "reply-1", domain.UploadTask{
		UploadID:   "u1",
		FileName:   "img.png",
		Generation: 1,
	}, []byte("png"))
	if err != nil || upTask.State != domain.UploadUploaded || upTask.ShortURL != "upload://short123" {
		t.Fatalf("unexpected upload result: %+v err=%v", upTask, err)
	}
	_, _ = svc.SaveLocalDraft(context.Background(), domain.Draft{
		DraftID:  "reply-1",
		Mode:     domain.ComposerReplyTopic,
		TopicID:  10,
		Body:     "enqueue-me",
		Revision: 1,
		Uploads:  []domain.UploadTask{upTask},
	})
	if _, err := svc.SubmitDraft(context.Background(), "reply-1", 1); err == nil {
		t.Fatalf("expected uninserted upload to block publish")
	}
	if _, err := svc.ConfirmUploadInserted(context.Background(), "reply-1", "u1"); err != nil {
		t.Fatalf("confirm upload inserted failed: %v", err)
	}

	// B08: Enqueued vs outcomeUnknown states.
	enqOp, err := svc.SubmitDraft(context.Background(), "reply-1", 1)
	if err != nil || enqOp.State != domain.SubmitEnqueued {
		t.Fatalf("expected SubmitEnqueued, got %+v err=%v", enqOp, err)
	}

	_, _ = svc.SaveLocalDraft(context.Background(), domain.Draft{
		DraftID:  "reply-timeout",
		Mode:     domain.ComposerReplyTopic,
		TopicID:  10,
		Body:     "timeout-me",
		Revision: 1,
	})
	unkOp, err := svc.SubmitDraft(context.Background(), "reply-timeout", 1)
	if err == nil || unkOp.State != domain.SubmitOutcomeUnknown {
		t.Fatalf("expected SubmitOutcomeUnknown on timeout, got %+v err=%v", unkOp, err)
	}
}

func TestB10AccountIsolationAcrossSwitchAndLogout(t *testing.T) {
	clk := &stubClock{now: time.Now()}
	sess := session.NewManager("linux.do", clk)
	store := storage.NewMemoryStore()
	auth := NewAuthService(sess, nil, store, nil)
	comp := NewComposerService(sess, store, nil, clk, nil)

	sess.ConfirmAccount("acct-a", "alice", "Alice")
	_, err := comp.SaveLocalDraft(context.Background(), domain.Draft{
		DraftID:  "a-draft",
		Mode:     domain.ComposerNewTopic,
		Title:    "Alice Draft",
		Body:     "Secret A",
		Revision: 1,
	})
	if err != nil {
		t.Fatalf("save A draft failed: %v", err)
	}

	// Switch to B: A's drafts must not be visible.
	sess.ConfirmAccount("acct-b", "bob", "Bob")
	bDrafts, _ := comp.ListDrafts(context.Background())
	if len(bDrafts) != 0 {
		t.Fatalf("account B saw account A drafts: %+v", bDrafts)
	}

	// Logout keeps protected drafts; re-login A restores draft without auto-submitting.
	_, _ = auth.Logout(context.Background(), false)
	sess.ConfirmAccount("acct-a", "alice", "Alice")
	aDrafts, _ := comp.ListDrafts(context.Background())
	if len(aDrafts) != 1 || aDrafts[0].Body != "Secret A" {
		t.Fatalf("expected account A draft preserved after logout+relogin, got %+v", aDrafts)
	}

	// Explicit clearAllData deletes drafts on logout.
	_, _ = auth.Logout(context.Background(), true)
	sess.ConfirmAccount("acct-a", "alice", "Alice")
	aDraftsAfterClear, _ := comp.ListDrafts(context.Background())
	if len(aDraftsAfterClear) != 0 {
		t.Fatalf("expected drafts deleted after clearAllData logout")
	}
}

func TestB11AndB12NotificationsBookmarksAndCacheClearSafety(t *testing.T) {
	clk := &stubClock{now: time.Now()}
	sess := session.NewManager("linux.do", clk)
	sess.ConfirmAccount("acct-1", "alice", "Alice")
	store := storage.NewMemoryStore()

	gw := &stubNotifGateway{
		bookmarks: []domain.Bookmark{{ID: 1, TopicID: 100, Name: "Saved"}},
	}
	ns := NewNotificationService(sess, gw, clk)

	// B11: live count version 5 must not be overwritten by older bootstrap version 2.
	ns.UpdateLiveCount(7, 5)
	snap := ns.ApplyBootstrapCount(1, 2)
	if snap.UnreadCount != 7 || snap.CountVersion != 5 {
		t.Fatalf("stale bootstrap regressed unread count: %+v", snap)
	}

	// Failed navigation leaves notification unread.
	ns.SetNotifications([]domain.Notification{{ID: 50, TopicID: 999, PostNumber: 3, Read: false}})
	if _, err := ns.ConfirmNotificationVisited(context.Background(), 50, false); err == nil {
		t.Fatalf("expected error when navigation failed")
	}
	snapAfterFail := ns.ApplyBootstrapCount(0, 1)
	if snapAfterFail.UnreadCount != 7 {
		t.Fatalf("unread count changed on failed navigation")
	}

	// B12: bookmark sync failure preserves existing bookmarks; cache clear preserves drafts & layouts.
	items, state, err := ns.SyncBookmarks(context.Background())
	if err != nil || len(items) != 1 || !state.HasSynced {
		t.Fatalf("initial bookmark sync failed: %+v %+v %v", items, state, err)
	}
	gw.bookmarksErr = errors.New("503")
	itemsAfterFail, stateAfterFail, err := ns.SyncBookmarks(context.Background())
	if err == nil || len(itemsAfterFail) != 1 || stateAfterFail.LastError == "" {
		t.Fatalf("expected existing bookmarks preserved on sync error, got %+v %+v %v", itemsAfterFail, stateAfterFail, err)
	}

	// Ordinary cache clear protects drafts & workspace layouts.
	scope := sess.Current().AccountScope()
	_, _ = store.SaveDraft(context.Background(), domain.Draft{DraftID: "keep-me", AccountScope: scope, Body: "safe", Revision: 1})
	_ = store.SaveLayout(context.Background(), DefaultBrowseLayout(scope, clk.Now()))
	_ = store.SaveTopicDetail(context.Background(), scope, domain.TopicDetail{Summary: domain.TopicSummary{ID: 88}})

	if err := store.ClearContentCache(context.Background(), scope); err != nil {
		t.Fatalf("cache clear failed: %v", err)
	}
	if _, ok, _ := store.GetTopicDetail(context.Background(), scope, 88); ok {
		t.Fatalf("expected topic cache to be cleared")
	}
	if _, ok, _ := store.GetDraft(context.Background(), scope, "keep-me"); !ok {
		t.Fatalf("draft must survive content cache clear")
	}
	if _, ok, _ := store.GetLayout(context.Background(), scope, "default"); !ok {
		t.Fatalf("layout must survive content cache clear")
	}
}
