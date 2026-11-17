package storage

import (
	"context"
	_ "embed"
	"sort"
	"sync"

	"github.com/junglecola001/LD-Dt/internal/domain"
)

// InitialSchemaSQL holds the baseline 11-table SQLite migration script.
//
//go:embed migrations/001_init.sql
var InitialSchemaSQL string

// MemoryStore provides deterministic, thread-safe storage implementing all repository ports.
// Used in Phase 0 contract tests and as a reference contract for the SQLite implementation.
type MemoryStore struct {
	mu       sync.RWMutex
	drafts   map[string]map[string]domain.Draft
	receipts map[string]map[string]domain.SubmitOperation
	layouts  map[string]map[string]domain.WorkspaceLayout
	topics   map[string]map[int64]domain.TopicDetail
	anchors  map[string]map[string]domain.ReadAnchor

	// FailNextDraftSave lets tests simulate B06 disk write errors.
	FailNextDraftSave bool
	// FailNextCacheClear lets tests verify B12 transaction failure behavior.
	FailNextCacheClear bool
}

// NewMemoryStore initializes isolated repositories per accountScope.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		drafts:   make(map[string]map[string]domain.Draft),
		receipts: make(map[string]map[string]domain.SubmitOperation),
		layouts:  make(map[string]map[string]domain.WorkspaceLayout),
		topics:   make(map[string]map[int64]domain.TopicDetail),
		anchors:  make(map[string]map[string]domain.ReadAnchor),
	}
}

func (s *MemoryStore) SaveDraft(_ context.Context, draft domain.Draft) (domain.Draft, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FailNextDraftSave {
		s.FailNextDraftSave = false
		return domain.Draft{}, domain.NewError(domain.ErrCodeStorageFailed, "本地草稿写入失败", true, nil)
	}
	bucket, ok := s.drafts[draft.AccountScope]
	if !ok {
		bucket = make(map[string]domain.Draft)
		s.drafts[draft.AccountScope] = bucket
	}
	if existing, exists := bucket[draft.DraftID]; exists && draft.Revision < existing.Revision {
		// Never let an older save overwrite a newer revision (B05).
		return existing, nil
	}
	bucket[draft.DraftID] = cloneDraft(draft)
	return cloneDraft(draft), nil
}

func (s *MemoryStore) GetDraft(_ context.Context, accountScope, draftID string) (domain.Draft, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	bucket := s.drafts[accountScope]
	d, ok := bucket[draftID]
	if !ok {
		return domain.Draft{}, false, nil
	}
	return cloneDraft(d), true, nil
}

func (s *MemoryStore) ListDrafts(_ context.Context, accountScope string) ([]domain.Draft, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	bucket := s.drafts[accountScope]
	out := make([]domain.Draft, 0, len(bucket))
	for _, d := range bucket {
		out = append(out, cloneDraft(d))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

func (s *MemoryStore) DeleteDraft(_ context.Context, accountScope, draftID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if bucket, ok := s.drafts[accountScope]; ok {
		delete(bucket, draftID)
	}
	return nil
}

func (s *MemoryStore) SaveSubmitReceipt(_ context.Context, op domain.SubmitOperation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	bucket, ok := s.receipts[op.AccountScope]
	if !ok {
		bucket = make(map[string]domain.SubmitOperation)
		s.receipts[op.AccountScope] = bucket
	}
	bucket[op.OperationID] = op
	return nil
}

func (s *MemoryStore) GetSubmitReceipt(_ context.Context, accountScope, operationID string) (domain.SubmitOperation, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	bucket := s.receipts[accountScope]
	op, ok := bucket[operationID]
	return op, ok, nil
}

func (s *MemoryStore) ListPendingReceipts(_ context.Context, accountScope string) ([]domain.SubmitOperation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	bucket := s.receipts[accountScope]
	var out []domain.SubmitOperation
	for _, op := range bucket {
		if op.State == domain.SubmitOutcomeUnknown || op.State == domain.SubmitSubmitting {
			out = append(out, op)
		}
	}
	return out, nil
}

func (s *MemoryStore) SaveLayout(_ context.Context, layout domain.WorkspaceLayout) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	bucket, ok := s.layouts[layout.AccountScope]
	if !ok {
		bucket = make(map[string]domain.WorkspaceLayout)
		s.layouts[layout.AccountScope] = bucket
	}
	copied := layout
	copied.Windows = append([]domain.WindowState(nil), layout.Windows...)
	bucket[layout.LayoutID] = copied
	return nil
}

func (s *MemoryStore) GetLayout(_ context.Context, accountScope, layoutID string) (domain.WorkspaceLayout, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	bucket := s.layouts[accountScope]
	l, ok := bucket[layoutID]
	if !ok {
		return domain.WorkspaceLayout{}, false, nil
	}
	l.Windows = append([]domain.WindowState(nil), l.Windows...)
	return l, true, nil
}

func (s *MemoryStore) ListLayouts(_ context.Context, accountScope string) ([]domain.WorkspaceLayout, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	bucket := s.layouts[accountScope]
	out := make([]domain.WorkspaceLayout, 0, len(bucket))
	for _, l := range bucket {
		l.Windows = append([]domain.WindowState(nil), l.Windows...)
		out = append(out, l)
	}
	return out, nil
}

func (s *MemoryStore) DeleteLayout(_ context.Context, accountScope, layoutID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if bucket, ok := s.layouts[accountScope]; ok {
		delete(bucket, layoutID)
	}
	return nil
}

func (s *MemoryStore) SaveTopicDetail(_ context.Context, accountScope string, detail domain.TopicDetail) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	bucket, ok := s.topics[accountScope]
	if !ok {
		bucket = make(map[int64]domain.TopicDetail)
		s.topics[accountScope] = bucket
	}
	copied := detail
	copied.Posts = append([]domain.Post(nil), detail.Posts...)
	copied.StreamPostIDs = append([]int64(nil), detail.StreamPostIDs...)
	bucket[detail.Summary.ID] = copied
	return nil
}

func (s *MemoryStore) GetTopicDetail(_ context.Context, accountScope string, topicID int64) (domain.TopicDetail, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	bucket := s.topics[accountScope]
	d, ok := bucket[topicID]
	if !ok {
		return domain.TopicDetail{}, false, nil
	}
	d.Posts = append([]domain.Post(nil), d.Posts...)
	d.StreamPostIDs = append([]int64(nil), d.StreamPostIDs...)
	return d, true, nil
}

func (s *MemoryStore) SaveReadAnchor(_ context.Context, anchor domain.ReadAnchor) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	bucket, ok := s.anchors[anchor.AccountScope]
	if !ok {
		bucket = make(map[string]domain.ReadAnchor)
		s.anchors[anchor.AccountScope] = bucket
	}
	bucket[anchorKey(anchor.InstanceID, anchor.TopicID)] = anchor
	return nil
}

func (s *MemoryStore) GetReadAnchor(_ context.Context, accountScope, instanceID string, topicID int64) (domain.ReadAnchor, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	bucket := s.anchors[accountScope]
	a, ok := bucket[anchorKey(instanceID, topicID)]
	return a, ok, nil
}

// ClearContentCache removes topic caches while strictly protecting drafts, receipts, and layouts (B12).
func (s *MemoryStore) ClearContentCache(_ context.Context, accountScope string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FailNextCacheClear {
		s.FailNextCacheClear = false
		return domain.NewError(domain.ErrCodeStorageFailed, "清理缓存失败，未修改本地数据", true, nil)
	}
	delete(s.topics, accountScope)
	return nil
}

// ClearAllAccountData explicitly removes drafts, receipts, caches, anchors, and layouts for an account.
func (s *MemoryStore) ClearAllAccountData(accountScope string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.drafts, accountScope)
	delete(s.receipts, accountScope)
	delete(s.layouts, accountScope)
	delete(s.topics, accountScope)
	delete(s.anchors, accountScope)
}

func anchorKey(instanceID string, topicID int64) string {
	return instanceID + ":" + itoa64(topicID)
}

func itoa64(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func cloneDraft(d domain.Draft) domain.Draft {
	d.Tags = append([]string(nil), d.Tags...)
	d.Uploads = append([]domain.UploadTask(nil), d.Uploads...)
	return d
}
