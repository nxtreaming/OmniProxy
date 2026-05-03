package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"OmniProxyBackend/internal/config"
	"OmniProxyBackend/internal/logs"
	"OmniProxyBackend/internal/storage"
	"OmniProxyBackend/internal/token"
)

func TestOpenRouterChatUsesSavedKeyAndRecordsUsage(t *testing.T) {
	var captured struct {
		Path          string
		Authorization string
		Model         string
		Messages      []openRouterChatMessage
		MaxTokens     int
		Temperature   float64
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.Path = r.URL.Path
		captured.Authorization = r.Header.Get("Authorization")

		var payload struct {
			Model       string                  `json:"model"`
			Messages    []openRouterChatMessage `json:"messages"`
			MaxTokens   int                     `json:"max_tokens"`
			Temperature float64                 `json:"temperature"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode upstream request: %v", err)
		}
		captured.Model = payload.Model
		captured.Messages = payload.Messages
		captured.MaxTokens = payload.MaxTokens
		captured.Temperature = payload.Temperature

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"model":"tencent/hy3-preview:free",
			"choices":[{"message":{"role":"assistant","content":"收到"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}
		}`))
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	item, err := manager.Add(token.UpsertRequest{
		Name:       "openrouter",
		Provider:   token.ProviderOpenRouter,
		TokenValue: "sk-or-v1-test-token",
	})
	if err != nil {
		t.Fatal(err)
	}

	app := &appServer{
		cfg: config.Config{
			OpenRouterBaseURL: upstream.URL + "/api/v1",
		},
		tokens: manager,
		logs:   logs.NewRecorder(10),
	}
	temp := 0.6
	result, err := app.openRouterChat(context.Background(), openRouterChatRequest{
		Model: "tencent/hy3-preview:free",
		Messages: []openRouterChatMessage{
			{Role: "user", Content: "你好"},
		},
		Temperature: &temp,
		MaxTokens:   256,
	})
	if err != nil {
		t.Fatal(err)
	}

	if captured.Path != "/api/v1/chat/completions" {
		t.Fatalf("unexpected upstream path: %s", captured.Path)
	}
	if captured.Authorization != "Bearer sk-or-v1-test-token" {
		t.Fatalf("unexpected authorization header: %s", captured.Authorization)
	}
	if captured.Model != "tencent/hy3-preview:free" || len(captured.Messages) != 1 || captured.Messages[0].Content != "你好" {
		t.Fatalf("unexpected upstream payload: %#v", captured)
	}
	if captured.MaxTokens != 256 || captured.Temperature != 0.6 {
		t.Fatalf("unexpected generation controls: %#v", captured)
	}
	if result.Message.Content != "收到" || result.Usage.TotalTokens != 7 || result.FinishReason != "stop" {
		t.Fatalf("unexpected chat response: %#v", result)
	}

	updated, err := manager.Get(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Stats.RequestCount != 1 || updated.Stats.InputTokens != 5 || updated.Stats.OutputTokens != 2 || updated.Stats.TotalTokens != 7 {
		t.Fatalf("usage stats were not recorded: %#v", updated.Stats)
	}
}

func TestOpenRouterChatRejectsEmptyModel(t *testing.T) {
	_, _, err := normalizeOpenRouterChatRequest(openRouterChatRequest{
		Messages: []openRouterChatMessage{{Role: "user", Content: "hello"}},
	})
	if err == nil || !strings.Contains(err.Error(), "请选择 OpenRouter 模型") {
		t.Fatalf("expected empty model error, got %v", err)
	}
}
