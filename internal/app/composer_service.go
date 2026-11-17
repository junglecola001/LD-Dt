package app

import (
	"context"
	"fmt"
	"sync"

	"github.com/junglecola001/LD-Dt/internal/domain"
	"github.com/junglecola001/LD-Dt/internal/session"
)

// ComposerGateway abstracts server draft sync, post submission, and uploads.
type ComposerGateway interface {
	SyncServerDraft(ctx context.Context, draft domain.Draft) (serverSequence int64, conflictBody string, conflict bool, err error)
	SubmitSnapshot(ctx context.Context, op domain.SubmitOperation, draft domain.Draft) (domain.SubmitOperation, error)
	UploadFile(ctx context.Context, task domain.UploadTask, data []byte) (shortURL string, resolvedURL string, err error)
}

// ComposerService enforces local-first persistence across all 4 composer modes,
// monotonic revision acks, cloud conflict preservation, per-draftId submit locking,
// immutable snapshot checks, and two-step upload acks (plan.md 3.8, 3.9, B05-B09).
type ComposerService struct {
	mu          sync.Mutex
	sessions    *session.Manager
	repo        domain.DraftRepository
	gateway     ComposerGateway
	clock       domain.Clock
	events      domain.EventEmitter
	submitLocks map[string]bool
	opCounter   int64
}

// NewComposerService constructs a ComposerService.
func NewComposerService(sessions *session.Manager, repo domain.DraftRepository, gateway ComposerGateway, clock domain.Clock, events domain.EventEmitter) *ComposerService {
	if clock == nil {
		clock = domain.SystemClock{}
	}
	return &ComposerService{
		sessions:    sessions,
		repo:        repo,
		gateway:     gateway,
		clock:       clock,
		events:      events,
		submitLocks: make(map[string]bool),
	}
}

// SaveLocalDraft persists any of newTopic / replyTopic / replyPost / editPost locally.
// Older revision callbacks never overwrite a newer revision (B05).
func (s *ComposerService) SaveLocalDraft(ctx context.Context, draft domain.Draft) (domain.Draft, error) {
	sess := s.sessions.Current()
	draft.SiteID = sess.SiteID
	draft.AccountScope = sess.AccountScope()
	draft.UpdatedAt = s.clock.Now()
	if draft.Revision <= 0 {
		draft.Revision = 1
	}

	existing, ok, err := s.repo.GetDraft(ctx, draft.AccountScope, draft.DraftID)
	if err == nil && ok && draft.Revision < existing.Revision {
		// Stale save ack must never downgrade or overwrite newer content.
		return existing, nil
	}

	draft.LocalState = domain.LocalDraftSaved
	if draft.CloudState == "" {
		draft.CloudState = domain.CloudDraftPending
	}
	saved, err := s.repo.SaveDraft(ctx, draft)
	if err != nil {
		draft.LocalState = domain.LocalDraftSaveFailed
		return draft, err
	}
	if s.events != nil {
		s.events.Emit("DraftSaved", saved)
	}
	return saved, nil
}

// ListDrafts returns all locally recoverable drafts scoped strictly to the current account (B10).
func (s *ComposerService) ListDrafts(ctx context.Context) ([]domain.Draft, error) {
	scope := s.sessions.Current().AccountScope()
	return s.repo.ListDrafts(ctx, scope)
}

// SyncCloudDraft optionally synchronizes local drafts to server while keeping local/cloud states separate (B06).
func (s *ComposerService) SyncCloudDraft(ctx context.Context, draftID string) (domain.Draft, error) {
	scope := s.sessions.Current().AccountScope()
	draft, ok, err := s.repo.GetDraft(ctx, scope, draftID)
	if err != nil || !ok {
		return domain.Draft{}, domain.NewError(domain.ErrCodeNotFound, "找不到本地草稿", false, err)
	}
	if s.gateway == nil {
		draft.CloudState = domain.CloudDraftDisabled
		return draft, nil
	}

	s.mu.Lock()
	if s.submitLocks[draftID] {
		s.mu.Unlock()
		// Suspend cloud autosave while submission is in flight (plan.md 3.8).
		return draft, nil
	}
	s.mu.Unlock()

	seq, conflictBody, conflict, err := s.gateway.SyncServerDraft(ctx, draft)
	if err != nil {
		draft.CloudState = domain.CloudDraftFailed
		saved, saveErr := s.repo.SaveDraft(ctx, draft)
		if saveErr != nil {
			return draft, saveErr
		}
		return saved, err
	}
	if conflict {
		// Keep both local body and remote conflictBody; never silently overwrite.
		draft.CloudState = domain.CloudDraftConflict
		draft.ConflictBody = conflictBody
		return s.repo.SaveDraft(ctx, draft)
	}
	draft.CloudState = domain.CloudDraftSynced
	draft.ServerSequence = seq
	draft.ConflictBody = ""
	return s.repo.SaveDraft(ctx, draft)
}

// StartUpload executes upload step 1 (queued -> uploading -> uploaded) and rejects late results
// if the task was cancelled or session switched (B09).
func (s *ComposerService) StartUpload(ctx context.Context, draftID string, task domain.UploadTask, data []byte) (domain.UploadTask, error) {
	sess := s.sessions.Current()
	scope := sess.AccountScope()
	startSessionGen := sess.SessionGeneration

	task.DraftID = draftID
	task.AccountScope = scope
	task.State = domain.UploadUploading

	if s.gateway == nil {
		task.State = domain.UploadFailed
		task.ErrorMessage = "未配置上传服务"
		return task, domain.NewError(domain.ErrCodeValidationFailed, task.ErrorMessage, false, nil)
	}

	shortURL, resolvedURL, err := s.gateway.UploadFile(ctx, task, data)
	if err != nil {
		if ctx.Err() != nil {
			task.State = domain.UploadCancelled
			return task, domain.NewError(domain.ErrCodeCancelled, "上传已取消", true, ctx.Err())
		}
		task.State = domain.UploadFailed
		task.ErrorMessage = err.Error()
		return task, err
	}

	if s.sessions.Generation() != startSessionGen || ctx.Err() != nil {
		task.State = domain.UploadCancelled
		return task, domain.NewError(domain.ErrCodeStaleSession, "会话已切换或任务已取消，忽略迟到上传结果", true, nil)
	}

	// Re-check latest persisted draft to verify task was not removed while uploading.
	draft, ok, err := s.repo.GetDraft(ctx, scope, draftID)
	if err == nil && ok {
		if found, cancelled := findUpload(draft.Uploads, task.UploadID); cancelled || (found != nil && found.Generation > task.Generation) {
			task.State = domain.UploadCancelled
			return task, domain.NewError(domain.ErrCodeCancelled, "上传占位已被移除，忽略迟到结果", true, nil)
		}
	}

	task.State = domain.UploadUploaded
	task.ShortURL = shortURL
	task.ResolvedURL = resolvedURL
	return task, nil
}

// ConfirmUploadInserted records step 2 ack (uploaded -> inserted) reusing receipt without re-uploading (B09).
func (s *ComposerService) ConfirmUploadInserted(ctx context.Context, draftID, uploadID string) (domain.Draft, error) {
	scope := s.sessions.Current().AccountScope()
	draft, ok, err := s.repo.GetDraft(ctx, scope, draftID)
	if err != nil || !ok {
		return domain.Draft{}, domain.NewError(domain.ErrCodeNotFound, "草稿不存在", false, err)
	}
	for i := range draft.Uploads {
		if draft.Uploads[i].UploadID == uploadID {
			if draft.Uploads[i].State == domain.UploadUploaded {
				draft.Uploads[i].State = domain.UploadInserted
			}
			break
		}
	}
	return s.repo.SaveDraft(ctx, draft)
}

// SubmitDraft acquires a per-draftId lock, verifies no unresolved uploads exist,
// snapshots immutable body/title, re-verifies revision before sending, and handles
// published / enqueued / rejected / outcomeUnknown distinctly (B07, B08, B09).
func (s *ComposerService) SubmitDraft(ctx context.Context, draftID string, expectedRevision int64) (domain.SubmitOperation, error) {
	sess := s.sessions.Current()
	scope := sess.AccountScope()
	if sess.Status == domain.SessionReauthNeeded {
		return domain.SubmitOperation{}, domain.NewError(domain.ErrCodeAuthExpired, "账号已失效，请重新连接后由您再次发起发布", true, nil)
	}

	s.mu.Lock()
	if s.submitLocks[draftID] {
		s.mu.Unlock()
		return domain.SubmitOperation{}, domain.NewError(domain.ErrCodeConflict, "该草稿正在提交中，请勿重复发送", true, nil)
	}
	s.submitLocks[draftID] = true
	s.opCounter++
	opID := fmt.Sprintf("op-%d-%s", s.opCounter, draftID)
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.submitLocks, draftID)
		s.mu.Unlock()
	}()

	draft, ok, err := s.repo.GetDraft(ctx, scope, draftID)
	if err != nil || !ok {
		return domain.SubmitOperation{}, domain.NewError(domain.ErrCodeNotFound, "找不到待发布的草稿", false, err)
	}

	// Re-verify revision: if user edited while confirming, abort submit and return to editing (B07).
	if expectedRevision > 0 && draft.Revision != expectedRevision {
		return domain.SubmitOperation{}, domain.NewError(domain.ErrCodeConflict, "草稿正文在确认期间已更新，请重新确认后发送", true, nil)
	}

	// Unresolved uploads block publish (B09).
	for _, u := range draft.Uploads {
		if u.State == domain.UploadQueued || u.State == domain.UploadUploading || u.State == domain.UploadFailed || u.State == domain.UploadUploaded {
			return domain.SubmitOperation{}, domain.NewError(domain.ErrCodeValidationFailed, "存在未完成或待插入的图片上传任务", true, nil)
		}
	}

	op := domain.SubmitOperation{
		OperationID:      opID,
		DraftID:          draftID,
		AccountScope:     scope,
		Mode:             draft.Mode,
		SnapshotRevision: draft.Revision,
		SnapshotBody:     draft.Body,
		SnapshotTitle:    draft.Title,
		State:            domain.SubmitSubmitting,
		UpdatedAt:        s.clock.Now(),
	}
	if err := s.repo.SaveSubmitReceipt(ctx, op); err != nil {
		return op, err
	}

	resultOp, err := s.gateway.SubmitSnapshot(ctx, op, draft)
	resultOp.OperationID = opID
	resultOp.DraftID = draftID
	resultOp.AccountScope = scope
	resultOp.SnapshotRevision = op.SnapshotRevision
	resultOp.SnapshotBody = op.SnapshotBody
	resultOp.SnapshotTitle = op.SnapshotTitle
	resultOp.UpdatedAt = s.clock.Now()

	if err != nil {
		if appErr, isApp := err.(*domain.AppError); isApp && appErr.Code == domain.ErrCodeOutcomeUnknown {
			resultOp.State = domain.SubmitOutcomeUnknown
			resultOp.ErrorCode = domain.ErrCodeOutcomeUnknown
			resultOp.ErrorMessage = appErr.Message
		} else {
			resultOp.State = domain.SubmitRejected
			resultOp.ErrorMessage = err.Error()
			if isApp {
				resultOp.ErrorCode = appErr.Code
			}
		}
		_ = s.repo.SaveSubmitReceipt(ctx, resultOp)
		return resultOp, err
	}

	_ = s.repo.SaveSubmitReceipt(ctx, resultOp)

	// Only archive/delete matching revision if published or enqueued; preserve newer edits made during flight (plan.md 3.8).
	if resultOp.State == domain.SubmitPublished || resultOp.State == domain.SubmitEnqueued {
		latest, stillExists, _ := s.repo.GetDraft(ctx, scope, draftID)
		if stillExists && latest.Revision == op.SnapshotRevision {
			_ = s.repo.DeleteDraft(ctx, scope, draftID)
		}
	}
	return resultOp, nil
}

func findUpload(uploads []domain.UploadTask, uploadID string) (*domain.UploadTask, bool) {
	for i := range uploads {
		if uploads[i].UploadID == uploadID {
			if uploads[i].State == domain.UploadCancelled {
				return &uploads[i], true
			}
			return &uploads[i], false
		}
	}
	return nil, true
}
