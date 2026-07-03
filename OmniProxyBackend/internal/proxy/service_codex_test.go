package proxy

import (
	"encoding/json"
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
	"time"
)

func TestServiceRoutesCodexAuthJSONToCodexBackend(t *testing.T) {
	var gotPath string
	var gotAuth string
	var gotAccount string
	var gotBody string

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotAccount = r.Header.Get("ChatGPT-Account-Id")
		gotBody = string(body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}`))
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.Add(token.UpsertRequest{
		Name:           "coder@example.com",
		Provider:       token.ProviderOpenAI,
		CredentialType: token.CredentialTypeCodexAuthJSON,
		TokenValue:     codexAuthJSONForServiceTest(t, "coder@example.com"),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.Add(token.UpsertRequest{Name: "api-key", Provider: token.ProviderOpenAI, TokenValue: "sk-primary-token"})
	if err != nil {
		t.Fatal(err)
	}

	service, err := NewService(config.Config{
		ProxyPort:          3000,
		ControlPort:        3890,
		OpenAIBaseURL:      "https://api.openai.com",
		CodexBaseURL:       upstream.URL + "/backend-api/codex",
		SwitchThreshold:    15,
		MaxRetries:         1,
		CodexUsageEndpoint: "https://chatgpt.com/backend-api/wham/usage",
	}, manager, logs.NewRecorder(10))
	if err != nil {
		t.Fatal(err)
	}

	requestBody := `{"model":"gpt-5.4","input":"payload"}`
	req := httptest.NewRequest(http.MethodPost, "/backend-api/codex/responses", stringsReader(requestBody))
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	if gotPath != "/backend-api/codex/responses" {
		t.Fatalf("unexpected upstream path: %s", gotPath)
	}
	if gotAuth != "Bearer codex-access-token" {
		t.Fatalf("unexpected Authorization header: %q", gotAuth)
	}
	if gotAccount != "account-123" {
		t.Fatalf("unexpected ChatGPT-Account-Id header: %q", gotAccount)
	}
	if gotBody != requestBody {
		t.Fatalf("expected codex request body to be preserved, got %q", gotBody)
	}
}

func TestServiceConvertsCodexChatCompletionsToResponses(t *testing.T) {
	var gotPath string
	var gotAuth string
	var gotAccount string
	var gotBeta string
	var gotBody map[string]any

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotAccount = r.Header.Get("ChatGPT-Account-Id")
		gotBeta = r.Header.Get("OpenAI-Beta")
		if err := json.Unmarshal(body, &gotBody); err != nil {
			t.Fatalf("invalid upstream body: %v body=%s", err, body)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strings.Join([]string{
			`data: {"type":"response.created","response":{"id":"resp_codex_chat","model":"gpt-5.4"}}`,
			``,
			`data: {"type":"response.output_text.delta","delta":"Hello "}`,
			``,
			`data: {"type":"response.output_text.delta","delta":"world"}`,
			``,
			`data: {"type":"response.completed","response":{"id":"resp_codex_chat","model":"gpt-5.4","status":"completed","usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n")))
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(token.UpsertRequest{
		Name:           "coder@example.com",
		Provider:       token.ProviderOpenAI,
		CredentialType: token.CredentialTypeCodexAuthJSON,
		TokenValue:     codexAuthJSONForServiceTest(t, "coder@example.com"),
	}); err != nil {
		t.Fatal(err)
	}

	service, err := NewService(config.Config{
		ProxyPort:       3000,
		ControlPort:     3890,
		CodexBaseURL:    upstream.URL + "/backend-api/codex",
		SwitchThreshold: 15,
		MaxRetries:      0,
	}, manager, logs.NewRecorder(10))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/codex/v1/chat/completions", stringsReader(`{
		"model":"gpt-5.4-high",
		"messages":[
			{"role":"system","content":"Be concise"},
			{"role":"user","content":"Hi"}
		],
		"max_completion_tokens":128
	}`))
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	if gotPath != "/backend-api/codex/responses" {
		t.Fatalf("unexpected upstream path: %s", gotPath)
	}
	if gotAuth != "Bearer codex-access-token" || gotAccount != "account-123" {
		t.Fatalf("unexpected codex auth headers auth=%q account=%q", gotAuth, gotAccount)
	}
	if gotBeta != "responses=experimental" {
		t.Fatalf("expected responses beta header, got %q", gotBeta)
	}
	if gotBody["model"] != "gpt-5.4" || gotBody["stream"] != true || gotBody["store"] != false {
		t.Fatalf("unexpected upstream model/stream/store body: %#v", gotBody)
	}
	if gotBody["instructions"] != "Be concise" {
		t.Fatalf("unexpected instructions: %#v", gotBody["instructions"])
	}
	if gotBody["max_output_tokens"].(float64) != 128 {
		t.Fatalf("unexpected max_output_tokens: %#v", gotBody["max_output_tokens"])
	}
	input, ok := gotBody["input"].([]any)
	if !ok || len(input) != 1 {
		t.Fatalf("expected one user input item, got %#v", gotBody["input"])
	}
	inputItem := input[0].(map[string]any)
	if inputItem["type"] != "message" || inputItem["role"] != "user" || inputItem["content"] != "Hi" {
		t.Fatalf("unexpected input item: %#v", inputItem)
	}

	var chat struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &chat); err != nil {
		t.Fatalf("invalid chat response: %v body=%s", err, res.Body.String())
	}
	if chat.ID != "resp_codex_chat" || chat.Object != "chat.completion" || chat.Model != "gpt-5.4" {
		t.Fatalf("unexpected chat metadata: %#v", chat)
	}
	if len(chat.Choices) != 1 || chat.Choices[0].Message.Role != "assistant" || chat.Choices[0].Message.Content != "Hello world" || chat.Choices[0].FinishReason != "stop" {
		t.Fatalf("unexpected chat choices: %#v", chat.Choices)
	}
	if chat.Usage.PromptTokens != 3 || chat.Usage.CompletionTokens != 2 || chat.Usage.TotalTokens != 5 {
		t.Fatalf("unexpected usage: %#v", chat.Usage)
	}
}

func TestServiceStreamsCodexChatCompletionsCompatibility(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/backend-api/codex/responses" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strings.Join([]string{
			`data: {"type":"response.created","response":{"id":"resp_stream","model":"gpt-5.4"}}`,
			``,
			`data: {"type":"response.output_text.delta","delta":"Hi"}`,
			``,
			`data: {"type":"response.completed","response":{"id":"resp_stream","model":"gpt-5.4","status":"completed"}}`,
			``,
		}, "\n")))
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(token.UpsertRequest{
		Name:           "coder@example.com",
		Provider:       token.ProviderOpenAI,
		CredentialType: token.CredentialTypeCodexAuthJSON,
		TokenValue:     codexAuthJSONForServiceTest(t, "coder@example.com"),
	}); err != nil {
		t.Fatal(err)
	}

	service, err := NewService(config.Config{
		ProxyPort:       3000,
		ControlPort:     3890,
		CodexBaseURL:    upstream.URL + "/backend-api/codex",
		SwitchThreshold: 15,
		MaxRetries:      0,
	}, manager, logs.NewRecorder(10))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/codex/v1/chat/completions", stringsReader(`{"model":"gpt-5.4","stream":true,"messages":[{"role":"user","content":"Hi"}]}`))
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	body := res.Body.String()
	for _, expected := range []string{`"object":"chat.completion.chunk"`, `"role":"assistant"`, `"content":"Hi"`, `data: [DONE]`} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected streaming body to contain %q, got %s", expected, body)
		}
	}
}

func TestServiceDoesNotRefreshCodexQuotaAfterTask(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}`))
	}))
	defer upstream.Close()

	quotaRefreshCalled := make(chan struct{}, 1)
	usage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		quotaRefreshCalled <- struct{}{}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"plan_type": "team",
			"rate_limit": {
				"primary_window": {"used_percent": 34, "reset_at": 1777299888},
				"secondary_window": {"used_percent": 50, "reset_at": 1777798105}
			}
		}`))
	}))
	defer usage.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	item, err := manager.Add(token.UpsertRequest{
		Provider:       token.ProviderOpenAI,
		CredentialType: token.CredentialTypeCodexAuthJSON,
		TokenValue:     codexAuthJSONForServiceTest(t, "coder@example.com"),
	})
	if err != nil {
		t.Fatal(err)
	}

	service, err := NewService(config.Config{
		ProxyPort:          3000,
		ControlPort:        3890,
		CodexBaseURL:       upstream.URL + "/backend-api/codex",
		SwitchThreshold:    15,
		MaxRetries:         0,
		CodexUsageEndpoint: usage.URL,
	}, manager, logs.NewRecorder(10))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/backend-api/codex/v1/responses", stringsReader(`{"input":"hello"}`))
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}

	select {
	case <-quotaRefreshCalled:
		t.Fatal("post-task codex quota refresh should not run from the proxy service")
	case <-time.After(150 * time.Millisecond):
	}

	updated, err := manager.Get(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Remaining != 100 || updated.Usage.SubscriptionQuotaAvailable {
		t.Fatalf("expected quota to remain unchanged until scheduled refresh, got remaining=%d usage=%#v", updated.Remaining, updated.Usage)
	}
}
