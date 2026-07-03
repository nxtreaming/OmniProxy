package proxy

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"omniproxy/internal/claudedesktop"
	"omniproxy/internal/config"
	"omniproxy/internal/logs"
	"omniproxy/internal/storage"
	"omniproxy/internal/token"
	"path/filepath"
	"strings"
	"testing"
)

func TestServiceAdaptsZoOpenAIChat(t *testing.T) {
	var askBody map[string]any
	var modelFetches int
	var askCalls int

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer zo_sk_test_token" {
			t.Fatalf("unexpected Authorization header: %q", r.Header.Get("Authorization"))
		}
		switch r.URL.Path {
		case "/models/available":
			modelFetches++
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"models":[{"model_name":"openai/gpt-5.5","label":"gpt-5.5","vendor":"openai"}]}`))
		case "/zo/ask":
			askCalls++
			body, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(body, &askBody); err != nil {
				t.Fatal(err)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"output":"hello from zo"}`))
		default:
			t.Fatalf("unexpected zo path: %s", r.URL.Path)
		}
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(token.UpsertRequest{Name: "zo", Provider: token.ProviderZo, TokenValue: "zo_sk_test_token"}); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(config.Config{
		ProxyPort:   3000,
		ControlPort: 3890,
		ZoBaseURL:   upstream.URL,
		GatewayRoutes: config.GatewayRoutes{
			Claude: config.GatewayRouteConfig{Provider: token.ProviderZo},
		},
		SwitchThreshold: 15,
		MaxRetries:      0,
	}, manager, logs.NewRecorder(10))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/zo/v1/chat/completions", stringsReader(`{"model":"gpt-5.5","messages":[{"role":"user","content":"hi"}]}`))
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	if modelFetches != 1 || askCalls != 1 {
		t.Fatalf("expected one model fetch and one ask call, got models=%d asks=%d", modelFetches, askCalls)
	}
	if askBody["model_name"] != "openai/gpt-5.5" {
		t.Fatalf("expected zo model mapping, got body=%#v", askBody)
	}
	input, _ := askBody["input"].(string)
	if !strings.Contains(input, "[user]: hi") {
		t.Fatalf("expected OpenAI messages to be folded into input, got %q", input)
	}

	var payload struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Choices) != 1 || payload.Choices[0].Message.Content != "hello from zo" {
		t.Fatalf("unexpected OpenAI response: %s", res.Body.String())
	}
}

func TestServiceRoutesZoModelsThroughOutboundProxy(t *testing.T) {
	upstreamHits := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits++
		_, _ = w.Write([]byte("direct"))
	}))
	defer upstream.Close()

	proxyHits := 0
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyHits++
		gotURL := ""
		if r.URL != nil {
			gotURL = r.URL.String()
		}
		if r.URL == nil || r.URL.Scheme != "http" || r.URL.Host != strings.TrimPrefix(upstream.URL, "http://") || r.URL.Path != "/models/available" {
			t.Fatalf("expected Zo models absolute upstream URL through proxy, got %q", gotURL)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer zo_sk_test_token" {
			t.Fatalf("unexpected Authorization header: %q", got)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"models":[{"model_name":"openai/gpt-5.5","label":"gpt-5.5","vendor":"openai"}]}`))
	}))
	defer proxyServer.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(token.UpsertRequest{Name: "zo", Provider: token.ProviderZo, TokenValue: "zo_sk_test_token"}); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(config.Config{
		ProxyPort:              3000,
		ControlPort:            3890,
		ZoBaseURL:              upstream.URL,
		OutboundProxyEnabled:   true,
		OutboundProxyURL:       proxyServer.URL,
		OutboundProxyProviders: []string{token.ProviderZo},
		SwitchThreshold:        15,
		MaxRetries:             0,
	}, manager, logs.NewRecorder(10))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/zo/v1/models", nil)
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	if proxyHits != 1 || upstreamHits != 0 {
		t.Fatalf("expected only proxy hit for Zo models, proxy=%d upstream=%d", proxyHits, upstreamHits)
	}
}

func TestMapZoModelMatchesVisibleLabels(t *testing.T) {
	models := []zoModel{
		{ModelName: "anthropic/claude-opus-4-7", Label: "Opus 4.7", Vendor: "anthropic"},
		{ModelName: "anthropic/claude-sonnet-4-6", Label: "Sonnet 4.6", Vendor: "anthropic"},
		{ModelName: "google/gemini-3.1-pro", Label: "Gemini 3.1 Pro", Vendor: "google"},
		{ModelName: "zai/glm-5", Label: "GLM 5", Vendor: "zai"},
		{ModelName: "minimax/minimax-2.7", Label: "MiniMax 2.7", Vendor: "minimax"},
		{ModelName: "openai/gpt-5.4", Label: "GPT-5.4", Vendor: "openai"},
		{ModelName: "openai/gpt-5.4-mini", Label: "GPT-5.4 mini", Vendor: "openai"},
		{ModelName: "openai/gpt-5.5", Label: "GPT-5.5", Vendor: "openai"},
		{ModelName: "deepseek/deepseek-v4-pro", Label: "DeepSeek V4 Pro", Vendor: "deepseek"},
	}
	cases := map[string]string{
		"claude-opus-4-7":   "anthropic/claude-opus-4-7",
		"claude-sonnet-4-6": "anthropic/claude-sonnet-4-6",
		"gemini-3.1-pro":    "google/gemini-3.1-pro",
		"glm-5":             "zai/glm-5",
		"minimax-2.7":       "minimax/minimax-2.7",
		"gpt-5.4":           "openai/gpt-5.4",
		"gpt-5.4-mini":      "openai/gpt-5.4-mini",
		"gpt-5.5":           "openai/gpt-5.5",
		"deepseek-v4-pro":   "deepseek/deepseek-v4-pro",
	}
	for clientModel, expected := range cases {
		if got := mapZoModel(clientModel, models); got != expected {
			t.Fatalf("mapZoModel(%q) = %q, want %q", clientModel, got, expected)
		}
	}
}

func TestServiceRoutesClaudeDesktopZoModelThroughZoGateway(t *testing.T) {
	localAppData := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("LOCALAPPDATA", localAppData)

	paths, err := claudedesktop.CurrentPaths()
	if err != nil {
		t.Fatal(err)
	}
	if err := claudedesktop.WriteRoutes(paths.RoutesPath, []claudedesktop.ModelRoute{
		{RouteID: "claude-sonnet-4-6", UpstreamModel: "claude-opus-4-7", LabelOverride: "Zo Claude Opus 4.7"},
	}); err != nil {
		t.Fatal(err)
	}

	var askBody map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer zo_sk_test_token" {
			t.Fatalf("unexpected Authorization header: %q", r.Header.Get("Authorization"))
		}
		switch r.URL.Path {
		case "/models/available":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"models":[{"model_name":"anthropic/claude-opus-4-7","label":"Opus 4.7","vendor":"anthropic"}]}`))
		case "/zo/ask":
			body, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(body, &askBody); err != nil {
				t.Fatal(err)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"output":"hello from desktop zo"}`))
		default:
			t.Fatalf("unexpected zo path: %s", r.URL.Path)
		}
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(token.UpsertRequest{Name: "zo", Provider: token.ProviderZo, TokenValue: "zo_sk_test_token"}); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(config.Config{
		ProxyPort:   3000,
		ControlPort: 3890,
		ZoBaseURL:   upstream.URL,
		GatewayRoutes: config.GatewayRoutes{
			Claude: config.GatewayRouteConfig{Provider: token.ProviderZo},
		},
		SwitchThreshold: 15,
		MaxRetries:      0,
	}, manager, logs.NewRecorder(10))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/claude-desktop/v1/messages", stringsReader(`{
		"model":"claude-sonnet-4-6",
		"messages":[{"role":"user","content":"hi"}]
	}`))
	req.Header.Set("Authorization", "Bearer "+claudedesktop.GatewayToken)
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	if askBody["model_name"] != "anthropic/claude-opus-4-7" {
		t.Fatalf("expected Claude Desktop route to use Zo Claude model, got body=%#v", askBody)
	}
}

func TestServiceAdaptsZoAnthropicToolUse(t *testing.T) {
	var askBody map[string]any

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer zo_sk_test_token" {
			t.Fatalf("unexpected Authorization header: %q", r.Header.Get("Authorization"))
		}
		switch r.URL.Path {
		case "/models/available":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"models":[{"model_name":"anthropic/claude-sonnet-4-5","label":"claude-sonnet-4-5","vendor":"anthropic"}]}`))
		case "/zo/ask":
			body, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(body, &askBody); err != nil {
				t.Fatal(err)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"output":{"text":"checking","tool_name":"Read","tool_args":"{\"file_path\":\"README.md\",\"reason\":\"noise\"}"}}`))
		default:
			t.Fatalf("unexpected zo path: %s", r.URL.Path)
		}
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(token.UpsertRequest{Name: "zo", Provider: token.ProviderZo, TokenValue: "zo_sk_test_token"}); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(config.Config{
		ProxyPort:       3000,
		ControlPort:     3890,
		ZoBaseURL:       upstream.URL,
		SwitchThreshold: 15,
		MaxRetries:      0,
	}, manager, logs.NewRecorder(10))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/zo/v1/messages", stringsReader(`{
		"model":"claude-sonnet-4-5",
		"messages":[{"role":"user","content":"read readme"}],
		"tools":[{"name":"Read","description":"Read a file","input_schema":{"type":"object","properties":{"file_path":{"type":"string"}}}}]
	}`))
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	if askBody["model_name"] != "anthropic/claude-sonnet-4-5" {
		t.Fatalf("expected zo anthropic model mapping, got body=%#v", askBody)
	}
	if askBody["output_format"] == nil {
		t.Fatalf("expected tool output_format to be sent, got body=%#v", askBody)
	}
	input, _ := askBody["input"].(string)
	if !strings.Contains(input, "Read") || !strings.Contains(input, "[user]: read readme") {
		t.Fatalf("expected tool instructions and user input, got %q", input)
	}

	var payload struct {
		Content []struct {
			Type  string         `json:"type"`
			Text  string         `json:"text"`
			Name  string         `json:"name"`
			Input map[string]any `json:"input"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.StopReason != "tool_use" || len(payload.Content) != 2 {
		t.Fatalf("unexpected Anthropic response: %s", res.Body.String())
	}
	if payload.Content[0].Type != "text" || payload.Content[0].Text != "checking" {
		t.Fatalf("unexpected text block: %#v", payload.Content[0])
	}
	if payload.Content[1].Type != "tool_use" || payload.Content[1].Name != "Read" || payload.Content[1].Input["file_path"] != "README.md" {
		t.Fatalf("unexpected tool block: %#v", payload.Content[1])
	}
	if _, ok := payload.Content[1].Input["reason"]; ok {
		t.Fatalf("tool noise argument should have been stripped: %#v", payload.Content[1].Input)
	}
}

func TestServiceAdaptsZoResponsesStream(t *testing.T) {
	var askBody map[string]any

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer zo_sk_test_token" {
			t.Fatalf("unexpected Authorization header: %q", r.Header.Get("Authorization"))
		}
		switch r.URL.Path {
		case "/models/available":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"models":[{"model_name":"openai/gpt-5.5","label":"gpt-5.5","vendor":"openai"}]}`))
		case "/zo/ask":
			body, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(body, &askBody); err != nil {
				t.Fatal(err)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"output":"hello codex"}`))
		default:
			t.Fatalf("unexpected zo path: %s", r.URL.Path)
		}
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(token.UpsertRequest{Name: "zo", Provider: token.ProviderZo, TokenValue: "zo_sk_test_token"}); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(config.Config{
		ProxyPort:       3000,
		ControlPort:     3890,
		ZoBaseURL:       upstream.URL,
		SwitchThreshold: 15,
		MaxRetries:      0,
	}, manager, logs.NewRecorder(10))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/zo/v1/responses", stringsReader(`{
		"model":"gpt-5.5",
		"instructions":[{"type":"input_text","text":"be concise"}],
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}],
		"stream":"true",
		"tools":{"tools":[{"type":"function","name":"Shell","description":"Run a shell command","parameters":{"type":"object","properties":{"cmd":{"type":"string"}}}}]}
	}`))
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	if askBody["model_name"] != "openai/gpt-5.5" {
		t.Fatalf("expected zo model mapping, got body=%#v", askBody)
	}
	input, _ := askBody["input"].(string)
	if !strings.Contains(input, "[instructions]: be concise") || !strings.Contains(input, "[user]: hi") {
		t.Fatalf("expected responses input to be folded into zo input, got %q", input)
	}
	if askBody["output_format"] == nil {
		t.Fatalf("expected responses tools object to be adapted, got body=%#v", askBody)
	}
	body := res.Body.String()
	if !strings.Contains(res.Header().Get("Content-Type"), "text/event-stream") ||
		!strings.Contains(body, "response.output_text.delta") ||
		!strings.Contains(body, "hello codex") ||
		!strings.Contains(body, "response.completed") {
		t.Fatalf("unexpected responses stream body headers=%v body=%s", res.Header(), body)
	}
}

func TestServiceRoutesOpenCodeRouterToZhipu(t *testing.T) {
	var upstreamPath string
	var authorization string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		authorization = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"usage":{"total_tokens":4}}`))
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(token.UpsertRequest{Name: "zhipu", Provider: token.ProviderZhipu, TokenValue: "zhipu-api-key-token"}); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(config.Config{
		ProxyPort:    3000,
		ControlPort:  3890,
		ZhipuBaseURL: upstream.URL + "/api/paas/v4",
		GatewayRoutes: config.GatewayRoutes{
			OpenAI: config.GatewayRouteConfig{Provider: token.ProviderZhipu},
		},
		SwitchThreshold: 15,
		MaxRetries:      0,
	}, manager, logs.NewRecorder(10))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/opencode-router/v1/chat/completions", stringsReader(`{"model":"glm-5.1","messages":[]}`))
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	if upstreamPath != "/api/paas/v4/chat/completions" || authorization != "Bearer zhipu-api-key-token" {
		t.Fatalf("unexpected zhipu route path=%q authorization=%q", upstreamPath, authorization)
	}
}
