package shield

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/junglecola001/LD-Dt/internal/domain"
)

// SyncResult captures cookies and browser metadata extracted from the isolated Edge window.
type SyncResult struct {
	CookieHeader string
	UserAgent    string
	HasSession   bool
	HasClearance bool
	PageURL      string
}

type cdpTarget struct {
	ID                   string `json:"id"`
	Type                 string `json:"type"`
	Title                string `json:"title"`
	URL                  string `json:"url"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

type cdpCookie struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Domain string `json:"domain"`
}

type cdpRequest struct {
	ID     int64  `json:"id"`
	Method string `json:"method"`
	Params any    `json:"params,omitempty"`
}

type cdpResponse struct {
	ID     int64           `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

var forbiddenBrowserHeaders = map[string]struct{}{
	"accept-charset":                {},
	"accept-encoding":               {},
	"access-control-request-headers": {},
	"access-control-request-method": {},
	"connection":                    {},
	"content-length":                {},
	"cookie":                        {},
	"cookie2":                       {},
	"date":                          {},
	"dnt":                           {},
	"expect":                        {},
	"host":                          {},
	"keep-alive":                    {},
	"origin":                        {},
	"referer":                       {},
	"te":                            {},
	"trailer":                       {},
	"transfer-encoding":             {},
	"upgrade":                       {},
	"user-agent":                    {},
	"via":                           {},
}

// Manager coordinates the isolated Edge --app window, automatic cookie sync over loopback CDP,
// and optional browser-stack HTTP transport (matching FluxDO WebViewHttpAdapter on Windows).
type Manager struct {
	dataDir      string
	fallbackOpen func(string) error

	mu          sync.Mutex
	cdpBaseURL  string
	cdpOrigin   string
	watchCancel context.CancelFunc
	nextID      atomic.Int64

	cdpClient *http.Client
	cdpCallMu sync.Mutex
}

// NewManager creates an isolated Edge CDP shield & login manager.
func NewManager(dataDir string, fallbackOpen func(string) error) *Manager {
	m := &Manager{
		dataDir:      dataDir,
		fallbackOpen: fallbackOpen,
		cdpClient: &http.Client{
			Timeout: 3 * time.Second,
		},
	}
	if port := readPersistedPort(dataDir); port > 0 {
		base := fmt.Sprintf("http://127.0.0.1:%d", port)
		if m.isCDPAlive(context.Background(), base) {
			m.cdpBaseURL = base
			m.cdpOrigin = base
		}
	}
	return m
}

// SetCDPBaseForTesting overrides the loopback CDP HTTP endpoint in unit tests.
func (m *Manager) SetCDPBaseForTesting(baseURL string) {
	m.mu.Lock()
	m.cdpBaseURL = strings.TrimRight(baseURL, "/")
	m.cdpOrigin = strings.TrimRight(baseURL, "/")
	m.mu.Unlock()
}

// HasActiveBrowser reports whether the isolated Edge CDP bridge is currently reachable.
func (m *Manager) HasActiveBrowser(ctx context.Context) bool {
	base, _ := m.cdpEndpoints()
	if base == "" {
		return false
	}
	return m.isCDPAlive(ctx, base)
}

func (m *Manager) cdpEndpoints() (string, string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cdpBaseURL, m.cdpOrigin
}

// LaunchAndWatch opens or navigates the isolated Edge --app window and automatically invokes
// onSynced when Cloudflare challenge clears or a Linux.do login session (_t cookie) appears.
func (m *Manager) LaunchAndWatch(targetURL string, onSynced func(SyncResult)) error {
	parsed, err := validateShieldURL(targetURL)
	if err != nil {
		return err
	}

	if runtime.GOOS != "windows" {
		if m.fallbackOpen == nil {
			return errors.New("未找到可用浏览器打开验证窗口")
		}
		return m.fallbackOpen(parsed.String())
	}

	// Reuse existing live EdgeBridge window if already running.
	base, _ := m.cdpEndpoints()
	if base != "" && m.isCDPAlive(context.Background(), base) {
		if navErr := m.navigateOrOpenTarget(context.Background(), parsed.String()); navErr == nil {
			m.startWatcher(onSynced)
			return nil
		}
	}

	edgePath := findEdgeExecutable()
	if edgePath == "" {
		if m.fallbackOpen == nil {
			return errors.New("未找到 Microsoft Edge 或可用浏览器")
		}
		return m.fallbackOpen(parsed.String())
	}

	port, err := allocateLoopbackPort()
	if err != nil {
		return LaunchWindow(parsed.String(), m.dataDir, m.fallbackOpen)
	}

	if _, startErr := startEdgeAppWindow(edgePath, parsed.String(), m.dataDir, port); startErr != nil {
		if m.fallbackOpen == nil {
			return startErr
		}
		return m.fallbackOpen(parsed.String())
	}

	// Give Edge a brief moment if an already-running EdgeBridge instance absorbed the window on its old port.
	newBase := fmt.Sprintf("http://127.0.0.1:%d", port)
	writePersistedPort(m.dataDir, port)

	m.mu.Lock()
	m.cdpBaseURL = newBase
	m.cdpOrigin = newBase
	m.mu.Unlock()

	m.startWatcher(onSynced)
	return nil
}

// Close cancels any active CDP watcher goroutine.
func (m *Manager) Close() {
	m.mu.Lock()
	if m.watchCancel != nil {
		m.watchCancel()
		m.watchCancel = nil
	}
	m.mu.Unlock()
}

func (m *Manager) startWatcher(onSynced func(SyncResult)) {
	if onSynced == nil {
		return
	}
	m.mu.Lock()
	if m.watchCancel != nil {
		m.watchCancel()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	m.watchCancel = cancel
	m.mu.Unlock()

	go func() {
		defer cancel()
		ticker := time.NewTicker(1200 * time.Millisecond)
		defer ticker.Stop()

		startedAt := time.Now()
		seenAlive := false
		lastSig := ""

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				base, _ := m.cdpEndpoints()
				if base == "" {
					return
				}
				if !m.isCDPAlive(ctx, base) {
					if seenAlive || time.Since(startedAt) > 15*time.Second {
						m.clearIfDead(base)
						return
					}
					continue
				}
				seenAlive = true

				res, ready, syncErr := m.SyncFromOpenWindow(ctx)
				if syncErr != nil || !ready {
					continue
				}

				sig := fmt.Sprintf("%v|%v|%s", res.HasSession, res.HasClearance, res.CookieHeader)
				if sig != lastSig && (res.HasSession || res.HasClearance) {
					lastSig = sig
					onSynced(res)
				}
			}
		}
	}()
}

func (m *Manager) clearIfDead(expectedBase string) {
	m.mu.Lock()
	if m.cdpBaseURL == expectedBase {
		m.cdpBaseURL = ""
		m.cdpOrigin = ""
		if profileDir := edgeBridgeProfileDir(m.dataDir); profileDir != "" {
			_ = os.Remove(filepath.Join(profileDir, "ld-cdp-port.txt"))
		}
	}
	m.mu.Unlock()
}

func (m *Manager) isCDPAlive(ctx context.Context, base string) bool {
	reqCtx, cancel := context.WithTimeout(ctx, 900*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, base+"/json/version", nil)
	if err != nil {
		return false
	}
	resp, err := m.cdpClient.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (m *Manager) listTargets(ctx context.Context, base string) ([]cdpTarget, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, base+"/json/list", nil)
	if err != nil {
		return nil, err
	}
	resp, err := m.cdpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cdp /json/list returned %d", resp.StatusCode)
	}
	var targets []cdpTarget
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&targets); err != nil {
		return nil, err
	}
	return targets, nil
}

func findLinuxDoTarget(targets []cdpTarget) (cdpTarget, bool) {
	var fallback cdpTarget
	hasFallback := false
	for _, t := range targets {
		if t.Type != "page" || t.WebSocketDebuggerURL == "" {
			continue
		}
		if strings.HasPrefix(t.URL, "https://linux.do") {
			if !strings.HasPrefix(t.URL, "https://linux.do/challenge") {
				return t, true
			}
			if !hasFallback {
				fallback = t
				hasFallback = true
			}
		} else if !hasFallback && strings.Contains(t.URL, "linux.do") {
			fallback = t
			hasFallback = true
		}
	}
	return fallback, hasFallback
}

func (m *Manager) navigateOrOpenTarget(ctx context.Context, targetURL string) error {
	base, origin := m.cdpEndpoints()
	if base == "" {
		return errors.New("no active CDP bridge")
	}
	targets, err := m.listTargets(ctx, base)
	if err != nil {
		return err
	}
	var chosen cdpTarget
	for _, t := range targets {
		if t.Type == "page" && t.WebSocketDebuggerURL != "" {
			chosen = t
			break
		}
	}
	if chosen.WebSocketDebuggerURL == "" {
		return errors.New("no debuggable page target")
	}
	m.cdpCallMu.Lock()
	defer m.cdpCallMu.Unlock()

	conn, err := m.dialTarget(ctx, chosen.WebSocketDebuggerURL, origin)
	if err != nil {
		return err
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	if _, err := m.cdpInvoke(ctx, conn, "Page.navigate", map[string]string{"url": targetURL}); err != nil {
		return err
	}
	_, _ = m.cdpInvoke(ctx, conn, "Page.bringToFront", nil)
	return nil
}

// SyncFromOpenWindow reads Linux.do cookies and User-Agent from the isolated Edge window over CDP.
// The boolean return value indicates whether a real https://linux.do document (past Cloudflare challenge) is loaded.
func (m *Manager) SyncFromOpenWindow(ctx context.Context) (SyncResult, bool, error) {
	base, origin := m.cdpEndpoints()
	if base == "" {
		return SyncResult{}, false, nil
	}
	targets, err := m.listTargets(ctx, base)
	if err != nil {
		return SyncResult{}, false, err
	}
	target, ok := findLinuxDoTarget(targets)
	if !ok {
		return SyncResult{}, false, nil
	}

	m.cdpCallMu.Lock()
	defer m.cdpCallMu.Unlock()

	conn, err := m.dialTarget(ctx, target.WebSocketDebuggerURL, origin)
	if err != nil {
		return SyncResult{}, false, err
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	stateRaw, err := m.evalValue(ctx, conn, `({
		href: String(location.href || ''),
		title: String(document.title || ''),
		ua: String(navigator.userAgent || ''),
		isChallenge: Boolean(window._cf_chl_opt) || document.title === 'Just a moment...'
	})`)
	if err != nil {
		return SyncResult{}, false, err
	}
	var pageState struct {
		Href        string `json:"href"`
		Title       string `json:"title"`
		UA          string `json:"ua"`
		IsChallenge bool   `json:"isChallenge"`
	}
	if err := json.Unmarshal(stateRaw, &pageState); err != nil {
		return SyncResult{}, false, err
	}

	cookiesRaw, err := m.cdpInvoke(ctx, conn, "Network.getCookies", map[string]any{
		"urls": []string{"https://linux.do"},
	})
	if err != nil {
		return SyncResult{}, false, err
	}
	var cookiePayload struct {
		Cookies []cdpCookie `json:"cookies"`
	}
	if err := json.Unmarshal(cookiesRaw, &cookiePayload); err != nil {
		return SyncResult{}, false, err
	}

	header, hasSession, hasClearance := formatCookieHeader(cookiePayload.Cookies)
	ready := strings.HasPrefix(pageState.Href, "https://linux.do") && !pageState.IsChallenge
	return SyncResult{
		CookieHeader: header,
		UserAgent:    pageState.UA,
		HasSession:   hasSession,
		HasClearance: hasClearance,
		PageURL:      pageState.Href,
	}, ready, nil
}

func formatCookieHeader(cookies []cdpCookie) (header string, hasSession bool, hasClearance bool) {
	byName := make(map[string]string, len(cookies))
	for _, c := range cookies {
		if c.Name == "" || c.Value == "" {
			continue
		}
		if c.Name == "_t" {
			hasSession = true
		}
		if c.Name == "cf_clearance" {
			hasClearance = true
		}
		byName[c.Name] = c.Value
	}
	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, name+"="+byName[name])
	}
	return strings.Join(parts, "; "), hasSession, hasClearance
}

// FetchViaBrowser executes a same-origin fetch() inside the active Edge linux.do page over CDP
// so requests share Edge's genuine TLS/HTTP3 stack and HttpOnly cookies. Returns handled=false
// when no live cleared linux.do page is available.
func (m *Manager) FetchViaBrowser(ctx context.Context, req *http.Request) (*http.Response, bool, error) {
	if req == nil || req.URL == nil {
		return nil, false, nil
	}
	host := strings.ToLower(req.URL.Hostname())
	if req.URL.Scheme != "https" || (host != "linux.do" && !strings.HasSuffix(host, ".linux.do")) {
		return nil, false, nil
	}

	base, origin := m.cdpEndpoints()
	if base == "" {
		return nil, false, nil
	}
	targets, err := m.listTargets(ctx, base)
	if err != nil {
		m.clearIfDead(base)
		return nil, false, nil
	}
	target, ok := findLinuxDoTarget(targets)
	if !ok {
		return nil, false, nil
	}

	var bodyStr *string
	if req.Body != nil {
		rawBody, readErr := io.ReadAll(io.LimitReader(req.Body, 4<<20))
		if readErr != nil {
			return nil, false, readErr
		}
		_ = req.Body.Close()
		s := string(rawBody)
		bodyStr = &s
	}

	safeHeaders := make(map[string]string)
	for k, vals := range req.Header {
		if len(vals) == 0 {
			continue
		}
		lk := strings.ToLower(strings.TrimSpace(k))
		if _, banned := forbiddenBrowserHeaders[lk]; banned {
			continue
		}
		safeHeaders[k] = strings.Join(vals, ", ")
	}

	urlJSON, _ := json.Marshal(req.URL.String())
	methodJSON, _ := json.Marshal(req.Method)
	headersJSON, _ := json.Marshal(safeHeaders)
	bodyJSON := []byte("null")
	if bodyStr != nil {
		bodyJSON, _ = json.Marshal(*bodyStr)
	}

	script := fmt.Sprintf(`(async () => {
		if (window._cf_chl_opt || document.title === 'Just a moment...' || !location.origin.startsWith('https://linux.do')) {
			return { handled: false };
		}
		const init = {
			method: %s,
			credentials: 'include',
			cache: 'no-store',
			headers: %s
		};
		const rawBody = %s;
		if (rawBody !== null && init.method !== 'GET' && init.method !== 'HEAD') {
			init.body = rawBody;
		}
		const resp = await fetch(%s, init);
		const buf = await resp.arrayBuffer();
		const bytes = new Uint8Array(buf);
		let bin = '';
		const chunk = 0x8000;
		for (let i = 0; i < bytes.length; i += chunk) {
			bin += String.fromCharCode(...bytes.subarray(i, i + chunk));
		}
		const headers = [];
		resp.headers.forEach((v, k) => headers.push([k, v]));
		return {
			handled: true,
			status: resp.status,
			statusText: resp.statusText,
			headers: headers,
			bodyBase64: btoa(bin)
		};
	})()`, string(methodJSON), string(headersJSON), string(bodyJSON), string(urlJSON))

	m.cdpCallMu.Lock()
	defer m.cdpCallMu.Unlock()

	callCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	conn, err := m.dialTarget(callCtx, target.WebSocketDebuggerURL, origin)
	if err != nil {
		return nil, false, nil
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	valRaw, err := m.evalValue(callCtx, conn, script)
	if err != nil {
		return nil, false, nil
	}

	var fetchRes struct {
		Handled    bool       `json:"handled"`
		Status     int        `json:"status"`
		StatusText string     `json:"statusText"`
		Headers    [][]string `json:"headers"`
		BodyBase64 string     `json:"bodyBase64"`
	}
	if err := json.Unmarshal(valRaw, &fetchRes); err != nil || !fetchRes.Handled {
		return nil, false, nil
	}

	decoded, err := base64.StdEncoding.DecodeString(fetchRes.BodyBase64)
	if err != nil {
		return nil, false, err
	}

	respHeaders := make(http.Header, len(fetchRes.Headers))
	for _, pair := range fetchRes.Headers {
		if len(pair) == 2 {
			respHeaders.Add(pair[0], pair[1])
		}
	}

	return &http.Response{
		StatusCode:    fetchRes.Status,
		Status:        fmt.Sprintf("%d %s", fetchRes.Status, fetchRes.StatusText),
		Header:        respHeaders,
		Body:          io.NopCloser(bytes.NewReader(decoded)),
		ContentLength: int64(len(decoded)),
		Request:       req,
	}, true, nil
}

func (m *Manager) dialTarget(ctx context.Context, wsURL, origin string) (*websocket.Conn, error) {
	var opts *websocket.DialOptions
	if origin != "" {
		opts = &websocket.DialOptions{
			HTTPHeader: http.Header{"Origin": {origin}},
		}
	}
	conn, _, err := websocket.Dial(ctx, wsURL, opts)
	if err != nil {
		return nil, err
	}
	conn.SetReadLimit(16 << 20)
	return conn, nil
}

func (m *Manager) cdpInvoke(ctx context.Context, conn *websocket.Conn, method string, params any) (json.RawMessage, error) {
	id := m.nextID.Add(1)
	if err := wsjson.Write(ctx, conn, cdpRequest{
		ID:     id,
		Method: method,
		Params: params,
	}); err != nil {
		return nil, err
	}
	for {
		var msg cdpResponse
		if err := wsjson.Read(ctx, conn, &msg); err != nil {
			return nil, err
		}
		if msg.ID != id {
			// Skip async CDP events emitted before our RPC response.
			continue
		}
		if msg.Error != nil {
			return nil, fmt.Errorf("cdp %s error %d: %s", method, msg.Error.Code, msg.Error.Message)
		}
		return msg.Result, nil
	}
}

func (m *Manager) evalValue(ctx context.Context, conn *websocket.Conn, expr string) (json.RawMessage, error) {
	raw, err := m.cdpInvoke(ctx, conn, "Runtime.evaluate", map[string]any{
		"expression":    expr,
		"awaitPromise":  true,
		"returnByValue": true,
	})
	if err != nil {
		return nil, err
	}
	var outer struct {
		Result struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
		ExceptionDetails *struct {
			Text string `json:"text"`
		} `json:"exceptionDetails"`
	}
	if err := json.Unmarshal(raw, &outer); err != nil {
		return nil, err
	}
	if outer.ExceptionDetails != nil {
		return nil, fmt.Errorf("cdp eval exception: %s", outer.ExceptionDetails.Text)
	}
	return outer.Result.Value, nil
}

// HybridTransport routes Linux.do requests through the live Edge browser network stack when available
// and falls back seamlessly to the persistent Go HTTPTransport when the Edge window is closed.
type HybridTransport struct {
	browser  *Manager
	fallback domain.HTTPTransport
}

// NewHybridTransport wraps a fallback HTTPTransport with Edge CDP browser-stack execution.
func NewHybridTransport(browser *Manager, fallback domain.HTTPTransport) *HybridTransport {
	return &HybridTransport{browser: browser, fallback: fallback}
}

// Do implements domain.HTTPTransport.
func (t *HybridTransport) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	if t.browser != nil {
		if resp, handled, err := t.browser.FetchViaBrowser(ctx, req); err == nil && handled {
			return resp, nil
		}
	}
	return t.fallback.Do(ctx, req)
}
