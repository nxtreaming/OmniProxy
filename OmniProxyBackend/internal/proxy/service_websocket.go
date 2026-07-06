package proxy

import (
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"io"
	"net/http"
	"net/url"
	"omniproxy/internal/history"
	"omniproxy/internal/logs"
	"omniproxy/internal/token"
	"strings"
	"time"
)

func (s *Service) serveCodexWebSocket(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	route := routeWithClient(r, s.router.Route(r.URL, nil))

	excluded := map[string]bool{}
	attempts := s.attemptsForRoute(route)

	var selected token.Token
	var upstream *websocket.Conn
	var upstreamResp *http.Response
	var lastErr error
	var lastStatus int
	lastRoute := route
	finishActive := func() {}
	retryChain := make([]history.RetryAttempt, 0, attempts)

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
		selected = tokenAttempt.selected

		targetURL, err := s.router.TargetWebSocketURL(attemptRoute, selected)
		if err != nil {
			lastErr = err
			s.tokens.Release(selected.ID)
			retryChain = appendRetryAttempt(retryChain, attempt, attemptRoute, &selected, 0, time.Since(attemptStart).Milliseconds(), false, err.Error())
			break
		}

		header := websocketRequestHeader(r.Header)
		removeClientIdentificationHeaders(header)
		if err := applyRouteAuth(header, selected, attemptRoute); err != nil {
			lastErr = err
			s.tokens.Release(selected.ID)
			retryChain = appendRetryAttempt(retryChain, attempt, attemptRoute, &selected, 0, time.Since(attemptStart).Milliseconds(), false, err.Error())
			break
		}

		finishActive = s.beginActiveRequest(r, attemptRoute, selected)
		dialer := websocket.Dialer{
			HandshakeTimeout:  45 * time.Second,
			Proxy:             s.proxyForRoute(attemptRoute),
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
		if s.shouldRetryUpstreamResponse(attemptRoute, selected, lastStatus, attempt, attempts) {
			switchMessage := upstreamWebSocketSwitchMessage(attemptRoute, selected, lastStatus)
			retryChain = s.retryUpstreamAttempt(r, attemptRoute, selected, lastStatus, responseHeaders(upstreamResp), attempt, attemptStart, retryChain, excluded, upstreamRespBody(upstreamResp), fmt.Sprintf("upstream websocket returned %d", lastStatus), switchMessage, switchMessage)
			continue
		}
		retryChain = appendRetryAttempt(retryChain, attempt, attemptRoute, &selected, lastStatus, time.Since(attemptStart).Milliseconds(), false, fmt.Sprintf("upstream websocket failed: %v", err))
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
			ClientKey:  lastRoute.ClientKey,
			ClientName: lastRoute.ClientName,
			Model:      lastRoute.Model,
			Status:     status,
			Duration:   time.Since(start).Milliseconds(),
			Message:    fmt.Sprintf("websocket proxy failed: %v", lastErr),
		})
		if len(retryChain) == 0 {
			retryChain = appendRetryAttempt(retryChain, 1, lastRoute, nil, status, time.Since(start).Milliseconds(), false, fmt.Sprintf("websocket proxy failed: %v", lastErr))
		}
		s.recordHistory(r, lastRoute, nil, status, time.Since(start).Milliseconds(), token.TokenConsumption{}, logs.LevelError, fmt.Sprintf("websocket proxy failed: %v", lastErr), retryChain...)
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
			ClientKey:  lastRoute.ClientKey,
			ClientName: lastRoute.ClientName,
			Model:      lastRoute.Model,
			Status:     http.StatusBadRequest,
			Duration:   time.Since(start).Milliseconds(),
			TokenName:  token.DisplayName(selected),
			Message:    fmt.Sprintf("websocket client upgrade failed: %v", err),
		})
		s.recordHistory(r, lastRoute, &selected, http.StatusBadRequest, time.Since(start).Milliseconds(), token.TokenConsumption{}, logs.LevelError, fmt.Sprintf("websocket client upgrade failed: %v", err))
		return
	}
	defer client.Close()

	_ = s.tokens.RecordUsage(selected.ID, -1)
	consumption, err := proxyWebSocketMessages(client, upstream)
	finishActive()
	_ = s.tokens.RecordProxyUsage(selected.ID, consumption)
	s.tokens.Release(selected.ID)

	level := logs.LevelInfo
	message := proxyLogMessage(lastRoute.Model, consumption, "websocket proxied")
	if err != nil && !isNormalWebSocketClose(err) {
		level = logs.LevelWarn
		message = fmt.Sprintf("websocket closed with error: %v", err)
	}
	s.logs.Add(logs.Entry{
		Level:      level,
		Method:     r.Method,
		Path:       r.URL.RequestURI(),
		ClientKey:  lastRoute.ClientKey,
		ClientName: lastRoute.ClientName,
		Model:      lastRoute.Model,
		Status:     http.StatusSwitchingProtocols,
		Duration:   time.Since(start).Milliseconds(),
		TokenName:  token.DisplayName(selected),
		Message:    message,
	})
	if len(retryChain) == 0 || retryChain[len(retryChain)-1].Status != http.StatusSwitchingProtocols {
		retryChain = appendRetryAttempt(retryChain, len(retryChain)+1, lastRoute, &selected, http.StatusSwitchingProtocols, time.Since(start).Milliseconds(), false, message)
	}
	s.recordHistory(r, lastRoute, &selected, http.StatusSwitchingProtocols, time.Since(start).Milliseconds(), consumption, level, message, retryChain...)
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
		InputTokens:         left.InputTokens + right.InputTokens,
		OutputTokens:        left.OutputTokens + right.OutputTokens,
		TotalTokens:         left.TotalTokens + right.TotalTokens,
		CacheCreationTokens: left.CacheCreationTokens + right.CacheCreationTokens,
		CacheReadTokens:     left.CacheReadTokens + right.CacheReadTokens,
	}
}

func isNormalWebSocketClose(err error) bool {
	return websocket.IsCloseError(err,
		websocket.CloseNormalClosure,
		websocket.CloseGoingAway,
		websocket.CloseNoStatusReceived)
}
