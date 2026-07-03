package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"omniproxy/internal/config"
	"omniproxy/internal/logs"
	"omniproxy/internal/storage"
	"omniproxy/internal/token"
	"path/filepath"
	"strings"
	"testing"
)

func TestServiceRoutesAnthropicRouterByConfiguredGateway(t *testing.T) {
	var gotPath string
	var gotKey string
	var gotBody string

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		gotPath = r.URL.Path
		gotKey = r.Header.Get("Api-Key")
		gotBody = string(body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`))
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "router-tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(token.UpsertRequest{
		Name:       "mimo",
		Provider:   token.ProviderXiaomi,
		TokenValue: "sk-mimo-router-key",
	}); err != nil {
		t.Fatal(err)
	}

	service, err := NewService(config.Config{
		ProxyPort:                 3000,
		ControlPort:               3890,
		XiaomiAPIAnthropicBaseURL: upstream.URL + "/anthropic",
		GatewayRoutes: config.GatewayRoutes{
			Claude: config.GatewayRouteConfig{Provider: token.ProviderXiaomi},
		},
		SwitchThreshold: 15,
		MaxRetries:      0,
	}, manager, logs.NewRecorder(10))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/anthropic-router/v1/messages", stringsReader(`{"model":"mimo-v2.5","messages":[]}`))
	req.Header.Set("Authorization", "Bearer caller")
	req.Header.Set("X-Api-Key", "caller")
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	if gotPath != "/anthropic/v1/messages" || gotKey != "sk-mimo-router-key" {
		t.Fatalf("unexpected configured Claude route path=%q key=%q", gotPath, gotKey)
	}
	if !strings.Contains(gotBody, `"model":"mimo-v2.5"`) {
		t.Fatalf("expected request model to be preserved, got %q", gotBody)
	}
	entries := service.logs.List()
	if len(entries) == 0 || entries[0].Model != "mimo-v2.5" {
		t.Fatalf("expected logs to include routed model, got %#v", entries)
	}
}

func TestServiceHandlesAnthropicRouterHealthProbe(t *testing.T) {
	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "router-probe-tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(config.Config{
		ProxyPort:        3000,
		ControlPort:      3890,
		AnthropicBaseURL: "https://api.anthropic.com",
		SwitchThreshold:  15,
		MaxRetries:       0,
	}, manager, logs.NewRecorder(10))
	if err != nil {
		t.Fatal(err)
	}

	headRes := httptest.NewRecorder()
	service.ServeHTTP(headRes, httptest.NewRequest(http.MethodHead, "/anthropic-router", nil))
	if headRes.Code != http.StatusOK {
		t.Fatalf("expected HEAD probe status 200, got %d body=%s", headRes.Code, headRes.Body.String())
	}
	if headRes.Body.Len() != 0 {
		t.Fatalf("expected HEAD probe to have empty body, got %q", headRes.Body.String())
	}

	getRes := httptest.NewRecorder()
	service.ServeHTTP(getRes, httptest.NewRequest(http.MethodGet, "/anthropic-router", nil))
	if getRes.Code != http.StatusOK {
		t.Fatalf("expected GET probe status 200, got %d body=%s", getRes.Code, getRes.Body.String())
	}
	if !strings.Contains(getRes.Body.String(), `"ok":true`) {
		t.Fatalf("expected GET probe health body, got %q", getRes.Body.String())
	}
}

func TestServiceRoutesGeminiNativeRequests(t *testing.T) {
	var upstreamPath string
	var geminiKey string
	var authorization string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		geminiKey = r.Header.Get("x-goog-api-key")
		authorization = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"usageMetadata":{"totalTokenCount":3}}`))
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(token.UpsertRequest{Name: "gemini", Provider: token.ProviderGemini, TokenValue: "gemini-api-key-token"}); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(config.Config{
		ProxyPort:       3000,
		ControlPort:     3890,
		GeminiBaseURL:   upstream.URL,
		SwitchThreshold: 15,
		MaxRetries:      0,
	}, manager, logs.NewRecorder(10))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/gemini/v1beta/models/gemini-3-pro-preview:generateContent", stringsReader(`{"contents":[]}`))
	req.Header.Set("Authorization", "Bearer caller")
	req.Header.Set("X-Goog-Api-Key", "caller")
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	if upstreamPath != "/v1beta/models/gemini-3-pro-preview:generateContent" || geminiKey != "gemini-api-key-token" || authorization != "" {
		t.Fatalf("unexpected gemini route path=%q key=%q authorization=%q", upstreamPath, geminiKey, authorization)
	}
}

func TestServiceRoutesOpenRouterRequests(t *testing.T) {
	var upstreamPath string
	var authorization string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		authorization = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"usage":{"total_tokens":5}}`))
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(token.UpsertRequest{Name: "openrouter", Provider: token.ProviderOpenRouter, TokenValue: "sk-or-test-token"}); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(config.Config{
		ProxyPort:         3000,
		ControlPort:       3890,
		OpenRouterBaseURL: upstream.URL + "/api/v1",
		SwitchThreshold:   15,
		MaxRetries:        0,
	}, manager, logs.NewRecorder(10))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/openrouter/v1/chat/completions", stringsReader(`{"model":"openai/gpt-test","messages":[]}`))
	req.Header.Set("Authorization", "Bearer caller")
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	if upstreamPath != "/api/v1/chat/completions" || authorization != "Bearer sk-or-test-token" {
		t.Fatalf("unexpected openrouter route path=%q authorization=%q", upstreamPath, authorization)
	}
}

func TestServiceRoutesTokenRouterRequests(t *testing.T) {
	var upstreamPath string
	var authorization string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		authorization = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"usage":{"total_tokens":7}}`))
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(token.UpsertRequest{Name: "tokenrouter", Provider: token.ProviderTokenRouter, TokenValue: "tr_test_token"}); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(config.Config{
		ProxyPort:          3000,
		ControlPort:        3890,
		TokenRouterBaseURL: upstream.URL,
		SwitchThreshold:    15,
		MaxRetries:         0,
	}, manager, logs.NewRecorder(10))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/tokenrouter/v1/chat/completions", stringsReader(`{"model":"auto:balance","messages":[]}`))
	req.Header.Set("Authorization", "Bearer caller")
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	if upstreamPath != "/v1/chat/completions" || authorization != "Bearer tr_test_token" {
		t.Fatalf("unexpected tokenrouter route path=%q authorization=%q", upstreamPath, authorization)
	}
}

func TestServiceRoutesSub2APIRequests(t *testing.T) {
	var upstreamPath string
	var authorization string
	var upstreamBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		authorization = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		upstreamBody = string(body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"usage":{"total_tokens":9}}`))
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(token.UpsertRequest{Name: "sub2api", Provider: token.ProviderSub2API, BaseURL: upstream.URL, TokenValue: "sub2api-api-key-token"}); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(config.Config{
		ProxyPort:       3000,
		ControlPort:     3890,
		SwitchThreshold: 15,
		MaxRetries:      0,
	}, manager, logs.NewRecorder(10))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/sub2api/responses", stringsReader(`{"model":"gpt-5.5","input":"hi","tools":[{"type":"image_generation"},{"type":"web_search_preview","config":{"tools":[{"type":"image_generation"}]}}],"tool_choice":{"type":"image_generation"},"include":["reasoning.encrypted_content","image_generation_call.partial_images"],"metadata":{"tool_choice":{"tool":{"type":"image_generation"}}}}`))
	req.Header.Set("Authorization", "Bearer caller")
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	if upstreamPath != "/v1/responses" || authorization != "Bearer sub2api-api-key-token" {
		t.Fatalf("unexpected sub2api route path=%q authorization=%q", upstreamPath, authorization)
	}
	if strings.Contains(upstreamBody, "image_generation") {
		t.Fatalf("sub2api codex text request should strip image_generation tool, got body=%s", upstreamBody)
	}
	if !strings.Contains(upstreamBody, "web_search_preview") {
		t.Fatalf("sub2api request should preserve non-image tools, got body=%s", upstreamBody)
	}
}

func TestServiceRoutesSub2APIAnthropicRequests(t *testing.T) {
	var upstreamPath string
	var apiKey string
	var authorization string
	var anthropicVersion string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		apiKey = r.Header.Get("x-api-key")
		authorization = r.Header.Get("Authorization")
		anthropicVersion = r.Header.Get("anthropic-version")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"usage":{"input_tokens":2,"output_tokens":3}}`))
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(token.UpsertRequest{Name: "sub2api", Provider: token.ProviderSub2API, BaseURL: upstream.URL, TokenValue: "sub2api-api-key-token"}); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(config.Config{
		ProxyPort:       3000,
		ControlPort:     3890,
		SwitchThreshold: 15,
		MaxRetries:      0,
	}, manager, logs.NewRecorder(10))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/sub2api/anthropic/v1/messages", stringsReader(`{"model":"claude-sonnet-4-5","messages":[]}`))
	req.Header.Set("Authorization", "Bearer caller")
	req.Header.Set("X-Api-Key", "caller")
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	if upstreamPath != "/v1/messages" || apiKey != "sub2api-api-key-token" || authorization != "" || anthropicVersion != "2023-06-01" {
		t.Fatalf("unexpected sub2api anthropic route path=%q key=%q authorization=%q version=%q", upstreamPath, apiKey, authorization, anthropicVersion)
	}
}

func TestServiceRoutesSub2APIGeminiRequests(t *testing.T) {
	var upstreamPath string
	var geminiKey string
	var authorization string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		geminiKey = r.Header.Get("x-goog-api-key")
		authorization = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"usageMetadata":{"totalTokenCount":5}}`))
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(token.UpsertRequest{Name: "sub2api", Provider: token.ProviderSub2API, BaseURL: upstream.URL, TokenValue: "sub2api-api-key-token"}); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(config.Config{
		ProxyPort:       3000,
		ControlPort:     3890,
		SwitchThreshold: 15,
		MaxRetries:      0,
	}, manager, logs.NewRecorder(10))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/sub2api/gemini/v1beta/models/gemini-3-pro-preview:generateContent", stringsReader(`{"contents":[]}`))
	req.Header.Set("Authorization", "Bearer caller")
	req.Header.Set("X-Goog-Api-Key", "caller")
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	if upstreamPath != "/v1beta/models/gemini-3-pro-preview:generateContent" || geminiKey != "sub2api-api-key-token" || authorization != "" {
		t.Fatalf("unexpected sub2api gemini route path=%q key=%q authorization=%q", upstreamPath, geminiKey, authorization)
	}
}

func TestServiceRoutesNewAPIRequests(t *testing.T) {
	var upstreamPath string
	var authorization string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		authorization = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"usage":{"total_tokens":9}}`))
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(token.UpsertRequest{Name: "newapi", Provider: token.ProviderNewAPI, BaseURL: upstream.URL, TokenValue: "newapi-api-key-token"}); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(config.Config{
		ProxyPort:       3000,
		ControlPort:     3890,
		SwitchThreshold: 15,
		MaxRetries:      0,
	}, manager, logs.NewRecorder(10))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/newapi/responses", stringsReader(`{"model":"gpt-5.5","input":"hi"}`))
	req.Header.Set("Authorization", "Bearer caller")
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	if upstreamPath != "/v1/responses" || authorization != "Bearer newapi-api-key-token" {
		t.Fatalf("unexpected new-api route path=%q authorization=%q", upstreamPath, authorization)
	}
}

func TestServiceRoutesNewAPIAnthropicRequests(t *testing.T) {
	var upstreamPath string
	var apiKey string
	var authorization string
	var anthropicVersion string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		apiKey = r.Header.Get("x-api-key")
		authorization = r.Header.Get("Authorization")
		anthropicVersion = r.Header.Get("anthropic-version")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"usage":{"input_tokens":2,"output_tokens":3}}`))
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(token.UpsertRequest{Name: "newapi", Provider: token.ProviderNewAPI, BaseURL: upstream.URL, TokenValue: "newapi-api-key-token"}); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(config.Config{
		ProxyPort:       3000,
		ControlPort:     3890,
		SwitchThreshold: 15,
		MaxRetries:      0,
	}, manager, logs.NewRecorder(10))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/newapi/anthropic/v1/messages", stringsReader(`{"model":"claude-sonnet-4-5","messages":[]}`))
	req.Header.Set("Authorization", "Bearer caller")
	req.Header.Set("X-Api-Key", "caller")
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	if upstreamPath != "/v1/messages" || apiKey != "newapi-api-key-token" || authorization != "" || anthropicVersion != "2023-06-01" {
		t.Fatalf("unexpected new-api anthropic route path=%q key=%q authorization=%q version=%q", upstreamPath, apiKey, authorization, anthropicVersion)
	}
}

func TestServiceRoutesAnyRouterRequests(t *testing.T) {
	var upstreamPath string
	var authorization string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		authorization = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"usage":{"total_tokens":9}}`))
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(token.UpsertRequest{Name: "anyrouter", Provider: token.ProviderAnyRouter, BaseURL: upstream.URL, TokenValue: "anyrouter-api-key-token"}); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(config.Config{
		ProxyPort:       3000,
		ControlPort:     3890,
		SwitchThreshold: 15,
		MaxRetries:      0,
	}, manager, logs.NewRecorder(10))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/anyrouter/responses", stringsReader(`{"model":"gpt-5-codex","input":"hi"}`))
	req.Header.Set("Authorization", "Bearer caller")
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	if upstreamPath != "/v1/responses" || authorization != "Bearer anyrouter-api-key-token" {
		t.Fatalf("unexpected anyrouter route path=%q authorization=%q", upstreamPath, authorization)
	}
}

func TestServiceRoutesPremRequestsAndRetriesAcrossKeys(t *testing.T) {
	var upstreamPaths []string
	var authorizations []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPaths = append(upstreamPaths, r.URL.Path)
		authorizations = append(authorizations, r.Header.Get("Authorization"))
		if len(authorizations) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"message":"rate limited"}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"usage":{"total_tokens":11}}`))
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	backup, err := manager.Add(token.UpsertRequest{Name: "prem-backup", Provider: token.ProviderPrem, TokenValue: "prem-backup-token"})
	if err != nil {
		t.Fatal(err)
	}
	primary, err := manager.Add(token.UpsertRequest{Name: "prem-primary", Provider: token.ProviderPrem, TokenValue: "prem-primary-token"})
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(config.Config{
		ProxyPort:       3000,
		ControlPort:     3890,
		PremBaseURL:     upstream.URL + "/v1",
		SwitchThreshold: 15,
		MaxRetries:      1,
	}, manager, logs.NewRecorder(10))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/prem/v1/chat/completions", stringsReader(`{"model":"qwen3.5","messages":[]}`))
	req.Header.Set("Authorization", "Bearer caller")
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	if len(upstreamPaths) != 2 || upstreamPaths[0] != "/openai/v1/chat/completions" || upstreamPaths[1] != "/openai/v1/chat/completions" {
		t.Fatalf("unexpected Prem upstream paths: %#v", upstreamPaths)
	}
	if len(authorizations) != 2 || authorizations[0] != "Bearer prem-primary-token" || authorizations[1] != "Bearer prem-backup-token" {
		t.Fatalf("expected Prem retry to rotate keys, got %#v", authorizations)
	}
	primaryState, err := manager.Get(primary.ID)
	if err != nil {
		t.Fatal(err)
	}
	backupState, err := manager.Get(backup.ID)
	if err != nil {
		t.Fatal(err)
	}
	if primaryState.Status != token.StatusExhausted || backupState.Status != token.StatusActive || backupState.Stats.TotalTokens != 11 {
		t.Fatalf("unexpected Prem token states primary=%#v backup=%#v", primaryState, backupState)
	}
}

func TestServiceRoutesPremAnthropicRequests(t *testing.T) {
	var upstreamPath string
	var authorization string
	var apiKey string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		authorization = r.Header.Get("Authorization")
		apiKey = r.Header.Get("x-api-key")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"usage":{"input_tokens":2,"output_tokens":3}}`))
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(token.UpsertRequest{Name: "prem", Provider: token.ProviderPrem, TokenValue: "prem-api-key-token"}); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(config.Config{
		ProxyPort:       3000,
		ControlPort:     3890,
		PremBaseURL:     upstream.URL + "/v1",
		SwitchThreshold: 15,
		MaxRetries:      0,
	}, manager, logs.NewRecorder(10))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/prem/anthropic/v1/messages", stringsReader(`{"model":"deepseek-v4-pro","messages":[]}`))
	req.Header.Set("Authorization", "Bearer caller")
	req.Header.Set("X-Api-Key", "caller")
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	if upstreamPath != "/anthropic/v1/messages" || authorization != "Bearer prem-api-key-token" || apiKey != "" {
		t.Fatalf("unexpected Prem Anthropic route path=%q authorization=%q apiKey=%q", upstreamPath, authorization, apiKey)
	}
}

func TestServiceRoutesAnyRouterAnthropicRequests(t *testing.T) {
	var upstreamPath string
	var apiKey string
	var authorization string
	var anthropicVersion string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		apiKey = r.Header.Get("x-api-key")
		authorization = r.Header.Get("Authorization")
		anthropicVersion = r.Header.Get("anthropic-version")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"usage":{"input_tokens":2,"output_tokens":3}}`))
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(token.UpsertRequest{Name: "anyrouter", Provider: token.ProviderAnyRouter, BaseURL: upstream.URL, TokenValue: "anyrouter-api-key-token"}); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(config.Config{
		ProxyPort:       3000,
		ControlPort:     3890,
		SwitchThreshold: 15,
		MaxRetries:      0,
	}, manager, logs.NewRecorder(10))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/anyrouter/anthropic/v1/messages", stringsReader(`{"model":"claude-opus-4-5-20251101","messages":[]}`))
	req.Header.Set("Authorization", "Bearer caller")
	req.Header.Set("X-Api-Key", "caller")
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	if upstreamPath != "/v1/messages" || apiKey != "anyrouter-api-key-token" || authorization != "" || anthropicVersion != "2023-06-01" {
		t.Fatalf("unexpected anyrouter anthropic route path=%q key=%q authorization=%q version=%q", upstreamPath, apiKey, authorization, anthropicVersion)
	}
}
