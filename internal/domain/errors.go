package domain

import "fmt"

// ErrCode classifies recoverable business and transport failures (plan.md 3.4, 7.6).
type ErrCode string

const (
	ErrCodeAuthExpired       ErrCode = "auth_expired"
	ErrCodeForbidden         ErrCode = "forbidden"
	ErrCodeNotFound          ErrCode = "not_found"
	ErrCodeRateLimited       ErrCode = "rate_limited"
	ErrCodeChallengeRequired ErrCode = "challenge_required"
	ErrCodeChallengePaused   ErrCode = "challenge_paused"
	ErrCodeConflict          ErrCode = "conflict"
	ErrCodeValidationFailed  ErrCode = "validation_failed"
	ErrCodeOutcomeUnknown    ErrCode = "outcome_unknown"
	ErrCodeNetworkOffline    ErrCode = "network_offline"
	ErrCodeServerError       ErrCode = "server_error"
	ErrCodeUnexpectedHTML    ErrCode = "unexpected_html"
	ErrCodeStorageFailed     ErrCode = "storage_failed"
	ErrCodeStaleSession      ErrCode = "stale_session"
	ErrCodeCancelled         ErrCode = "cancelled"
)

// AppError represents a structured domain error safe for UI display and diagnostics.
// Sensitive tokens, cookies, and forum post contents must never be placed in Message.
type AppError struct {
	Code              ErrCode `json:"code"`
	Message           string  `json:"message"`
	Recoverable       bool    `json:"recoverable"`
	RetryAfterSeconds int     `json:"retryAfterSeconds,omitempty"`
	ActionHint        string  `json:"actionHint,omitempty"`
	Cause             error   `json:"-"`
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Cause
}

// NewError builds a domain AppError without leaking internal transport secrets.
func NewError(code ErrCode, message string, recoverable bool, cause error) *AppError {
	return &AppError{
		Code:        code,
		Message:     message,
		Recoverable: recoverable,
		Cause:       cause,
	}
}
