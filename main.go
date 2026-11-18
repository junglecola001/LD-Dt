package main

import (
	"context"
	"embed"
	"log"
	"os"
	"path/filepath"

	"github.com/junglecola001/LD-Dt/internal/app"
	"github.com/junglecola001/LD-Dt/internal/discourse"
	"github.com/junglecola001/LD-Dt/internal/domain"
	"github.com/junglecola001/LD-Dt/internal/network"
	"github.com/junglecola001/LD-Dt/internal/platform/shield"
	"github.com/junglecola001/LD-Dt/internal/session"
	"github.com/junglecola001/LD-Dt/internal/storage"
	"github.com/wailsapp/wails/v3/pkg/application"
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

	var openExternal func(string) error
	shieldMgr := shield.NewManager(dataDir, func(u string) error {
		if openExternal != nil {
			return openExternal(u)
		}
		return nil
	})
	defer shieldMgr.Close()

	hybridTransport := shield.NewHybridTransport(shieldMgr, transport)

	coord := network.NewRequestCoordinator(
		network.DefaultConfig("linux.do", "https://linux.do/site.json"),
		clock,
		hybridTransport,
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
	openExternal = wailsApp.Browser.OpenURL

	authSvc.SetHooks(app.AuthBridgeHooks{
		Transport: transport,
		OpenShieldWindow: func(targetURL string) error {
			// Launch an isolated native Edge app window with loopback CDP sync and
			// FluxDO-style browser fetch transport so login + Cloudflare clearance
			// automatically propagate into LD Studio (plan.md 7.3).
			return shieldMgr.LaunchAndWatch(targetURL, func(res shield.SyncResult) {
				go authSvc.SyncAndProbeFromBrowser(context.Background(), res.CookieHeader, res.UserAgent)
			})
		},
		SyncFromShieldWindow: func(ctx context.Context) error {
			res, ready, syncErr := shieldMgr.SyncFromOpenWindow(ctx)
			if syncErr != nil || !ready || res.CookieHeader == "" {
				return syncErr
			}
			return transport.ApplyManualCredentials(domain.CredentialsInput{
				CookieHeader: res.CookieHeader,
				UserAgent:    res.UserAgent,
			})
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
