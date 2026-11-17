package main

import (
	"context"
	"embed"
	"log"
	"net/http"
	"time"

	"github.com/junglecola001/LD-Dt/internal/app"
	"github.com/junglecola001/LD-Dt/internal/discourse"
	"github.com/junglecola001/LD-Dt/internal/domain"
	"github.com/junglecola001/LD-Dt/internal/network"
	"github.com/junglecola001/LD-Dt/internal/session"
	"github.com/junglecola001/LD-Dt/internal/storage"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

type stdlibTransport struct {
	client *http.Client
}

func (t *stdlibTransport) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	return t.client.Do(req.WithContext(ctx))
}

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

func main() {
	clock := domain.SystemClock{}
	sessMgr := session.NewManager("linux.do", clock)
	store := storage.NewMemoryStore()

	emitter := &wailsEventEmitter{}
	httpClient := &http.Client{Timeout: 20 * time.Second}
	coord := network.NewRequestCoordinator(
		network.DefaultConfig("linux.do", "https://linux.do/about.json"),
		clock,
		&stdlibTransport{client: httpClient},
		sessMgr,
		nil,
		emitter,
	)
	adapter := discourse.NewAdapter("https://linux.do", coord)

	authSvc := app.NewAuthService(sessMgr, coord, store, emitter)
	topicSvc := app.NewTopicService(sessMgr, store, adapter, clock, emitter)
	composerSvc := app.NewComposerService(sessMgr, store, nil, clock, emitter)
	notifSvc := app.NewNotificationService(sessMgr, nil, clock)
	workspaceSvc := app.NewWorkspaceService(store, sessMgr, clock)

	wailsApp := application.New(application.Options{
		Name:        "LD Studio",
		Description: "Unofficial Linux.do Windows Desktop Workspace",
		Services: []application.Service{
			application.NewService(authSvc),
			application.NewService(topicSvc),
			application.NewService(composerSvc),
			application.NewService(notifSvc),
			application.NewService(workspaceSvc),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})
	emitter.app = wailsApp

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
