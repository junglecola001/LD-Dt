package network

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/junglecola001/LD-Dt/internal/domain"
)

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock(t time.Time) *fakeClock {
	return &fakeClock{now: t}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func (c *fakeClock) After(d time.Duration) <-chan time.Time {
	ch := make(chan time.Time, 1)
	c.Advance(d)
	ch <- c.Now()
	return ch
}

type mockTransport struct {
	handler func(req *http.Request) (*http.Response, error)
	calls   atomic.Int32
}

func (m *mockTransport) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	m.calls.Add(1)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return m.handler(req)
}

type mockSessionStore struct {
	gen atomic.Int64
}

func (m *mockSessionStore) Generation() int64 {
	return m.gen.Load()
}

func (m *mockSessionStore) ApplySyncedCookies(string, int64, []domain.CookieRecord) error { return nil }
func (m *mockSessionStore) ExportCookies(string) []domain.CookieRecord                   { return nil }
func (m *mockSessionStore) ClearSite(string) error                                       { return nil }

type mockChallengeHost struct {
	calls atomic.Int32
	fn    func(ctx context.Context) error
}

func (m *mockChallengeHost) RunChallenge(ctx context.Context, _ string, _ int64) error {
	m.calls.Add(1)
	if m.fn != nil {
		return m.fn(ctx)
	}
	return nil
}

func makeHTTPResp(status int, contentType string, headers map[string]string, body string) *http.Response {
	h := make(http.Header)
	if contentType != "" {
		h.Set("Content-Type", contentType)
	}
	for k, v := range headers {
		h.Set(k, v)
	}
	return &http.Response{
		StatusCode: status,
		Header:     h,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestResponseClassifierMatrix(t *testing.T) {
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	cls := ResponseClassifier{}

	t.Run("plain json 429 with retry-after seconds", func(t *testing.T) {
		resp := makeHTTPResp(429, "application/json", map[string]string{"Retry-After": "45"}, `{"errors":["Too many"]}`)
		out := cls.Classify(resp, []byte(`{"errors":["Too many"]}`), true, now)
		if out.Kind != ClassRateLimited || !out.HasValidWait || out.WaitDuration != 45*time.Second {
			t.Fatalf("unexpected classification: %+v", out)
		}
	})

	t.Run("retry-after http-date respects server Date header", func(t *testing.T) {
		serverNow := now.Add(5 * time.Minute)
		retryAt := serverNow.Add(90 * time.Second)
		resp := makeHTTPResp(429, "application/json", map[string]string{
			"Date":        serverNow.Format(http.TimeFormat),
			"Retry-After": retryAt.Format(http.TimeFormat),
		}, `{}`)
		out := cls.Classify(resp, []byte(`{}`), true, now)
		if out.Kind != ClassRateLimited || !out.HasValidWait || out.WaitDuration != 90*time.Second {
			t.Fatalf("expected 90s wait using server Date, got %+v", out)
		}
	})

	t.Run("invalid or past retry-after marks wait invalid", func(t *testing.T) {
		for _, val := range []string{"garbage", "0", "-5", now.Add(-10 * time.Second).Format(http.TimeFormat)} {
			resp := makeHTTPResp(429, "application/json", map[string]string{"Retry-After": val}, `{}`)
			out := cls.Classify(resp, []byte(`{}`), true, now)
			if out.Kind != ClassRateLimited || out.HasValidWait {
				t.Fatalf("expected invalid wait for %q, got %+v", val, out)
			}
		}
	})

	t.Run("discourse json extras wait_seconds", func(t *testing.T) {
		body := `{"extras":{"wait_seconds":18}}`
		resp := makeHTTPResp(429, "application/json", nil, body)
		out := cls.Classify(resp, []byte(body), true, now)
		if out.Kind != ClassRateLimited || !out.HasValidWait || out.WaitDuration != 18*time.Second {
			t.Fatalf("unexpected discourse wait classification: %+v", out)
		}
	})

	t.Run("json containing challenge phrase is NOT misclassified as CF challenge", func(t *testing.T) {
		body := `{"raw":"<title>Just a moment...</title> /cdn-cgi/challenge-platform/"}`
		resp := makeHTTPResp(403, "application/json", nil, body)
		out := cls.Classify(resp, []byte(body), true, now)
		if out.Kind != ClassForbidden {
			t.Fatalf("expected plain forbidden for json 403, got %+v", out)
		}
	})

	t.Run("cf-mitigated challenge header on 403", func(t *testing.T) {
		resp := makeHTTPResp(403, "text/html", map[string]string{"cf-mitigated": "challenge"}, "<html></html>")
		out := cls.Classify(resp, []byte("<html></html>"), true, now)
		if out.Kind != ClassChallengeRequired {
			t.Fatalf("expected challenge required, got %+v", out)
		}
	})

	t.Run("200 html login page when expecting json maps to auth expired", func(t *testing.T) {
		body := `<html><body data-discourse-login="true">login</body></html>`
		resp := makeHTTPResp(200, "text/html; charset=utf-8", nil, body)
		out := cls.Classify(resp, []byte(body), true, now)
		if out.Kind != ClassAuthExpired {
			t.Fatalf("expected auth expired, got %+v", out)
		}
	})
}

func TestRequestCoordinatorDedup10Windows(t *testing.T) {
	clk := newFakeClock(time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC))
	sess := &mockSessionStore{}
	sess.gen.Store(1)

	started := make(chan struct{})
	release := make(chan struct{})
	tr := &mockTransport{
		handler: func(req *http.Request) (*http.Response, error) {
			close(started)
			<-release
			return makeHTTPResp(200, "application/json", nil, `{"id":42}`), nil
		},
	}

	coord := NewRequestCoordinator(DefaultConfig("linux.do", ""), clk, tr, sess, nil, nil)
	coord.jitterFn = func(time.Duration) time.Duration { return 0 }

	var wg sync.WaitGroup
	errs := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := coord.DoSafeGET(context.Background(), "topic:42", true, func() (*http.Request, error) {
				return http.NewRequest(http.MethodGet, "https://linux.do/t/42.json", nil)
			})
			if err != nil {
				errs <- err
				return
			}
			if string(res.Body) != `{"id":42}` {
				errs <- errors.New("unexpected body")
			}
		}()
	}

	<-started
	close(release)
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("unexpected error: %v", err)
	}
	if tr.calls.Load() != 1 {
		t.Fatalf("expected 1 deduplicated HTTP call across 10 windows, got %d", tr.calls.Load())
	}
}

func TestRequestCoordinatorChallengeCoalescingAndPausedFallback(t *testing.T) {
	clk := newFakeClock(time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC))
	sess := &mockSessionStore{}
	sess.gen.Store(1)

	tr := &mockTransport{
		handler: func(req *http.Request) (*http.Response, error) {
			return makeHTTPResp(403, "text/html", map[string]string{"cf-mitigated": "challenge"}, "<html>challenge</html>"), nil
		},
	}
	chHost := &mockChallengeHost{}

	cfg := DefaultConfig("linux.do", "https://linux.do/session/current.json")
	cfg.MinDispatchInterval = 0
	coord := NewRequestCoordinator(cfg, clk, tr, sess, chHost, nil)

	var wg sync.WaitGroup
	results := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := coord.DoSafeGET(context.Background(), "topic:"+string(rune('a'+idx)), true, func() (*http.Request, error) {
				return http.NewRequest(http.MethodGet, "https://linux.do/t/1.json", nil)
			})
			results <- err
		}(i)
	}
	wg.Wait()
	close(results)

	if chHost.calls.Load() != 1 {
		t.Fatalf("expected exactly 1 shared challenge round for 10 concurrent failures, got %d", chHost.calls.Load())
	}
	for err := range results {
		var appErr *domain.AppError
		if !errors.As(err, &appErr) || appErr.Code != domain.ErrCodeChallengePaused {
			t.Fatalf("expected ErrCodeChallengePaused when probe still blocked, got %v", err)
		}
	}
}

func TestRequestCoordinatorLong429CooldownAndWriteSafety(t *testing.T) {
	clk := newFakeClock(time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC))
	sess := &mockSessionStore{}
	sess.gen.Store(1)

	tr := &mockTransport{
		handler: func(req *http.Request) (*http.Response, error) {
			return makeHTTPResp(429, "application/json", map[string]string{"Retry-After": "90"}, `{}`), nil
		},
	}
	cfg := DefaultConfig("linux.do", "")
	cfg.MinDispatchInterval = 0
	coord := NewRequestCoordinator(cfg, clk, tr, sess, nil, nil)

	_, err := coord.DoSafeGET(context.Background(), "feed:latest", true, func() (*http.Request, error) {
		return http.NewRequest(http.MethodGet, "https://linux.do/latest.json", nil)
	})
	var appErr *domain.AppError
	if !errors.As(err, &appErr) || appErr.Code != domain.ErrCodeRateLimited || appErr.RetryAfterSeconds != 90 {
		t.Fatalf("expected 90s rate limit error, got %v", err)
	}

	// Subsequent request during cooldown must NOT hit transport.
	_, err2 := coord.DoSafeGET(context.Background(), "feed:top", true, func() (*http.Request, error) {
		return http.NewRequest(http.MethodGet, "https://linux.do/top.json", nil)
	})
	if !errors.As(err2, &appErr) || appErr.Code != domain.ErrCodeRateLimited {
		t.Fatalf("expected immediate cooldown rejection, got %v", err2)
	}
	if tr.calls.Load() != 1 {
		t.Fatalf("expected 1 transport call total during cooldown, got %d", tr.calls.Load())
	}

	// Advance past cooldown and test DoWrite transport timeout -> outcomeUnknown (never auto-retried).
	clk.Advance(95 * time.Second)
	writeTr := &mockTransport{
		handler: func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("connection reset after write")
		},
	}
	coord.transport = writeTr
	_, writeErr := coord.DoWrite(context.Background(), true, func() (*http.Request, error) {
		return http.NewRequest(http.MethodPost, "https://linux.do/posts.json", strings.NewReader(`{}`))
	})
	if !errors.As(writeErr, &appErr) || appErr.Code != domain.ErrCodeOutcomeUnknown {
		t.Fatalf("expected ErrCodeOutcomeUnknown on write transport failure, got %v", writeErr)
	}
	if writeTr.calls.Load() != 1 {
		t.Fatalf("DoWrite must never auto-retry, got %d calls", writeTr.calls.Load())
	}
}
