package main

import (
	"context"
	"embed"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/junglecola001/LD-Dt/internal/app"
	"github.com/junglecola001/LD-Dt/internal/discourse"
	"github.com/junglecola001/LD-Dt/internal/domain"
	"github.com/junglecola001/LD-Dt/internal/network"
	"github.com/junglecola001/LD-Dt/internal/session"
	"github.com/junglecola001/LD-Dt/internal/storage"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

type wailsEventEmitter struct {
	app *application.App
}

func (e *wailsEventEmitter) Emit(eventName string, payload any) {
	if e.app != nil {
		e.app.Event.Emit(eventName, payload)
	}
}

func init() {
	application.RegisterEvent[domain.NetworkStatusSnapshot]("NetworkStateChanged")
	application.RegisterEvent[domain.SiteSession]("SessionChanged")
	application.RegisterEvent[domain.Draft]("DraftSaved")
}

func appDataDir() string {
	if base := os.Getenv("LOCALAPPDATA"); base != "" {
		return filepath.Join(base, "LD-Studio")
	}
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return filepath.Join(dir, "LD-Studio")
	}
	return filepath.Join(".", ".ld-studio-data")
}

func main() {
	dataDir := appDataDir()
	webviewDataDir := filepath.Join(dataDir, "WebView2")
	credsPath := filepath.Join(dataDir, "credentials.json")

	clock := domain.SystemClock{}
	sessMgr := session.NewManager("linux.do", clock)
	store := storage.NewMemoryStore()
	emitter := &wailsEventEmitter{}

	transport, err := session.NewBrowserLikeTransport("https://linux.do", credsPath)
	if err != nil {
		log.Fatalf("init transport: %v", err)
	}

	coord := network.NewRequestCoordinator(
		network.DefaultConfig("linux.do", "https://linux.do/site.json"),
		clock,
		transport,
		sessMgr,
		nil,
		emitter,
	)
	adapter := discourse.NewAdapter("https://linux.do", coord)

	authSvc := app.NewAuthService(sessMgr, coord, store, emitter)
	topicSvc := app.NewTopicService(sessMgr, store, adapter, clock, emitter)
	searchSvc := app.NewSearchService(sessMgr, adapter)
	composerSvc := app.NewComposerService(sessMgr, store, nil, clock, emitter)
	notifSvc := app.NewNotificationService(sessMgr, adapter, clock)
	workspaceSvc := app.NewWorkspaceService(store, sessMgr, clock)

	wailsApp := application.New(application.Options{
		Name:        "LD Studio",
		Description: "Unofficial Linux.do Windows Desktop Workspace",
		Services: []application.Service{
			application.NewService(authSvc),
			application.NewService(topicSvc),
			application.NewService(searchSvc),
			application.NewService(composerSvc),
			application.NewService(notifSvc),
			application.NewService(workspaceSvc),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Windows: application.WindowsOptions{
			WebviewUserDataPath: webviewDataDir,
		},
	})
	emitter.app = wailsApp

	var shieldMu sync.Mutex
	var shieldWin *application.WebviewWindow

	authSvc.SetHooks(app.AuthBridgeHooks{
		Transport: transport,
		OpenShieldWindow: func(targetURL string) error {
			shieldMu.Lock()
			defer shieldMu.Unlock()
			if shieldWin != nil {
				shieldWin.SetURL(targetURL)
				shieldWin.Show()
				shieldWin.Focus()
				return nil
			}
			// Isolated secondary WebView2 window with AllowSimpleEventEmit disabled (plan.md 7.3).
			shieldWin = wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
				Name:                 "ld-shield-host",
				Title:                "LD Studio · 站点验证与登录",
				Width:                540,
				Height:               680,
				AlwaysOnTop:          true,
				AllowSimpleEventEmit: false,
				BackgroundColour:     application.NewRGB(250, 249, 245),
				URL:                  targetURL,
			})
			shieldWin.OnWindowEvent(events.Common.WindowClosing, func(_ *application.WindowEvent) {
				shieldMu.Lock()
				shieldWin = nil
				shieldMu.Unlock()
			})
			shieldWin.Center()
			shieldWin.Show()
			return nil
		},
		OpenExternalURL: func(targetURL string) error {
			return wailsApp.Browser.OpenURL(targetURL)
		},
		ProbeCurrentUser: func(ctx context.Context) (string, string, string, error) {
			return adapter.FetchCurrentUser(ctx)
		},
		ProbeSiteMetadata: func(ctx context.Context) error {
			_, err := topicSvc.GetSiteMetadata(ctx)
			return err
		},
	})

	wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "LD Studio",
		Width:            1366,
		Height:           820,
		MinWidth:         800,
		MinHeight:        520,
		BackgroundColour: application.NewRGB(237, 234, 227), // warm paper #EDEAE3
		URL:              "/",
	})

	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}
