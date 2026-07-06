package proxy

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"omniproxy/internal/config"
	"omniproxy/internal/token"
)

type RouteDiagnosticRequest struct {
	Client string `json:"client,omitempty"`
	Method string `json:"method,omitempty"`
	Path   string `json:"path,omitempty"`
	Model  string `json:"model,omitempty"`
}

type RouteDiagnostic struct {
	OK            bool                       `json:"ok"`
	Message       string                     `json:"message,omitempty"`
	Method        string                     `json:"method"`
	Path          string                     `json:"path"`
	RequestModel  string                     `json:"requestModel,omitempty"`
	RoutedModel   string                     `json:"routedModel,omitempty"`
	ClientKey     string                     `json:"clientKey,omitempty"`
	ClientName    string                     `json:"clientName,omitempty"`
	Protocol      string                     `json:"protocol,omitempty"`
	SelectedIndex int                        `json:"selectedIndex"`
	Chain         []RouteDiagnosticCandidate `json:"chain"`
}

type RouteDiagnosticCandidate struct {
	Index               int    `json:"index"`
	Role                string `json:"role"`
	Available           bool   `json:"available"`
	Issue               string `json:"issue,omitempty"`
	Provider            string `json:"provider"`
	CredentialType      string `json:"credentialType,omitempty"`
	Protocol            string `json:"protocol,omitempty"`
	Model               string `json:"model,omitempty"`
	Path                string `json:"path,omitempty"`
	BaseURL             string `json:"baseUrl,omitempty"`
	TargetURL           string `json:"targetUrl,omitempty"`
	TokenID             string `json:"tokenId,omitempty"`
	TokenName           string `json:"tokenName,omitempty"`
	TokenStatus         string `json:"tokenStatus,omitempty"`
	TokenCredentialType string `json:"tokenCredentialType,omitempty"`
	TokenRemaining      int    `json:"tokenRemaining,omitempty"`
	TokenSelected       bool   `json:"tokenSelected,omitempty"`
}

func DiagnoseRoute(cfg config.Config, manager *token.Manager, req RouteDiagnosticRequest) RouteDiagnostic {
	cfg = config.Normalize(cfg)
	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method == "" {
		method = http.MethodPost
	}
	path := strings.TrimSpace(req.Path)
	if path == "" {
		path = diagnosticPathForClient(req.Client, req.Model, cfg)
	}
	body := diagnosticBodyForPath(path, req.Model)

	incoming := diagnosticURL(path)
	httpReq, _ := http.NewRequest(method, incoming.String(), nil)
	route := routeWithClient(httpReq, NewRouter(cfg).Route(incoming, body))
	candidates := routeCandidates(route)

	out := RouteDiagnostic{
		Method:        method,
		Path:          incoming.RequestURI(),
		RequestModel:  strings.TrimSpace(req.Model),
		RoutedModel:   route.Model,
		ClientKey:     route.ClientKey,
		ClientName:    route.ClientName,
		Protocol:      route.Protocol,
		SelectedIndex: -1,
	}
	for index, candidate := range candidates {
		item := diagnosticCandidate(cfg, manager, candidate, index)
		out.Chain = append(out.Chain, item)
		if out.SelectedIndex < 0 && item.Available {
			out.SelectedIndex = index
			out.OK = true
			out.Message = "路由可用"
		}
	}
	if !out.OK {
		out.Message = "没有可用的主后端或备用后端"
	}
	return out
}

func diagnosticPathForClient(client string, model string, cfg config.Config) string {
	switch strings.ToLower(strings.TrimSpace(client)) {
	case clientClaude:
		return "/anthropic-router/v1/messages"
	case clientClaudeDesktop:
		return "/claude-desktop/v1/messages"
	case clientGemini:
		model = strings.TrimSpace(model)
		if model == "" {
			model = cfg.GatewayRoutes.Gemini.Model
		}
		if model == "" {
			model = "gemini-pro"
		}
		return "/gemini/v1beta/models/" + url.PathEscape(model) + ":generateContent"
	case clientOpenCode, clientDeepSeekTUI, config.GatewayRouteOpenAI:
		return "/opencode-router/v1/chat/completions"
	case clientPi:
		return "/pi-router/v1/chat/completions"
	case clientCodex:
		return "/codex/v1/chat/completions"
	default:
		return "/codex/v1/chat/completions"
	}
}

func diagnosticBodyForPath(path string, model string) []byte {
	if strings.TrimSpace(model) == "" || strings.Contains(path, "/models/") {
		return []byte(`{}`)
	}
	body, err := json.Marshal(map[string]any{
		"model":    strings.TrimSpace(model),
		"messages": []any{},
	})
	if err != nil {
		return []byte(`{}`)
	}
	return body
}

func diagnosticURL(path string) *url.URL {
	path = strings.TrimSpace(path)
	if path == "" {
		path = "/"
	}
	parsed, err := url.Parse(path)
	if err == nil && parsed.IsAbs() {
		return parsed
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	parsed, err = url.Parse("http://omniproxy.local" + path)
	if err != nil {
		return &url.URL{Scheme: "http", Host: "omniproxy.local", Path: "/"}
	}
	return parsed
}

func diagnosticCandidate(cfg config.Config, manager *token.Manager, route routeInfo, index int) RouteDiagnosticCandidate {
	selected, ok, issue := diagnosticSelectToken(cfg, manager, route)
	baseURL := NewRouter(cfg).BaseURL(route, selected)
	targetURL := ""
	if target, err := NewRouter(cfg).TargetURL(route, selected); err == nil {
		targetURL = target
	} else if issue == "" {
		issue = err.Error()
	}
	out := RouteDiagnosticCandidate{
		Index:          index,
		Role:           "备用",
		Available:      ok && targetURL != "",
		Issue:          issue,
		Provider:       route.Provider,
		CredentialType: route.CredentialType,
		Protocol:       route.Protocol,
		Model:          route.Model,
		Path:           route.Path,
		BaseURL:        baseURL,
		TargetURL:      targetURL,
	}
	if index == 0 {
		out.Role = "主后端"
	}
	if ok {
		out.TokenID = selected.ID
		out.TokenName = token.DisplayName(selected)
		out.TokenStatus = string(selected.Status)
		out.TokenCredentialType = selected.CredentialType
		out.TokenRemaining = selected.Remaining
		out.TokenSelected = selected.Selected
	}
	if out.Available {
		out.Issue = ""
	}
	return out
}

func diagnosticSelectToken(cfg config.Config, manager *token.Manager, route routeInfo) (token.Token, bool, string) {
	if manager == nil {
		return token.Token{}, false, "账号管理器未就绪"
	}
	provider := token.NormalizeProvider(route.Provider)
	credentialType := strings.TrimSpace(route.CredentialType)
	items := manager.List()
	if provider == token.ProviderOpenAI && credentialType == "" && route.ClientKey == clientCodex {
		preferCodexAuth := func(item token.Token) bool {
			return item.CredentialType == token.CredentialTypeCodexAuthJSON
		}
		return diagnosticPickToken(items, cfg.SchedulingMode, provider, "", preferCodexAuth)
	}
	if provider == token.ProviderXiaomi && credentialType == "" {
		preferred := preferredMimoCredentialType(cfg)
		if cfg.SchedulingMode == config.SchedulingModeBalanced {
			if selected, ok, _ := diagnosticPickToken(items, cfg.SchedulingMode, provider, preferred, nil); ok {
				return selected, true, ""
			}
			return diagnosticPickToken(items, cfg.SchedulingMode, provider, "", nil)
		}
		return diagnosticPickToken(items, cfg.SchedulingMode, provider, "", func(item token.Token) bool {
			return item.CredentialType == preferred
		})
	}
	return diagnosticPickToken(items, cfg.SchedulingMode, provider, credentialType, nil)
}

func diagnosticPickToken(items []token.Token, schedulingMode string, provider string, credentialType string, preferred func(token.Token) bool) (token.Token, bool, string) {
	provider = token.NormalizeProvider(provider)
	credentialType = strings.TrimSpace(strings.ToLower(credentialType))
	if credentialType != "" {
		if _, normalized, err := token.NormalizeProviderAndCredential(provider, credentialType); err == nil {
			credentialType = normalized
		}
	}

	selectedIDs := map[string]bool{}
	for _, item := range items {
		if item.Selected && token.NormalizeProvider(item.Provider) == provider {
			selectedIDs[item.ID] = true
		}
	}
	hasSelection := len(selectedIDs) > 0
	statuses := []token.Status{token.StatusActive, token.StatusLow}
	for _, status := range statuses {
		if preferred != nil {
			if selected, ok := diagnosticFirstMatchingToken(items, schedulingMode, provider, credentialType, status, selectedIDs, hasSelection, preferred); ok {
				return selected, true, ""
			}
			if hasSelection {
				if selected, ok := diagnosticFirstMatchingToken(items, schedulingMode, provider, credentialType, status, nil, false, preferred); ok {
					return selected, true, ""
				}
			}
		}
		if selected, ok := diagnosticFirstMatchingToken(items, schedulingMode, provider, credentialType, status, selectedIDs, hasSelection, nil); ok {
			return selected, true, ""
		}
		if hasSelection {
			if selected, ok := diagnosticFirstMatchingToken(items, schedulingMode, provider, credentialType, status, nil, false, nil); ok {
				return selected, true, ""
			}
		}
	}
	return token.Token{}, false, diagnosticNoTokenIssue(items, provider, credentialType)
}

func diagnosticFirstMatchingToken(items []token.Token, schedulingMode string, provider string, credentialType string, status token.Status, selectedIDs map[string]bool, hasSelection bool, preferred func(token.Token) bool) (token.Token, bool) {
	matches := make([]diagnosticTokenMatch, 0, len(items))
	for index, item := range items {
		if !diagnosticUsableTokenMatches(item, provider, credentialType, status, selectedIDs, hasSelection) {
			continue
		}
		if preferred != nil && !preferred(item) {
			continue
		}
		matches = append(matches, diagnosticTokenMatch{Token: item, Index: index})
	}
	if len(matches) == 0 {
		return token.Token{}, false
	}
	if schedulingMode == config.SchedulingModeBalanced {
		sort.SliceStable(matches, func(i, j int) bool {
			return diagnosticBalancedLess(matches[i], matches[j])
		})
	}
	return matches[0].Token, true
}

type diagnosticTokenMatch struct {
	Token token.Token
	Index int
}

func diagnosticUsableTokenMatches(item token.Token, provider string, credentialType string, status token.Status, selectedIDs map[string]bool, hasSelection bool) bool {
	if hasSelection && !selectedIDs[item.ID] {
		return false
	}
	if item.Disabled {
		return false
	}
	if token.NormalizeProvider(item.Provider) != provider {
		return false
	}
	if credentialType != "" && item.CredentialType != credentialType {
		return false
	}
	if item.Status != status {
		return false
	}
	return strings.TrimSpace(item.TokenValue) != ""
}

func diagnosticBalancedLess(left diagnosticTokenMatch, right diagnosticTokenMatch) bool {
	if left.Token.Remaining != right.Token.Remaining {
		return left.Token.Remaining > right.Token.Remaining
	}
	if left.Token.LastUsedAt == nil && right.Token.LastUsedAt != nil {
		return true
	}
	if left.Token.LastUsedAt != nil && right.Token.LastUsedAt == nil {
		return false
	}
	if left.Token.LastUsedAt != nil && right.Token.LastUsedAt != nil && !left.Token.LastUsedAt.Equal(*right.Token.LastUsedAt) {
		return left.Token.LastUsedAt.Before(*right.Token.LastUsedAt)
	}
	if left.Token.Stats.RequestCount != right.Token.Stats.RequestCount {
		return left.Token.Stats.RequestCount < right.Token.Stats.RequestCount
	}
	if !left.Token.CreatedAt.Equal(right.Token.CreatedAt) {
		return left.Token.CreatedAt.Before(right.Token.CreatedAt)
	}
	return left.Index > right.Index
}

func diagnosticNoTokenIssue(items []token.Token, provider string, credentialType string) string {
	total := 0
	disabled := 0
	withoutValue := 0
	statusBlocked := 0
	credentialBlocked := 0
	for _, item := range items {
		if token.NormalizeProvider(item.Provider) != provider {
			continue
		}
		total++
		if item.Disabled {
			disabled++
			continue
		}
		if strings.TrimSpace(item.TokenValue) == "" {
			withoutValue++
			continue
		}
		if credentialType != "" && item.CredentialType != credentialType {
			credentialBlocked++
			continue
		}
		if item.Status != token.StatusActive && item.Status != token.StatusLow {
			statusBlocked++
		}
	}
	switch {
	case total == 0:
		return "没有该厂商账号"
	case disabled == total:
		return "该厂商账号都已停用"
	case withoutValue == total:
		return "该厂商账号缺少凭据"
	case credentialType != "" && credentialBlocked > 0:
		return "没有匹配凭据类型的可用账号"
	case statusBlocked > 0:
		return "账号当前不可调度"
	default:
		return "没有可用账号"
	}
}

func ValidateRouteDiagnosticRequest(req RouteDiagnosticRequest) error {
	if strings.TrimSpace(req.Path) == "" {
		return nil
	}
	parsed := diagnosticURL(req.Path)
	if parsed.Path == "" {
		return errors.New("diagnostic path is invalid")
	}
	return nil
}
