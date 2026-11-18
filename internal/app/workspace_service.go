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

// DefaultBrowseLayout builds a compact, full-space workspace:
// left docked topic list, center docked reading panel, and right full-height vertical notifications strip.
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
				Bounds:      domain.WindowBounds{X: 0, Y: 0, Width: 310, Height: 640},
				ZIndex:      1,
			},
			{
				InstanceID:    "topic-1001",
				PanelType:     "topic",
				Title:         "主题阅读",
				ResourceKey:   accountScope + ":topic:1001",
				DisplayMode:   "docked",
				ViewState:     "normal",
				DockRegion:    "center",
				Bounds:        domain.WindowBounds{X: 318, Y: 0, Width: 560, Height: 640},
				RestoreBounds: domain.WindowBounds{X: 330, Y: 24, Width: 620, Height: 520},
				ZIndex:        2,
				TopicID:       1001,
			},
			{
				InstanceID:    "notifications-main",
				PanelType:     "notifications",
				Title:         "通知",
				ResourceKey:   accountScope + ":notifications",
				DisplayMode:   "docked",
				ViewState:     "normal",
				DockRegion:    "right",
				Bounds:        domain.WindowBounds{X: 886, Y: 0, Width: 268, Height: 640},
				RestoreBounds: domain.WindowBounds{X: 720, Y: 24, Width: 340, Height: 520},
				ZIndex:        3,
			},
		},
	}
}
