package storage

import (
	"context"
	"testing"
	"time"

	"github.com/junglecola001/LD-Dt/internal/domain"
)

func TestSQLiteStoreMigrationDraftIsolationAndCacheClear(t *testing.T) {
	store, err := OpenSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLiteStore failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)

	_, err = store.SaveDraft(ctx, domain.Draft{
		DraftID:      "d1",
		AccountScope: "linux.do:alice",
		Mode:         domain.ComposerEditPost,
		EditPostID:   99,
		Body:         "v2",
		Revision:     2,
		LocalState:   domain.LocalDraftSaved,
		UpdatedAt:    now,
	})
	if err != nil {
		t.Fatalf("SaveDraft failed: %v", err)
	}

	// Older revision must not overwrite newer draft.
	res, err := store.SaveDraft(ctx, domain.Draft{
		DraftID:      "d1",
		AccountScope: "linux.do:alice",
		Mode:         domain.ComposerEditPost,
		Body:         "v1-stale",
		Revision:     1,
		UpdatedAt:    now,
	})
	if err != nil || res.Body != "v2" || res.Revision != 2 {
		t.Fatalf("stale revision overwrote SQLite draft: %+v err=%v", res, err)
	}

	// Different account scope cannot read alice's draft.
	if _, ok, err := store.GetDraft(ctx, "linux.do:bob", "d1"); err != nil || ok {
		t.Fatalf("expected account isolation, got ok=%v err=%v", ok, err)
	}

	// Save layout and cached topic, then clear content cache.
	if err := store.SaveLayout(ctx, domain.WorkspaceLayout{
		SchemaVersion: 1,
		LayoutID:      "default",
		Name:          "浏览",
		AccountScope:  "linux.do:alice",
		UpdatedAt:     now,
	}); err != nil {
		t.Fatalf("SaveLayout failed: %v", err)
	}
	if err := store.SaveTopicCached(ctx, "linux.do:alice", domain.TopicSummary{ID: 55, Title: "Cached"}, now); err != nil {
		t.Fatalf("SaveTopicCached failed: %v", err)
	}

	if err := store.ClearContentCache(ctx, "linux.do:alice"); err != nil {
		t.Fatalf("ClearContentCache failed: %v", err)
	}

	var count int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM topics WHERE account_scope = ?`, "linux.do:alice").Scan(&count); err != nil || count != 0 {
		t.Fatalf("expected cached topics cleared, got count=%d err=%v", count, err)
	}
	if _, ok, err := store.GetDraft(ctx, "linux.do:alice", "d1"); err != nil || !ok {
		t.Fatalf("draft must survive ClearContentCache: ok=%v err=%v", ok, err)
	}
	if _, ok, err := store.GetLayout(ctx, "linux.do:alice", "default"); err != nil || !ok {
		t.Fatalf("layout must survive ClearContentCache: ok=%v err=%v", ok, err)
	}
}
