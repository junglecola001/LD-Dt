package app

import (
	"context"
	"sync"
	"time"

	"github.com/junglecola001/LD-Dt/internal/domain"
	"github.com/junglecola001/LD-Dt/internal/session"
)

// FeedFetcher resolves a FeedQuery page from network or mock adapter.
type FeedFetcher interface {
	FetchFeed(ctx context.Context, query domain.FeedQuery) (domain.FeedPage, error)
	FetchTopic(ctx context.Context, siteID string, topicID int64, aroundPostNumber int) (domain.TopicDetail, error)
}

// TopicService manages topic discovery queries, deduplicated pagination, shared topic entities,
// window-local read anchors, and active-window ReadTracker arbitration (B01, B02, B03).
type TopicService struct {
	mu       sync.Mutex
	sessions *session.Manager
	cache    domain.CacheRepository
	fetcher  FeedFetcher
	clock    domain.Clock
	events   domain.EventEmitter

	activeFeedGenerations map[string]int64
	feedPages             map[string][]domain.TopicSummary
	readTracker           *ReadTracker
}

// NewTopicService creates a TopicService with isolated feed query state and ReadTracker.
func NewTopicService(sessions *session.Manager, cache domain.CacheRepository, fetcher FeedFetcher, clock domain.Clock, events domain.EventEmitter) *TopicService {
	if clock == nil {
		clock = domain.SystemClock{}
	}
	return &TopicService{
		sessions:              sessions,
		cache:                 cache,
		fetcher:               fetcher,
		clock:                 clock,
		events:                events,
		activeFeedGenerations: make(map[string]int64),
		feedPages:             make(map[string][]domain.TopicSummary),
		readTracker:           NewReadTracker(clock),
	}
}

// LoadFeed enforces query generation ordering and deduplicates paginated topics by ID (B01).
func (s *TopicService) LoadFeed(ctx context.Context, windowInstanceID string, query domain.FeedQuery) (domain.FeedPage, error) {
	sess := s.sessions.Current()
	query.SiteID = sess.SiteID
	query.AccountScope = sess.AccountScope()
	startSessionGen := sess.SessionGeneration

	s.mu.Lock()
	if query.QueryGeneration > s.activeFeedGenerations[windowInstanceID] {
		s.activeFeedGenerations[windowInstanceID] = query.QueryGeneration
	}
	expectedQueryGen := s.activeFeedGenerations[windowInstanceID]
	s.mu.Unlock()

	if query.QueryGeneration < expectedQueryGen {
		return domain.FeedPage{}, domain.NewError(domain.ErrCodeCancelled, "已切换到新筛选条件", true, nil)
	}

	page, err := s.fetcher.FetchFeed(ctx, query)
	if err != nil {
		return domain.FeedPage{}, err
	}

	if s.sessions.Generation() != startSessionGen {
		return domain.FeedPage{}, domain.NewError(domain.ErrCodeStaleSession, "会话已切换，忽略旧账号列表结果", true, nil)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if query.QueryGeneration < s.activeFeedGenerations[windowInstanceID] {
		return domain.FeedPage{}, domain.NewError(domain.ErrCodeCancelled, "慢响应已被新筛选覆盖", true, nil)
	}

	key := query.CacheKey()
	var merged []domain.TopicSummary
	if query.Page > 0 {
		merged = append(merged, s.feedPages[key]...)
	}
	seen := make(map[int64]struct{}, len(merged))
	for _, existing := range merged {
		seen[existing.ID] = struct{}{}
	}
	for _, t := range page.Topics {
		if _, dup := seen[t.ID]; !dup {
			seen[t.ID] = struct{}{}
			merged = append(merged, t)
		}
	}
	s.feedPages[key] = merged
	page.QueryKey = key
	page.Topics = append([]domain.TopicSummary(nil), merged...)
	return page, nil
}

// OpenTopic resolves a topic and optional target post number without resetting sibling windows (B02).
func (s *TopicService) OpenTopic(ctx context.Context, instanceID string, topicID int64, targetPostNumber int) (domain.TopicDetail, domain.ReadAnchor, error) {
	sess := s.sessions.Current()
	scope := sess.AccountScope()
	startGen := sess.SessionGeneration

	if cached, ok, _ := s.cache.GetTopicDetail(ctx, scope, topicID); ok {
		if targetPostNumber <= 0 || hasPostNumber(cached.Posts, targetPostNumber) {
			anchor, hasAnchor, _ := s.cache.GetReadAnchor(ctx, scope, instanceID, topicID)
			if !hasAnchor && targetPostNumber > 0 {
				anchor = domain.ReadAnchor{
					InstanceID:   instanceID,
					SiteID:       sess.SiteID,
					AccountScope: scope,
					TopicID:      topicID,
					PostNumber:   targetPostNumber,
					UpdatedAt:    s.clock.Now(),
				}
			}
			return cached, anchor, nil
		}
	}

	detail, err := s.fetcher.FetchTopic(ctx, sess.SiteID, topicID, targetPostNumber)
	if err != nil {
		return domain.TopicDetail{}, domain.ReadAnchor{}, err
	}
	if s.sessions.Generation() != startGen {
		return domain.TopicDetail{}, domain.ReadAnchor{}, domain.NewError(domain.ErrCodeStaleSession, "会话已切换", true, nil)
	}

	detail.TargetPostNumber = targetPostNumber
	detail.FetchedAt = s.clock.Now()
	_ = s.cache.SaveTopicDetail(ctx, scope, detail)

	anchor, hasAnchor, _ := s.cache.GetReadAnchor(ctx, scope, instanceID, topicID)
	if targetPostNumber > 0 {
		anchor = domain.ReadAnchor{
			InstanceID:   instanceID,
			SiteID:       sess.SiteID,
			AccountScope: scope,
			TopicID:      topicID,
			PostNumber:   targetPostNumber,
			UpdatedAt:    s.clock.Now(),
		}
		_ = s.cache.SaveReadAnchor(ctx, anchor)
	} else if !hasAnchor && len(detail.Posts) > 0 {
		anchor = domain.ReadAnchor{
			InstanceID:   instanceID,
			SiteID:       sess.SiteID,
			AccountScope: scope,
			TopicID:      topicID,
			PostNumber:   detail.Posts[0].PostNumber,
			PostID:       detail.Posts[0].ID,
			UpdatedAt:    s.clock.Now(),
		}
	}

	return detail, anchor, nil
}

// SaveReadAnchor persists window-specific reading position without marking the topic read on the server.
func (s *TopicService) SaveReadAnchor(ctx context.Context, anchor domain.ReadAnchor) error {
	sess := s.sessions.Current()
	anchor.SiteID = sess.SiteID
	anchor.AccountScope = sess.AccountScope()
	anchor.UpdatedAt = s.clock.Now()
	return s.cache.SaveReadAnchor(ctx, anchor)
}

// UpdateReadVisibility routes focus/minimized/modal state to ReadTracker (B03).
func (s *TopicService) UpdateReadVisibility(activeInstanceID string, topicID int64, appFocused, minimized, modalBlocked bool) time.Duration {
	return s.readTracker.Update(activeInstanceID, topicID, appFocused, minimized, modalBlocked)
}

// FlushTopicReadTime returns bounded accumulated read duration for a topic without duplicating across windows.
func (s *TopicService) FlushTopicReadTime(topicID int64) time.Duration {
	return s.readTracker.FlushTopic(topicID)
}

func hasPostNumber(posts []domain.Post, target int) bool {
	for _, p := range posts {
		if p.PostNumber == target {
			return true
		}
	}
	return false
}

// ReadTracker arbitrates active-window reading duration across multiple windows (plan.md 3.7, B03).
type ReadTracker struct {
	mu           sync.Mutex
	clock        domain.Clock
	activeTopic  int64
	activeInst   string
	trackingFrom time.Time
	accumulated  map[int64]time.Duration
}

// NewReadTracker constructs a ReadTracker.
func NewReadTracker(clock domain.Clock) *ReadTracker {
	if clock == nil {
		clock = domain.SystemClock{}
	}
	return &ReadTracker{
		clock:       clock,
		accumulated: make(map[int64]time.Duration),
	}
}

// Update records elapsed time only while app is focused, window is active/unminimized, and no modal blocks it.
func (r *ReadTracker) Update(instanceID string, topicID int64, appFocused, minimized, modalBlocked bool) time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.clock.Now()

	if r.activeTopic > 0 && !r.trackingFrom.IsZero() {
		delta := now.Sub(r.trackingFrom)
		if delta > 0 {
			r.accumulated[r.activeTopic] += delta
		}
	}

	if !appFocused || minimized || modalBlocked || topicID <= 0 || instanceID == "" {
		r.activeTopic = 0
		r.activeInst = ""
		r.trackingFrom = time.Time{}
		return r.accumulated[topicID]
	}

	r.activeTopic = topicID
	r.activeInst = instanceID
	r.trackingFrom = now
	return r.accumulated[topicID]
}

// FlushTopic collects and resets pending read duration for server timings reporting.
func (r *ReadTracker) FlushTopic(topicID int64) time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.clock.Now()
	if r.activeTopic == topicID && !r.trackingFrom.IsZero() {
		delta := now.Sub(r.trackingFrom)
		if delta > 0 {
			r.accumulated[topicID] += delta
		}
		r.trackingFrom = now
	}
	total := r.accumulated[topicID]
	delete(r.accumulated, topicID)
	return total
}
