package proxy

import (
	"fmt"
	"net/url"
	"strings"

	"omniproxy/internal/token"
)

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
	provider := token.NormalizeProvider(route.Provider)
	if basePathHasVersionSuffix(basePath) && strings.HasPrefix(path, "/v1/") && (route.Protocol == "openai" || provider == token.ProviderAnyRouter || provider == token.ProviderForge) {
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
