package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/junglecola001/LD-Dt/internal/domain"
	_ "modernc.org/sqlite"
)

// SQLiteStore implements durable SQLite persistence using pure-Go modernc.org/sqlite.
type SQLiteStore struct {
	db *sql.DB
}

// OpenSQLiteStore opens a SQLite database and applies the initial migration schema.
func OpenSQLiteStore(dsn string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.Exec(InitialSchemaSQL); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &SQLiteStore{db: db}, nil
}

// Close closes the underlying SQLite connection pool.
func (s *SQLiteStore) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

// SaveDraft persists a composer draft only if revision >= existing stored revision (B05).
func (s *SQLiteStore) SaveDraft(ctx context.Context, draft domain.Draft) (domain.Draft, error) {
	if draft.Revision <= 0 {
		draft.Revision = 1
	}
	if existing, ok, err := s.GetDraft(ctx, draft.AccountScope, draft.DraftID); err == nil && ok && draft.Revision < existing.Revision {
		return existing, nil
	}

	raw, err := json.Marshal(draft)
	if err != nil {
		return domain.Draft{}, err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO drafts (account_scope, draft_id, mode, topic_id, edit_post_id, revision, payload_json, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(account_scope, draft_id) DO UPDATE SET
			mode = excluded.mode,
			topic_id = excluded.topic_id,
			edit_post_id = excluded.edit_post_id,
			revision = excluded.revision,
			payload_json = excluded.payload_json,
			updated_at = excluded.updated_at
		WHERE excluded.revision >= drafts.revision
	`, draft.AccountScope, draft.DraftID, string(draft.Mode), draft.TopicID, draft.EditPostID, draft.Revision, string(raw), draft.UpdatedAt.Unix())
	if err != nil {
		return domain.Draft{}, domain.NewError(domain.ErrCodeStorageFailed, "SQLite 写入草稿失败", true, err)
	}
	return draft, nil
}

// GetDraft loads a single draft isolated by accountScope.
func (s *SQLiteStore) GetDraft(ctx context.Context, accountScope, draftID string) (domain.Draft, bool, error) {
	var payload string
	err := s.db.QueryRowContext(ctx, `
		SELECT payload_json FROM drafts WHERE account_scope = ? AND draft_id = ?
	`, accountScope, draftID).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Draft{}, false, nil
	}
	if err != nil {
		return domain.Draft{}, false, err
	}
	var out domain.Draft
	if err := json.Unmarshal([]byte(payload), &out); err != nil {
		return domain.Draft{}, false, err
	}
	return out, true, nil
}

// ListDrafts lists recoverable drafts for an account ordered by recency.
func (s *SQLiteStore) ListDrafts(ctx context.Context, accountScope string) ([]domain.Draft, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT payload_json FROM drafts WHERE account_scope = ? ORDER BY updated_at DESC
	`, accountScope)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Draft
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var d domain.Draft
		if err := json.Unmarshal([]byte(payload), &d); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// DeleteDraft deletes a draft within an account scope.
func (s *SQLiteStore) DeleteDraft(ctx context.Context, accountScope, draftID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM drafts WHERE account_scope = ? AND draft_id = ?`, accountScope, draftID)
	return err
}

// SaveLayout persists a workspace layout snapshot.
func (s *SQLiteStore) SaveLayout(ctx context.Context, layout domain.WorkspaceLayout) error {
	raw, err := json.Marshal(layout)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO workspaces (account_scope, layout_id, schema_version, payload_json, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(account_scope, layout_id) DO UPDATE SET
			schema_version = excluded.schema_version,
			payload_json = excluded.payload_json,
			updated_at = excluded.updated_at
	`, layout.AccountScope, layout.LayoutID, layout.SchemaVersion, string(raw), layout.UpdatedAt.Unix())
	return err
}

// GetLayout loads a saved workspace layout.
func (s *SQLiteStore) GetLayout(ctx context.Context, accountScope, layoutID string) (domain.WorkspaceLayout, bool, error) {
	var payload string
	err := s.db.QueryRowContext(ctx, `
		SELECT payload_json FROM workspaces WHERE account_scope = ? AND layout_id = ?
	`, accountScope, layoutID).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.WorkspaceLayout{}, false, nil
	}
	if err != nil {
		return domain.WorkspaceLayout{}, false, err
	}
	var layout domain.WorkspaceLayout
	if err := json.Unmarshal([]byte(payload), &layout); err != nil {
		return domain.WorkspaceLayout{}, false, err
	}
	return layout, true, nil
}

// ClearContentCache removes cached topics and posts in a single transaction while keeping drafts, outbox, and layouts intact (B12).
func (s *SQLiteStore) ClearContentCache(ctx context.Context, accountScope string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM posts WHERE account_scope = ?`, accountScope); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM topics WHERE account_scope = ?`, accountScope); err != nil {
		return err
	}
	return tx.Commit()
}

// SaveTopicCached inserts or updates a cached topic row for cache-clear verification.
func (s *SQLiteStore) SaveTopicCached(ctx context.Context, accountScope string, summary domain.TopicSummary, fetchedAt time.Time) error {
	raw, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO topics (site_id, account_scope, topic_id, title, slug, category_id, highest_post_number, last_read_post_number, notification_level, payload_json, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(site_id, account_scope, topic_id) DO UPDATE SET
			title = excluded.title,
			payload_json = excluded.payload_json,
			fetched_at = excluded.fetched_at
	`, "linux.do", accountScope, summary.ID, summary.Title, summary.Slug, summary.CategoryID, summary.HighestPostNumber, summary.LastReadPostNumber, int(summary.NotificationLevel), string(raw), fetchedAt.Unix())
	return err
}
