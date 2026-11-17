package app

import (
	"context"
	"sync"

	"github.com/junglecola001/LD-Dt/internal/domain"
	"github.com/junglecola001/LD-Dt/internal/session"
)

// SearchFetcher resolves global or in-topic search queries.
type SearchFetcher interface {
	Search(ctx context.Context, query domain.SearchQuery) (domain.SearchResult, error)
}

// SearchService isolates input/cursor per window instance and prevents slow older responses
// from overwriting newer query generations (B04).
type SearchService struct {
	mu          sync.Mutex
	sessions    *session.Manager
	fetcher     SearchFetcher
	generations map[string]int64
}

// NewSearchService constructs a SearchService.
func NewSearchService(sessions *session.Manager, fetcher SearchFetcher) *SearchService {
	return &SearchService{
		sessions:    sessions,
		fetcher:     fetcher,
		generations: make(map[string]int64),
	}
}

// Execute runs a search tagged with instanceId and queryGeneration.
func (s *SearchService) Execute(ctx context.Context, query domain.SearchQuery) (domain.SearchResult, error) {
	sess := s.sessions.Current()
	query.SiteID = sess.SiteID
	query.AccountScope = sess.AccountScope()
	startGen := sess.SessionGeneration

	s.mu.Lock()
	if query.QueryGeneration > s.generations[query.InstanceID] {
		s.generations[query.InstanceID] = query.QueryGeneration
	}
	expected := s.generations[query.InstanceID]
	s.mu.Unlock()

	if query.QueryGeneration < expected {
		return domain.SearchResult{}, domain.NewError(domain.ErrCodeCancelled, "已输入新查询", true, nil)
	}

	res, err := s.fetcher.Search(ctx, query)
	if err != nil {
		return domain.SearchResult{}, err
	}
	if s.sessions.Generation() != startGen {
		return domain.SearchResult{}, domain.NewError(domain.ErrCodeStaleSession, "会话已切换", true, nil)
	}

	s.mu.Lock()
	latest := s.generations[query.InstanceID]
	s.mu.Unlock()
	if query.QueryGeneration < latest {
		return domain.SearchResult{}, domain.NewError(domain.ErrCodeCancelled, "旧搜索结果已丢弃", true, nil)
	}
	res.InstanceID = query.InstanceID
	res.QueryGeneration = query.QueryGeneration
	return res, nil
}
