package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"omniproxy/internal/config"
	"omniproxy/internal/history"
	"omniproxy/internal/logs"
	"omniproxy/internal/token"
	"strings"
	"time"
)

func (s *Service) prepareCandidateTokenAttempt(ctx context.Context, r *http.Request, route routeInfo, excluded map[string]bool, retryChain []history.RetryAttempt, attempt int, attemptStart time.Time) (routeInfo, tokenAttemptOutcome) {
	candidates := routeCandidates(route)
	lastRoute := candidates[0]
	var lastOutcome tokenAttemptOutcome
	for _, candidate := range candidates {
		lastRoute = candidate
		outcome := s.prepareTokenAttempt(ctx, r, candidate, excluded, retryChain, attempt, attemptStart)
		retryChain = outcome.retryChain
		outcome.retryChain = retryChain
		lastOutcome = outcome
		if outcome.ready {
			return candidate, outcome
		}
		if !errors.Is(outcome.err, token.ErrNoActiveToken) {
			return candidate, outcome
		}
	}
	if lastOutcome.retryChain == nil {
		lastOutcome.retryChain = retryChain
	}
	return lastRoute, lastOutcome
}

func routeCandidates(route routeInfo) []routeInfo {
	candidates := make([]routeInfo, 0, 1+len(route.Fallbacks))
	primary := route
	primary.Fallbacks = nil
	candidates = append(candidates, primary)
	for _, fallback := range route.Fallbacks {
		fallback.ClientKey = route.ClientKey
		fallback.ClientName = route.ClientName
		fallback.Fallbacks = nil
		candidates = append(candidates, fallback)
	}
	return candidates
}

func routeCandidateCount(route routeInfo) int {
	return 1 + len(route.Fallbacks)
}

func (s *Service) prepareTokenAttempt(ctx context.Context, r *http.Request, route routeInfo, excluded map[string]bool, retryChain []history.RetryAttempt, attempt int, attemptStart time.Time) tokenAttemptOutcome {
	selected, err := s.acquireToken(route, excluded)
	if err != nil {
		return tokenAttemptOutcome{
			retryChain: appendRetryAttempt(retryChain, attempt, route, nil, 0, time.Since(attemptStart).Milliseconds(), false, err.Error()),
			stop:       true,
			err:        err,
		}
	}

	selected, err = s.refreshSelectedToken(ctx, selected, false)
	if err != nil {
		_ = s.tokens.MarkInvalid(selected.ID, fmt.Sprintf("codex token refresh failed: %v", err))
		s.tokens.Release(selected.ID)
		excluded[selected.ID] = true
		s.logs.Add(logs.Entry{
			Level:      logs.LevelWarn,
			Method:     r.Method,
			Path:       r.URL.RequestURI(),
			ClientKey:  route.ClientKey,
			ClientName: route.ClientName,
			Model:      route.Model,
			TokenName:  token.DisplayName(selected),
			Message:    fmt.Sprintf("codex token refresh failed: %v", err),
		})
		return tokenAttemptOutcome{
			retryChain: appendRetryAttempt(retryChain, attempt, route, &selected, 0, time.Since(attemptStart).Milliseconds(), false, err.Error()),
			err:        err,
		}
	}

	return tokenAttemptOutcome{
		selected:   selected,
		retryChain: retryChain,
		ready:      true,
	}
}

func (s *Service) retryUpstreamAttempt(r *http.Request, route routeInfo, selected token.Token, status int, header http.Header, attempt int, attemptStart time.Time, retryChain []history.RetryAttempt, excluded map[string]bool, body io.Closer, exhaustedReason string, logMessage string, historyMessage string) []history.RetryAttempt {
	_ = s.tokens.RecordProxyUsage(selected.ID, token.TokenConsumption{})
	cooldownUntil := s.cooldownUntilForUpstreamResponse(route, selected, status, header)
	cooldownTriggered := cooldownUntil != nil
	if cooldownTriggered {
		_ = s.tokens.MarkExhaustedUntil(selected.ID, exhaustedReason, cooldownUntil)
	}
	excluded[selected.ID] = true
	closeBody(body)
	s.tokens.Release(selected.ID)
	s.logs.Add(logs.Entry{
		Level:      logs.LevelWarn,
		Method:     r.Method,
		Path:       r.URL.RequestURI(),
		ClientKey:  route.ClientKey,
		ClientName: route.ClientName,
		Model:      route.Model,
		Status:     status,
		TokenName:  token.DisplayName(selected),
		Message:    logMessage,
	})
	return appendRetryAttempt(retryChain, attempt, route, &selected, status, time.Since(attemptStart).Milliseconds(), cooldownTriggered, historyMessage)
}

func (s *Service) acquireToken(route routeInfo, excluded map[string]bool) (token.Token, error) {
	provider := token.NormalizeProvider(route.Provider)
	credentialType := strings.TrimSpace(route.CredentialType)
	if provider == token.ProviderOpenAI && credentialType == "" && route.ClientKey == clientCodex {
		preferCodexAuth := func(item token.Token) bool {
			return item.CredentialType == token.CredentialTypeCodexAuthJSON
		}
		if s.cfg.SchedulingMode == config.SchedulingModeBalanced {
			return s.tokens.AcquireBalancedPreferredMatching(provider, "", excluded, preferCodexAuth)
		}
		return s.tokens.AcquirePreferredMatching(provider, "", excluded, preferCodexAuth)
	}
	if provider == token.ProviderXiaomi && credentialType == "" {
		preferred := preferredMimoCredentialType(s.cfg)
		if s.cfg.SchedulingMode == config.SchedulingModeBalanced {
			selected, err := s.tokens.AcquireBalancedMatching(provider, preferred, excluded)
			if err == nil {
				return selected, nil
			}
			if !errors.Is(err, token.ErrNoActiveToken) {
				return token.Token{}, err
			}
			return s.tokens.AcquireBalancedMatching(provider, "", excluded)
		}
		return s.tokens.AcquirePreferredMatching(provider, "", excluded, func(item token.Token) bool {
			return item.CredentialType == preferred
		})
	}

	if s.cfg.SchedulingMode == config.SchedulingModeBalanced {
		return s.tokens.AcquireBalancedMatching(provider, credentialType, excluded)
	}
	return s.tokens.AcquireMatching(provider, credentialType, excluded)
}

func preferredMimoCredentialType(cfg config.Config) string {
	cfg = config.Normalize(cfg)
	if cfg.XiaomiCredentialPriority == config.MimoCredentialPriorityAPIKey {
		return token.CredentialTypeAPIKey
	}
	return token.CredentialTypeMimoTokenPlan
}

func (s *Service) attemptsForRoute(route routeInfo) int {
	attempts := s.retry.Attempts()
	if isMimoCredentialPriorityRoute(route) && attempts < 2 {
		attempts = 2
	}
	return attempts * routeCandidateCount(route)
}

func (s *Service) shouldRetryUpstreamResponse(route routeInfo, selected token.Token, status int, attempt int, attempts int) bool {
	if attempt >= attempts {
		return false
	}
	if s.retry.IsRetryableStatus(status) {
		return true
	}
	return isMimoCredentialFallbackStatus(route, selected, status)
}

func (s *Service) cooldownUntilForUpstreamResponse(route routeInfo, selected token.Token, status int, header http.Header) *time.Time {
	if until := s.retry.CooldownUntil(status, header); until != nil {
		return until
	}
	if isMimoCredentialFallbackStatus(route, selected, status) {
		return cooldownUntilFromHeadersAt(s.retry.now(), header)
	}
	return nil
}

func isMimoCredentialPriorityRoute(route routeInfo) bool {
	return token.NormalizeProvider(route.Provider) == token.ProviderXiaomi &&
		strings.TrimSpace(route.CredentialType) == ""
}

func isMimoCredentialFallbackStatus(route routeInfo, selected token.Token, status int) bool {
	if !isMimoCredentialPriorityRoute(route) {
		return false
	}
	switch selected.CredentialType {
	case "", token.CredentialTypeAPIKey, token.CredentialTypeMimoTokenPlan:
	default:
		return false
	}
	switch status {
	case http.StatusBadRequest, http.StatusUnauthorized, http.StatusPaymentRequired, http.StatusForbidden:
		return true
	default:
		return false
	}
}

func upstreamSwitchMessage(route routeInfo, selected token.Token, status int) string {
	if isMimoCredentialFallbackStatus(route, selected, status) {
		return "switching MiMo credential after upstream credential fallback response"
	}
	return "switching token after retryable upstream response"
}

func upstreamWebSocketSwitchMessage(route routeInfo, selected token.Token, status int) string {
	if isMimoCredentialFallbackStatus(route, selected, status) {
		return "switching MiMo credential after upstream websocket credential fallback response"
	}
	return "switching token after retryable upstream websocket response"
}

func (s *Service) refreshSelectedToken(ctx context.Context, selected token.Token, force bool) (token.Token, error) {
	if s.tokenRefresher == nil || (selected.CredentialType != token.CredentialTypeCodexAuthJSON && selected.CredentialType != token.CredentialTypeClaudeOAuth) {
		return selected, nil
	}
	updated, _, err := s.tokenRefresher(ctx, selected, force)
	if err != nil {
		return selected, err
	}
	return updated, nil
}
