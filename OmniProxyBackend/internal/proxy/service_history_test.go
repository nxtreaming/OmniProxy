package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"omniproxy/internal/config"
	"omniproxy/internal/history"
	"omniproxy/internal/logs"
	"omniproxy/internal/storage"
	"omniproxy/internal/token"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestServiceRecordsTokenUsageFromJSONResponse(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"usage":{"input_tokens":120,"output_tokens":45,"total_tokens":165,"input_tokens_details":{"cached_tokens":80},"cache_creation_input_tokens":6}}`))
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
	if updated.Stats.CacheCreationTokens != 6 || updated.Stats.CacheReadTokens != 80 {
		t.Fatalf("unexpected cache stats: %#v", updated.Stats)
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

func TestServiceRecordsCodexWorkspaceTokenName(t *testing.T) {
	var gotAccount string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAccount = r.Header.Get("ChatGPT-Account-Id")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"model":"gpt-response","usage":{"input_tokens":5,"output_tokens":4,"total_tokens":9}}`))
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	item, err := manager.Add(token.UpsertRequest{
		Name:           "coder@example.com",
		Provider:       token.ProviderOpenAI,
		CredentialType: token.CredentialTypeCodexAuthJSON,
		TokenValue:     codexAuthJSONForServiceTestWithCredentials(t, "coder@example.com", "47336c9d1234567890af1e46", "codex-access-token"),
	})
	if err != nil {
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
		CodexBaseURL:    upstream.URL + "/backend-api/codex",
		SwitchThreshold: 15,
		MaxRetries:      0,
	}, manager, logRecorder, recorder)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/backend-api/codex/responses", stringsReader(`{"input":"hello"}`))
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	if gotAccount != "47336c9d1234567890af1e46" {
		t.Fatalf("expected selected workspace account id upstream, got %q", gotAccount)
	}
	entries := recorder.List(history.Filter{Limit: 10})
	if len(entries) != 1 {
		t.Fatalf("expected 1 history entry, got %#v", entries)
	}
	wantName := "coder@example.com (account_id: 47336c9d...af1e46)"
	if entries[0].TokenID != item.ID || entries[0].TokenName != wantName {
		t.Fatalf("expected workspace-aware token record, got %#v", entries[0])
	}
	logEntries := logRecorder.List()
	if len(logEntries) != 1 || logEntries[0].TokenName != wantName {
		t.Fatalf("expected workspace-aware log record, got %#v", logEntries)
	}
	updated, err := manager.Get(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Stats.RequestCount != 1 || updated.Stats.TotalTokens != 9 {
		t.Fatalf("expected usage stats to stay on selected token id, got %#v", updated.Stats)
	}
}

func TestServiceRecordsClientToolInHistoryAndLogs(t *testing.T) {
	var forwardedClientHeader string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forwardedClientHeader = r.Header.Get("X-OmniProxy-Client")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"usage":{"total_tokens":3}}`))
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

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", stringsReader(`{"model":"gpt-test"}`))
	req.Header.Set("X-OmniProxy-Client", "Codex")
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	entries := recorder.List(history.Filter{Client: "codex", Limit: 10})
	if len(entries) != 1 || entries[0].ClientKey != "codex" || entries[0].ClientName != "Codex" {
		t.Fatalf("expected history to record codex client, got %#v", entries)
	}
	if forwardedClientHeader != "" {
		t.Fatalf("client identification header should not be forwarded upstream, got %q", forwardedClientHeader)
	}
	logEntries := logRecorder.List()
	if len(logEntries) != 1 || logEntries[0].ClientKey != "codex" || logEntries[0].ClientName != "Codex" {
		t.Fatalf("expected logs to record codex client, got %#v", logEntries)
	}
}

func TestServiceReportsActiveProxyRequestsByClientAndAccount(t *testing.T) {
	releaseUpstream := make(chan struct{})
	upstreamStarted := make(chan struct{}, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamStarted <- struct{}{}
		<-releaseUpstream
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"usage":{"total_tokens":2}}`))
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(token.UpsertRequest{Name: "primary", Provider: "openai", TokenValue: "sk-primary-token"}); err != nil {
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

	done := make(chan struct{})
	go func() {
		req := httptest.NewRequest(http.MethodPost, "/opencode-router/v1/chat/completions", stringsReader(`{"model":"gpt-test"}`))
		res := httptest.NewRecorder()
		service.ServeHTTP(res, req)
		close(done)
	}()

	select {
	case <-upstreamStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for upstream request")
	}

	active := service.ActiveRequests()
	if len(active) != 1 {
		t.Fatalf("expected one active request, got %#v", active)
	}
	if active[0].ClientKey != "opencode" || active[0].TokenName != "primary" || active[0].Model != "gpt-test" {
		t.Fatalf("unexpected active request metadata: %#v", active[0])
	}

	close(releaseUpstream)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for proxy request to finish")
	}
	if active := service.ActiveRequests(); len(active) != 0 {
		t.Fatalf("expected active requests to clear after completion, got %#v", active)
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
