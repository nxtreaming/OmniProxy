package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"OmniProxyBackend/internal/config"
	"OmniProxyBackend/internal/history"
	"OmniProxyBackend/internal/logs"
	"OmniProxyBackend/internal/token"
	"github.com/gorilla/websocket"
)

type Service struct {
	cfg     config.Config
	tokens  *token.Manager
	logs    *logs.Recorder
	history *history.Recorder
	client  *http.Client
}

const maxProxyRequestBodyBytes = 32 * 1024 * 1024

var errRequestBodyTooLarge = errors.New("request body too large")

func NewService(cfg config.Config, tokens *token.Manager, recorder *logs.Recorder, historyRecorders ...*history.Recorder) (*Service, error) {
	cfg = config.Normalize(cfg)
	for provider, baseURL := range map[string]string{
		token.ProviderOpenAI:          cfg.OpenAIBaseURL,
		token.ProviderAnthropic:       cfg.AnthropicBaseURL,
		token.ProviderDeepSeek:        cfg.DeepSeekBaseURL,
		token.ProviderKimi:            cfg.KimiBaseURL,
		"deepseek_anthropic":          cfg.DeepSeekAnthropicBaseURL,
		"xiaomi_api":                  cfg.XiaomiAPIBaseURL,
		"xiaomi_api_anthropic":        cfg.XiaomiAPIAnthropicBaseURL,
		"xiaomi_token_plan":           cfg.XiaomiTokenPlanBaseURL,
		"xiaomi_token_plan_anthropic": cfg.XiaomiTokenPlanAnthropicBaseURL,
		"codex":                       cfg.CodexBaseURL,
	} {
		if strings.TrimSpace(baseURL) == "" {
			continue
		}
		if _, err := url.ParseRequestURI(baseURL); err != nil {
			return nil, fmt.Errorf("invalid %s base url: %w", provider, err)
		}
	}

	var requestHistory *history.Recorder
	if len(historyRecorders) > 0 {
		requestHistory = historyRecorders[0]
	}

	return &Service{
		cfg:     cfg,
		tokens:  tokens,
		logs:    recorder,
		history: requestHistory,
		client:  &http.Client{Timeout: 0},
	}, nil
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if isCodexResponsesWebSocket(r) {
		if s.cfg.WebSocketMode == config.WebSocketModeDisabled {
			s.logs.Add(logs.Entry{
				Level:   logs.LevelWarn,
				Method:  r.Method,
				Path:    r.URL.RequestURI(),
				Status:  http.StatusForbidden,
				Message: "websocket proxy disabled",
			})
			s.recordHistory(r, routeInfo{Provider: token.ProviderOpenAI, CredentialType: token.CredentialTypeCodexAuthJSON, Protocol: "openai", Path: r.URL.Path, RawQuery: r.URL.RawQuery}, nil, http.StatusForbidden, 0, token.TokenConsumption{}, logs.LevelWarn, "websocket proxy disabled")
			http.Error(w, "websocket proxy disabled", http.StatusForbidden)
			return
		}
		s.serveCodexWebSocket(w, r)
		return
	}
	if isAnthropicRouterProbe(r) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if r.Method != http.MethodHead {
			_, _ = w.Write([]byte(`{"ok":true,"service":"omniproxy anthropic router"}`))
		}
		return
	}
	if isCodexResponsesProbe(r) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if r.Method != http.MethodHead {
			_, _ = w.Write([]byte(`{"ok":true,"service":"omniproxy codex proxy"}`))
		}
		return
	}

	start := time.Now()
	bodyBytes, err := readProxyRequestBody(r.Body)
	if err != nil {
		if errors.Is(err, errRequestBodyTooLarge) {
			s.recordHistory(r, routeInfo{Provider: token.ProviderOpenAI, Path: r.URL.Path, RawQuery: r.URL.RawQuery}, nil, http.StatusRequestEntityTooLarge, time.Since(start).Milliseconds(), token.TokenConsumption{}, logs.LevelWarn, errRequestBodyTooLarge.Error())
			http.Error(w, errRequestBodyTooLarge.Error(), http.StatusRequestEntityTooLarge)
			return
		}
		s.recordHistory(r, routeInfo{Provider: token.ProviderOpenAI, Path: r.URL.Path, RawQuery: r.URL.RawQuery}, nil, http.StatusBadRequest, time.Since(start).Milliseconds(), token.TokenConsumption{}, logs.LevelWarn, "failed to read request body")
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	excluded := map[string]bool{}
	attempts := s.cfg.MaxRetries + 1
	if attempts < 1 {
		attempts = 1
	}

	var lastErr error
	var lastStatus int
	route := s.routeForRequest(r.URL, bodyBytes)
	retryChain := make([]history.RetryAttempt, 0, attempts)

	for attempt := 1; attempt <= attempts; attempt++ {
		attemptStart := time.Now()
		selected, err := s.acquireToken(route, excluded)
		if err != nil {
			lastErr = err
			retryChain = appendRetryAttempt(retryChain, attempt, route, nil, 0, time.Since(attemptStart).Milliseconds(), false, err.Error())
			break
		}

		resp, err := s.forward(r.Context(), r, route, bodyBytes, selected)
		if err != nil {
			lastErr = err
			_ = s.tokens.RecordProxyUsage(selected.ID, token.TokenConsumption{})
			s.tokens.Release(selected.ID)
			excluded[selected.ID] = true
			s.logs.Add(logs.Entry{
				Level:     logs.LevelWarn,
				Method:    r.Method,
				Path:      r.URL.RequestURI(),
				TokenName: selected.Name,
				Message:   proxyLogMessage(route.Model, token.TokenConsumption{}, "upstream request failed, trying next token"),
			})
			retryChain = appendRetryAttempt(retryChain, attempt, route, &selected, http.StatusBadGateway, time.Since(attemptStart).Milliseconds(), false, fmt.Sprintf("upstream request failed: %v", err))
			continue
		}

		lastStatus = resp.StatusCode
		if remaining, ok := parseRemaining(resp.Header); ok {
			_ = s.tokens.RecordUsage(selected.ID, remaining)
		} else {
			_ = s.tokens.RecordUsage(selected.ID, -1)
		}

		if shouldRetry(resp.StatusCode) && attempt < attempts {
			_ = s.tokens.RecordProxyUsage(selected.ID, token.TokenConsumption{})
			if resp.StatusCode == http.StatusTooManyRequests {
				_ = s.tokens.MarkExhaustedUntil(selected.ID, "upstream returned 429", cooldownUntilFromHeaders(resp.Header))
			}
			excluded[selected.ID] = true
			closeBody(resp.Body)
			s.tokens.Release(selected.ID)
			s.logs.Add(logs.Entry{
				Level:     logs.LevelWarn,
				Method:    r.Method,
				Path:      r.URL.RequestURI(),
				Status:    resp.StatusCode,
				TokenName: selected.Name,
				Message:   proxyLogMessage(route.Model, token.TokenConsumption{}, "switching token after retryable upstream response"),
			})
			retryChain = appendRetryAttempt(retryChain, attempt, route, &selected, resp.StatusCode, time.Since(attemptStart).Milliseconds(), resp.StatusCode == http.StatusTooManyRequests, "switching token after retryable upstream response")
			continue
		}

		consumption, responseBody := s.writeResponse(w, resp)
		_ = s.tokens.RecordProxyUsage(selected.ID, consumption)
		cooldownTriggered := resp.StatusCode == http.StatusTooManyRequests
		if cooldownTriggered {
			_ = s.tokens.MarkExhaustedUntil(selected.ID, "upstream returned 429", cooldownUntilFromHeaders(resp.Header))
		}
		s.tokens.Release(selected.ID)
		s.refreshQuotaAfterTask(selected)
		historyMessage := proxyHistoryMessage(resp.StatusCode, route.Model, consumption, "request proxied", responseBody)
		retryChain = appendRetryAttempt(retryChain, attempt, route, &selected, resp.StatusCode, time.Since(attemptStart).Milliseconds(), cooldownTriggered, historyMessage)
		s.logs.Add(logs.Entry{
			Level:     levelForStatus(resp.StatusCode),
			Method:    r.Method,
			Path:      r.URL.RequestURI(),
			Status:    resp.StatusCode,
			Duration:  time.Since(start).Milliseconds(),
			TokenName: selected.Name,
			Message:   proxyLogMessage(route.Model, consumption, "request proxied"),
		})
		s.recordHistory(r, route, &selected, resp.StatusCode, time.Since(start).Milliseconds(), consumption, levelForStatus(resp.StatusCode), historyMessage, retryChain...)
		return
	}

	status := http.StatusBadGateway
	if errors.Is(lastErr, token.ErrNoActiveToken) {
		status = http.StatusServiceUnavailable
	}
	if lastStatus != 0 {
		status = lastStatus
	}

	s.logs.Add(logs.Entry{
		Level:    logs.LevelError,
		Method:   r.Method,
		Path:     r.URL.RequestURI(),
		Status:   status,
		Duration: time.Since(start).Milliseconds(),
		Message:  fmt.Sprintf("proxy failed: %v", lastErr),
	})
	if len(retryChain) == 0 {
		retryChain = appendRetryAttempt(retryChain, 1, route, nil, status, time.Since(start).Milliseconds(), false, fmt.Sprintf("proxy failed: %v", lastErr))
	}
	s.recordHistory(r, route, nil, status, time.Since(start).Milliseconds(), token.TokenConsumption{}, logs.LevelError, fmt.Sprintf("proxy failed: %v", lastErr), retryChain...)
	http.Error(w, http.StatusText(status), status)
}

func (s *Service) forward(ctx context.Context, original *http.Request, route routeInfo, body []byte, selected token.Token) (*http.Response, error) {
	targetURL, err := s.targetURL(route, selected)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, original.Method, targetURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	copyHeader(req.Header, original.Header)
	removeHopHeaders(req.Header)
	if err := applyRouteAuth(req.Header, selected, route); err != nil {
		return nil, err
	}
	req.Host = req.URL.Host

	return s.client.Do(req)
}

func (s *Service) refreshQuotaAfterTask(selected token.Token) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		validator, err := NewValidator(s.cfg)
		if err != nil {
			s.logs.Add(logs.Entry{Level: logs.LevelWarn, TokenName: selected.Name, Message: fmt.Sprintf("post-task quota refresh skipped: %v", err)})
			return
		}

		result, err := validator.Validate(ctx, selected)
		if result.Remaining != nil {
			_ = s.tokens.RecordUsage(selected.ID, *result.Remaining)
		}
		if result.Usage != nil {
			_ = s.tokens.RecordUsageInfo(selected.ID, *result.Usage)
		}
		if result.Status == http.StatusUnauthorized || result.Status == http.StatusForbidden {
			_ = s.tokens.MarkInvalid(selected.ID, fmt.Sprintf("post-task quota refresh returned %d", result.Status))
		}
		if result.Status == http.StatusTooManyRequests {
			_ = s.tokens.MarkExhaustedUntil(selected.ID, "post-task quota refresh returned 429", cooldownUntilFromValidation(result))
		}
		if result.OK && result.Remaining == nil && result.Usage == nil {
			_ = s.tokens.MarkActive(selected.ID)
		}
		if err != nil {
			s.logs.Add(logs.Entry{Level: logs.LevelWarn, TokenName: selected.Name, Message: fmt.Sprintf("post-task quota refresh failed: %v", err)})
		}
	}()
}

func (s *Service) serveCodexWebSocket(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	route := s.routeForRequest(r.URL, nil)
	route.CredentialType = token.CredentialTypeCodexAuthJSON

	excluded := map[string]bool{}
	attempts := s.cfg.MaxRetries + 1
	if attempts < 1 {
		attempts = 1
	}

	var selected token.Token
	var upstream *websocket.Conn
	var upstreamResp *http.Response
	var lastErr error
	var lastStatus int
	retryChain := make([]history.RetryAttempt, 0, attempts)

	for attempt := 1; attempt <= attempts; attempt++ {
		attemptStart := time.Now()
		next, err := s.acquireToken(route, excluded)
		if err != nil {
			lastErr = err
			retryChain = appendRetryAttempt(retryChain, attempt, route, nil, 0, time.Since(attemptStart).Milliseconds(), false, err.Error())
			break
		}
		selected = next

		targetURL, err := s.targetWebSocketURL(route, selected)
		if err != nil {
			lastErr = err
			s.tokens.Release(selected.ID)
			retryChain = appendRetryAttempt(retryChain, attempt, route, &selected, 0, time.Since(attemptStart).Milliseconds(), false, err.Error())
			break
		}

		header := websocketRequestHeader(r.Header)
		if err := applyRouteAuth(header, selected, route); err != nil {
			lastErr = err
			s.tokens.Release(selected.ID)
			retryChain = appendRetryAttempt(retryChain, attempt, route, &selected, 0, time.Since(attemptStart).Milliseconds(), false, err.Error())
			break
		}

		dialer := websocket.Dialer{
			HandshakeTimeout:  45 * time.Second,
			Proxy:             http.ProxyFromEnvironment,
			Subprotocols:      websocket.Subprotocols(r),
			EnableCompression: true,
		}
		upstream, upstreamResp, err = dialer.DialContext(r.Context(), targetURL, header)
		if err == nil {
			break
		}

		lastErr = err
		if upstreamResp != nil {
			lastStatus = upstreamResp.StatusCode
		}
		if shouldRetry(lastStatus) && attempt < attempts {
			_ = s.tokens.RecordProxyUsage(selected.ID, token.TokenConsumption{})
			if lastStatus == http.StatusTooManyRequests {
				_ = s.tokens.MarkExhaustedUntil(selected.ID, "upstream websocket returned 429", cooldownUntilFromHeaders(responseHeaders(upstreamResp)))
			}
			excluded[selected.ID] = true
			closeBody(upstreamRespBody(upstreamResp))
			s.tokens.Release(selected.ID)
			s.logs.Add(logs.Entry{
				Level:     logs.LevelWarn,
				Method:    r.Method,
				Path:      r.URL.RequestURI(),
				Status:    lastStatus,
				TokenName: selected.Name,
				Message:   "switching token after retryable upstream websocket response",
			})
			retryChain = appendRetryAttempt(retryChain, attempt, route, &selected, lastStatus, time.Since(attemptStart).Milliseconds(), lastStatus == http.StatusTooManyRequests, "switching token after retryable upstream websocket response")
			continue
		}
		retryChain = appendRetryAttempt(retryChain, attempt, route, &selected, lastStatus, time.Since(attemptStart).Milliseconds(), false, fmt.Sprintf("upstream websocket failed: %v", err))
		s.tokens.Release(selected.ID)
		break
	}

	if upstream == nil {
		status := http.StatusBadGateway
		if errors.Is(lastErr, token.ErrNoActiveToken) {
			status = http.StatusServiceUnavailable
		}
		if lastStatus != 0 {
			status = lastStatus
		}
		closeBody(upstreamRespBody(upstreamResp))
		s.logs.Add(logs.Entry{
			Level:    logs.LevelError,
			Method:   r.Method,
			Path:     r.URL.RequestURI(),
			Status:   status,
			Duration: time.Since(start).Milliseconds(),
			Message:  fmt.Sprintf("websocket proxy failed: %v", lastErr),
		})
		if len(retryChain) == 0 {
			retryChain = appendRetryAttempt(retryChain, 1, route, nil, status, time.Since(start).Milliseconds(), false, fmt.Sprintf("websocket proxy failed: %v", lastErr))
		}
		s.recordHistory(r, route, nil, status, time.Since(start).Milliseconds(), token.TokenConsumption{}, logs.LevelError, fmt.Sprintf("websocket proxy failed: %v", lastErr), retryChain...)
		http.Error(w, http.StatusText(status), status)
		return
	}
	defer upstream.Close()
	closeBody(upstreamRespBody(upstreamResp))

	responseHeader := http.Header{}
	if subprotocol := upstream.Subprotocol(); subprotocol != "" {
		responseHeader.Set("Sec-Websocket-Protocol", subprotocol)
	}
	upgrader := websocket.Upgrader{
		CheckOrigin:       func(*http.Request) bool { return true },
		EnableCompression: true,
	}
	client, err := upgrader.Upgrade(w, r, responseHeader)
	if err != nil {
		s.tokens.Release(selected.ID)
		s.logs.Add(logs.Entry{
			Level:     logs.LevelError,
			Method:    r.Method,
			Path:      r.URL.RequestURI(),
			Status:    http.StatusBadRequest,
			Duration:  time.Since(start).Milliseconds(),
			TokenName: selected.Name,
			Message:   fmt.Sprintf("websocket client upgrade failed: %v", err),
		})
		s.recordHistory(r, route, &selected, http.StatusBadRequest, time.Since(start).Milliseconds(), token.TokenConsumption{}, logs.LevelError, fmt.Sprintf("websocket client upgrade failed: %v", err))
		return
	}
	defer client.Close()

	_ = s.tokens.RecordUsage(selected.ID, -1)
	consumption, err := proxyWebSocketMessages(client, upstream)
	_ = s.tokens.RecordProxyUsage(selected.ID, consumption)
	s.tokens.Release(selected.ID)
	s.refreshQuotaAfterTask(selected)

	level := logs.LevelInfo
	message := proxyLogMessage(route.Model, consumption, "websocket proxied")
	if err != nil && !isNormalWebSocketClose(err) {
		level = logs.LevelWarn
		message = fmt.Sprintf("websocket closed with error: %v", err)
	}
	s.logs.Add(logs.Entry{
		Level:     level,
		Method:    r.Method,
		Path:      r.URL.RequestURI(),
		Status:    http.StatusSwitchingProtocols,
		Duration:  time.Since(start).Milliseconds(),
		TokenName: selected.Name,
		Message:   message,
	})
	if len(retryChain) == 0 || retryChain[len(retryChain)-1].Status != http.StatusSwitchingProtocols {
		retryChain = appendRetryAttempt(retryChain, len(retryChain)+1, route, &selected, http.StatusSwitchingProtocols, time.Since(start).Milliseconds(), false, message)
	}
	s.recordHistory(r, route, &selected, http.StatusSwitchingProtocols, time.Since(start).Milliseconds(), consumption, level, message, retryChain...)
}

func (s *Service) targetWebSocketURL(route routeInfo, selected token.Token) (string, error) {
	targetURL, err := s.targetURL(route, selected)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(targetURL)
	if err != nil {
		return "", err
	}
	switch parsed.Scheme {
	case "https":
		parsed.Scheme = "wss"
	case "http":
		parsed.Scheme = "ws"
	default:
		return "", fmt.Errorf("unsupported websocket upstream scheme %q", parsed.Scheme)
	}
	return parsed.String(), nil
}

func (s *Service) targetURL(route routeInfo, selected token.Token) (string, error) {
	baseURL := s.baseURL(route, selected)
	if baseURL == "" {
		return "", fmt.Errorf("%s upstream base url is not configured", route.Provider)
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}

	out := *base
	out.Path = singleJoiningSlash(base.Path, upstreamPath(route.Path, selected))
	out.RawQuery = route.RawQuery
	return out.String(), nil
}

type routeInfo struct {
	Provider       string
	CredentialType string
	Protocol       string
	Model          string
	Path           string
	RawQuery       string
}

func (s *Service) acquireToken(route routeInfo, excluded map[string]bool) (token.Token, error) {
	if s.cfg.SchedulingMode == config.SchedulingModeBalanced {
		return s.tokens.AcquireBalancedMatching(route.Provider, route.CredentialType, excluded)
	}
	return s.tokens.AcquireMatching(route.Provider, route.CredentialType, excluded)
}

func (s *Service) routeForRequest(incoming *url.URL, body []byte) routeInfo {
	provider := token.ProviderOpenAI
	credentialType := ""
	path := incoming.Path
	model := requestModel(body)
	if isAnthropicRouterPath(path) {
		return routeInfo{
			Provider: providerForModel(model),
			Protocol: "anthropic",
			Model:    model,
			Path:     stripPathPrefix(path, "/anthropic-router"),
			RawQuery: incoming.RawQuery,
		}
	}

	trimmed := strings.TrimPrefix(path, "/")
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) > 0 {
		candidate := strings.ToLower(parts[0])
		switch candidate {
		case token.ProviderOpenAI, token.ProviderAnthropic, token.ProviderDeepSeek, token.ProviderKimi, token.ProviderXiaomi:
			provider = candidate
			if len(parts) == 2 {
				path = "/" + parts[1]
			} else {
				path = "/"
			}
		case "codex":
			provider = token.ProviderOpenAI
			credentialType = token.CredentialTypeCodexAuthJSON
			if len(parts) == 2 {
				path = "/" + parts[1]
			} else {
				path = "/"
			}
		}
	}
	if isCodexProxyPath(path) {
		credentialType = token.CredentialTypeCodexAuthJSON
	}
	protocol := "openai"
	if provider == token.ProviderAnthropic {
		protocol = "anthropic"
	}
	if provider == token.ProviderDeepSeek {
		if path == "/anthropic" {
			path = "/"
			protocol = "anthropic"
		} else if strings.HasPrefix(path, "/anthropic/") {
			path = "/" + strings.TrimPrefix(path, "/anthropic/")
			protocol = "anthropic"
		}
	}
	if provider == token.ProviderKimi {
		if path == "/anthropic" {
			path = "/"
			protocol = "anthropic"
		} else if strings.HasPrefix(path, "/anthropic/") {
			path = "/" + strings.TrimPrefix(path, "/anthropic/")
			protocol = "anthropic"
		}
	}
	if provider == token.ProviderXiaomi {
		if path == "/anthropic" {
			path = "/"
			protocol = "anthropic"
		} else if strings.HasPrefix(path, "/anthropic/") {
			path = "/" + strings.TrimPrefix(path, "/anthropic/")
			protocol = "anthropic"
		}
	}
	return routeInfo{Provider: provider, CredentialType: credentialType, Protocol: protocol, Model: model, Path: path, RawQuery: incoming.RawQuery}
}

func isAnthropicRouterPath(path string) bool {
	return path == "/anthropic-router" || strings.HasPrefix(path, "/anthropic-router/")
}

func isAnthropicRouterProbe(r *http.Request) bool {
	if r.URL == nil || r.URL.Path != "/anthropic-router" {
		return false
	}
	return r.Method == http.MethodHead || r.Method == http.MethodGet
}

func isCodexResponsesProbe(r *http.Request) bool {
	if r.URL == nil || (r.Method != http.MethodHead && r.Method != http.MethodGet) {
		return false
	}
	if isWebSocketUpgrade(r) {
		return false
	}
	path := stripPathPrefix(r.URL.Path, "/backend-api/codex")
	path = stripPathPrefix(path, "/v1")
	return path == "/responses"
}

func isCodexResponsesWebSocket(r *http.Request) bool {
	if r.URL == nil || r.Method != http.MethodGet || !isWebSocketUpgrade(r) {
		return false
	}
	path := stripPathPrefix(r.URL.Path, "/backend-api/codex")
	path = stripPathPrefix(path, "/v1")
	return path == "/responses"
}

func isWebSocketUpgrade(r *http.Request) bool {
	return tokenListContains(r.Header.Get("Connection"), "upgrade") &&
		strings.EqualFold(strings.TrimSpace(r.Header.Get("Upgrade")), "websocket")
}

func requestModel(body []byte) string {
	var payload struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.Model)
}

func providerForModel(model string) string {
	model = strings.ToLower(strings.TrimSpace(model))
	if model == "" {
		return token.ProviderAnthropic
	}

	if strings.HasPrefix(model, "mimo-") {
		return token.ProviderXiaomi
	}
	if strings.HasPrefix(model, "deepseek-") {
		return token.ProviderDeepSeek
	}
	if strings.HasPrefix(model, "kimi-") {
		return token.ProviderKimi
	}
	return token.ProviderAnthropic
}

func (s *Service) baseURL(route routeInfo, selected token.Token) string {
	if isCodexCredential(selected) {
		return s.cfg.CodexBaseURL
	}

	switch token.NormalizeProvider(route.Provider) {
	case token.ProviderAnthropic:
		return s.cfg.AnthropicBaseURL
	case token.ProviderDeepSeek:
		if route.Protocol == "anthropic" {
			return s.cfg.DeepSeekAnthropicBaseURL
		}
		return s.cfg.DeepSeekBaseURL
	case token.ProviderKimi:
		return s.cfg.KimiBaseURL
	case token.ProviderXiaomi:
		if selected.CredentialType == token.CredentialTypeMimoTokenPlan {
			if route.Protocol == "anthropic" {
				return s.cfg.XiaomiTokenPlanAnthropicBaseURL
			}
			return s.cfg.XiaomiTokenPlanBaseURL
		}
		if route.Protocol == "anthropic" {
			return s.cfg.XiaomiAPIAnthropicBaseURL
		}
		return s.cfg.XiaomiAPIBaseURL
	default:
		if s.cfg.OpenAIBaseURL != "" {
			return s.cfg.OpenAIBaseURL
		}
		return s.cfg.UpstreamBaseURL
	}
}

func isCodexCredential(selected token.Token) bool {
	return token.NormalizeProvider(selected.Provider) == token.ProviderOpenAI &&
		selected.CredentialType == token.CredentialTypeCodexAuthJSON
}

func upstreamPath(path string, selected token.Token) string {
	if !isCodexCredential(selected) {
		return path
	}
	next := stripPathPrefix(path, "/backend-api/codex")
	next = stripPathPrefix(next, "/v1")
	if next == "" {
		return "/"
	}
	return next
}

func stripPathPrefix(path string, prefix string) string {
	if path == prefix {
		return "/"
	}
	withSlash := prefix + "/"
	if strings.HasPrefix(path, withSlash) {
		return "/" + strings.TrimPrefix(path, withSlash)
	}
	return path
}

func isCodexProxyPath(path string) bool {
	return path == "/backend-api/codex" || strings.HasPrefix(path, "/backend-api/codex/")
}

func (s *Service) writeResponse(w http.ResponseWriter, resp *http.Response) (token.TokenConsumption, []byte) {
	defer closeBody(resp.Body)

	capture := &usageCapture{}
	removeHopHeaders(resp.Header)
	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)

	target := io.Writer(w)
	if flusher, ok := w.(http.Flusher); ok {
		target = flushWriter{writer: w, flusher: flusher}
	}
	_, _ = io.Copy(io.MultiWriter(target, capture), resp.Body)
	body := capture.Bytes()
	return parseTokenConsumption(resp.Header, body), body
}

func parseRemaining(header http.Header) (int, bool) {
	keys := []string{
		"x-ratelimit-remaining-tokens",
		"x-ratelimit-remaining",
		"x-ratelimit-remaining-requests",
	}
	for _, key := range keys {
		value := strings.TrimSpace(header.Get(key))
		if value == "" {
			continue
		}
		parsed, err := strconv.Atoi(value)
		if err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func shouldRetry(status int) bool {
	return status == http.StatusTooManyRequests || status == http.StatusBadGateway || status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout
}

func levelForStatus(status int) logs.Level {
	if status >= 500 {
		return logs.LevelError
	}
	if status >= 400 {
		return logs.LevelWarn
	}
	return logs.LevelInfo
}

func (s *Service) recordHistory(r *http.Request, route routeInfo, selected *token.Token, status int, duration int64, consumption token.TokenConsumption, level logs.Level, message string, retryChain ...history.RetryAttempt) {
	if s.history == nil {
		return
	}
	entry := history.Entry{
		Level:             string(level),
		Method:            r.Method,
		Path:              r.URL.RequestURI(),
		Provider:          token.NormalizeProvider(route.Provider),
		Protocol:          route.Protocol,
		Model:             route.Model,
		Status:            status,
		Duration:          duration,
		InputTokens:       consumption.InputTokens,
		OutputTokens:      consumption.OutputTokens,
		TotalTokens:       consumption.TotalTokens,
		CooldownTriggered: retryChainCooldownTriggered(retryChain),
		Message:           message,
	}
	if entry.Protocol == "" {
		entry.Protocol = "openai"
	}
	if selected != nil {
		entry.TokenID = selected.ID
		entry.TokenName = selected.Name
	}
	if len(retryChain) > 0 {
		entry.RetryChain = append([]history.RetryAttempt(nil), retryChain...)
	}
	s.history.Add(entry)
}

func appendRetryAttempt(chain []history.RetryAttempt, attempt int, route routeInfo, selected *token.Token, status int, duration int64, cooldownTriggered bool, message string) []history.RetryAttempt {
	item := history.RetryAttempt{
		Attempt:           attempt,
		Provider:          token.NormalizeProvider(route.Provider),
		Protocol:          route.Protocol,
		Model:             route.Model,
		Status:            status,
		Duration:          duration,
		CooldownTriggered: cooldownTriggered,
		Message:           strings.TrimSpace(message),
	}
	if item.Protocol == "" {
		item.Protocol = "openai"
	}
	if selected != nil {
		item.TokenID = selected.ID
		item.TokenName = selected.Name
	}
	return append(chain, item)
}

func retryChainCooldownTriggered(chain []history.RetryAttempt) bool {
	for _, attempt := range chain {
		if attempt.CooldownTriggered {
			return true
		}
	}
	return false
}

func proxyHistoryMessage(status int, model string, consumption token.TokenConsumption, fallback string, body []byte) string {
	if status >= 400 {
		text := fmt.Sprintf("upstream returned %d", status)
		if statusText := http.StatusText(status); statusText != "" {
			text = fmt.Sprintf("%s %s", text, statusText)
		}
		if summary := upstreamErrorSummary(body); summary != "" {
			text = summary
		}
		return proxyLogMessage(model, consumption, text)
	}
	return proxyLogMessage(model, consumption, fallback)
}

func upstreamErrorSummary(body []byte) string {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return ""
	}

	var payload any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err == nil {
		if message := findErrorSummary(payload); message != "" {
			return message
		}
	}
	return limitSummary(strings.Join(strings.Fields(string(body)), " "))
}

func findErrorSummary(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range []string{"message", "detail", "error_description"} {
			if message, ok := typed[key].(string); ok && strings.TrimSpace(message) != "" {
				return limitSummary(message)
			}
		}
		if errorValue, ok := typed["error"]; ok {
			if message, ok := errorValue.(string); ok && strings.TrimSpace(message) != "" {
				return limitSummary(message)
			}
			if message := findErrorSummary(errorValue); message != "" {
				return message
			}
		}
		for _, child := range typed {
			if message := findErrorSummary(child); message != "" {
				return message
			}
		}
	case []any:
		for _, child := range typed {
			if message := findErrorSummary(child); message != "" {
				return message
			}
		}
	}
	return ""
}

func limitSummary(value string) string {
	value = strings.TrimSpace(value)
	const max = 320
	if len(value) <= max {
		return value
	}
	return value[:max] + "..."
}

func proxyLogMessage(model string, consumption token.TokenConsumption, fallback string) string {
	message := fallback
	if consumption.TotalTokens <= 0 {
		if model == "" {
			return message
		}
		return fmt.Sprintf("%s, model=%s", message, model)
	}
	message = fmt.Sprintf("%s, used %d tokens", message, consumption.TotalTokens)
	if model == "" {
		return message
	}
	return fmt.Sprintf("%s, model=%s", message, model)
}

func copyHeader(dst, src http.Header) {
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func removeHopHeaders(header http.Header) {
	for _, key := range []string{
		"Connection",
		"Proxy-Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te",
		"Trailer",
		"Transfer-Encoding",
		"Upgrade",
	} {
		header.Del(key)
	}
}

func websocketRequestHeader(src http.Header) http.Header {
	dst := http.Header{}
	for key, values := range src {
		if isWebSocketRequestHeader(key) {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
	return dst
}

func isWebSocketRequestHeader(key string) bool {
	switch http.CanonicalHeaderKey(key) {
	case "Connection",
		"Host",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Proxy-Connection",
		"Sec-Websocket-Accept",
		"Sec-Websocket-Extensions",
		"Sec-Websocket-Key",
		"Sec-Websocket-Protocol",
		"Sec-Websocket-Version",
		"Te",
		"Trailer",
		"Transfer-Encoding",
		"Upgrade":
		return true
	default:
		return false
	}
}

func proxyWebSocketMessages(client *websocket.Conn, upstream *websocket.Conn) (token.TokenConsumption, error) {
	resultCh := make(chan websocketCopyResult, 2)
	go func() {
		resultCh <- copyWebSocketMessages(upstream, client, false)
	}()
	go func() {
		resultCh <- copyWebSocketMessages(client, upstream, true)
	}()

	first := <-resultCh
	closeMessage := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
	deadline := time.Now().Add(time.Second)
	_ = client.WriteControl(websocket.CloseMessage, closeMessage, deadline)
	_ = upstream.WriteControl(websocket.CloseMessage, closeMessage, deadline)
	_ = client.Close()
	_ = upstream.Close()
	second := <-resultCh
	return addTokenConsumption(first.consumption, second.consumption), first.err
}

type websocketCopyResult struct {
	consumption token.TokenConsumption
	err         error
}

func copyWebSocketMessages(dst *websocket.Conn, src *websocket.Conn, captureUsage bool) websocketCopyResult {
	var total token.TokenConsumption
	for {
		messageType, reader, err := src.NextReader()
		if err != nil {
			return websocketCopyResult{consumption: total, err: err}
		}
		writer, err := dst.NextWriter(messageType)
		if err != nil {
			return websocketCopyResult{consumption: total, err: err}
		}
		target := io.Writer(writer)
		capture := &usageCapture{}
		if captureUsage && (messageType == websocket.TextMessage || messageType == websocket.BinaryMessage) {
			target = io.MultiWriter(writer, capture)
		}
		_, copyErr := io.Copy(target, reader)
		closeErr := writer.Close()
		if copyErr != nil {
			return websocketCopyResult{consumption: total, err: copyErr}
		}
		if closeErr != nil {
			return websocketCopyResult{consumption: total, err: closeErr}
		}
		if capture.buf.Len() > 0 {
			usage := parseTokenConsumption(http.Header{"Content-Type": []string{"application/json"}}, capture.Bytes())
			total = addTokenConsumption(total, usage)
		}
	}
}

func addTokenConsumption(left token.TokenConsumption, right token.TokenConsumption) token.TokenConsumption {
	return token.TokenConsumption{
		InputTokens:  left.InputTokens + right.InputTokens,
		OutputTokens: left.OutputTokens + right.OutputTokens,
		TotalTokens:  left.TotalTokens + right.TotalTokens,
	}
}

func isNormalWebSocketClose(err error) bool {
	return websocket.IsCloseError(err,
		websocket.CloseNormalClosure,
		websocket.CloseGoingAway,
		websocket.CloseNoStatusReceived)
}

func tokenListContains(value string, token string) bool {
	for _, part := range strings.Split(value, ",") {
		if strings.EqualFold(strings.TrimSpace(part), token) {
			return true
		}
	}
	return false
}

func upstreamRespBody(resp *http.Response) io.Closer {
	if resp == nil {
		return nil
	}
	return resp.Body
}

func responseHeaders(resp *http.Response) http.Header {
	if resp == nil {
		return nil
	}
	return resp.Header
}

func cooldownUntilFromHeaders(header http.Header) *time.Time {
	now := time.Now()
	for _, key := range []string{
		"Retry-After",
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

func readProxyRequestBody(body io.ReadCloser) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	defer closeBody(body)

	limited := io.LimitReader(body, maxProxyRequestBodyBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if len(data) > maxProxyRequestBodyBytes {
		return nil, errRequestBodyTooLarge
	}
	return data, nil
}

func closeBody(body io.Closer) {
	if body != nil {
		_ = body.Close()
	}
}

func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	default:
		return a + b
	}
}

type flushWriter struct {
	writer  io.Writer
	flusher http.Flusher
}

func (w flushWriter) Write(p []byte) (int, error) {
	n, err := w.writer.Write(p)
	w.flusher.Flush()
	return n, err
}
