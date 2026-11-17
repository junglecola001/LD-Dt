-- LD Studio initial SQLite schema (plan.md 7.4)
-- Separates account_scope caches from protected drafts, receipts, and layouts.

CREATE TABLE IF NOT EXISTS topics (
    site_id TEXT NOT NULL,
    account_scope TEXT NOT NULL,
    topic_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    slug TEXT NOT NULL,
    category_id INTEGER NOT NULL DEFAULT 0,
    highest_post_number INTEGER NOT NULL DEFAULT 0,
    last_read_post_number INTEGER NOT NULL DEFAULT 0,
    notification_level INTEGER NOT NULL DEFAULT 1,
    payload_json TEXT NOT NULL,
    fetched_at INTEGER NOT NULL,
    PRIMARY KEY (site_id, account_scope, topic_id)
);

CREATE TABLE IF NOT EXISTS posts (
    site_id TEXT NOT NULL,
    account_scope TEXT NOT NULL,
    topic_id INTEGER NOT NULL,
    post_id INTEGER NOT NULL,
    post_number INTEGER NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    payload_json TEXT NOT NULL,
    fetched_at INTEGER NOT NULL,
    PRIMARY KEY (site_id, account_scope, post_id)
);

CREATE TABLE IF NOT EXISTS users (
    site_id TEXT NOT NULL,
    username TEXT NOT NULL,
    payload_json TEXT NOT NULL,
    fetched_at INTEGER NOT NULL,
    PRIMARY KEY (site_id, username)
);

CREATE TABLE IF NOT EXISTS categories (
    site_id TEXT NOT NULL,
    account_scope TEXT NOT NULL,
    category_id INTEGER NOT NULL,
    payload_json TEXT NOT NULL,
    fetched_at INTEGER NOT NULL,
    PRIMARY KEY (site_id, account_scope, category_id)
);

CREATE TABLE IF NOT EXISTS notifications (
    site_id TEXT NOT NULL,
    account_scope TEXT NOT NULL,
    notification_id INTEGER NOT NULL,
    is_read INTEGER NOT NULL DEFAULT 0,
    topic_id INTEGER NOT NULL DEFAULT 0,
    post_number INTEGER NOT NULL DEFAULT 0,
    payload_json TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (site_id, account_scope, notification_id)
);

CREATE TABLE IF NOT EXISTS bookmarks (
    site_id TEXT NOT NULL,
    account_scope TEXT NOT NULL,
    bookmark_id INTEGER NOT NULL,
    topic_id INTEGER NOT NULL,
    post_id INTEGER NOT NULL DEFAULT 0,
    payload_json TEXT NOT NULL,
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (site_id, account_scope, bookmark_id)
);

CREATE TABLE IF NOT EXISTS history (
    account_scope TEXT NOT NULL,
    instance_id TEXT NOT NULL,
    topic_id INTEGER NOT NULL,
    post_number INTEGER NOT NULL,
    post_id INTEGER NOT NULL,
    offset_ratio REAL NOT NULL DEFAULT 0,
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (account_scope, instance_id, topic_id)
);

CREATE TABLE IF NOT EXISTS drafts (
    account_scope TEXT NOT NULL,
    draft_id TEXT NOT NULL,
    mode TEXT NOT NULL,
    topic_id INTEGER NOT NULL DEFAULT 0,
    edit_post_id INTEGER NOT NULL DEFAULT 0,
    revision INTEGER NOT NULL DEFAULT 1,
    payload_json TEXT NOT NULL,
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (account_scope, draft_id)
);

CREATE TABLE IF NOT EXISTS workspaces (
    account_scope TEXT NOT NULL,
    layout_id TEXT NOT NULL,
    schema_version INTEGER NOT NULL,
    payload_json TEXT NOT NULL,
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (account_scope, layout_id)
);

CREATE TABLE IF NOT EXISTS sync_state (
    account_scope TEXT NOT NULL,
    domain_key TEXT NOT NULL,
    cursor TEXT NOT NULL DEFAULT '',
    version INTEGER NOT NULL DEFAULT 0,
    payload_json TEXT NOT NULL,
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (account_scope, domain_key)
);

CREATE TABLE IF NOT EXISTS outbox (
    account_scope TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    draft_id TEXT NOT NULL,
    state TEXT NOT NULL,
    payload_json TEXT NOT NULL,
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (account_scope, operation_id)
);
