# LD Studio Business Rules & FluxDO Origin Mapping

> Status: Phase 0 Baseline (`plan.md` Sections 3.4–3.13, 7.5–7.8, 10.4)
> Upstream reference: `Lingyan000/fluxdo` @ `185ccaa418dab8ea40192301e39065706192f866` (2026-11-15 11:54:25 UTC)

## 1. Core Domain Objects & Invariants (`plan.md` 3.4)

| Entity | Identity & Ownership | Required Invariants |
| --- | --- | --- |
| `SiteSession` | `siteId + accountId + sessionGeneration` | Account identity is confirmed by the Go desktop service. Anonymous and authenticated caches are isolated; stale `sessionGeneration` responses must never enter a newer session. |
| `Topic` / `Post` | `siteId + topicId / postId` (+ `accountScope` for private snapshots) | Shared canonical entity cache across all windows; opening multiple windows never duplicates HTTP clients or full caches. |
| `FeedQuery` / `SearchQuery` | Full query parameters + `accountScope` | Category, tag, filter, sort, time range, and keyword are part of query identity; pagination cursors cannot leak across different queries. |
| `WindowInstance` | `instanceId + resourceKey` | Each window owns independent scroll, reading anchor, in-topic search input, and focus; closing a window releases the view without deleting content or drafts. |
| `Draft` | `draftId + owner + mode + target` (+ optional `serverDraftKey`/`sequence`) | Covers `newTopic`, `replyTopic`, `replyPost`, `editPost`. Only one writable editor per `draftId`. Changing target never silently rebinds an existing draft. |
| `UploadTask` | `uploadId + draftId + editorRevision + insertAnchor` | Upload completion (`uploaded`) and body insertion (`inserted`) are separate acknowledgements. Late results cannot write into cancelled tasks or another account's draft. |
| `SubmitOperation` | `operationId + draftId + snapshotRevision` | Bound to an immutable exported snapshot. Local `operationId` tracks client intent only and does not claim server-side exactly-once idempotency. |
| `ReadAnchor` / `ReadTracker` | Local anchor per `instanceId`; server read cursor per account | Restoring scroll position, accumulating valid reading time, and marking read/unread are separate operations. Fetching a topic never marks it read. |
| `Notification` / `Bookmark` | `siteId + accountId + serverId` | Unread count, list, and Windows system toast IDs reconcile independently. Marking a notification read does not mark the entire topic read. |

### Global Non-Negotiable Invariants

1. Unsaved drafts are never silently closed or discarded.
2. Reconnecting to the network never auto-publishes queued or pending posts.
3. Window reflow, docking, floating, or minimization never mutates draft body text.
4. Ordinary cache clearing never deletes drafts, submission receipts, or workspace layouts.
5. Permissions come from server capabilities and responses; UI button visibility is not authorization.
6. Permission denied (`403`), missing/deleted content (`404`), expired auth (`401`/login redirect), rate limit (`429`), and Cloudflare challenge are classified and presented distinctly.

## 2. State Machines (`plan.md` 3.8, 3.9, 7.6)

### 2.1 Draft & Submission Axes
- **Local Draft**: `restoring -> editing -> saving -> saved | saveFailed`
- **Cloud Draft (optional)**: `disabled | pending -> syncing -> synced | conflict | failed`
- **Submission**: `idle -> preparing -> submitting -> published | enqueued | rejected | outcomeUnknown`

### 2.2 Upload Lifecycle
`queued -> uploading -> uploaded -> inserted` (with `failed | cancelled`; `retryInsert` reuses existing upload receipt without re-uploading).

### 2.3 Network Coordinator
`Ready -> RequestCoordinator -> ResponseClassifier`
- Success -> `Ready`
- Challenge evidence (`cf-mitigated: challenge` or verified HTML challenge signature) -> `ChallengeRequired` (single visible challenge round -> session sync -> single safe GET probe -> `Ready | Paused`)
- Rate limit (`429` without challenge or explicit Discourse wait) -> `CoolingDown` (shared `blockedUntil` across all windows -> single safe GET probe if within budget -> `Ready | CoolingDown | Paused`)
- Auth invalid (`401` / HTML login redirect) -> `AuthExpired` (flush drafts, pause private writes/sync, preserve workspace & caches)
- Forbidden (`403` without challenge) -> `Forbidden` (do not trigger challenge modal or retry)
- Offline / transient `5xx` -> budgeted safe GET retries (`1s`, `2s` + jitter, max 3 attempts) -> `Ready | Offline`

## 3. Acceptance Scenarios B01–B12 (`plan.md` 10.4)

| ID | Scenario | Required Behavior | Origin / Enhancement Note |
| --- | --- | --- | --- |
| B01 | Slow Feed A overtaken by Filter B; B next page fails | Discard A; keep B rows/scroll/filter; retry recovers only failed page without duplicate `topicId`s | FluxDO `B1` + account/site key isolation & isolated page-2 error |
| B02 | Two windows on same topic; notification jumps to uncached post | Single shared GET; independent anchors/find; fetch target chunk before scrolling; don't reset sibling window | FluxDO `B2` + unified `OpenTopic` target command |
| B03 | Window minimized/modal-blocked/blurred; then like/refresh | Pause `ReadTracker`; refresh does not mark whole topic read; resume avoids double-counting | FluxDO `B3` + active-window arbitration & no mark-read-on-refresh |
| B04 | Search `q1` slower than `q2`; two topic windows search locally | `q1` never overwrites `q2`; input & cursor per `instanceId`; no dispatch during IME composition | FluxDO `B4` + query generation guard & IME composition gate |
| B05 | Edit/new/reply closed after typing; stale save ack arrives late | Only monotonic latest revision ack shows "Saved"; all modes recover locally on restart | FluxDO `B5` + explicit local ack & `editPost` local recovery |
| B06 | Cloud draft sequence conflict, offline, or local disk write error | Separate local/cloud status; retain both copies on conflict; block silent close on local save failure | FluxDO `B5/B6` + mandatory dual-copy conflict retention |
| B07 | Typing during publish confirm, rapid shortcut presses, IME selection | Never submit stale snapshot; lock per `draftId`; ignore `Ctrl+Enter` during IME composition | FluxDO `B6` + per-`draftId` lock & composition check |
| B08 | Server returns `enqueued` / `rejected` / timeout (`outcomeUnknown`) | Distinct states (`enqueued`, `rejected`, `outcomeUnknown`); never fake publish or auto-replay unknown writes | FluxDO `B6` + explicit `outcomeUnknown` & persistent receipt |
| B09 | Upload succeeds then insert fails; late completion after cancel | `retryInsert` reuses upload receipt; cancelled task never inserts; unresolved uploads block publish | FluxDO `B7` + dual ack (`uploaded`/`inserted`) & generation guard |
| B10 | Account A expires mid-draft -> login B -> re-login A | Draft stays isolated to A; stale A responses never enter B; A resumes without auto-submit | FluxDO `B10` + explicit `sessionGeneration` & account namespace |
| B11 | Live unread count updated before stale bootstrap; unauthorized target | Count never regresses; failed target stays retryable without advancing topic read cursor; DND respected | FluxDO `B8` + strict separation of notification read vs topic read |
| B12 | Bookmark CRUD/initial sync failure or ordinary cache clear | Server-confirmed targeted updates; failed sync doesn't wipe existing list; cache clear preserves drafts/receipts/layouts | FluxDO `B9` + independent sync state & protected tables |
