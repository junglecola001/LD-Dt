package shield

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

type mockFallbackTransport struct {
	called bool
}

func (m *mockFallbackTransport) Do(_ context.Context, _ *http.Request) (*http.Response, error) {
	m.called = true
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"fallback":true}`)),
	}, nil
}

func TestCDPBridgeSyncAndHybridFetch(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/json/version":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"Browser":"Edg/132"}`))
		case "/json/list":
			wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/devtools/page/1"
			_ = json.NewEncoder(w).Encode([]cdpTarget{
				{
					ID:                   "1",
					Type:                 "page",
					Title:                "LINUX DO",
					URL:                  "https://linux.do/latest",
					WebSocketDebuggerURL: wsURL,
				},
			})
		case "/devtools/page/1":
			conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
			if err != nil {
				return
			}
			defer conn.Close(websocket.StatusNormalClosure, "")
			for {
				var req cdpRequest
				if err := wsjson.Read(r.Context(), conn, &req); err != nil {
					return
				}
				switch req.Method {
				case "Network.getCookies":
					_ = wsjson.Write(r.Context(), conn, map[string]any{
						"id": req.ID,
						"result": map[string]any{
							"cookies": []map[string]string{
								{"name": "_t", "value": "secret-token", "domain": "linux.do"},
								{"name": "cf_clearance", "value": "cf-ok", "domain": ".linux.do"},
							},
						},
					})
				case "Runtime.evaluate":
					params, _ := json.Marshal(req.Params)
					var value any
					if strings.Contains(string(params), "isChallenge") {
						value = map[string]any{
							"href":        "https://linux.do/latest",
							"title":       "LINUX DO",
							"ua":          "EdgeMock/132",
							"isChallenge": false,
						}
					} else {
						body := base64.StdEncoding.EncodeToString([]byte(`{"ok":true,"via":"edge"}`))
						value = map[string]any{
							"handled":    true,
							"status":     200,
							"statusText": "OK",
							"headers":    [][]string{{"content-type", "application/json"}},
							"bodyBase64": body,
						}
					}
					_ = wsjson.Write(r.Context(), conn, map[string]any{
						"id": req.ID,
						"result": map[string]any{
							"result": map[string]any{"value": value},
						},
					})
				}
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	mgr := NewManager(t.TempDir(), nil)
	mgr.SetCDPBaseForTesting(srv.URL)

	syncRes, ready, err := mgr.SyncFromOpenWindow(context.Background())
	if err != nil {
		t.Fatalf("SyncFromOpenWindow failed: %v", err)
	}
	if !ready || !syncRes.HasSession || !syncRes.HasClearance {
		t.Fatalf("unexpected sync result: ready=%v res=%+v", ready, syncRes)
	}
	if syncRes.UserAgent != "EdgeMock/132" {
		t.Fatalf("unexpected user-agent: %q", syncRes.UserAgent)
	}
	if !strings.Contains(syncRes.CookieHeader, "_t=secret-token") {
		t.Fatalf("expected _t cookie in %q", syncRes.CookieHeader)
	}

	fb := &mockFallbackTransport{}
	hybrid := NewHybridTransport(mgr, fb)
	req, _ := http.NewRequest(http.MethodGet, "https://linux.do/session/current.json", nil)
	resp, err := hybrid.Do(context.Background(), req)
	if err != nil {
		t.Fatalf("hybrid Do failed: %v", err)
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	if fb.called {
		t.Fatal("expected CDP browser transport to handle request before fallback")
	}
	if !strings.Contains(string(bodyBytes), `"via":"edge"`) {
		t.Fatalf("unexpected response body: %s", string(bodyBytes))
	}

	// Non-linux.do or offline CDP falls back to fallback transport.
	mgr.SetCDPBaseForTesting("")
	resp2, err := hybrid.Do(context.Background(), req)
	if err != nil {
		t.Fatalf("fallback Do failed: %v", err)
	}
	_ = resp2.Body.Close()
	if !fb.called {
		t.Fatal("expected fallback transport when CDP bridge is inactive")
	}
}
