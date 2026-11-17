package app

import (
	"context"
	"time"

	"github.com/junglecola001/LD-Dt/internal/domain"
	"github.com/junglecola001/LD-Dt/internal/session"
)

const CurrentLayoutSchemaVersion = 1

// WorkspaceService persists DAW workspace layouts separately from drafts and secrets.
type WorkspaceService struct {
	repo     domain.WorkspaceRepository
	sessions *session.Manager
	clock    domain.Clock
}

// NewWorkspaceService constructs a WorkspaceService.
func NewWorkspaceService(repo domain.WorkspaceRepository, sessions *session.Manager, clock domain.Clock) *WorkspaceService {
	if clock == nil {
		clock = domain.SystemClock{}
	}
	return &WorkspaceService{
		repo:     repo,
		sessions: sessions,
		clock:    clock,
	}
}

// SaveLayout validates and persists a layout snapshot without credentials or draft bodies.
func (s *WorkspaceService) SaveLayout(ctx context.Context, layout domain.WorkspaceLayout) (domain.WorkspaceLayout, error) {
	scope := s.sessions.Current().AccountScope()
	layout.AccountScope = scope
	if layout.SchemaVersion <= 0 {
		layout.SchemaVersion = CurrentLayoutSchemaVersion
	}
	if layout.LayoutID == "" {
		layout.LayoutID = "default"
	}
	layout.UpdatedAt = s.clock.Now()
	if err := s.repo.SaveLayout(ctx, layout); err != nil {
		return domain.WorkspaceLayout{}, err
	}
	return layout, nil
}

// LoadOrDefaultLayout loads a saved layout, migrating older schemas or falling back to default presets if corrupt.
func (s *WorkspaceService) LoadOrDefaultLayout(ctx context.Context, layoutID string) domain.WorkspaceLayout {
	scope := s.sessions.Current().AccountScope()
	if layoutID == "" {
		layoutID = "default"
	}
	layout, ok, err := s.repo.GetLayout(ctx, scope, layoutID)
	if err != nil || !ok || len(layout.Windows) == 0 || layout.SchemaVersion > CurrentLayoutSchemaVersion {
		return DefaultBrowseLayout(scope, s.clock.Now())
	}
	return layout
}

// DefaultBrowseLayout builds the default Phase 0 workspace showing a topic list, reading window, and floating composer.
func DefaultBrowseLayout(accountScope string, now time.Time) domain.WorkspaceLayout {
	return domain.WorkspaceLayout{
		SchemaVersion:  CurrentLayoutSchemaVersion,
		LayoutID:       "default",
		Name:           "浏览",
		PresetID:       "browse",
		AccountScope:   accountScope,
		ActiveInstance: "topic-1001",
		UpdatedAt:      now,
		Windows: []domain.WindowState{
			{
				InstanceID:  "feed-latest",
				PanelType:   "topicList",
				Title:       "最新话题",
				ResourceKey: accountScope + ":feed:latest",
				DisplayMode: "docked",
				ViewState:   "normal",
				DockRegion:  "left",
				Bounds:      domain.WindowBounds{X: 0, Y: 0, Width: 360, Height: 640},
				ZIndex:      1,
			},
			{
				InstanceID:    "topic-1001",
				PanelType:     "topic",
				Title:         "欢迎来到 LD Studio：Windows 工作区架构与阅读体验",
				ResourceKey:   accountScope + ":topic:1001",
				DisplayMode:   "floating",
				ViewState:     "normal",
				Bounds:        domain.WindowBounds{X: 380, Y: 24, Width: 620, Height: 520},
				RestoreBounds: domain.WindowBounds{X: 380, Y: 24, Width: 620, Height: 520},
				ZIndex:        10,
				TopicID:       1001,
			},
			{
				InstanceID:    "composer-demo",
				PanelType:     "composer",
				Title:         "回复草稿",
				ResourceKey:   accountScope + ":draft:draft-welcome",
				DisplayMode:   "floating",
				ViewState:     "normal",
				Bounds:        domain.WindowBounds{X: 500, Y: 240, Width: 520, Height: 340},
				RestoreBounds: domain.WindowBounds{X: 500, Y: 240, Width: 520, Height: 340},
				ZIndex:        11,
				DraftID:       "draft-welcome",
				TopicID:       1001,
			},
			{
				InstanceID:    "notifications-main",
				PanelType:     "notifications",
				Title:         "通知",
				ResourceKey:   accountScope + ":notifications",
				DisplayMode:   "floating",
				ViewState:     "minimized",
				Bounds:        domain.WindowBounds{X: 160, Y: 80, Width: 400, Height: 420},
				RestoreBounds: domain.WindowBounds{X: 160, Y: 80, Width: 400, Height: 420},
				ZIndex:        8,
				UnreadBadge:   2,
			},
		},
	}
}
