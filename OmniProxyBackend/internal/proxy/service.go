package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"omniproxy/internal/claudedesktop"
	"omniproxy/internal/config"
	"omniproxy/internal/history"
	"omniproxy/internal/logs"
	"omniproxy/internal/token"
	"sort"
	"sync"
	"time"
)

type Service struct {
	cfg            config.Config
	tokens         *token.Manager
	logs           *logs.Recorder
	history        *history.Recorder
	router         Router
	retry          RetryPolicy
	client         *http.Client
	proxyClient    *http.Client
	proxyURL       *url.URL
	tokenRefresher TokenRefresher
	zoModelsMu     sync.Mutex
	zoModelsCache  map[string]zoModelCacheEntry
	activeMu       sync.RWMutex
	activeSeq      int64
	activeRequests map[int64]ActiveRequest
	activity       ActivityObserver
}

const maxProxyRequestBodyBytes = 32 * 1024 * 1024

var errRequestBodyTooLarge = errors.New("request body too large")

type TokenRefresher func(context.Context, token.Token, bool) (token.Token, bool, error)

type ActiveRequest struct {
	ID         int64     `json:"id"`
	StartedAt  time.Time `json:"startedAt"`
	ClientKey  string    `json:"clientKey,omitempty"`
	ClientName string    `json:"clientName,omitempty"`
	Method     string    `json:"method,omitempty"`
	Path       string    `json:"path,omitempty"`
	Provider   string    `json:"provider,omitempty"`
	Protocol   string    `json:"protocol,omitempty"`
	Model      string    `json:"model,omitempty"`
	TokenID    string    `json:"tokenId,omitempty"`
	TokenName  string    `json:"tokenName,omitempty"`
}

type ActivityObserver interface {
	ActiveRequestStarted(ActiveRequest)
	ActiveRequestFinished(ActiveRequest)
}

type tokenAttemptOutcome struct {
	selected   token.Token
	retryChain []history.RetryAttempt
	ready      bool
	stop       bool
	err        error
}

func NewService(cfg config.Config, tokens *token.Manager, recorder *logs.Recorder, historyRecorders ...*history.Recorder) (*Service, error) {
	cfg = config.Normalize(cfg)
	if err := ValidateProxyBaseURLs(cfg); err != nil {
		return nil, err
	}
	outboundProxy, err := outboundProxyURL(cfg)
	if err != nil {
		return nil, err
	}

	var requestHistory *history.Recorder
	if len(historyRecorders) > 0 {
		requestHistory = historyRecorders[0]
	}

	var proxyClient *http.Client
	if outboundProxy != nil {
		proxyClient = newHTTPClient(0, outboundProxy)
	}

	return &Service{
		cfg:         cfg,
		tokens:      tokens,
		logs:        recorder,
		history:     requestHistory,
		router:      NewRouter(cfg),
		retry:       NewRetryPolicy(cfg),
		client:      newHTTPClient(0, nil),
		proxyClient: proxyClient,
		proxyURL:    outboundProxy,
	}, nil
}

func (s *Service) SetTokenRefresher(refresher TokenRefresher) {
	s.tokenRefresher = refresher
}

func (s *Service) SetActivityObserver(observer ActivityObserver) {
	s.activity = observer
}

func (s *Service) ActiveRequests() []ActiveRequest {
	s.activeMu.RLock()
	defer s.activeMu.RUnlock()

	out := make([]ActiveRequest, 0, len(s.activeRequests))
	for _, item := range s.activeRequests {
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].StartedAt.Before(out[j].StartedAt)
	})
	return out
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if isCodexResponsesWebSocket(r) {
		route := routeWithClient(r, s.router.Route(r.URL, nil))
		if s.cfg.WebSocketMode == config.WebSocketModeDisabled {
			s.logs.Add(logs.Entry{
				Level:      logs.LevelWarn,
				Method:     r.Method,
				Path:       r.URL.RequestURI(),
				ClientKey:  route.ClientKey,
				ClientName: route.ClientName,
				Status:     http.StatusForbidden,
				Message:    "websocket proxy disabled",
			})
			s.recordHistory(r, route, nil, http.StatusForbidden, 0, token.TokenConsumption{}, logs.LevelWarn, "websocket proxy disabled")
			http.Error(w, "websocket proxy disabled", http.StatusForbidden)
			return
		}
		if !isAllowedWebSocketOrigin(r) {
			s.logs.Add(logs.Entry{
				Level:      logs.LevelWarn,
				Method:     r.Method,
				Path:       r.URL.RequestURI(),
				ClientKey:  route.ClientKey,
				ClientName: route.ClientName,
				Status:     http.StatusForbidden,
				Message:    "websocket origin not allowed",
			})
			s.recordHistory(r, route, nil, http.StatusForbidden, 0, token.TokenConsumption{}, logs.LevelWarn, "websocket origin not allowed")
			http.Error(w, "websocket origin not allowed", http.StatusForbidden)
			return
		}
		s.serveCodexWebSocket(w, r)
		return
	}
	if r.URL != nil && claudedesktop.IsModelsPath(r.URL.Path) {
		s.serveClaudeDesktopModels(w, r)
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
	if isOpenCodeRouterProbe(r) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if r.Method != http.MethodHead {
			_, _ = w.Write([]byte(`{"ok":true,"service":"omniproxy opencode router"}`))
		}
		return
	}
	if isPiRouterProbe(r) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if r.Method != http.MethodHead {
			_, _ = w.Write([]byte(`{"ok":true,"service":"omniproxy pi router"}`))
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
	bodyBytes, decodedBody, err := readProxyRequestBody(r.Body, r.Header.Get("Content-Encoding"))
	if err != nil {
		route := routeWithClient(r, routeInfo{Provider: token.ProviderOpenAI, Path: r.URL.Path, RawQuery: r.URL.RawQuery})
		if errors.Is(err, errRequestBodyTooLarge) {
			s.recordHistory(r, route, nil, http.StatusRequestEntityTooLarge, time.Since(start).Milliseconds(), token.TokenConsumption{}, logs.LevelWarn, errRequestBodyTooLarge.Error())
			http.Error(w, errRequestBodyTooLarge.Error(), http.StatusRequestEntityTooLarge)
			return
		}
		s.recordHistory(r, route, nil, http.StatusBadRequest, time.Since(start).Milliseconds(), token.TokenConsumption{}, logs.LevelWarn, "failed to read request body")
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	if decodedBody {
		r.Header.Del("Content-Encoding")
		r.Header.Del("Content-Length")
	}

	excluded := map[string]bool{}

	var lastErr error
	var lastStatus int
	if r.URL != nil && claudedesktop.IsMessagesPath(r.URL.Path) {
		if err := claudedesktop.ValidateGatewayAuth(r.Header); err != nil {
			route := routeWithClient(r, routeInfo{Provider: token.ProviderAnthropic, Protocol: "anthropic", Path: r.URL.Path})
			s.logs.Add(logs.Entry{
				Level:      logs.LevelWarn,
				Method:     r.Method,
				Path:       r.URL.RequestURI(),
				ClientKey:  route.ClientKey,
				ClientName: route.ClientName,
				Status:     http.StatusUnauthorized,
				Message:    err.Error(),
			})
			s.recordHistory(r, route, nil, http.StatusUnauthorized, time.Since(start).Milliseconds(), token.TokenConsumption{}, logs.LevelWarn, err.Error())
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		routes, err := claudedesktop.LoadRoutes()
		if err != nil {
			route := routeWithClient(r, routeInfo{Provider: token.ProviderAnthropic, Protocol: "anthropic", Path: r.URL.Path})
			s.recordHistory(r, route, nil, http.StatusServiceUnavailable, time.Since(start).Milliseconds(), token.TokenConsumption{}, logs.LevelError, err.Error())
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		updatedBody, _, err := claudedesktop.RewriteRequestBody(bodyBytes, routes)
		if err != nil {
			route := routeWithClient(r, routeInfo{Provider: token.ProviderAnthropic, Protocol: "anthropic", Path: r.URL.Path})
			s.recordHistory(r, route, nil, http.StatusBadRequest, time.Since(start).Milliseconds(), token.TokenConsumption{}, logs.LevelWarn, err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		bodyBytes = updatedBody
	}
	route := routeWithClient(r, s.router.Route(r.URL, bodyBytes))
	attempts := s.attemptsForRoute(route)
	retryChain := make([]history.RetryAttempt, 0, attempts)
	lastRoute := route

	for attempt := 1; attempt <= attempts; attempt++ {
		attemptStart := time.Now()
		attemptRoute, tokenAttempt := s.prepareCandidateTokenAttempt(r.Context(), r, route, excluded, retryChain, attempt, attemptStart)
		lastRoute = attemptRoute
		retryChain = tokenAttempt.retryChain
		if !tokenAttempt.ready {
			lastErr = tokenAttempt.err
			if tokenAttempt.stop {
				break
			}
			continue
		}
		selected := tokenAttempt.selected

		finishActive := s.beginActiveRequest(r, attemptRoute, selected)
		resp, err := s.forward(r.Context(), r, attemptRoute, bodyBytes, selected)
		if err != nil {
			finishActive()
			lastErr = err
			_ = s.tokens.RecordProxyUsage(selected.ID, token.TokenConsumption{})
			s.tokens.Release(selected.ID)
			excluded[selected.ID] = true
			s.logs.Add(logs.Entry{
				Level:      logs.LevelWarn,
				Method:     r.Method,
				Path:       r.URL.RequestURI(),
				ClientKey:  attemptRoute.ClientKey,
				ClientName: attemptRoute.ClientName,
				Model:      attemptRoute.Model,
				TokenName:  selected.Name,
				Message:    proxyLogMessage(attemptRoute.Model, token.TokenConsumption{}, "upstream request failed, trying next token"),
			})
			retryChain = appendRetryAttempt(retryChain, attempt, attemptRoute, &selected, http.StatusBadGateway, time.Since(attemptStart).Milliseconds(), false, fmt.Sprintf("upstream request failed: %v", err))
			continue
		}

		lastStatus = resp.StatusCode
		if remaining, ok := parseRemaining(resp.Header); ok {
			_ = s.tokens.RecordUsage(selected.ID, remaining)
		} else {
			_ = s.tokens.RecordUsage(selected.ID, -1)
		}

		if s.shouldRetryUpstreamResponse(attemptRoute, selected, resp.StatusCode, attempt, attempts) {
			finishActive()
			switchMessage := upstreamSwitchMessage(attemptRoute, selected, resp.StatusCode)
			retryChain = s.retryUpstreamAttempt(r, attemptRoute, selected, resp.StatusCode, resp.Header, attempt, attemptStart, retryChain, excluded, resp.Body, fmt.Sprintf("upstream returned %d", resp.StatusCode), proxyLogMessage(attemptRoute.Model, token.TokenConsumption{}, switchMessage), switchMessage)
			continue
		}

		consumption, responseBody := s.writeResponse(w, resp)
		finishActive()
		if responseModel := parseResponseModel(resp.Header, responseBody); responseModel != "" {
			attemptRoute.Model = responseModel
		}
		_ = s.tokens.RecordProxyUsage(selected.ID, consumption)
		cooldownUntil := s.cooldownUntilForUpstreamResponse(attemptRoute, selected, resp.StatusCode, resp.Header)
		cooldownTriggered := cooldownUntil != nil
		if cooldownTriggered {
			_ = s.tokens.MarkExhaustedUntil(selected.ID, fmt.Sprintf("upstream returned %d", resp.StatusCode), cooldownUntil)
		}
		s.tokens.Release(selected.ID)
		historyMessage := proxyHistoryMessage(resp.StatusCode, attemptRoute.Model, consumption, "request proxied", responseBody)
		retryChain = appendRetryAttempt(retryChain, attempt, attemptRoute, &selected, resp.StatusCode, time.Since(attemptStart).Milliseconds(), cooldownTriggered, historyMessage)
		s.logs.Add(logs.Entry{
			Level:      levelForStatus(resp.StatusCode),
			Method:     r.Method,
			Path:       r.URL.RequestURI(),
			ClientKey:  attemptRoute.ClientKey,
			ClientName: attemptRoute.ClientName,
			Model:      attemptRoute.Model,
			Status:     resp.StatusCode,
			Duration:   time.Since(start).Milliseconds(),
			TokenName:  selected.Name,
			Message:    proxyLogMessage(attemptRoute.Model, consumption, "request proxied"),
		})
		s.recordHistory(r, attemptRoute, &selected, resp.StatusCode, time.Since(start).Milliseconds(), consumption, levelForStatus(resp.StatusCode), historyMessage, retryChain...)
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
		Level:      logs.LevelError,
		Method:     r.Method,
		Path:       r.URL.RequestURI(),
		ClientKey:  lastRoute.ClientKey,
		ClientName: lastRoute.ClientName,
		Model:      lastRoute.Model,
		Status:     status,
		Duration:   time.Since(start).Milliseconds(),
		Message:    fmt.Sprintf("proxy failed: %v", lastErr),
	})
	if len(retryChain) == 0 {
		retryChain = appendRetryAttempt(retryChain, 1, lastRoute, nil, status, time.Since(start).Milliseconds(), false, fmt.Sprintf("proxy failed: %v", lastErr))
	}
	s.recordHistory(r, lastRoute, nil, status, time.Since(start).Milliseconds(), token.TokenConsumption{}, logs.LevelError, fmt.Sprintf("proxy failed: %v", lastErr), retryChain...)
	http.Error(w, http.StatusText(status), status)
}

func (s *Service) serveClaudeDesktopModels(w http.ResponseWriter, r *http.Request) {
	route := routeWithClient(r, routeInfo{Provider: token.ProviderAnthropic, Protocol: "anthropic", Path: r.URL.Path})
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err := claudedesktop.ValidateGatewayAuth(r.Header); err != nil {
		s.logs.Add(logs.Entry{
			Level:      logs.LevelWarn,
			Method:     r.Method,
			Path:       r.URL.RequestURI(),
			ClientKey:  route.ClientKey,
			ClientName: route.ClientName,
			Status:     http.StatusUnauthorized,
			Message:    err.Error(),
		})
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	routes, err := claudedesktop.LoadRoutes()
	if err != nil {
		s.logs.Add(logs.Entry{
			Level:      logs.LevelError,
			Method:     r.Method,
			Path:       r.URL.RequestURI(),
			ClientKey:  route.ClientKey,
			ClientName: route.ClientName,
			Status:     http.StatusServiceUnavailable,
			Message:    err.Error(),
		})
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	if err := json.NewEncoder(w).Encode(claudedesktop.ModelListResponse(routes)); err != nil {
		s.logs.Add(logs.Entry{
			Level:      logs.LevelError,
			Method:     r.Method,
			Path:       r.URL.RequestURI(),
			ClientKey:  route.ClientKey,
			ClientName: route.ClientName,
			Status:     http.StatusOK,
			Message:    fmt.Sprintf("write Claude Desktop models response: %v", err),
		})
	}
}

func (s *Service) forward(ctx context.Context, original *http.Request, route routeInfo, body []byte, selected token.Token) (*http.Response, error) {
	if isCodexChatCompletionsRoute(route, selected) {
		return s.forwardCodexChatCompletions(ctx, original, route, body, selected)
	}
	if token.NormalizeProvider(route.Provider) == token.ProviderZo {
		return s.forwardZo(ctx, original, route, body, selected)
	}

	targetURL, err := s.router.TargetURL(route, selected)
	if err != nil {
		return nil, err
	}
	if updatedBody, changed := normalizeSub2APIRequestBody(original.URL.Path, route, body); changed {
		body = updatedBody
	}
	if updatedBody, changed := normalizeRequestBodyModel(body); changed {
		body = updatedBody
	}

	req, err := http.NewRequestWithContext(ctx, original.Method, targetURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	copyHeader(req.Header, original.Header)
	removeHopHeaders(req.Header)
	removeClientIdentificationHeaders(req.Header)
	if err := applyRouteAuth(req.Header, selected, route); err != nil {
		return nil, err
	}
	req.Host = req.URL.Host

	return s.clientForRoute(route).Do(req)
}

func (s *Service) clientForRoute(route routeInfo) *http.Client {
	if s.proxyClient != nil && outboundProxyMatchesRoute(route, s.cfg) {
		return s.proxyClient
	}
	return s.client
}

func (s *Service) proxyForRoute(route routeInfo) func(*http.Request) (*url.URL, error) {
	if s.proxyURL != nil && outboundProxyMatchesRoute(route, s.cfg) {
		return http.ProxyURL(s.proxyURL)
	}
	return http.ProxyFromEnvironment
}
