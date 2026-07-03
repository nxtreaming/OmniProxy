package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"OmniProxyBackend/internal/claudedesktop"
	"OmniProxyBackend/internal/config"
	"OmniProxyBackend/internal/token"
)

type Router struct {
	cfg config.Config
}

type routeInfo struct {
	Provider       string
	CredentialType string
	Protocol       string
	ClientKey      string
	ClientName     string
	Model          string
	Path           string
	RawQuery       string
	Fallbacks      []routeInfo
}

func NewRouter(cfg config.Config) Router {
	return Router{cfg: config.Normalize(cfg)}
}

func (r Router) Route(incoming *url.URL, body []byte) routeInfo {
	path := incoming.Path
	model := requestModel(incoming, body)
	if isAnthropicRouterPath(path) {
		route := r.gatewayRoute(config.GatewayRouteClaude, model)
		return r.gatewayRouteInfo(route, "anthropic", stripPathPrefix(path, "/anthropic-router"), incoming.RawQuery)
	}
	if claudedesktop.IsGatewayPath(path) {
		route := r.gatewayRoute(config.GatewayRouteClaude, model)
		return r.gatewayRouteInfo(route, "anthropic", claudedesktop.StripGatewayPath(path), incoming.RawQuery)
	}
	if isOpenCodeRouterPath(path) {
		route := r.gatewayRoute(config.GatewayRouteOpenAI, model)
		return r.gatewayRouteInfo(route, "openai", stripPathPrefix(path, "/opencode-router"), incoming.RawQuery)
	}
	if isPiRouterPath(path) {
		route := r.gatewayRoute(config.GatewayRouteOpenAI, model)
		return r.gatewayRouteInfo(route, "openai", stripPathPrefix(path, "/pi-router"), incoming.RawQuery)
	}
	if isCodexProxyPath(path) {
		route := r.gatewayRoute(config.GatewayRouteCodex, model)
		return r.gatewayRouteInfo(route, "openai", stripPathPrefix(path, "/backend-api/codex"), incoming.RawQuery)
	}
	if isCodexGatewayPath(path) {
		route := r.gatewayRoute(config.GatewayRouteCodex, model)
		return r.gatewayRouteInfo(route, "openai", stripPathPrefix(path, "/codex"), incoming.RawQuery)
	}
	if isGeminiGatewayPath(path) {
		route := r.gatewayRoute(config.GatewayRouteGemini, model)
		return r.gatewayRouteInfo(route, "gemini", stripPathPrefix(path, "/gemini"), incoming.RawQuery)
	}

	defaultRoute := r.gatewayRoute(config.GatewayRouteOpenAI, model)
	provider := defaultRoute.Provider
	credentialType := defaultRoute.CredentialType
	model = defaultRoute.Model
	directProvider := false

	trimmed := strings.TrimPrefix(path, "/")
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) > 0 {
		candidate := strings.ToLower(parts[0])
		switch candidate {
		case token.ProviderOpenAI, token.ProviderAnthropic, token.ProviderDeepSeek, token.ProviderKimi, token.ProviderXiaomi, token.ProviderZhipu, token.ProviderMiniMax, token.ProviderGemini, token.ProviderOpenRouter, token.ProviderTokenRouter, token.ProviderSub2API, token.ProviderNewAPI, token.ProviderAnyRouter, token.ProviderZo, token.ProviderPrem, token.ProviderCustom:
			directProvider = true
			provider = candidate
			credentialType = ""
			if len(parts) == 2 {
				path = "/" + parts[1]
			} else {
				path = "/"
			}
		}
	}
	if !directProvider {
		return r.gatewayRouteInfo(defaultRoute, "openai", path, incoming.RawQuery)
	}
	protocol := protocolForRoute(provider, &path)
	if provider == token.ProviderPrem {
		path = premProxyPath(protocol, path)
	} else if gatewayProviderUsesProtocolPrefixes(provider) && protocol == "openai" {
		path = versionedGatewayOpenAIPath(path)
	}
	return routeInfo{Provider: provider, CredentialType: credentialType, Protocol: protocol, Model: model, Path: path, RawQuery: incoming.RawQuery}
}

func (r Router) gatewayRoute(name string, requestModel string) config.GatewayRouteConfig {
	routes := config.Normalize(r.cfg).GatewayRoutes
	var route config.GatewayRouteConfig
	switch name {
	case config.GatewayRouteCodex:
		route = routes.Codex
	case config.GatewayRouteClaude:
		route = routes.Claude
	case config.GatewayRouteOpenAI:
		route = routes.OpenAI
	case config.GatewayRouteGemini:
		route = routes.Gemini
	default:
		route = routes.OpenAI
	}
	if model := strings.TrimSpace(requestModel); model != "" {
		route.Model = model
		for i := range route.Fallbacks {
			route.Fallbacks[i].Model = model
		}
	}
	return route
}

func (r Router) gatewayRouteInfo(route config.GatewayRouteConfig, protocol string, path string, rawQuery string) routeInfo {
	primary := gatewayBackendRoute(route, protocol, path, rawQuery)
	for _, fallback := range route.Fallbacks {
		primary.Fallbacks = append(primary.Fallbacks, gatewayBackendRoute(fallback, protocol, path, rawQuery))
	}
	return primary
}

func gatewayBackendRoute(route config.GatewayRouteConfig, protocol string, path string, rawQuery string) routeInfo {
	provider := token.NormalizeProvider(route.Provider)
	return routeInfo{
		Provider:       provider,
		CredentialType: route.CredentialType,
		Protocol:       protocol,
		Model:          route.Model,
		Path:           gatewayBackendPath(provider, protocol, path),
		RawQuery:       rawQuery,
	}
}

func gatewayBackendPath(provider string, protocol string, path string) string {
	if provider == token.ProviderPrem {
		return premProxyPath(protocol, path)
	}
	if gatewayProviderUsesProtocolPrefixes(provider) && protocol == "openai" {
		return versionedGatewayOpenAIPath(path)
	}
	return path
}

func (r Router) TargetWebSocketURL(route routeInfo, selected token.Token) (string, error) {
	targetURL, err := r.TargetURL(route, selected)
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

func (r Router) TargetURL(route routeInfo, selected token.Token) (string, error) {
	baseURL := r.BaseURL(route, selected)
	if baseURL == "" {
		return "", fmt.Errorf("%s upstream base url is not configured", route.Provider)
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}

	out := *base
	out.Path = singleJoiningSlash(base.Path, upstreamPathForBase(base.Path, route, selected))
	out.RawQuery = route.RawQuery
	return out.String(), nil
}

func (r Router) BaseURL(route routeInfo, selected token.Token) string {
	return routeBaseURL(r.cfg, route, selected)
}

func protocolForRoute(provider string, path *string) string {
	protocol := "openai"
	if provider == token.ProviderAnthropic {
		protocol = "anthropic"
	}
	switch provider {
	case token.ProviderGemini:
		protocol = "gemini"
	case token.ProviderSub2API, token.ProviderNewAPI:
		if stripProtocolPrefix(path, "/anthropic") {
			protocol = "anthropic"
		} else if stripProtocolPrefix(path, "/gemini") {
			protocol = "gemini"
		}
	case token.ProviderAnyRouter:
		if stripProtocolPrefix(path, "/anthropic") {
			protocol = "anthropic"
		}
	case token.ProviderPrem:
		if stripProtocolPrefix(path, "/anthropic") {
			protocol = "anthropic"
		} else if stripProtocolPrefix(path, "/openai") {
			protocol = "openai"
		} else if isAnthropicMessagePath(*path) {
			protocol = "anthropic"
		}
	case token.ProviderDeepSeek, token.ProviderKimi, token.ProviderXiaomi, token.ProviderZhipu, token.ProviderMiniMax, token.ProviderZo, token.ProviderCustom:
		if stripProtocolPrefix(path, "/anthropic") {
			protocol = "anthropic"
		}
	}
	if provider == token.ProviderZo && isAnthropicMessagePath(*path) {
		protocol = "anthropic"
	}
	return protocol
}

func isAnthropicMessagePath(path string) bool {
	path = strings.TrimSpace(path)
	return path == "/messages" || path == "/v1/messages" || path == "/v1/v1/messages"
}

func stripProtocolPrefix(path *string, prefix string) bool {
	if *path == prefix {
		*path = "/"
		return true
	}
	withSlash := prefix + "/"
	if strings.HasPrefix(*path, withSlash) {
		*path = "/" + strings.TrimPrefix(*path, withSlash)
		return true
	}
	return false
}

func gatewayProviderUsesProtocolPrefixes(provider string) bool {
	return provider == token.ProviderSub2API || provider == token.ProviderNewAPI || provider == token.ProviderAnyRouter
}

func versionedGatewayOpenAIPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || path == "/" || path == "/v1" || strings.HasPrefix(path, "/v1/") {
		return path
	}
	return singleJoiningSlash("/v1", path)
}

func premProxyPath(protocol string, path string) string {
	if protocol == "anthropic" {
		return singleJoiningSlash("/anthropic", versionedGatewayOpenAIPath(path))
	}
	return singleJoiningSlash("/openai", versionedGatewayOpenAIPath(path))
}

func isAnthropicRouterPath(path string) bool {
	return path == "/anthropic-router" || strings.HasPrefix(path, "/anthropic-router/")
}

func isOpenCodeRouterPath(path string) bool {
	return path == "/opencode-router" || strings.HasPrefix(path, "/opencode-router/")
}

func isPiRouterPath(path string) bool {
	return path == "/pi-router" || strings.HasPrefix(path, "/pi-router/")
}

func isGeminiGatewayPath(path string) bool {
	return path == "/gemini" || strings.HasPrefix(path, "/gemini/")
}

func isCodexGatewayPath(path string) bool {
	return path == "/codex" || strings.HasPrefix(path, "/codex/")
}

func isAnthropicRouterProbe(r *http.Request) bool {
	if r.URL == nil || r.URL.Path != "/anthropic-router" {
		return false
	}
	return r.Method == http.MethodHead || r.Method == http.MethodGet
}

func isOpenCodeRouterProbe(r *http.Request) bool {
	if r.URL == nil || r.URL.Path != "/opencode-router" {
		return false
	}
	return r.Method == http.MethodHead || r.Method == http.MethodGet
}

func isPiRouterProbe(r *http.Request) bool {
	if r.URL == nil || r.URL.Path != "/pi-router" {
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
	path = stripPathPrefix(path, "/codex")
	path = stripPathPrefix(path, "/v1")
	return path == "/responses"
}

func isCodexResponsesWebSocket(r *http.Request) bool {
	if r.URL == nil || r.Method != http.MethodGet || !isWebSocketUpgrade(r) {
		return false
	}
	path := stripPathPrefix(r.URL.Path, "/backend-api/codex")
	path = stripPathPrefix(path, "/codex")
	path = stripPathPrefix(path, "/v1")
	return path == "/responses"
}

func isWebSocketUpgrade(r *http.Request) bool {
	return tokenListContains(r.Header.Get("Connection"), "upgrade") &&
		strings.EqualFold(strings.TrimSpace(r.Header.Get("Upgrade")), "websocket")
}

func requestModel(incoming *url.URL, body []byte) string {
	var payload struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &payload); err == nil {
		if model := strings.TrimSpace(payload.Model); model != "" {
			return model
		}
	}
	if incoming == nil {
		return ""
	}
	if model := strings.TrimSpace(incoming.Query().Get("model")); model != "" {
		return model
	}
	return requestModelFromPath(incoming.Path)
}

func requestModelFromPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	const marker = "/models/"
	index := strings.Index(path, marker)
	if index < 0 {
		return ""
	}
	model := path[index+len(marker):]
	if slash := strings.Index(model, "/"); slash >= 0 {
		model = model[:slash]
	}
	if colon := strings.Index(model, ":"); colon >= 0 {
		model = model[:colon]
	}
	return strings.TrimSpace(model)
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
	if strings.HasPrefix(model, "glm-") || strings.HasPrefix(model, "zhipu-") {
		return token.ProviderZhipu
	}
	if strings.HasPrefix(model, "minimax-") {
		return token.ProviderMiniMax
	}
	if model == "claude-opus-4-7" || model == "claude-sonnet-4-6" {
		return token.ProviderZo
	}
	return token.ProviderAnthropic
}

func providerForOpenCodeModel(model string) string {
	model = strings.ToLower(strings.TrimSpace(model))
	if model == "" {
		return token.ProviderOpenAI
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
	if strings.HasPrefix(model, "glm-") || strings.HasPrefix(model, "zhipu-") {
		return token.ProviderZhipu
	}
	if strings.HasPrefix(model, "minimax-") {
		return token.ProviderMiniMax
	}
	if strings.HasPrefix(model, "auto:") || strings.HasPrefix(model, "tokenrouter:") || strings.HasPrefix(model, "tokenrouter/") {
		return token.ProviderTokenRouter
	}
	if strings.Contains(model, "/") {
		return token.ProviderOpenRouter
	}
	if strings.HasPrefix(model, "custom-") {
		return token.ProviderCustom
	}
	return token.ProviderOpenAI
}

func providerForPiModel(model string) (string, string) {
	provider := providerForOpenCodeModel(model)
	credentialType := ""
	if provider == token.ProviderOpenAI {
		credentialType = token.CredentialTypeAPIKey
	}
	return provider, credentialType
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
	next = stripPathPrefix(next, "/codex")
	next = stripPathPrefix(next, "/v1")
	if next == "" {
		return "/"
	}
	return next
}

func upstreamPathForBase(basePath string, route routeInfo, selected token.Token) string {
	path := upstreamPath(route.Path, selected)
	if basePathHasVersionSuffix(basePath) && strings.HasPrefix(path, "/v1/") && (route.Protocol == "openai" || token.NormalizeProvider(route.Provider) == token.ProviderAnyRouter) {
		return "/" + strings.TrimPrefix(path, "/v1/")
	}
	return path
}

func basePathHasVersionSuffix(path string) bool {
	path = strings.Trim(strings.TrimSpace(path), "/")
	if path == "" {
		return false
	}
	parts := strings.Split(path, "/")
	last := strings.ToLower(parts[len(parts)-1])
	if len(last) < 2 || last[0] != 'v' {
		return false
	}
	for _, char := range last[1:] {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
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

func tokenListContains(value string, token string) bool {
	for _, part := range strings.Split(value, ",") {
		if strings.EqualFold(strings.TrimSpace(part), token) {
			return true
		}
	}
	return false
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
