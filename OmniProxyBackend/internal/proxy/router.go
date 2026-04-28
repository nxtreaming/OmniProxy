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
	protocol := protocolForRoute(provider, &path)
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
	out.Path = singleJoiningSlash(base.Path, upstreamPath(route.Path, selected))
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
	case token.ProviderXiaomi:
		if selected.CredentialType == token.CredentialTypeMimoTokenPlan {
			if route.Protocol == "anthropic" {
				return r.cfg.XiaomiTokenPlanAnthropicBaseURL
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
	case token.ProviderDeepSeek, token.ProviderKimi, token.ProviderXiaomi:
		if *path == "/anthropic" {
			*path = "/"
			protocol = "anthropic"
		} else if strings.HasPrefix(*path, "/anthropic/") {
			*path = "/" + strings.TrimPrefix(*path, "/anthropic/")
			protocol = "anthropic"
		}
	}
	return protocol
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
	return token.ProviderAnthropic
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
