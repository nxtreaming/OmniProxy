package proxy

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"OmniProxyBackend/internal/claudedesktop"
	"OmniProxyBackend/internal/config"
	"OmniProxyBackend/internal/history"
	"OmniProxyBackend/internal/logs"
	"OmniProxyBackend/internal/token"
	"github.com/gorilla/websocket"
	"github.com/klauspost/compress/zstd"
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
		route := routeWithClient(r, routeInfo{Provider: token.ProviderOpenAI, CredentialType: token.CredentialTypeCodexAuthJSON, Protocol: "openai", Path: r.URL.Path, RawQuery: r.URL.RawQuery})
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

	for attempt := 1; attempt <= attempts; attempt++ {
		attemptStart := time.Now()
		tokenAttempt := s.prepareTokenAttempt(r.Context(), r, route, excluded, retryChain, attempt, attemptStart)
		retryChain = tokenAttempt.retryChain
		if !tokenAttempt.ready {
			lastErr = tokenAttempt.err
			if tokenAttempt.stop {
				break
			}
			continue
		}
		selected := tokenAttempt.selected

		finishActive := s.beginActiveRequest(r, route, selected)
		resp, err := s.forward(r.Context(), r, route, bodyBytes, selected)
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
				ClientKey:  route.ClientKey,
				ClientName: route.ClientName,
				Model:      route.Model,
				TokenName:  selected.Name,
				Message:    proxyLogMessage(route.Model, token.TokenConsumption{}, "upstream request failed, trying next token"),
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

		if s.shouldRetryUpstreamResponse(route, selected, resp.StatusCode, attempt, attempts) {
			finishActive()
			switchMessage := upstreamSwitchMessage(route, selected, resp.StatusCode)
			retryChain = s.retryUpstreamAttempt(r, route, selected, resp.StatusCode, resp.Header, attempt, attemptStart, retryChain, excluded, resp.Body, fmt.Sprintf("upstream returned %d", resp.StatusCode), proxyLogMessage(route.Model, token.TokenConsumption{}, switchMessage), switchMessage)
			continue
		}

		consumption, responseBody := s.writeResponse(w, resp)
		finishActive()
		if responseModel := parseResponseModel(resp.Header, responseBody); responseModel != "" {
			route.Model = responseModel
		}
		_ = s.tokens.RecordProxyUsage(selected.ID, consumption)
		cooldownUntil := s.cooldownUntilForUpstreamResponse(route, selected, resp.StatusCode, resp.Header)
		cooldownTriggered := cooldownUntil != nil
		if cooldownTriggered {
			_ = s.tokens.MarkExhaustedUntil(selected.ID, fmt.Sprintf("upstream returned %d", resp.StatusCode), cooldownUntil)
		}
		s.tokens.Release(selected.ID)
		historyMessage := proxyHistoryMessage(resp.StatusCode, route.Model, consumption, "request proxied", responseBody)
		retryChain = appendRetryAttempt(retryChain, attempt, route, &selected, resp.StatusCode, time.Since(attemptStart).Milliseconds(), cooldownTriggered, historyMessage)
		s.logs.Add(logs.Entry{
			Level:      levelForStatus(resp.StatusCode),
			Method:     r.Method,
			Path:       r.URL.RequestURI(),
			ClientKey:  route.ClientKey,
			ClientName: route.ClientName,
			Model:      route.Model,
			Status:     resp.StatusCode,
			Duration:   time.Since(start).Milliseconds(),
			TokenName:  selected.Name,
			Message:    proxyLogMessage(route.Model, consumption, "request proxied"),
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
		Level:      logs.LevelError,
		Method:     r.Method,
		Path:       r.URL.RequestURI(),
		ClientKey:  route.ClientKey,
		ClientName: route.ClientName,
		Model:      route.Model,
		Status:     status,
		Duration:   time.Since(start).Milliseconds(),
		Message:    fmt.Sprintf("proxy failed: %v", lastErr),
	})
	if len(retryChain) == 0 {
		retryChain = appendRetryAttempt(retryChain, 1, route, nil, status, time.Since(start).Milliseconds(), false, fmt.Sprintf("proxy failed: %v", lastErr))
	}
	s.recordHistory(r, route, nil, status, time.Since(start).Milliseconds(), token.TokenConsumption{}, logs.LevelError, fmt.Sprintf("proxy failed: %v", lastErr), retryChain...)
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
	if s.proxyClient != nil && outboundProxyMatchesModel(route.Model, s.cfg.OutboundProxyModels) {
		return s.proxyClient
	}
	return s.client
}

func (s *Service) proxyForRoute(route routeInfo) func(*http.Request) (*url.URL, error) {
	if s.proxyURL != nil && outboundProxyMatchesModel(route.Model, s.cfg.OutboundProxyModels) {
		return http.ProxyURL(s.proxyURL)
	}
	return http.ProxyFromEnvironment
}

func (s *Service) serveCodexWebSocket(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	route := routeWithClient(r, s.router.Route(r.URL, nil))
	route.CredentialType = token.CredentialTypeCodexAuthJSON

	excluded := map[string]bool{}
	attempts := s.attemptsForRoute(route)

	var selected token.Token
	var upstream *websocket.Conn
	var upstreamResp *http.Response
	var lastErr error
	var lastStatus int
	finishActive := func() {}
	retryChain := make([]history.RetryAttempt, 0, attempts)

	for attempt := 1; attempt <= attempts; attempt++ {
		attemptStart := time.Now()
		tokenAttempt := s.prepareTokenAttempt(r.Context(), r, route, excluded, retryChain, attempt, attemptStart)
		retryChain = tokenAttempt.retryChain
		if !tokenAttempt.ready {
			lastErr = tokenAttempt.err
			if tokenAttempt.stop {
				break
			}
			continue
		}
		selected = tokenAttempt.selected

		targetURL, err := s.router.TargetWebSocketURL(route, selected)
		if err != nil {
			lastErr = err
			s.tokens.Release(selected.ID)
			retryChain = appendRetryAttempt(retryChain, attempt, route, &selected, 0, time.Since(attemptStart).Milliseconds(), false, err.Error())
			break
		}

		header := websocketRequestHeader(r.Header)
		removeClientIdentificationHeaders(header)
		if err := applyRouteAuth(header, selected, route); err != nil {
			lastErr = err
			s.tokens.Release(selected.ID)
			retryChain = appendRetryAttempt(retryChain, attempt, route, &selected, 0, time.Since(attemptStart).Milliseconds(), false, err.Error())
			break
		}

		finishActive = s.beginActiveRequest(r, route, selected)
		dialer := websocket.Dialer{
			HandshakeTimeout:  45 * time.Second,
			Proxy:             s.proxyForRoute(route),
			Subprotocols:      websocket.Subprotocols(r),
			EnableCompression: true,
		}
		upstream, upstreamResp, err = dialer.DialContext(r.Context(), targetURL, header)
		if err == nil {
			break
		}
		finishActive()

		lastErr = err
		if upstreamResp != nil {
			lastStatus = upstreamResp.StatusCode
		}
		if s.shouldRetryUpstreamResponse(route, selected, lastStatus, attempt, attempts) {
			switchMessage := upstreamWebSocketSwitchMessage(route, selected, lastStatus)
			retryChain = s.retryUpstreamAttempt(r, route, selected, lastStatus, responseHeaders(upstreamResp), attempt, attemptStart, retryChain, excluded, upstreamRespBody(upstreamResp), fmt.Sprintf("upstream websocket returned %d", lastStatus), switchMessage, switchMessage)
			continue
		}
		retryChain = appendRetryAttempt(retryChain, attempt, route, &selected, lastStatus, time.Since(attemptStart).Milliseconds(), false, fmt.Sprintf("upstream websocket failed: %v", err))
		s.tokens.Release(selected.ID)
		break
	}

	if upstream == nil {
		finishActive()
		status := http.StatusBadGateway
		if errors.Is(lastErr, token.ErrNoActiveToken) {
			status = http.StatusServiceUnavailable
		}
		if lastStatus != 0 {
			status = lastStatus
		}
		closeBody(upstreamRespBody(upstreamResp))
		s.logs.Add(logs.Entry{
			Level:      logs.LevelError,
			Method:     r.Method,
			Path:       r.URL.RequestURI(),
			ClientKey:  route.ClientKey,
			ClientName: route.ClientName,
			Model:      route.Model,
			Status:     status,
			Duration:   time.Since(start).Milliseconds(),
			Message:    fmt.Sprintf("websocket proxy failed: %v", lastErr),
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
		CheckOrigin:       isAllowedWebSocketOrigin,
		EnableCompression: true,
	}
	client, err := upgrader.Upgrade(w, r, responseHeader)
	if err != nil {
		finishActive()
		s.tokens.Release(selected.ID)
		s.logs.Add(logs.Entry{
			Level:      logs.LevelError,
			Method:     r.Method,
			Path:       r.URL.RequestURI(),
			ClientKey:  route.ClientKey,
			ClientName: route.ClientName,
			Model:      route.Model,
			Status:     http.StatusBadRequest,
			Duration:   time.Since(start).Milliseconds(),
			TokenName:  selected.Name,
			Message:    fmt.Sprintf("websocket client upgrade failed: %v", err),
		})
		s.recordHistory(r, route, &selected, http.StatusBadRequest, time.Since(start).Milliseconds(), token.TokenConsumption{}, logs.LevelError, fmt.Sprintf("websocket client upgrade failed: %v", err))
		return
	}
	defer client.Close()

	_ = s.tokens.RecordUsage(selected.ID, -1)
	consumption, err := proxyWebSocketMessages(client, upstream)
	finishActive()
	_ = s.tokens.RecordProxyUsage(selected.ID, consumption)
	s.tokens.Release(selected.ID)

	level := logs.LevelInfo
	message := proxyLogMessage(route.Model, consumption, "websocket proxied")
	if err != nil && !isNormalWebSocketClose(err) {
		level = logs.LevelWarn
		message = fmt.Sprintf("websocket closed with error: %v", err)
	}
	s.logs.Add(logs.Entry{
		Level:      level,
		Method:     r.Method,
		Path:       r.URL.RequestURI(),
		ClientKey:  route.ClientKey,
		ClientName: route.ClientName,
		Model:      route.Model,
		Status:     http.StatusSwitchingProtocols,
		Duration:   time.Since(start).Milliseconds(),
		TokenName:  selected.Name,
		Message:    message,
	})
	if len(retryChain) == 0 || retryChain[len(retryChain)-1].Status != http.StatusSwitchingProtocols {
		retryChain = appendRetryAttempt(retryChain, len(retryChain)+1, route, &selected, http.StatusSwitchingProtocols, time.Since(start).Milliseconds(), false, message)
	}
	s.recordHistory(r, route, &selected, http.StatusSwitchingProtocols, time.Since(start).Milliseconds(), consumption, level, message, retryChain...)
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
			TokenName:  selected.Name,
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
		TokenName:  selected.Name,
		Message:    logMessage,
	})
	return appendRetryAttempt(retryChain, attempt, route, &selected, status, time.Since(attemptStart).Milliseconds(), cooldownTriggered, historyMessage)
}

func (s *Service) acquireToken(route routeInfo, excluded map[string]bool) (token.Token, error) {
	provider := token.NormalizeProvider(route.Provider)
	credentialType := strings.TrimSpace(route.CredentialType)
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
		return 2
	}
	return attempts
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
		ClientKey:         route.ClientKey,
		ClientName:        route.ClientName,
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

func (s *Service) beginActiveRequest(r *http.Request, route routeInfo, selected token.Token) func() {
	s.activeMu.Lock()
	defer s.activeMu.Unlock()

	if s.activeRequests == nil {
		s.activeRequests = map[int64]ActiveRequest{}
	}
	s.activeSeq++
	id := s.activeSeq
	path := ""
	method := ""
	if r != nil {
		method = r.Method
		if r.URL != nil {
			path = r.URL.RequestURI()
		}
	}
	s.activeRequests[id] = ActiveRequest{
		ID:         id,
		StartedAt:  time.Now(),
		ClientKey:  route.ClientKey,
		ClientName: route.ClientName,
		Method:     method,
		Path:       path,
		Provider:   token.NormalizeProvider(route.Provider),
		Protocol:   route.Protocol,
		Model:      route.Model,
		TokenID:    selected.ID,
		TokenName:  selected.Name,
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			s.activeMu.Lock()
			delete(s.activeRequests, id)
			s.activeMu.Unlock()
		})
	}
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

func proxyLogMessage(_ string, consumption token.TokenConsumption, fallback string) string {
	message := fallback
	if consumption.TotalTokens <= 0 {
		return message
	}
	return fmt.Sprintf("%s, used %d tokens", message, consumption.TotalTokens)
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

func removeClientIdentificationHeaders(header http.Header) {
	for _, key := range []string{
		"X-OmniProxy-Client",
		"X-Client-Name",
		"X-Source-Client",
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

func isAllowedWebSocketOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" && scheme != "wails" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "wails.localhost"
}

func isWebSocketRequestHeader(key string) bool {
	switch http.CanonicalHeaderKey(key) {
	case "Connection",
		"Host",
		"Origin",
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

func readProxyRequestBody(body io.ReadCloser, contentEncoding string) ([]byte, bool, error) {
	if body == nil {
		return nil, false, nil
	}
	defer closeBody(body)

	reader, decoded, err := decodedRequestBodyReader(body, contentEncoding)
	if err != nil {
		return nil, false, err
	}
	if decodedCloser, ok := reader.(io.Closer); ok {
		defer closeBody(decodedCloser)
	}

	limited := io.LimitReader(reader, maxProxyRequestBodyBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, decoded, err
	}
	if len(data) > maxProxyRequestBodyBytes {
		return nil, decoded, errRequestBodyTooLarge
	}
	return data, decoded, nil
}

func decodedRequestBodyReader(body io.Reader, contentEncoding string) (io.Reader, bool, error) {
	encoding := strings.ToLower(strings.TrimSpace(contentEncoding))
	if encoding == "" || encoding == "identity" {
		return body, false, nil
	}
	parts := strings.Split(encoding, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	if len(parts) != 1 {
		return body, false, nil
	}
	switch parts[0] {
	case "zstd":
		reader, err := zstd.NewReader(body)
		if err != nil {
			return nil, false, err
		}
		return reader.IOReadCloser(), true, nil
	case "gzip":
		reader, err := gzip.NewReader(body)
		if err != nil {
			return nil, false, err
		}
		return reader, true, nil
	case "deflate":
		reader, err := zlib.NewReader(body)
		if err != nil {
			return nil, false, err
		}
		return reader, true, nil
	default:
		return body, false, nil
	}
}

func closeBody(body io.Closer) {
	if body != nil {
		_ = body.Close()
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
