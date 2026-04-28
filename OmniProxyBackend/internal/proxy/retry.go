package proxy

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"OmniProxyBackend/internal/config"
)

type RetryPolicy struct {
	maxRetries        int
	transientCooldown time.Duration
	now               func() time.Time
}

func NewRetryPolicy(cfg config.Config) RetryPolicy {
	cfg = config.Normalize(cfg)
	return RetryPolicy{
		maxRetries:        cfg.MaxRetries,
		transientCooldown: 30 * time.Second,
		now:               time.Now,
	}
}

func (p RetryPolicy) Attempts() int {
	attempts := p.maxRetries + 1
	if attempts < 1 {
		return 1
	}
	return attempts
}

func (p RetryPolicy) ShouldRetryStatus(status int, attempt int) bool {
	return attempt < p.Attempts() && p.IsRetryableStatus(status)
}

func (p RetryPolicy) IsRetryableStatus(status int) bool {
	switch status {
	case http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func (p RetryPolicy) CooldownUntil(status int, header http.Header) *time.Time {
	now := p.now()
	if status == http.StatusTooManyRequests {
		return cooldownUntilFromHeadersAt(now, header)
	}
	if status >= 500 && status <= 599 {
		if until := retryAfterUntil(now, header); until != nil {
			return until
		}
		until := now.Add(p.transientCooldown)
		return &until
	}
	return nil
}

func cooldownUntilFromHeaders(header http.Header) *time.Time {
	return cooldownUntilFromHeadersAt(time.Now(), header)
}

func cooldownUntilFromHeadersAt(now time.Time, header http.Header) *time.Time {
	if until := retryAfterUntil(now, header); until != nil {
		return until
	}
	for _, key := range []string{
		"X-RateLimit-Reset",
		"X-RateLimit-Reset-Requests",
		"X-RateLimit-Reset-Tokens",
	} {
		value := strings.TrimSpace(header.Get(key))
		if value == "" {
			continue
		}
		if until, ok := parseCooldownTime(now, value); ok && until.After(now) {
			return &until
		}
	}
	until := now.Add(5 * time.Minute)
	return &until
}

func retryAfterUntil(now time.Time, header http.Header) *time.Time {
	value := strings.TrimSpace(header.Get("Retry-After"))
	if value == "" {
		return nil
	}
	if until, ok := parseCooldownTime(now, value); ok && until.After(now) {
		return &until
	}
	return nil
}

func cooldownUntilFromValidation(result ValidationResult) *time.Time {
	now := time.Now()
	if result.Usage != nil && result.Usage.PrimaryResetAt > now.Unix() {
		until := time.Unix(result.Usage.PrimaryResetAt, 0)
		return &until
	}
	until := now.Add(5 * time.Minute)
	return &until
}

func parseCooldownTime(now time.Time, value string) (time.Time, bool) {
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds > 1_000_000_000 {
			return time.Unix(int64(seconds), 0), true
		}
		return now.Add(time.Duration(seconds) * time.Second), true
	}
	if parsed, err := http.ParseTime(value); err == nil {
		return parsed, true
	}
	if duration, err := time.ParseDuration(value); err == nil {
		return now.Add(duration), true
	}
	return time.Time{}, false
}
