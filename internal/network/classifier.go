package network

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ClassificationKind identifies the normalized network outcome before JSON unmarshalling.
type ClassificationKind string

const (
	ClassOK                ClassificationKind = "ok"
	ClassChallengeRequired ClassificationKind = "challenge_required"
	ClassRateLimited       ClassificationKind = "rate_limited"
	ClassAuthExpired       ClassificationKind = "auth_expired"
	ClassForbidden         ClassificationKind = "forbidden"
	ClassNotFound          ClassificationKind = "not_found"
	ClassUnexpectedHTML    ClassificationKind = "unexpected_html"
	ClassServerError       ClassificationKind = "server_error"
	ClassClientError       ClassificationKind = "client_error"
)

// Classification holds response inspection results derived before business parsing.
type Classification struct {
	Kind              ClassificationKind
	StatusCode        int
	WaitDuration      time.Duration
	HasValidWait      bool
	ChallengeEvidence string
	Message           string
}

var (
	textWaitSecondsRe = regexp.MustCompile(`(?i)(?:wait|retry\s+in|请等待|等待)\s*(\d+)\s*(?:seconds?|secs?|s|秒)`)
)

// ResponseClassifier separates Cloudflare challenges, 429 rate limits, auth expiry, and HTML fallbacks.
// Behavioral baseline adapted from FluxDO commit 185ccaa418dab8ea40192301e39065706192f866 (F1, F4).
type ResponseClassifier struct{}

// Classify inspects HTTP status, headers, content-type, and a bounded body snippet.
func (ResponseClassifier) Classify(resp *http.Response, body []byte, expectJSON bool, now time.Time) Classification {
	if resp == nil {
		return Classification{Kind: ClassServerError, Message: "empty response"}
	}

	status := resp.StatusCode
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	isHTML := strings.Contains(contentType, "text/html") || strings.Contains(contentType, "application/xhtml+xml")
	isJSON := strings.Contains(contentType, "application/json") || strings.Contains(contentType, "+json")

	waitDur, hasWait := parseWaitDuration(resp, body, isJSON, now)

	// 1. Explicit Cloudflare mitigation header takes precedence, while preserving wait duration if 429 also specifies one.
	if strings.EqualFold(strings.TrimSpace(resp.Header.Get("cf-mitigated")), "challenge") {
		return Classification{
			Kind:              ClassChallengeRequired,
			StatusCode:        status,
			WaitDuration:      waitDur,
			HasValidWait:      hasWait,
			ChallengeEvidence: "cf-mitigated: challenge",
			Message:           "站点需要完成人机验证",
		}
	}

	// 2. HTML challenge markers are ONLY checked on HTML responses (F1: never misclassify JSON error text).
	if isHTML {
		if marker := detectHTMLChallengeMarker(string(body)); marker != "" {
			return Classification{
				Kind:              ClassChallengeRequired,
				StatusCode:        status,
				WaitDuration:      waitDur,
				HasValidWait:      hasWait,
				ChallengeEvidence: marker,
				Message:           "站点需要完成人机验证",
			}
		}
	}

	// 3. Rate limit (429 without challenge evidence).
	if status == http.StatusTooManyRequests {
		return Classification{
			Kind:         ClassRateLimited,
			StatusCode:   status,
			WaitDuration: waitDur,
			HasValidWait: hasWait,
			Message:      "访问过于频繁，请稍后重试",
		}
	}

	// 4. Auth expiry (401 or redirect/HTML login page).
	if status == http.StatusUnauthorized {
		return Classification{
			Kind:       ClassAuthExpired,
			StatusCode: status,
			Message:    "登录状态已失效，需要重新连接",
		}
	}

	// 5. Plain 403 without challenge evidence is Forbidden, not Cloudflare challenge.
	if status == http.StatusForbidden {
		return Classification{
			Kind:       ClassForbidden,
			StatusCode: status,
			Message:    "当前账号没有权限访问该内容",
		}
	}

	if status == http.StatusNotFound {
		return Classification{
			Kind:       ClassNotFound,
			StatusCode: status,
			Message:    "目标内容不存在或已被移除",
		}
	}

	if status >= 500 {
		return Classification{
			Kind:       ClassServerError,
			StatusCode: status,
			Message:    "站点服务暂时不可用",
		}
	}

	// 6. Expected JSON but received HTML (e.g. login redirect rendered as 200 HTML or maintenance page).
	if expectJSON && isHTML {
		if looksLikeLoginHTML(string(body)) {
			return Classification{
				Kind:       ClassAuthExpired,
				StatusCode: status,
				Message:    "登录状态已失效，需要重新连接",
			}
		}
		return Classification{
			Kind:       ClassUnexpectedHTML,
			StatusCode: status,
			Message:    "站点返回了非预期网页内容",
		}
	}

	if status >= 400 {
		return Classification{
			Kind:       ClassClientError,
			StatusCode: status,
			Message:    "请求参数或内容状态异常",
		}
	}

	return Classification{
		Kind:       ClassOK,
		StatusCode: status,
	}
}

func detectHTMLChallengeMarker(snippet string) string {
	lower := strings.ToLower(snippet)
	markers := []string{
		"/cdn-cgi/challenge-platform/",
		"window._cf_chl_opt",
		"challenges.cloudflare.com/turnstile",
		"<title>just a moment...</title>",
		"cf-turnstile-response",
	}
	for _, m := range markers {
		if strings.Contains(lower, m) {
			return m
		}
	}
	return ""
}

func looksLikeLoginHTML(snippet string) bool {
	lower := strings.ToLower(snippet)
	return strings.Contains(lower, "data-discourse-login") ||
		strings.Contains(lower, "/session/sso") ||
		(strings.Contains(lower, "<form") && strings.Contains(lower, "/login"))
}

// parseWaitDuration prioritizes Retry-After, then structured Discourse fields, then bounded text fallback.
func parseWaitDuration(resp *http.Response, body []byte, isJSON bool, localNow time.Time) (time.Duration, bool) {
	refNow := localNow
	if dateHeader := strings.TrimSpace(resp.Header.Get("Date")); dateHeader != "" {
		if serverNow, err := http.ParseTime(dateHeader); err == nil {
			refNow = serverNow
		}
	}

	if ra := strings.TrimSpace(resp.Header.Get("Retry-After")); ra != "" {
		if secs, err := strconv.Atoi(ra); err == nil {
			if secs > 0 {
				return time.Duration(secs) * time.Second, true
			}
			return 0, false
		}
		if retryTime, err := http.ParseTime(ra); err == nil {
			diff := retryTime.Sub(refNow)
			if diff > 0 {
				return diff, true
			}
			return 0, false
		}
		// Malformed Retry-After must not fall back to immediate retry.
		return 0, false
	}

	if isJSON && len(body) > 0 {
		var payload struct {
			WaitSeconds int `json:"wait_seconds"`
			Extras      struct {
				WaitSeconds int `json:"wait_seconds"`
				TimeLeft    int `json:"time_left"`
			} `json:"extras"`
		}
		if err := json.Unmarshal(body, &payload); err == nil {
			for _, v := range []int{payload.Extras.WaitSeconds, payload.Extras.TimeLeft, payload.WaitSeconds} {
				if v > 0 {
					return time.Duration(v) * time.Second, true
				}
			}
		}
	}

	if matches := textWaitSecondsRe.FindStringSubmatch(string(body)); len(matches) == 2 {
		if secs, err := strconv.Atoi(matches[1]); err == nil && secs > 0 {
			return time.Duration(secs) * time.Second, true
		}
	}

	return 0, false
}
