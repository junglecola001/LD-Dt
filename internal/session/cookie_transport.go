package session

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/junglecola001/LD-Dt/internal/domain"
)

const defaultDesktopUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36 Edg/131.0.0.0"

type persistedCreds struct {
	Cookies    []domain.CookieRecord `json:"cookies,omitempty"`
	UserAPIKey string                `json:"userApiKey,omitempty"`
	UserAgent  string                `json:"userAgent,omitempty"`
}

// BrowserLikeTransport executes HTTPS requests to Linux.do using a persistent CookieJar,
// browser-standard headers, and optional Discourse User-Api-Key / cf_clearance credentials.
type BrowserLikeTransport struct {
	mu          sync.RWMutex
	baseURL     *url.URL
	client      *http.Client
	jar         *cookiejar.Jar
	persistPath string
	userAgent   string
	userAPIKey  string
	rawCookies  map[string]domain.CookieRecord
}

// NewBrowserLikeTransport initializes a CookieJar-backed HTTPTransport for the given HTTPS site URL.
func NewBrowserLikeTransport(siteURL, persistPath string) (*BrowserLikeTransport, error) {
	parsed, err := url.Parse(siteURL)
	if err != nil {
		return nil, err
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	t := &BrowserLikeTransport{
		baseURL:     parsed,
		jar:         jar,
		persistPath: persistPath,
		userAgent:   defaultDesktopUserAgent,
		rawCookies:  make(map[string]domain.CookieRecord),
		client: &http.Client{
			Timeout: 25 * time.Second,
			Jar:     jar,
		},
	}
	t.loadFromDisk()
	return t, nil
}

// Do attaches browser-like headers, User-Api-Key (if present), and invokes the underlying HTTP client.
func (t *BrowserLikeTransport) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	t.mu.RLock()
	ua := t.userAgent
	apiKey := t.userAPIKey
	t.mu.RUnlock()

	cloned := req.Clone(ctx)
	if cloned.Header.Get("User-Agent") == "" {
		cloned.Header.Set("User-Agent", ua)
	}
	if cloned.Header.Get("Accept") == "" {
		cloned.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	}
	if cloned.Header.Get("Accept-Language") == "" {
		cloned.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	}
	if cloned.Header.Get("X-Requested-With") == "" {
		cloned.Header.Set("X-Requested-With", "XMLHttpRequest")
	}
	if apiKey != "" && cloned.Header.Get("User-Api-Key") == "" {
		cloned.Header.Set("User-Api-Key", apiKey)
	}

	resp, err := t.client.Do(cloned)
	if err != nil {
		return nil, err
	}
	// Persist updated Set-Cookie headers if any arrived.
	if len(resp.Cookies()) > 0 {
		t.captureResponseCookies(resp.Cookies())
	}
	return resp, nil
}

// ApplyManualCredentials parses a browser Cookie header (e.g. cf_clearance=...; _t=...),
// optional User-Api-Key, and optional User-Agent, normalizing host-only session cookies.
func (t *BrowserLikeTransport) ApplyManualCredentials(input domain.CredentialsInput) error {
	t.mu.Lock()
	if ua := strings.TrimSpace(input.UserAgent); ua != "" {
		t.userAgent = ua
	}
	if key := strings.TrimSpace(input.UserAPIKey); key != "" {
		t.userAPIKey = key
	}

	if raw := strings.TrimSpace(input.CookieHeader); raw != "" {
		raw = strings.TrimPrefix(raw, "Cookie:")
		raw = strings.TrimPrefix(raw, "cookie:")
		pairs := strings.Split(raw, ";")
		var httpCookies []*http.Cookie
		host := t.baseURL.Hostname()
		for _, pair := range pairs {
			pair = strings.TrimSpace(pair)
			if pair == "" {
				continue
			}
			eq := strings.IndexByte(pair, '=')
			if eq <= 0 {
				continue
			}
			name := strings.TrimSpace(pair[:eq])
			val := strings.TrimSpace(pair[eq+1:])
			if name == "" || val == "" {
				continue
			}
			rec := domain.CookieRecord{
				Name:     name,
				Value:    val,
				Domain:   host,
				Path:     "/",
				Expires:  time.Now().Add(30 * 24 * time.Hour),
				Secure:   true,
				HTTPOnly: name == "_t" || name == "_forum_session" || name == "cf_clearance",
				SameSite: "Lax",
			}
			t.rawCookies[name] = rec
			httpCookies = append(httpCookies, &http.Cookie{
				Name:     rec.Name,
				Value:    rec.Value,
				Path:     "/",
				Domain:   host,
				Expires:  rec.Expires,
				Secure:   true,
				HttpOnly: rec.HTTPOnly,
			})
		}
		if len(httpCookies) > 0 {
			t.jar.SetCookies(t.baseURL, httpCookies)
		}
	}
	t.mu.Unlock()
	return t.saveToDisk()
}

// Clear clears stored cookies and User-Api-Key on logout.
func (t *BrowserLikeTransport) Clear() error {
	t.mu.Lock()
	t.userAPIKey = ""
	t.rawCookies = make(map[string]domain.CookieRecord)
	if freshJar, err := cookiejar.New(nil); err == nil {
		t.jar = freshJar
		t.client.Jar = freshJar
	}
	path := t.persistPath
	t.mu.Unlock()
	if path != "" {
		_ = os.Remove(path)
	}
	return nil
}

// HasAuthMaterial reports whether a User-Api-Key or Discourse session cookie is currently loaded.
func (t *BrowserLikeTransport) HasAuthMaterial() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.userAPIKey != "" {
		return true
	}
	if _, ok := t.rawCookies["_t"]; ok {
		return true
	}
	for _, c := range t.jar.Cookies(t.baseURL) {
		if c.Name == "_t" && c.Value != "" {
			return true
		}
	}
	return false
}

func (t *BrowserLikeTransport) captureResponseCookies(cookies []*http.Cookie) {
	t.mu.Lock()
	for _, c := range cookies {
		if c.Value == "" {
			delete(t.rawCookies, c.Name)
			continue
		}
		t.rawCookies[c.Name] = domain.CookieRecord{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   t.baseURL.Hostname(),
			Path:     "/",
			Expires:  c.Expires,
			Secure:   c.Secure,
			HTTPOnly: c.HttpOnly,
		}
	}
	t.mu.Unlock()
	_ = t.saveToDisk()
}

func (t *BrowserLikeTransport) saveToDisk() error {
	t.mu.RLock()
	path := t.persistPath
	if path == "" {
		t.mu.RUnlock()
		return nil
	}
	payload := persistedCreds{
		UserAPIKey: t.userAPIKey,
		UserAgent:  t.userAgent,
	}
	for _, rec := range t.rawCookies {
		payload.Cookies = append(payload.Cookies, rec)
	}
	t.mu.RUnlock()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

func (t *BrowserLikeTransport) loadFromDisk() {
	if t.persistPath == "" {
		return
	}
	raw, err := os.ReadFile(t.persistPath)
	if err != nil {
		return
	}
	var payload persistedCreds
	if err := json.Unmarshal(raw, &payload); err != nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if payload.UserAgent != "" {
		t.userAgent = payload.UserAgent
	}
	t.userAPIKey = payload.UserAPIKey
	var httpCookies []*http.Cookie
	for _, rec := range payload.Cookies {
		if rec.Name == "" || rec.Value == "" {
			continue
		}
		t.rawCookies[rec.Name] = rec
		httpCookies = append(httpCookies, &http.Cookie{
			Name:     rec.Name,
			Value:    rec.Value,
			Path:     "/",
			Domain:   t.baseURL.Hostname(),
			Expires:  rec.Expires,
			Secure:   true,
			HttpOnly: rec.HTTPOnly,
		})
	}
	if len(httpCookies) > 0 {
		t.jar.SetCookies(t.baseURL, httpCookies)
	}
}
