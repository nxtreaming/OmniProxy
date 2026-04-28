package proxy

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"OmniProxyBackend/internal/config"
	"OmniProxyBackend/internal/history"
	"OmniProxyBackend/internal/logs"
	"OmniProxyBackend/internal/storage"
	"OmniProxyBackend/internal/token"
	"github.com/gorilla/websocket"
)

func TestServiceRetries429WithNextTokenAndPreservesBody(t *testing.T) {
	var authHeaders []string
	var bodies []string

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		authHeaders = append(authHeaders, r.Header.Get("Authorization"))
		bodies = append(bodies, string(body))

		if len(authHeaders) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte("limit"))
			return
		}

		w.Header().Set("x-ratelimit-remaining-tokens", "88")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	backup, err := manager.Add(token.UpsertRequest{Name: "backup", Provider: "openai", TokenValue: "sk-backup-token"})
	if err != nil {
		t.Fatal(err)
	}
	primary, err := manager.Add(token.UpsertRequest{Name: "primary", Provider: "openai", TokenValue: "sk-primary-token"})
	if err != nil {
		t.Fatal(err)
	}
	recorder, err := history.NewRecorder(storage.NewJSONStore[[]history.Entry](filepath.Join(t.TempDir(), "history.json")), 100)
	if err != nil {
		t.Fatal(err)
	}

	service, err := NewService(config.Config{
		ProxyPort:       3000,
		ControlPort:     3890,
		UpstreamBaseURL: upstream.URL,
		SwitchThreshold: 15,
		MaxRetries:      1,
	}, manager, logs.NewRecorder(10), recorder)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions?stream=true", io.NopCloser(stringsReader("payload")))
	res := httptest.NewRecorder()

	service.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	if got := res.Body.String(); got != "ok" {
		t.Fatalf("expected upstream body, got %q", got)
	}
	if len(authHeaders) != 2 {
		t.Fatalf("expected 2 upstream attempts, got %d", len(authHeaders))
	}
	if authHeaders[0] != "Bearer sk-primary-token" {
		t.Fatalf("expected first auth to use primary token, got %q", authHeaders[0])
	}
	if authHeaders[1] != "Bearer sk-backup-token" {
		t.Fatalf("expected second auth to use backup token, got %q", authHeaders[1])
	}
	if bodies[0] != "payload" || bodies[1] != "payload" {
		t.Fatalf("request body was not preserved across retry: %#v", bodies)
	}

	items := manager.List()
	statusByID := map[string]token.Status{}
	requestsByID := map[string]int64{}
	cooldownByID := map[string]*time.Time{}
	for _, item := range items {
		statusByID[item.ID] = item.Status
		requestsByID[item.ID] = item.Stats.RequestCount
		cooldownByID[item.ID] = item.CooldownUntil
	}
	if statusByID[primary.ID] != token.StatusExhausted {
		t.Fatalf("expected primary to be exhausted, got %s", statusByID[primary.ID])
	}
	if cooldownByID[primary.ID] == nil || !cooldownByID[primary.ID].After(time.Now()) {
		t.Fatalf("expected primary to enter cooldown, got %v", cooldownByID[primary.ID])
	}
	if statusByID[backup.ID] != token.StatusActive {
		t.Fatalf("expected backup to stay active, got %s", statusByID[backup.ID])
	}
	if requestsByID[primary.ID] != 1 {
		t.Fatalf("expected primary request count 1, got %d", requestsByID[primary.ID])
	}
	if requestsByID[backup.ID] != 1 {
		t.Fatalf("expected backup request count 1, got %d", requestsByID[backup.ID])
	}
	entries := recorder.List(history.Filter{Limit: 10})
	if len(entries) != 1 {
		t.Fatalf("expected one request history entry, got %#v", entries)
	}
	if !entries[0].CooldownTriggered {
		t.Fatalf("expected request history to mark retry cooldown: %#v", entries[0])
	}
	if len(entries[0].RetryChain) != 2 {
		t.Fatalf("expected retry chain to include both attempts, got %#v", entries[0].RetryChain)
	}
	if !entries[0].RetryChain[0].CooldownTriggered || entries[0].RetryChain[0].Status != http.StatusTooManyRequests {
		t.Fatalf("expected first retry attempt to record 429 cooldown, got %#v", entries[0].RetryChain[0])
	}
	if entries[0].RetryChain[1].TokenName != "backup" || entries[0].RetryChain[1].Status != http.StatusOK {
		t.Fatalf("expected second retry attempt to use backup successfully, got %#v", entries[0].RetryChain[1])
	}
}

func TestServiceRetries500AndShortCoolsTransientToken(t *testing.T) {
	attempts := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"message":"temporary upstream failure"}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	backup, err := manager.Add(token.UpsertRequest{Name: "backup", Provider: "openai", TokenValue: "sk-backup-token"})
	if err != nil {
		t.Fatal(err)
	}
	primary, err := manager.Add(token.UpsertRequest{Name: "primary", Provider: "openai", TokenValue: "sk-primary-token"})
	if err != nil {
		t.Fatal(err)
	}
	recorder, err := history.NewRecorder(storage.NewJSONStore[[]history.Entry](filepath.Join(t.TempDir(), "history.json")), 100)
	if err != nil {
		t.Fatal(err)
	}

	service, err := NewService(config.Config{
		ProxyPort:       3000,
		ControlPort:     3890,
		UpstreamBaseURL: upstream.URL,
		SwitchThreshold: 15,
		MaxRetries:      1,
	}, manager, logs.NewRecorder(10), recorder)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", stringsReader(`{"model":"gpt-5.5"}`))
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusOK || attempts != 2 {
		t.Fatalf("expected retry to recover with 200 after 2 attempts, got status=%d attempts=%d body=%s", res.Code, attempts, res.Body.String())
	}
	primaryState, err := manager.Get(primary.ID)
	if err != nil {
		t.Fatal(err)
	}
	if primaryState.Status != token.StatusExhausted || primaryState.CooldownUntil == nil || !primaryState.CooldownUntil.After(time.Now()) {
		t.Fatalf("expected primary to enter short transient cooldown, got %#v", primaryState)
	}
	backupState, err := manager.Get(backup.ID)
	if err != nil {
		t.Fatal(err)
	}
	if backupState.Status != token.StatusActive {
		t.Fatalf("expected backup to stay active, got %s", backupState.Status)
	}
	entries := recorder.List(history.Filter{Limit: 10})
	if len(entries) != 1 || len(entries[0].RetryChain) != 2 {
		t.Fatalf("expected one history entry with two retry attempts, got %#v", entries)
	}
	if entries[0].RetryChain[0].Status != http.StatusInternalServerError || !entries[0].RetryChain[0].CooldownTriggered {
		t.Fatalf("expected 500 retry attempt to record transient cooldown, got %#v", entries[0].RetryChain[0])
	}
}

func TestServiceRecordsTokenUsageFromJSONResponse(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"usage":{"input_tokens":120,"output_tokens":45,"total_tokens":165}}`))
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	item, err := manager.Add(token.UpsertRequest{Name: "primary", Provider: "openai", TokenValue: "sk-primary-token"})
	if err != nil {
		t.Fatal(err)
	}

	service, err := NewService(config.Config{
		ProxyPort:       3000,
		ControlPort:     3890,
		UpstreamBaseURL: upstream.URL,
		SwitchThreshold: 15,
		MaxRetries:      1,
	}, manager, logs.NewRecorder(10))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", stringsReader("payload"))
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	updated, err := manager.Get(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Stats.TotalTokens != 165 || updated.Stats.InputTokens != 120 || updated.Stats.OutputTokens != 45 {
		t.Fatalf("unexpected stats: %#v", updated.Stats)
	}
	if updated.Stats.RequestCount != 1 {
		t.Fatalf("expected request count 1, got %d", updated.Stats.RequestCount)
	}
}

func TestServiceRecordsPersistentRequestHistory(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"usage":{"input_tokens":8,"output_tokens":5,"total_tokens":13}}`))
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(token.UpsertRequest{Name: "primary", Provider: "openai", TokenValue: "sk-primary-token"}); err != nil {
		t.Fatal(err)
	}
	recorder, err := history.NewRecorder(storage.NewJSONStore[[]history.Entry](filepath.Join(t.TempDir(), "history.json")), 100)
	if err != nil {
		t.Fatal(err)
	}

	logRecorder := logs.NewRecorder(10)
	service, err := NewService(config.Config{
		ProxyPort:       3000,
		ControlPort:     3890,
		OpenAIBaseURL:   upstream.URL,
		SwitchThreshold: 15,
		MaxRetries:      0,
	}, manager, logRecorder, recorder)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", stringsReader(`{"model":"gpt-test","input":"hello"}`))
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	entries := recorder.List(history.Filter{Limit: 10})
	if len(entries) != 1 {
		t.Fatalf("expected 1 history entry, got %#v", entries)
	}
	entry := entries[0]
	if entry.Provider != token.ProviderOpenAI || entry.Model != "gpt-test" || entry.Status != http.StatusOK {
		t.Fatalf("unexpected history route metadata: %#v", entry)
	}
	if entry.TokenName != "primary" || entry.TotalTokens != 13 || entry.InputTokens != 8 || entry.OutputTokens != 5 {
		t.Fatalf("unexpected history usage metadata: %#v", entry)
	}
	logEntries := logRecorder.List()
	if len(logEntries) != 1 {
		t.Fatalf("expected 1 log entry, got %#v", logEntries)
	}
	if logEntries[0].Model != "gpt-test" {
		t.Fatalf("expected structured log model gpt-test, got %#v", logEntries[0])
	}
	if strings.Contains(logEntries[0].Message, "model=") {
		t.Fatalf("log message should not carry structured model metadata: %#v", logEntries[0])
	}
}

func TestServiceRecordsLogModelFromQuery(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(token.UpsertRequest{Name: "primary", Provider: "openai", TokenValue: "sk-primary-token"}); err != nil {
		t.Fatal(err)
	}

	logRecorder := logs.NewRecorder(10)
	service, err := NewService(config.Config{
		ProxyPort:       3000,
		ControlPort:     3890,
		OpenAIBaseURL:   upstream.URL,
		SwitchThreshold: 15,
		MaxRetries:      0,
	}, manager, logRecorder)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/responses?model=gpt-query", stringsReader(`{}`))
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	logEntries := logRecorder.List()
	if len(logEntries) != 1 {
		t.Fatalf("expected 1 log entry, got %#v", logEntries)
	}
	if logEntries[0].Model != "gpt-query" {
		t.Fatalf("expected query model in structured log, got %#v", logEntries[0])
	}
}

func TestServiceRecordsCodexModelFromJSONResponse(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"model":"gpt-response","usage":{"input_tokens":3,"output_tokens":4,"total_tokens":7}}`))
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
	historyRecorder, err := history.NewRecorder(storage.NewJSONStore[[]history.Entry](filepath.Join(t.TempDir(), "history.json")), 100)
	if err != nil {
		t.Fatal(err)
	}
	logRecorder := logs.NewRecorder(10)

	service, err := NewService(config.Config{
		ProxyPort:       3000,
		ControlPort:     3890,
		CodexBaseURL:    upstream.URL + "/backend-api/codex",
		SwitchThreshold: 15,
		MaxRetries:      0,
	}, manager, logRecorder, historyRecorder)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/backend-api/codex/responses", stringsReader(`{"input":"hello"}`))
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	historyEntries := historyRecorder.List(history.Filter{Limit: 10})
	if len(historyEntries) != 1 {
		t.Fatalf("expected 1 history entry, got %#v", historyEntries)
	}
	if historyEntries[0].Model != "gpt-response" {
		t.Fatalf("expected codex model from response, got %#v", historyEntries[0])
	}
	logEntries := logRecorder.List()
	if len(logEntries) != 1 || logEntries[0].Model != "gpt-response" {
		t.Fatalf("expected codex log model from response, got %#v", logEntries)
	}
}

func TestServiceRejectsOversizedRequestBody(t *testing.T) {
	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(config.Config{
		ProxyPort:       3000,
		ControlPort:     3890,
		UpstreamBaseURL: "https://api.openai.com",
		SwitchThreshold: 15,
		MaxRetries:      1,
	}, manager, logs.NewRecorder(10))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", io.LimitReader(repeatingReader{}, maxProxyRequestBodyBytes+1))
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status 413, got %d body=%s", res.Code, res.Body.String())
	}
}

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

func TestServiceDoesNotRefreshAPIKeyQuotaAfterTask(t *testing.T) {
	quotaRefreshCalled := make(chan struct{}, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/responses":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}`))
		case "/v1/models":
			quotaRefreshCalled <- struct{}{}
			w.Header().Set("x-ratelimit-remaining-tokens", "64")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[]}`))
		default:
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	item, err := manager.Add(token.UpsertRequest{Name: "primary", Provider: token.ProviderOpenAI, TokenValue: "sk-primary-token"})
	if err != nil {
		t.Fatal(err)
	}

	service, err := NewService(config.Config{
		ProxyPort:       3000,
		ControlPort:     3890,
		OpenAIBaseURL:   upstream.URL,
		SwitchThreshold: 15,
		MaxRetries:      0,
	}, manager, logs.NewRecorder(10))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", stringsReader(`{"input":"hello"}`))
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}

	select {
	case <-quotaRefreshCalled:
		t.Fatal("post-task API key quota refresh should not run from the proxy service")
	case <-time.After(150 * time.Millisecond):
	}

	updated, err := manager.Get(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Remaining != 100 || updated.Usage.APIRemaining != 0 {
		t.Fatalf("expected quota to remain unchanged until scheduled refresh, got remaining=%d usage=%#v", updated.Remaining, updated.Usage)
	}
}

func TestServiceIgnoresIncomingCodexAccountIDForGatewayScheduling(t *testing.T) {
	var gotAuth string
	var gotAccount string

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAccount = r.Header.Get("ChatGPT-Account-Id")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"usage":{"input_tokens":4,"output_tokens":2,"total_tokens":6}}`))
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	preferred, err := manager.Add(token.UpsertRequest{
		Provider:       token.ProviderOpenAI,
		CredentialType: token.CredentialTypeCodexAuthJSON,
		TokenValue:     codexAuthJSONForServiceTestWithCredentials(t, "preferred@example.com", "account-preferred", "preferred-access-token"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.RecordUsage(preferred.ID, 8); err != nil {
		t.Fatal(err)
	}
	_, err = manager.Add(token.UpsertRequest{
		Provider:       token.ProviderOpenAI,
		CredentialType: token.CredentialTypeCodexAuthJSON,
		TokenValue:     codexAuthJSONForServiceTestWithCredentials(t, "other@example.com", "account-other", "other-access-token"),
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
		CodexUsageEndpoint: "https://chatgpt.com/backend-api/wham/usage",
	}, manager, logs.NewRecorder(10))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/backend-api/codex/v1/responses", stringsReader(`{"input":"hello"}`))
	req.Header.Set("Authorization", "Bearer caller-access-token")
	req.Header.Set("ChatGPT-Account-Id", "account-preferred")
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	if gotAuth != "Bearer other-access-token" {
		t.Fatalf("expected gateway-selected account auth, got %q", gotAuth)
	}
	if gotAccount != "account-other" {
		t.Fatalf("expected gateway-selected account id, got %q", gotAccount)
	}
}

func TestServiceBalancedSchedulingChoosesHigherRemainingCodexAccount(t *testing.T) {
	var gotAuth string
	var gotAccount string

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAccount = r.Header.Get("ChatGPT-Account-Id")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"usage":{"input_tokens":4,"output_tokens":2,"total_tokens":6}}`))
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "balanced-codex-tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	lower, err := manager.Add(token.UpsertRequest{
		Provider:       token.ProviderOpenAI,
		CredentialType: token.CredentialTypeCodexAuthJSON,
		TokenValue:     codexAuthJSONForServiceTestWithCredentials(t, "lower@example.com", "account-lower", "lower-access-token"),
	})
	if err != nil {
		t.Fatal(err)
	}
	higher, err := manager.Add(token.UpsertRequest{
		Provider:       token.ProviderOpenAI,
		CredentialType: token.CredentialTypeCodexAuthJSON,
		TokenValue:     codexAuthJSONForServiceTestWithCredentials(t, "higher@example.com", "account-higher", "higher-access-token"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.RecordUsage(lower.ID, 35); err != nil {
		t.Fatal(err)
	}
	if err := manager.RecordUsage(higher.ID, 90); err != nil {
		t.Fatal(err)
	}

	service, err := NewService(config.Config{
		ProxyPort:          3000,
		ControlPort:        3890,
		SchedulingMode:     config.SchedulingModeBalanced,
		CodexBaseURL:       upstream.URL + "/backend-api/codex",
		SwitchThreshold:    15,
		MaxRetries:         0,
		CodexUsageEndpoint: "https://chatgpt.com/backend-api/wham/usage",
	}, manager, logs.NewRecorder(10))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/backend-api/codex/v1/responses", stringsReader(`{"input":"hello"}`))
	req.Header.Set("ChatGPT-Account-Id", "account-lower")
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	if gotAuth != "Bearer higher-access-token" {
		t.Fatalf("expected balanced scheduler to choose higher quota auth, got %q", gotAuth)
	}
	if gotAccount != "account-higher" {
		t.Fatalf("expected balanced scheduler to choose higher quota account, got %q", gotAccount)
	}
}

func TestServiceProxiesCodexResponsesWebSocket(t *testing.T) {
	type capture struct {
		path    string
		auth    string
		account string
		beta    string
		message string
		err     error
	}
	captures := make(chan capture, 1)
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			captures <- capture{err: err}
			return
		}
		defer conn.Close()

		_, payload, err := conn.ReadMessage()
		if err != nil {
			captures <- capture{err: err}
			return
		}
		captures <- capture{
			path:    r.URL.Path,
			auth:    r.Header.Get("Authorization"),
			account: r.Header.Get("ChatGPT-Account-Id"),
			beta:    r.Header.Get("OpenAI-Beta"),
			message: string(payload),
		}
		responsePayload := `{"type":"response.completed","response":{"usage":{"input_tokens":11,"output_tokens":7,"total_tokens":18}}}`
		if err := conn.WriteMessage(websocket.TextMessage, []byte(responsePayload)); err != nil {
			t.Errorf("failed to write upstream websocket response: %v", err)
		}
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "ws-tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	item, err := manager.Add(token.UpsertRequest{
		Name:           "coder@example.com",
		Provider:       token.ProviderOpenAI,
		CredentialType: token.CredentialTypeCodexAuthJSON,
		TokenValue:     codexAuthJSONForServiceTestWithCredentials(t, "coder@example.com", "account-ws", "ws-access-token"),
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
		CodexUsageEndpoint: "https://chatgpt.com/backend-api/wham/usage",
	}, manager, logs.NewRecorder(10))
	if err != nil {
		t.Fatal(err)
	}
	local := httptest.NewServer(service)
	defer local.Close()

	dialURL := "ws" + strings.TrimPrefix(local.URL, "http") + "/backend-api/codex/responses"
	headers := http.Header{
		"ChatGPT-Account-Id": []string{"account-ws"},
		"OpenAI-Beta":        []string{"responses_websockets=2026-02-06"},
	}
	conn, _, err := websocket.DefaultDialer.Dial(dialURL, headers)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"client_event"}`)); err != nil {
		t.Fatal(err)
	}
	messageType, payload, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if messageType != websocket.TextMessage || string(payload) != `{"type":"response.completed","response":{"usage":{"input_tokens":11,"output_tokens":7,"total_tokens":18}}}` {
		t.Fatalf("unexpected websocket response type=%d payload=%q", messageType, string(payload))
	}

	var got capture
	select {
	case got = <-captures:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for upstream websocket capture")
	}
	if got.err != nil {
		t.Fatal(got.err)
	}
	if got.path != "/backend-api/codex/responses" {
		t.Fatalf("unexpected upstream websocket path: %s", got.path)
	}
	if got.auth != "Bearer ws-access-token" {
		t.Fatalf("unexpected websocket Authorization header: %q", got.auth)
	}
	if got.account != "account-ws" {
		t.Fatalf("unexpected websocket account header: %q", got.account)
	}
	if got.beta != "responses_websockets=2026-02-06" {
		t.Fatalf("expected websocket beta header to be preserved, got %q", got.beta)
	}
	if got.message != `{"type":"client_event"}` {
		t.Fatalf("expected websocket message to be proxied, got %q", got.message)
	}
	_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
	_ = conn.Close()

	deadline := time.After(2 * time.Second)
	for {
		updated, err := manager.Get(item.ID)
		if err != nil {
			t.Fatal(err)
		}
		if updated.Stats.RequestCount == 1 {
			if updated.Stats.InputTokens != 11 || updated.Stats.OutputTokens != 7 || updated.Stats.TotalTokens != 18 {
				t.Fatalf("unexpected websocket token stats: %#v", updated.Stats)
			}
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for websocket token stats: %#v", updated.Stats)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestServiceRejectsCodexResponsesWebSocketWhenDisabled(t *testing.T) {
	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "ws-disabled-tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(config.Config{
		ProxyPort:       3000,
		ControlPort:     3890,
		CodexBaseURL:    "https://chatgpt.com/backend-api/codex",
		SwitchThreshold: 15,
		MaxRetries:      0,
		WebSocketMode:   config.WebSocketModeDisabled,
	}, manager, logs.NewRecorder(10))
	if err != nil {
		t.Fatal(err)
	}
	local := httptest.NewServer(service)
	defer local.Close()

	dialURL := "ws" + strings.TrimPrefix(local.URL, "http") + "/backend-api/codex/responses"
	conn, resp, err := websocket.DefaultDialer.Dial(dialURL, http.Header{
		"OpenAI-Beta": []string{"responses_websockets=2026-02-06"},
	})
	if conn != nil {
		_ = conn.Close()
	}
	if err == nil {
		t.Fatal("expected websocket dial to fail when disabled")
	}
	if resp == nil {
		t.Fatal("expected handshake response")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", resp.StatusCode)
	}
}

func TestServiceHandlesCodexResponsesProbe(t *testing.T) {
	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "codex-probe-tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(config.Config{
		ProxyPort:       3000,
		ControlPort:     3890,
		CodexBaseURL:    "https://chatgpt.com/backend-api/codex",
		SwitchThreshold: 15,
		MaxRetries:      0,
	}, manager, logs.NewRecorder(10))
	if err != nil {
		t.Fatal(err)
	}

	headRes := httptest.NewRecorder()
	service.ServeHTTP(headRes, httptest.NewRequest(http.MethodHead, "/backend-api/codex/v1/responses", nil))
	if headRes.Code != http.StatusOK {
		t.Fatalf("expected HEAD codex probe status 200, got %d body=%s", headRes.Code, headRes.Body.String())
	}
	if headRes.Body.Len() != 0 {
		t.Fatalf("expected HEAD codex probe to have empty body, got %q", headRes.Body.String())
	}

	getRes := httptest.NewRecorder()
	service.ServeHTTP(getRes, httptest.NewRequest(http.MethodGet, "/backend-api/codex/responses", nil))
	if getRes.Code != http.StatusOK {
		t.Fatalf("expected GET codex probe status 200, got %d body=%s", getRes.Code, getRes.Body.String())
	}
	if !strings.Contains(getRes.Body.String(), `"ok":true`) {
		t.Fatalf("expected GET codex probe health body, got %q", getRes.Body.String())
	}
}

func TestServiceRoutesXiaomiByCredentialTypeAndProtocol(t *testing.T) {
	var gotPaths []string
	var gotKeys []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/models") {
			w.Header().Set("x-ratelimit-remaining-tokens", "90")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[]}`))
			return
		}
		gotPaths = append(gotPaths, r.URL.Path)
		gotKeys = append(gotKeys, r.Header.Get("Api-Key"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`))
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "paygo-tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.Add(token.UpsertRequest{
		Name:       "paygo",
		Provider:   token.ProviderXiaomi,
		TokenValue: "sk-paygo-secret",
	})
	if err != nil {
		t.Fatal(err)
	}

	service, err := NewService(config.Config{
		ProxyPort:                       3000,
		ControlPort:                     3890,
		XiaomiAPIBaseURL:                upstream.URL + "/v1",
		XiaomiAPIAnthropicBaseURL:       upstream.URL + "/anthropic",
		XiaomiTokenPlanBaseURL:          upstream.URL + "/token-plan/v1",
		XiaomiTokenPlanAnthropicBaseURL: upstream.URL + "/token-plan/anthropic",
		SwitchThreshold:                 15,
		MaxRetries:                      1,
	}, manager, logs.NewRecorder(10))
	if err != nil {
		t.Fatal(err)
	}

	service.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/xiaomi/chat/completions", stringsReader("{}")))

	tokenPlanManager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "token-plan-tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tokenPlanManager.Add(token.UpsertRequest{
		Name:           "token-plan",
		Provider:       token.ProviderXiaomi,
		CredentialType: token.CredentialTypeMimoTokenPlan,
		TokenValue:     "tp-token-plan-secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	tokenPlanService, err := NewService(config.Config{
		ProxyPort:                       3000,
		ControlPort:                     3890,
		XiaomiAPIBaseURL:                upstream.URL + "/v1",
		XiaomiAPIAnthropicBaseURL:       upstream.URL + "/anthropic",
		XiaomiTokenPlanBaseURL:          upstream.URL + "/token-plan/v1",
		XiaomiTokenPlanAnthropicBaseURL: upstream.URL + "/token-plan/anthropic",
		SwitchThreshold:                 15,
		MaxRetries:                      1,
	}, tokenPlanManager, logs.NewRecorder(10))
	if err != nil {
		t.Fatal(err)
	}
	tokenPlanService.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/xiaomi/anthropic/v1/messages", stringsReader("{}")))

	if len(gotPaths) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(gotPaths))
	}
	if gotPaths[0] != "/v1/chat/completions" || gotKeys[0] != "sk-paygo-secret" {
		t.Fatalf("unexpected paygo route path=%q key=%q", gotPaths[0], gotKeys[0])
	}
	if gotPaths[1] != "/token-plan/anthropic/v1/messages" || gotKeys[1] != "tp-token-plan-secret" {
		t.Fatalf("unexpected token plan route path=%q key=%q", gotPaths[1], gotKeys[1])
	}
}

func TestServiceRoutesAnthropicRouterByModel(t *testing.T) {
	var mimoPath string
	var mimoKey string
	var mimoBodies []string
	var deepSeekPath string
	var deepSeekKey string
	var deepSeekAuthorization string
	var kimiPath string
	var kimiKey string
	var kimiAuthorization string
	var officialPath string
	var officialKey string

	mimoUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/models") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[]}`))
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		mimoPath = r.URL.Path
		mimoKey = r.Header.Get("Api-Key")
		mimoBodies = append(mimoBodies, string(body))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`))
	}))
	defer mimoUpstream.Close()

	deepSeekUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/models") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[]}`))
			return
		}
		deepSeekPath = r.URL.Path
		deepSeekKey = r.Header.Get("X-Api-Key")
		deepSeekAuthorization = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"usage":{"input_tokens":2,"output_tokens":2,"total_tokens":4}}`))
	}))
	defer deepSeekUpstream.Close()

	kimiUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/models") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[]}`))
			return
		}
		kimiPath = r.URL.Path
		kimiKey = r.Header.Get("X-Api-Key")
		kimiAuthorization = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"usage":{"input_tokens":3,"output_tokens":3,"total_tokens":6}}`))
	}))
	defer kimiUpstream.Close()

	officialUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/models") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[]}`))
			return
		}
		officialPath = r.URL.Path
		officialKey = r.Header.Get("X-Api-Key")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"usage":{"input_tokens":2,"output_tokens":3,"total_tokens":5}}`))
	}))
	defer officialUpstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "router-tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(token.UpsertRequest{
		Name:       "anthropic",
		Provider:   token.ProviderAnthropic,
		TokenValue: "sk-ant-router-key",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(token.UpsertRequest{
		Name:       "mimo",
		Provider:   token.ProviderXiaomi,
		TokenValue: "sk-mimo-router-key",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(token.UpsertRequest{
		Name:       "deepseek",
		Provider:   token.ProviderDeepSeek,
		TokenValue: "sk-deepseek-router-key",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(token.UpsertRequest{
		Name:       "kimi",
		Provider:   token.ProviderKimi,
		TokenValue: "sk-kimi-router-key",
	}); err != nil {
		t.Fatal(err)
	}

	service, err := NewService(config.Config{
		ProxyPort:                 3000,
		ControlPort:               3890,
		AnthropicBaseURL:          officialUpstream.URL,
		DeepSeekBaseURL:           deepSeekUpstream.URL,
		DeepSeekAnthropicBaseURL:  deepSeekUpstream.URL + "/anthropic",
		KimiBaseURL:               kimiUpstream.URL + "/coding",
		XiaomiAPIBaseURL:          mimoUpstream.URL + "/v1",
		XiaomiAPIAnthropicBaseURL: mimoUpstream.URL + "/anthropic",
		SwitchThreshold:           15,
		MaxRetries:                0,
	}, manager, logs.NewRecorder(10))
	if err != nil {
		t.Fatal(err)
	}

	mimoReq := httptest.NewRequest(http.MethodPost, "/anthropic-router/v1/messages", stringsReader(`{"model":"mimo-v2.5-pro","messages":[]}`))
	mimoReq.Header.Set("Authorization", "Bearer caller")
	mimoReq.Header.Set("X-Api-Key", "caller")
	mimoRes := httptest.NewRecorder()
	service.ServeHTTP(mimoRes, mimoReq)

	mimoStandardReq := httptest.NewRequest(http.MethodPost, "/anthropic-router/v1/messages", stringsReader(`{"model":"mimo-v2.5","messages":[]}`))
	mimoStandardReq.Header.Set("Authorization", "Bearer caller")
	mimoStandardReq.Header.Set("X-Api-Key", "caller")
	mimoStandardRes := httptest.NewRecorder()
	service.ServeHTTP(mimoStandardRes, mimoStandardReq)

	deepSeekReq := httptest.NewRequest(http.MethodPost, "/anthropic-router/v1/messages", stringsReader(`{"model":"deepseek-v4-pro[1m]","messages":[]}`))
	deepSeekReq.Header.Set("Authorization", "Bearer caller")
	deepSeekReq.Header.Set("Api-Key", "caller")
	deepSeekRes := httptest.NewRecorder()
	service.ServeHTTP(deepSeekRes, deepSeekReq)

	kimiReq := httptest.NewRequest(http.MethodPost, "/anthropic-router/v1/messages", stringsReader(`{"model":"kimi-for-coding","messages":[]}`))
	kimiReq.Header.Set("Authorization", "Bearer caller")
	kimiReq.Header.Set("Api-Key", "caller")
	kimiRes := httptest.NewRecorder()
	service.ServeHTTP(kimiRes, kimiReq)

	officialReq := httptest.NewRequest(http.MethodPost, "/anthropic-router/v1/messages", stringsReader(`{"model":"claude-sonnet-4-5","messages":[]}`))
	officialReq.Header.Set("Authorization", "Bearer caller")
	officialReq.Header.Set("Api-Key", "caller")
	officialRes := httptest.NewRecorder()
	service.ServeHTTP(officialRes, officialReq)

	if mimoRes.Code != http.StatusOK {
		t.Fatalf("expected mimo route status 200, got %d body=%s", mimoRes.Code, mimoRes.Body.String())
	}
	if mimoStandardRes.Code != http.StatusOK {
		t.Fatalf("expected standard mimo route status 200, got %d body=%s", mimoStandardRes.Code, mimoStandardRes.Body.String())
	}
	if deepSeekRes.Code != http.StatusOK {
		t.Fatalf("expected deepseek route status 200, got %d body=%s", deepSeekRes.Code, deepSeekRes.Body.String())
	}
	if kimiRes.Code != http.StatusOK {
		t.Fatalf("expected kimi route status 200, got %d body=%s", kimiRes.Code, kimiRes.Body.String())
	}
	if officialRes.Code != http.StatusOK {
		t.Fatalf("expected official route status 200, got %d body=%s", officialRes.Code, officialRes.Body.String())
	}
	if mimoPath != "/anthropic/v1/messages" || mimoKey != "sk-mimo-router-key" {
		t.Fatalf("unexpected mimo route path=%q key=%q", mimoPath, mimoKey)
	}
	if len(mimoBodies) != 2 {
		t.Fatalf("expected 2 mimo requests, got %d", len(mimoBodies))
	}
	if !strings.Contains(mimoBodies[0], `"model":"mimo-v2.5-pro"`) {
		t.Fatalf("expected pro mimo model to be preserved, got %q", mimoBodies[0])
	}
	if !strings.Contains(mimoBodies[1], `"model":"mimo-v2.5"`) {
		t.Fatalf("expected standard mimo model to be preserved, got %q", mimoBodies[1])
	}
	if strings.Contains(mimoBodies[1], `"model":"mimo-v2.5-pro"`) {
		t.Fatalf("standard mimo model was rewritten to pro: %q", mimoBodies[1])
	}
	if deepSeekPath != "/anthropic/v1/messages" || deepSeekKey != "sk-deepseek-router-key" || deepSeekAuthorization != "" {
		t.Fatalf("unexpected deepseek route path=%q key=%q authorization=%q", deepSeekPath, deepSeekKey, deepSeekAuthorization)
	}
	if kimiPath != "/coding/v1/messages" || kimiKey != "sk-kimi-router-key" || kimiAuthorization != "" {
		t.Fatalf("unexpected kimi route path=%q key=%q authorization=%q", kimiPath, kimiKey, kimiAuthorization)
	}
	if officialPath != "/v1/messages" || officialKey != "sk-ant-router-key" {
		t.Fatalf("unexpected official route path=%q key=%q", officialPath, officialKey)
	}
	entries := service.logs.List()
	hasStandardMimoLog := false
	for _, entry := range entries {
		if entry.Model == "mimo-v2.5" {
			hasStandardMimoLog = true
			break
		}
	}
	if !hasStandardMimoLog {
		t.Fatalf("expected logs to include standard mimo model, got %#v", entries)
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

func TestParseTokenConsumptionFromSSE(t *testing.T) {
	body := []byte(strings.Join([]string{
		`event: response.output_text.delta`,
		`data: {"type":"response.output_text.delta","delta":"hello"}`,
		``,
		`event: response.completed`,
		`data: {"type":"response.completed","response":{"model":"gpt-5.5","usage":{"input_tokens":20,"output_tokens":8,"total_tokens":28}}}`,
		``,
		`data: [DONE]`,
	}, "\n"))

	usage := parseTokenConsumption(http.Header{"Content-Type": []string{"text/event-stream"}}, body)
	if usage.TotalTokens != 28 || usage.InputTokens != 20 || usage.OutputTokens != 8 {
		t.Fatalf("unexpected usage: %#v", usage)
	}
	if model := parseResponseModel(http.Header{"Content-Type": []string{"text/event-stream"}}, body); model != "gpt-5.5" {
		t.Fatalf("expected response model gpt-5.5, got %q", model)
	}
}

func stringsReader(value string) io.Reader {
	return strings.NewReader(value)
}

type repeatingReader struct{}

func (repeatingReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'x'
	}
	return len(p), nil
}

func codexAuthJSONForServiceTest(t *testing.T, email string) string {
	return codexAuthJSONForServiceTestWithCredentials(t, email, "account-123", "codex-access-token")
}

func codexAuthJSONForServiceTestWithCredentials(t *testing.T, email string, accountID string, accessToken string) string {
	t.Helper()

	payload, err := json.Marshal(map[string]any{
		"https://api.openai.com/profile": map[string]string{"email": email},
	})
	if err != nil {
		t.Fatal(err)
	}
	idToken := "header." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
	authJSON, err := json.Marshal(map[string]any{
		"tokens": map[string]string{
			"access_token": accessToken,
			"account_id":   accountID,
			"id_token":     idToken,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(authJSON)
}
