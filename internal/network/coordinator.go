package network

import (
	"context"
	"errors"
	"io"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/junglecola001/LD-Dt/internal/domain"
)

const (
	defaultMaxConcurrent       = 2
	defaultMinDispatchInterval = 750 * time.Millisecond
	defaultSlidingWindow       = 3 * time.Second
	defaultMaxPerWindow        = 4
	defaultMaxQueued           = 50
	defaultSafeGETMaxAttempts  = 3
	defaultLogicalBudget       = 30 * time.Second
	defaultUnknown429Cooldown  = 60 * time.Second
	defaultAutoProbeMaxWait    = 30 * time.Second
	defaultChallengeTimeout    = 120 * time.Second
	defaultChallengeCooldown   = 60 * time.Second
)

// CoordinatorConfig tunes conservative Phase 0 budgets (plan.md 7.7).
type CoordinatorConfig struct {
	SiteID              string
	ProbeURL            string
	MaxConcurrent       int
	MinDispatchInterval time.Duration
	SlidingWindow       time.Duration
	MaxPerWindow        int
	MaxQueued           int
	SafeGETMaxAttempts  int
	LogicalBudget       time.Duration
	Unknown429Cooldown  time.Duration
	AutoProbeMaxWait    time.Duration
	ChallengeTimeout    time.Duration
	ChallengeCooldown   time.Duration
}

// DefaultConfig returns conservative Phase 0 rate-limit parameters for Linux.do.
func DefaultConfig(siteID, probeURL string) CoordinatorConfig {
	return CoordinatorConfig{
		SiteID:              siteID,
		ProbeURL:            probeURL,
		MaxConcurrent:       defaultMaxConcurrent,
		MinDispatchInterval: defaultMinDispatchInterval,
		SlidingWindow:       defaultSlidingWindow,
		MaxPerWindow:        defaultMaxPerWindow,
		MaxQueued:           defaultMaxQueued,
		SafeGETMaxAttempts:  defaultSafeGETMaxAttempts,
		LogicalBudget:       defaultLogicalBudget,
		Unknown429Cooldown:  defaultUnknown429Cooldown,
		AutoProbeMaxWait:    defaultAutoProbeMaxWait,
		ChallengeTimeout:    defaultChallengeTimeout,
		ChallengeCooldown:   defaultChallengeCooldown,
	}
}

// RawResponse encapsulates a verified response body after classification.
type RawResponse struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

type inFlightGET struct {
	mu       sync.Mutex
	waiters  int
	cancelFn context.CancelFunc
	done     chan struct{}
	resp     RawResponse
	err      error
}

type activeChallenge struct {
	done chan struct{}
	err  error
}

// RequestCoordinator enforces site-wide rate limits, GET deduplication, challenge coalescing, and session isolation.
// Contrasts with FluxDO F2/F5 by making 429 cooldowns global across all request types and refusing automatic write replay.
type RequestCoordinator struct {
	cfg        CoordinatorConfig
	clock      domain.Clock
	transport  domain.HTTPTransport
	sessions   domain.SessionCookieStore
	classifier ResponseClassifier
	challenge  domain.ChallengeHost
	events     domain.EventEmitter

	mu                     sync.Mutex
	phase                  domain.NetworkPhase
	blockedUntil           time.Time
	challengeCooldownUntil time.Time
	lastErrorCode          domain.ErrCode
	statusMessage          string
	inFlightCount          int
	queuedCount            int
	lastDispatchAt         time.Time
	recentDispatches       []time.Time
	dedupGETs              map[string]*inFlightGET
	currentChallenge       *activeChallenge
	jitterFn               func(max time.Duration) time.Duration
}

// NewRequestCoordinator builds a shared site coordinator.
func NewRequestCoordinator(
	cfg CoordinatorConfig,
	clock domain.Clock,
	transport domain.HTTPTransport,
	sessions domain.SessionCookieStore,
	challenge domain.ChallengeHost,
	events domain.EventEmitter,
) *RequestCoordinator {
	if clock == nil {
		clock = domain.SystemClock{}
	}
	return &RequestCoordinator{
		cfg:       cfg,
		clock:     clock,
		transport: transport,
		sessions:  sessions,
		challenge: challenge,
		events:    events,
		phase:     domain.NetworkReady,
		dedupGETs: make(map[string]*inFlightGET),
		jitterFn:  defaultJitter,
	}
}

func defaultJitter(max time.Duration) time.Duration {
	if max <= 0 {
		return 0
	}
	return time.Duration(rand.Int63n(int64(max)))
}

// Status returns the single source of truth for the UI status bar.
func (c *RequestCoordinator) Status() domain.NetworkStatusSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.clock.Now()
	phase := c.phase
	var retrySecs int
	var blockedUnix int64
	if !c.blockedUntil.IsZero() && c.blockedUntil.After(now) {
		blockedUnix = c.blockedUntil.Unix()
		retrySecs = int(c.blockedUntil.Sub(now).Seconds())
		if retrySecs < 1 {
			retrySecs = 1
		}
		if phase == domain.NetworkReady {
			phase = domain.NetworkCoolingDown
		}
	} else if phase == domain.NetworkCoolingDown {
		phase = domain.NetworkReady
	}
	return domain.NetworkStatusSnapshot{
		SiteID:            c.cfg.SiteID,
		Phase:             phase,
		BlockedUntilUnix:  blockedUnix,
		RetryAfterSeconds: retrySecs,
		InFlightCount:     c.inFlightCount,
		QueuedCount:       c.queuedCount,
		LastErrorCode:     c.lastErrorCode,
		Message:           c.statusMessage,
	}
}

// DoSafeGET executes a deduplicated, budgeted read request. Cancelling a single caller does not abort
// the underlying GET if other windows are still awaiting the same key.
func (c *RequestCoordinator) DoSafeGET(ctx context.Context, dedupKey string, expectJSON bool, buildReq func() (*http.Request, error)) (RawResponse, error) {
	startGen := c.currentGen()

	c.mu.Lock()
	if existing, ok := c.dedupGETs[dedupKey]; ok {
		existing.mu.Lock()
		existing.waiters++
		existing.mu.Unlock()
		c.mu.Unlock()
		return c.awaitDedupGET(ctx, dedupKey, existing, startGen)
	}

	reqCtx, cancel := context.WithCancel(context.Background())
	flight := &inFlightGET{
		waiters:  1,
		cancelFn: cancel,
		done:     make(chan struct{}),
	}
	c.dedupGETs[dedupKey] = flight
	c.mu.Unlock()

	go func() {
		defer close(flight.done)
		resp, err := c.executeSafeGETWithBudget(reqCtx, startGen, expectJSON, buildReq)
		flight.resp = resp
		flight.err = err

		c.mu.Lock()
		if c.dedupGETs[dedupKey] == flight {
			delete(c.dedupGETs, dedupKey)
		}
		c.mu.Unlock()
	}()

	return c.awaitDedupGET(ctx, dedupKey, flight, startGen)
}

func (c *RequestCoordinator) awaitDedupGET(ctx context.Context, dedupKey string, flight *inFlightGET, startGen int64) (RawResponse, error) {
	select {
	case <-flight.done:
		if c.currentGen() != startGen {
			return RawResponse{}, domain.NewError(domain.ErrCodeStaleSession, "会话已切换，丢弃旧请求结果", true, nil)
		}
		return flight.resp, flight.err
	case <-ctx.Done():
		flight.mu.Lock()
		flight.waiters--
		remaining := flight.waiters
		flight.mu.Unlock()
		if remaining == 0 {
			flight.cancelFn()
			c.mu.Lock()
			if c.dedupGETs[dedupKey] == flight {
				delete(c.dedupGETs, dedupKey)
			}
			c.mu.Unlock()
		}
		return RawResponse{}, domain.NewError(domain.ErrCodeCancelled, "请求已取消", true, ctx.Err())
	}
}

// DoWrite executes a non-idempotent mutation once through the site gate.
// It NEVER auto-retries on 429 or transport timeout after potential delivery (plan.md 7.6 rule 7).
func (c *RequestCoordinator) DoWrite(ctx context.Context, expectJSON bool, buildReq func() (*http.Request, error)) (RawResponse, error) {
	startGen := c.currentGen()

	if err := c.checkPreDispatchCooldown(ctx); err != nil {
		return RawResponse{}, err
	}

	raw, cls, err := c.dispatchOnce(ctx, startGen, expectJSON, buildReq)
	if err != nil {
		if appErr, ok := err.(*domain.AppError); ok && appErr.Code == domain.ErrCodeNetworkOffline {
			// A transport error after sending a write may have reached the server.
			return RawResponse{}, domain.NewError(domain.ErrCodeOutcomeUnknown, "写入结果待确认，请核对服务端状态后再决定是否重发", true, err)
		}
		return RawResponse{}, err
	}

	switch cls.Kind {
	case ClassOK:
		return raw, nil
	case ClassRateLimited:
		c.applyRateLimit(cls)
		return RawResponse{}, c.rateLimitError(cls)
	case ClassChallengeRequired:
		c.applyChallengeState(cls)
		return RawResponse{}, domain.NewError(domain.ErrCodeChallengeRequired, "需要完成站点验证后由用户重新发起提交", true, nil)
	default:
		return RawResponse{}, c.mapClassificationToError(cls)
	}
}

func (c *RequestCoordinator) executeSafeGETWithBudget(ctx context.Context, startGen int64, expectJSON bool, buildReq func() (*http.Request, error)) (RawResponse, error) {
	deadline := c.clock.Now().Add(c.cfg.LogicalBudget)
	attempts := 0
	probedAfter429 := false
	probedAfterChallenge := false

	for {
		if ctx.Err() != nil {
			return RawResponse{}, domain.NewError(domain.ErrCodeCancelled, "请求已取消", true, ctx.Err())
		}
		if c.currentGen() != startGen {
			return RawResponse{}, domain.NewError(domain.ErrCodeStaleSession, "会话已切换", true, nil)
		}
		if c.clock.Now().After(deadline) {
			return RawResponse{}, domain.NewError(domain.ErrCodeNetworkOffline, "请求超出总等待预算", true, nil)
		}

		if err := c.checkPreDispatchCooldown(ctx); err != nil {
			return RawResponse{}, err
		}

		attempts++
		raw, cls, err := c.dispatchOnce(ctx, startGen, expectJSON, buildReq)
		if err != nil {
			var appErr *domain.AppError
			if errors.As(err, &appErr) && appErr.Code == domain.ErrCodeNetworkOffline && attempts < c.cfg.SafeGETMaxAttempts {
				backoff := time.Duration(attempts) * time.Second
				if c.jitterFn != nil {
					backoff += c.jitterFn(500 * time.Millisecond)
				}
				if c.clock.Now().Add(backoff).After(deadline) {
					return RawResponse{}, err
				}
				if sleepErr := c.sleep(ctx, backoff); sleepErr != nil {
					return RawResponse{}, sleepErr
				}
				continue
			}
			return RawResponse{}, err
		}

		switch cls.Kind {
		case ClassOK:
			c.markReady()
			return raw, nil

		case ClassRateLimited:
			c.applyRateLimit(cls)
			if probedAfter429 || !cls.HasValidWait || cls.WaitDuration > c.cfg.AutoProbeMaxWait {
				return RawResponse{}, c.rateLimitError(cls)
			}
			if c.clock.Now().Add(cls.WaitDuration).After(deadline) {
				return RawResponse{}, c.rateLimitError(cls)
			}
			probedAfter429 = true
			// Sleep outside concurrency slots before performing the single allowed probe.
			if sleepErr := c.sleep(ctx, cls.WaitDuration); sleepErr != nil {
				return RawResponse{}, sleepErr
			}
			continue

		case ClassChallengeRequired:
			if cls.HasValidWait && cls.WaitDuration > 0 {
				c.applyRateLimit(cls)
				if cls.WaitDuration > c.cfg.AutoProbeMaxWait || c.clock.Now().Add(cls.WaitDuration).After(deadline) {
					return RawResponse{}, c.rateLimitError(cls)
				}
				if sleepErr := c.sleep(ctx, cls.WaitDuration); sleepErr != nil {
					return RawResponse{}, sleepErr
				}
			}
			if probedAfterChallenge {
				c.enterChallengePaused()
				return RawResponse{}, domain.NewError(domain.ErrCodeChallengePaused, "站点验证后仍被拦截，请稍后再试或使用原站访问", true, nil)
			}
			probedAfterChallenge = true
			if chErr := c.runSharedChallengeAndProbe(ctx, startGen); chErr != nil {
				return RawResponse{}, chErr
			}
			// Probe succeeded; now retry the original GET once.
			continue

		case ClassServerError:
			if attempts < c.cfg.SafeGETMaxAttempts {
				backoff := time.Duration(attempts) * time.Second
				if c.jitterFn != nil {
					backoff += c.jitterFn(500 * time.Millisecond)
				}
				if c.clock.Now().Add(backoff).After(deadline) {
					return RawResponse{}, c.mapClassificationToError(cls)
				}
				if sleepErr := c.sleep(ctx, backoff); sleepErr != nil {
					return RawResponse{}, sleepErr
				}
				continue
			}
			c.setPhase(domain.NetworkOffline, domain.ErrCodeServerError, cls.Message)
			return RawResponse{}, c.mapClassificationToError(cls)

		default:
			if cls.Kind == ClassAuthExpired {
				c.setPhase(domain.NetworkAuthExpired, domain.ErrCodeAuthExpired, cls.Message)
			} else if cls.Kind == ClassForbidden {
				c.setPhase(domain.NetworkForbidden, domain.ErrCodeForbidden, cls.Message)
			}
			return RawResponse{}, c.mapClassificationToError(cls)
		}
	}
}

func (c *RequestCoordinator) runSharedChallengeAndProbe(ctx context.Context, startGen int64) error {
	c.mu.Lock()
	now := c.clock.Now()
	if !c.challengeCooldownUntil.IsZero() && c.challengeCooldownUntil.After(now) {
		c.mu.Unlock()
		return domain.NewError(domain.ErrCodeChallengePaused, "验证冷却中，请稍后重试", true, nil)
	}
	if c.currentChallenge != nil {
		ch := c.currentChallenge
		c.mu.Unlock()
		select {
		case <-ch.done:
			return ch.err
		case <-ctx.Done():
			return domain.NewError(domain.ErrCodeCancelled, "验证等待已取消", true, ctx.Err())
		}
	}

	ch := &activeChallenge{done: make(chan struct{})}
	c.currentChallenge = ch
	c.phase = domain.NetworkChallengeRequired
	c.lastErrorCode = domain.ErrCodeChallengeRequired
	c.statusMessage = "需要完成站点验证"
	c.mu.Unlock()
	c.emitStatus()

	var challengeErr error
	defer func() {
		ch.err = challengeErr
		close(ch.done)
		c.mu.Lock()
		if c.currentChallenge == ch {
			c.currentChallenge = nil
		}
		c.mu.Unlock()
	}()

	if c.challenge == nil {
		challengeErr = domain.NewError(domain.ErrCodeChallengeRequired, "未配置验证宿主", false, nil)
		return challengeErr
	}

	chCtx, cancel := context.WithTimeout(ctx, c.cfg.ChallengeTimeout)
	defer cancel()

	if err := c.challenge.RunChallenge(chCtx, c.cfg.SiteID, startGen); err != nil {
		c.mu.Lock()
		c.challengeCooldownUntil = c.clock.Now().Add(c.cfg.ChallengeCooldown)
		c.phase = domain.NetworkChallengePaused
		c.lastErrorCode = domain.ErrCodeChallengePaused
		c.statusMessage = "验证未完成，已进入冷却"
		c.mu.Unlock()
		c.emitStatus()
		challengeErr = domain.NewError(domain.ErrCodeChallengeRequired, "验证未完成", true, err)
		return challengeErr
	}

	if c.currentGen() != startGen {
		challengeErr = domain.NewError(domain.ErrCodeStaleSession, "会话已切换", true, nil)
		return challengeErr
	}

	// Single safe GET probe after session sync; never release queued flood before probe succeeds.
	if c.cfg.ProbeURL != "" {
		_, cls, err := c.dispatchOnce(ctx, startGen, true, func() (*http.Request, error) {
			return http.NewRequest(http.MethodGet, c.cfg.ProbeURL, nil)
		})
		if err != nil || cls.Kind != ClassOK {
			c.enterChallengePaused()
			if err != nil {
				challengeErr = err
			} else {
				challengeErr = domain.NewError(domain.ErrCodeChallengePaused, "验证后接口仍被阻止，已暂停自动试探", true, nil)
			}
			return challengeErr
		}
	}

	c.markReady()
	return nil
}

func (c *RequestCoordinator) checkPreDispatchCooldown(ctx context.Context) error {
	c.mu.Lock()
	now := c.clock.Now()
	blocked := c.blockedUntil
	c.mu.Unlock()
	if !blocked.IsZero() && blocked.After(now) {
		secs := int(blocked.Sub(now).Seconds())
		if secs < 1 {
			secs = 1
		}
		err := domain.NewError(domain.ErrCodeRateLimited, "请求冷却中，请稍后重试", true, nil)
		err.RetryAfterSeconds = secs
		return err
	}
	if ctx.Err() != nil {
		return domain.NewError(domain.ErrCodeCancelled, "请求已取消", true, ctx.Err())
	}
	return nil
}

func (c *RequestCoordinator) dispatchOnce(ctx context.Context, expectedGen int64, expectJSON bool, buildReq func() (*http.Request, error)) (RawResponse, Classification, error) {
	if err := c.acquireSlot(ctx); err != nil {
		return RawResponse{}, Classification{}, err
	}
	defer c.releaseSlot()

	if c.currentGen() != expectedGen {
		return RawResponse{}, Classification{}, domain.NewError(domain.ErrCodeStaleSession, "会话已切换", true, nil)
	}

	req, err := buildReq()
	if err != nil {
		return RawResponse{}, Classification{}, domain.NewError(domain.ErrCodeValidationFailed, "构建请求失败", false, err)
	}

	resp, err := c.transport.Do(ctx, req)
	if err != nil {
		if ctx.Err() != nil {
			return RawResponse{}, Classification{}, domain.NewError(domain.ErrCodeCancelled, "请求已取消", true, ctx.Err())
		}
		return RawResponse{}, Classification{}, domain.NewError(domain.ErrCodeNetworkOffline, "网络连接失败", true, err)
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if readErr != nil {
		return RawResponse{}, Classification{}, domain.NewError(domain.ErrCodeNetworkOffline, "读取响应失败", true, readErr)
	}

	cls := c.classifier.Classify(resp, body, expectJSON, c.clock.Now())
	return RawResponse{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header.Clone(),
		Body:       body,
	}, cls, nil
}

func (c *RequestCoordinator) acquireSlot(ctx context.Context) error {
	c.mu.Lock()
	if c.queuedCount >= c.cfg.MaxQueued {
		c.mu.Unlock()
		return domain.NewError(domain.ErrCodeRateLimited, "待发送队列已满，请稍后再试", true, nil)
	}
	c.queuedCount++
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		if c.queuedCount > 0 {
			c.queuedCount--
		}
		c.mu.Unlock()
	}()

	for {
		if ctx.Err() != nil {
			return domain.NewError(domain.ErrCodeCancelled, "请求已取消", true, ctx.Err())
		}

		c.mu.Lock()
		now := c.clock.Now()
		if !c.blockedUntil.IsZero() && c.blockedUntil.After(now) {
			secs := int(c.blockedUntil.Sub(now).Seconds())
			if secs < 1 {
				secs = 1
			}
			c.mu.Unlock()
			err := domain.NewError(domain.ErrCodeRateLimited, "站点限流冷却中", true, nil)
			err.RetryAfterSeconds = secs
			return err
		}

		// Prune dispatches outside sliding window.
		cutoff := now.Add(-c.cfg.SlidingWindow)
		kept := c.recentDispatches[:0]
		for _, t := range c.recentDispatches {
			if t.After(cutoff) {
				kept = append(kept, t)
			}
		}
		c.recentDispatches = kept

		var waitFor time.Duration
		if c.inFlightCount >= c.cfg.MaxConcurrent {
			waitFor = 25 * time.Millisecond
		} else if !c.lastDispatchAt.IsZero() && now.Sub(c.lastDispatchAt) < c.cfg.MinDispatchInterval {
			waitFor = c.cfg.MinDispatchInterval - now.Sub(c.lastDispatchAt)
		} else if len(c.recentDispatches) >= c.cfg.MaxPerWindow {
			waitFor = c.recentDispatches[0].Add(c.cfg.SlidingWindow).Sub(now)
			if waitFor <= 0 {
				waitFor = 10 * time.Millisecond
			}
		}

		if waitFor == 0 {
			c.inFlightCount++
			c.lastDispatchAt = now
			c.recentDispatches = append(c.recentDispatches, now)
			c.mu.Unlock()
			return nil
		}
		c.mu.Unlock()

		if err := c.sleep(ctx, waitFor); err != nil {
			return err
		}
	}
}

func (c *RequestCoordinator) releaseSlot() {
	c.mu.Lock()
	if c.inFlightCount > 0 {
		c.inFlightCount--
	}
	c.mu.Unlock()
}

func (c *RequestCoordinator) sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	select {
	case <-c.clock.After(d):
		return nil
	case <-ctx.Done():
		return domain.NewError(domain.ErrCodeCancelled, "请求已取消", true, ctx.Err())
	}
}

func (c *RequestCoordinator) applyRateLimit(cls Classification) {
	wait := c.cfg.Unknown429Cooldown
	if cls.HasValidWait && cls.WaitDuration > 0 {
		wait = cls.WaitDuration
	}
	until := c.clock.Now().Add(wait)
	c.mu.Lock()
	if until.After(c.blockedUntil) {
		c.blockedUntil = until
	}
	c.phase = domain.NetworkCoolingDown
	c.lastErrorCode = domain.ErrCodeRateLimited
	c.statusMessage = cls.Message
	c.mu.Unlock()
	c.emitStatus()
}

func (c *RequestCoordinator) applyChallengeState(cls Classification) {
	c.mu.Lock()
	if cls.HasValidWait && cls.WaitDuration > 0 {
		until := c.clock.Now().Add(cls.WaitDuration)
		if until.After(c.blockedUntil) {
			c.blockedUntil = until
		}
	}
	c.phase = domain.NetworkChallengeRequired
	c.lastErrorCode = domain.ErrCodeChallengeRequired
	c.statusMessage = "需要完成站点验证"
	c.mu.Unlock()
	c.emitStatus()
}

func (c *RequestCoordinator) enterChallengePaused() {
	c.mu.Lock()
	c.challengeCooldownUntil = c.clock.Now().Add(c.cfg.ChallengeCooldown)
	c.phase = domain.NetworkChallengePaused
	c.lastErrorCode = domain.ErrCodeChallengePaused
	c.statusMessage = "验证后接口仍受限，请稍后重试"
	c.mu.Unlock()
	c.emitStatus()
}

func (c *RequestCoordinator) markReady() {
	c.mu.Lock()
	changed := c.phase != domain.NetworkReady || !c.blockedUntil.IsZero()
	c.phase = domain.NetworkReady
	c.blockedUntil = time.Time{}
	c.lastErrorCode = ""
	c.statusMessage = ""
	c.mu.Unlock()
	if changed {
		c.emitStatus()
	}
}

func (c *RequestCoordinator) setPhase(phase domain.NetworkPhase, code domain.ErrCode, msg string) {
	c.mu.Lock()
	c.phase = phase
	c.lastErrorCode = code
	c.statusMessage = msg
	c.mu.Unlock()
	c.emitStatus()
}

func (c *RequestCoordinator) rateLimitError(cls Classification) *domain.AppError {
	wait := c.cfg.Unknown429Cooldown
	if cls.HasValidWait && cls.WaitDuration > 0 {
		wait = cls.WaitDuration
	}
	err := domain.NewError(domain.ErrCodeRateLimited, cls.Message, true, nil)
	err.RetryAfterSeconds = int(wait.Seconds())
	if err.RetryAfterSeconds < 1 {
		err.RetryAfterSeconds = 1
	}
	return err
}

func (c *RequestCoordinator) mapClassificationToError(cls Classification) *domain.AppError {
	switch cls.Kind {
	case ClassAuthExpired:
		return domain.NewError(domain.ErrCodeAuthExpired, cls.Message, true, nil)
	case ClassForbidden:
		return domain.NewError(domain.ErrCodeForbidden, cls.Message, false, nil)
	case ClassNotFound:
		return domain.NewError(domain.ErrCodeNotFound, cls.Message, false, nil)
	case ClassUnexpectedHTML:
		return domain.NewError(domain.ErrCodeUnexpectedHTML, cls.Message, true, nil)
	case ClassServerError:
		return domain.NewError(domain.ErrCodeServerError, cls.Message, true, nil)
	case ClassClientError:
		return domain.NewError(domain.ErrCodeValidationFailed, cls.Message, false, nil)
	default:
		return domain.NewError(domain.ErrCodeServerError, cls.Message, true, nil)
	}
}

func (c *RequestCoordinator) currentGen() int64 {
	if c.sessions == nil {
		return 1
	}
	return c.sessions.Generation()
}

func (c *RequestCoordinator) emitStatus() {
	if c.events != nil {
		c.events.Emit("NetworkStateChanged", c.Status())
	}
}
