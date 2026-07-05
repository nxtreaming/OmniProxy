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

func TestServiceRoutesClaudeDeepSeekModelToDeepSeekAnthropicGateway(t *testing.T) {
	var gotPath string
	var gotKey string
	var gotBody string

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		gotPath = r.URL.Path
		gotKey = r.Header.Get("x-api-key")
		gotBody = string(body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`))
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "deepseek-tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(token.UpsertRequest{
		Name:       "deepseek",
		Provider:   token.ProviderDeepSeek,
		TokenValue: "sk-deepseek-router-key",
	}); err != nil {
		t.Fatal(err)
	}

	service, err := NewService(config.Config{
		ProxyPort:                3000,
		ControlPort:              3890,
		DeepSeekAnthropicBaseURL: upstream.URL + "/anthropic",
		GatewayRoutes:            config.GatewayRoutes{},
		SwitchThreshold:          15,
		MaxRetries:               0,
	}, manager, logs.NewRecorder(10))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/anthropic-router/v1/messages", stringsReader(`{"model":"deepseek-v4-pro[1m]","messages":[]}`))
	req.Header.Set("Authorization", "Bearer caller")
	req.Header.Set("X-Api-Key", "caller")
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	if gotPath != "/anthropic/v1/messages" || gotKey != "sk-deepseek-router-key" {
		t.Fatalf("unexpected DeepSeek route path=%q key=%q", gotPath, gotKey)
	}
	if !strings.Contains(gotBody, `"model":"deepseek-v4-pro"`) || strings.Contains(gotBody, "deepseek-v4-pro[1m]") {
		t.Fatalf("expected request model to normalize for upstream, got %q", gotBody)
	}
}
