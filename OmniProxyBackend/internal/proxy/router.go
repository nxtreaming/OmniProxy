package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

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
}

func NewRouter(cfg config.Config) Router {
	return Router{cfg: config.Normalize(cfg)}
}

func (r Router) Route(incoming *url.URL, body []byte) routeInfo {
	provider := token.ProviderOpenAI
	credentialType := ""
	path := incoming.Path
	model := requestModel(incoming, body)
	if isAnthropicRouterPath(path) {
		return routeInfo{
			Provider: providerForModel(model),
			Protocol: "anthropic",
			Model:    model,
			Path:     stripPathPrefix(path, "/anthropic-router"),
			RawQuery: incoming.RawQuery,
		}
	}
	if isOpenCodeRouterPath(path) {
		return routeInfo{
			Provider: providerForOpenCodeModel(model),
			Protocol: "openai",
			Model:    model,
			Path:     stripPathPrefix(path, "/opencode-router"),
			RawQuery: incoming.RawQuery,
		}
	}
	if isPiRouterPath(path) {
		provider, credentialType := providerForPiModel(model)
		return routeInfo{
			Provider:       provider,
			CredentialType: credentialType,
			Protocol:       "openai",
			Model:          model,
			Path:           stripPathPrefix(path, "/pi-router"),
			RawQuery:       incoming.RawQuery,
		}
	}

	trimmed := strings.TrimPrefix(path, "/")
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) > 0 {
		candidate := strings.ToLower(parts[0])
		switch candidate {
		case token.ProviderOpenAI, token.ProviderAnthropic, token.ProviderDeepSeek, token.ProviderKimi, token.ProviderXiaomi, token.ProviderZhipu, token.ProviderMiniMax, token.ProviderGemini, token.ProviderOpenRouter, token.ProviderTokenRouter, token.ProviderSub2API, token.ProviderCustom:
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
	protocol := protocolForRoute(provider, &path)
	if provider == token.ProviderSub2API && protocol == "openai" {
		path = versionedSub2APIOpenAIPath(path)
	}
	return routeInfo{Provider: provider, CredentialType: credentialType, Protocol: protocol, Model: model, Path: path, RawQuery: incoming.RawQuery}
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
	if isCodexCredential(selected) {
		return r.cfg.CodexBaseURL
	}

	switch token.NormalizeProvider(route.Provider) {
	case token.ProviderAnthropic:
		return r.cfg.AnthropicBaseURL
	case token.ProviderDeepSeek:
		if route.Protocol == "anthropic" {
			return r.cfg.DeepSeekAnthropicBaseURL
		}
		return r.cfg.DeepSeekBaseURL
	case token.ProviderKimi:
		return r.cfg.KimiBaseURL
	case token.ProviderZhipu:
		if route.Protocol == "anthropic" {
			return r.cfg.ZhipuAnthropicBaseURL
		}
		return r.cfg.ZhipuBaseURL
	case token.ProviderMiniMax:
		if route.Protocol == "anthropic" {
			return r.cfg.MiniMaxAnthropicBaseURL
		}
		return r.cfg.MiniMaxBaseURL
	case token.ProviderGemini:
		return r.cfg.GeminiBaseURL
	case token.ProviderOpenRouter:
		return r.cfg.OpenRouterBaseURL
	case token.ProviderTokenRouter:
		return r.cfg.TokenRouterBaseURL
	case token.ProviderSub2API:
		if strings.TrimSpace(selected.BaseURL) != "" {
			return selected.BaseURL
		}
		return r.cfg.Sub2APIBaseURL
	case token.ProviderCustom:
		if route.Protocol == "anthropic" && r.cfg.CustomGatewayAnthropicBaseURL != "" {
			return r.cfg.CustomGatewayAnthropicBaseURL
		}
		return r.cfg.CustomGatewayBaseURL
	case token.ProviderXiaomi:
		if selected.CredentialType == token.CredentialTypeMimoTokenPlan {
			if route.Protocol == "anthropic" {
				if selected.Region == token.MimoRegionSGP {
					return r.cfg.XiaomiTokenPlanSGPAnthropicBaseURL
				}
				return r.cfg.XiaomiTokenPlanAnthropicBaseURL
			}
			if selected.Region == token.MimoRegionSGP {
				return r.cfg.XiaomiTokenPlanSGPBaseURL
			}
			return r.cfg.XiaomiTokenPlanBaseURL
		}
		if route.Protocol == "anthropic" {
			return r.cfg.XiaomiAPIAnthropicBaseURL
		}
		return r.cfg.XiaomiAPIBaseURL
	default:
		if r.cfg.OpenAIBaseURL != "" {
			return r.cfg.OpenAIBaseURL
		}
		return r.cfg.UpstreamBaseURL
	}
}

func protocolForRoute(provider string, path *string) string {
	protocol := "openai"
	if provider == token.ProviderAnthropic {
		protocol = "anthropic"
	}
	switch provider {
	case token.ProviderGemini:
		protocol = "gemini"
	case token.ProviderSub2API:
		if stripProtocolPrefix(path, "/anthropic") {
			protocol = "anthropic"
		} else if stripProtocolPrefix(path, "/gemini") {
			protocol = "gemini"
		}
	case token.ProviderDeepSeek, token.ProviderKimi, token.ProviderXiaomi, token.ProviderZhipu, token.ProviderMiniMax, token.ProviderCustom:
		if stripProtocolPrefix(path, "/anthropic") {
			protocol = "anthropic"
		}
	}
	return protocol
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

func versionedSub2APIOpenAIPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || path == "/" || path == "/v1" || strings.HasPrefix(path, "/v1/") {
		return path
	}
	return singleJoiningSlash("/v1", path)
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
	return strings.TrimSpace(incoming.Query().Get("model"))
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
	next = stripPathPrefix(next, "/v1")
	if next == "" {
		return "/"
	}
	return next
}

func upstreamPathForBase(basePath string, route routeInfo, selected token.Token) string {
	path := upstreamPath(route.Path, selected)
	if route.Protocol == "openai" && basePathHasVersionSuffix(basePath) && strings.HasPrefix(path, "/v1/") {
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
