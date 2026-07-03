package proxy

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"OmniProxyBackend/internal/claudedesktop"
	"OmniProxyBackend/internal/config"
	"OmniProxyBackend/internal/history"
	"OmniProxyBackend/internal/logs"
	"OmniProxyBackend/internal/storage"
	"OmniProxyBackend/internal/token"
	"github.com/gorilla/websocket"
	"github.com/klauspost/compress/zstd"
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

func TestServiceFallsBackToGatewayBackupProviderWhenPrimaryHasNoToken(t *testing.T) {
	var authHeaders []string
	var paths []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeaders = append(authHeaders, r.Header.Get("Authorization"))
		paths = append(paths, r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("backup-ok"))
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(token.UpsertRequest{Name: "deepseek-backup", Provider: token.ProviderDeepSeek, TokenValue: "sk-deepseek-token"}); err != nil {
		t.Fatal(err)
	}
	recorder, err := history.NewRecorder(storage.NewJSONStore[[]history.Entry](filepath.Join(t.TempDir(), "history.json")), 100)
	if err != nil {
		t.Fatal(err)
	}

	service, err := NewService(config.Config{
		ProxyPort:       3000,
		ControlPort:     3890,
		OpenAIBaseURL:   upstream.URL,
		DeepSeekBaseURL: upstream.URL,
		GatewayRoutes: config.GatewayRoutes{
			OpenAI: config.GatewayRouteConfig{
				Provider:       token.ProviderOpenAI,
				CredentialType: token.CredentialTypeAPIKey,
				Model:          "gpt-route",
				Fallbacks: []config.GatewayRouteConfig{
					{Provider: token.ProviderDeepSeek, CredentialType: token.CredentialTypeAPIKey},
				},
			},
		},
		SwitchThreshold: 15,
		MaxRetries:      0,
	}, manager, logs.NewRecorder(10), recorder)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/opencode-router/v1/chat/completions", stringsReader(`{"model":"gpt-5.4","messages":[]}`))
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusOK || res.Body.String() != "backup-ok" {
		t.Fatalf("expected backup provider response, status=%d body=%q", res.Code, res.Body.String())
	}
	if len(authHeaders) != 1 || authHeaders[0] != "Bearer sk-deepseek-token" {
		t.Fatalf("expected DeepSeek backup token auth, got %#v", authHeaders)
	}
	if len(paths) != 1 || paths[0] != "/v1/chat/completions" {
		t.Fatalf("expected OpenAI-compatible backup path, got %#v", paths)
	}
	entries := recorder.List(history.Filter{Limit: 10})
	if len(entries) != 1 {
		t.Fatalf("expected one history entry, got %#v", entries)
	}
	if entries[0].Provider != token.ProviderDeepSeek {
		t.Fatalf("expected history to record backup provider, got %#v", entries[0])
	}
	if len(entries[0].RetryChain) != 2 || entries[0].RetryChain[0].Provider != token.ProviderOpenAI || entries[0].RetryChain[1].Provider != token.ProviderDeepSeek {
		t.Fatalf("expected retry chain to include primary miss and backup success, got %#v", entries[0].RetryChain)
	}
}

func TestServiceRoutesConfiguredProvidersThroughOutboundProxy(t *testing.T) {
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
		if r.URL == nil || r.URL.Scheme != "http" || r.URL.Host != strings.TrimPrefix(upstream.URL, "http://") {
			t.Fatalf("expected absolute upstream URL through proxy, got %q", gotURL)
		}
		_, _ = w.Write([]byte("proxied"))
	}))
	defer proxyServer.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(token.UpsertRequest{Name: "primary", Provider: "openai", TokenValue: "sk-primary-token"}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(token.UpsertRequest{Name: "deepseek", Provider: token.ProviderDeepSeek, TokenValue: "sk-deepseek-token"}); err != nil {
		t.Fatal(err)
	}

	service, err := NewService(config.Config{
		ProxyPort:              3000,
		ControlPort:            3890,
		OpenAIBaseURL:          upstream.URL,
		DeepSeekBaseURL:        upstream.URL,
		OutboundProxyEnabled:   true,
		OutboundProxyURL:       proxyServer.URL,
		OutboundProxyProviders: []string{token.ProviderOpenAI},
		SwitchThreshold:        15,
		MaxRetries:             0,
	}, manager, logs.NewRecorder(10))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", stringsReader(`{"model":"gpt-5.4","messages":[]}`))
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)
	if res.Code != http.StatusOK || res.Body.String() != "proxied" {
		t.Fatalf("expected proxied response, status=%d body=%q", res.Code, res.Body.String())
	}
	if proxyHits != 1 || upstreamHits != 0 {
		t.Fatalf("expected only proxy hit for matched model, proxy=%d upstream=%d", proxyHits, upstreamHits)
	}

	req = httptest.NewRequest(http.MethodPost, "/deepseek/v1/chat/completions", stringsReader(`{"model":"deepseek-v4-pro","messages":[]}`))
	res = httptest.NewRecorder()
	service.ServeHTTP(res, req)
	if res.Code != http.StatusOK || res.Body.String() != "direct" {
		t.Fatalf("expected direct response, status=%d body=%q", res.Code, res.Body.String())
	}
	if proxyHits != 1 || upstreamHits != 1 {
		t.Fatalf("expected unselected provider to bypass proxy, proxy=%d upstream=%d", proxyHits, upstreamHits)
	}
}

func TestServiceRoutesCodexModelListThroughOutboundProxy(t *testing.T) {
	upstreamHits := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits++
		_, _ = w.Write([]byte("direct-models"))
	}))
	defer upstream.Close()

	proxyHits := 0
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyHits++
		gotURL := ""
		if r.URL != nil {
			gotURL = r.URL.String()
		}
		if r.URL == nil || r.URL.Scheme != "http" || r.URL.Host != strings.TrimPrefix(upstream.URL, "http://") || r.URL.Path != "/backend-api/codex/models" {
			t.Fatalf("expected Codex models absolute upstream URL through proxy, got %q", gotURL)
		}
		if got := r.URL.Query().Get("client_version"); got != "0.132.0" {
			t.Fatalf("expected client_version query to be preserved, got %q", got)
		}
		_, _ = w.Write([]byte("proxied-models"))
	}))
	defer proxyServer.Close()

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
		ProxyPort:              3000,
		ControlPort:            3890,
		CodexBaseURL:           upstream.URL + "/backend-api/codex",
		OutboundProxyEnabled:   true,
		OutboundProxyURL:       proxyServer.URL,
		OutboundProxyProviders: []string{token.ProviderOpenAI},
		SwitchThreshold:        15,
		MaxRetries:             0,
	}, manager, logs.NewRecorder(10))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/backend-api/codex/models?client_version=0.132.0", nil)
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusOK || res.Body.String() != "proxied-models" {
		t.Fatalf("expected proxied model-list response, status=%d body=%q", res.Code, res.Body.String())
	}
	if proxyHits != 1 || upstreamHits != 0 {
		t.Fatalf("expected only proxy hit for Codex model list, proxy=%d upstream=%d", proxyHits, upstreamHits)
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

func TestServiceClearsStaleIncomingCodexAccountID(t *testing.T) {
	var gotAuth string
	var gotAccount string

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAccount = r.Header.Get("ChatGPT-Account-Id")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`))
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.Add(token.UpsertRequest{
		Provider:       token.ProviderOpenAI,
		CredentialType: token.CredentialTypeCodexAuthJSON,
		TokenValue:     codexAuthJSONForServiceTestWithCredentials(t, "selected@example.com", "", "selected-access-token"),
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
	req.Header.Set("ChatGPT-Account-Id", "last-login-account")
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	if gotAuth != "Bearer selected-access-token" {
		t.Fatalf("expected selected account auth, got %q", gotAuth)
	}
	if gotAccount != "" {
		t.Fatalf("expected stale incoming account id to be cleared, got %q", gotAccount)
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
		origin  string
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
			origin:  r.Header.Get("Origin"),
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
		"Origin":             []string{"http://localhost:5173"},
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
	if got.origin != "" {
		t.Fatalf("expected local browser origin not to be forwarded upstream, got %q", got.origin)
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

func TestServiceRejectsCodexResponsesWebSocketFromNonLocalOrigin(t *testing.T) {
	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "ws-origin-tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(config.Config{
		ProxyPort:          3000,
		ControlPort:        3890,
		CodexBaseURL:       "https://chatgpt.com/backend-api/codex",
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
	conn, resp, err := websocket.DefaultDialer.Dial(dialURL, http.Header{
		"Origin": []string{"https://evil.example"},
	})
	if conn != nil {
		_ = conn.Close()
	}
	if err == nil {
		t.Fatal("expected websocket dial to fail for non-local origin")
	}
	if resp == nil {
		t.Fatal("expected handshake response")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", resp.StatusCode)
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

func TestServicePrefersConfiguredXiaomiCredentialType(t *testing.T) {
	tests := []struct {
		name         string
		priority     string
		addOrder     []string
		expectedPath string
		expectedKey  string
	}{
		{
			name:         "token plan priority overrides queue order",
			priority:     config.MimoCredentialPriorityTokenPlan,
			addOrder:     []string{token.CredentialTypeMimoTokenPlan, token.CredentialTypeAPIKey},
			expectedPath: "/token-plan/anthropic/v1/messages",
			expectedKey:  "tp-token-plan-secret",
		},
		{
			name:         "api priority overrides queue order",
			priority:     config.MimoCredentialPriorityAPIKey,
			addOrder:     []string{token.CredentialTypeAPIKey, token.CredentialTypeMimoTokenPlan},
			expectedPath: "/anthropic/v1/messages",
			expectedKey:  "sk-paygo-secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath string
			var gotKey string
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotKey = r.Header.Get("Api-Key")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`))
			}))
			defer upstream.Close()

			manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
			if err != nil {
				t.Fatal(err)
			}
			for _, credentialType := range tt.addOrder {
				req := token.UpsertRequest{
					Provider:       token.ProviderXiaomi,
					CredentialType: credentialType,
				}
				if credentialType == token.CredentialTypeMimoTokenPlan {
					req.Name = "token-plan"
					req.TokenValue = "tp-token-plan-secret"
				} else {
					req.Name = "paygo"
					req.TokenValue = "sk-paygo-secret"
				}
				if _, err := manager.Add(req); err != nil {
					t.Fatal(err)
				}
			}

			service, err := NewService(config.Config{
				ProxyPort:                       3000,
				ControlPort:                     3890,
				XiaomiAPIBaseURL:                upstream.URL + "/v1",
				XiaomiAPIAnthropicBaseURL:       upstream.URL + "/anthropic",
				XiaomiTokenPlanBaseURL:          upstream.URL + "/token-plan/v1",
				XiaomiTokenPlanAnthropicBaseURL: upstream.URL + "/token-plan/anthropic",
				XiaomiCredentialPriority:        tt.priority,
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
			res := httptest.NewRecorder()
			service.ServeHTTP(res, req)
			if res.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
			}
			if gotPath != tt.expectedPath || gotKey != tt.expectedKey {
				t.Fatalf("unexpected preferred MiMo route path=%q key=%q", gotPath, gotKey)
			}
		})
	}
}

func TestServiceFallsBackBetweenMimoCredentialTypesOnQuotaResponse(t *testing.T) {
	tests := []struct {
		name         string
		priority     string
		expectedPath []string
		expectedKey  []string
		firstToken   string
		secondToken  string
	}{
		{
			name:         "token plan priority falls back to pay-as-you-go api",
			priority:     config.MimoCredentialPriorityTokenPlan,
			expectedPath: []string{"/token-plan/anthropic/v1/messages", "/anthropic/v1/messages"},
			expectedKey:  []string{"tp-token-plan-secret", "sk-paygo-secret"},
			firstToken:   "token-plan",
			secondToken:  "paygo",
		},
		{
			name:         "api priority falls back to token plan",
			priority:     config.MimoCredentialPriorityAPIKey,
			expectedPath: []string{"/anthropic/v1/messages", "/token-plan/anthropic/v1/messages"},
			expectedKey:  []string{"sk-paygo-secret", "tp-token-plan-secret"},
			firstToken:   "paygo",
			secondToken:  "token-plan",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPaths []string
			var gotKeys []string
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPaths = append(gotPaths, r.URL.Path)
				gotKeys = append(gotKeys, r.Header.Get("Api-Key"))
				if len(gotPaths) == 1 {
					w.WriteHeader(http.StatusPaymentRequired)
					_, _ = w.Write([]byte(`{"error":{"message":"quota exhausted"}}`))
					return
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`))
			}))
			defer upstream.Close()

			manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
			if err != nil {
				t.Fatal(err)
			}
			tokenPlan, err := manager.Add(token.UpsertRequest{
				Name:           "token-plan",
				Provider:       token.ProviderXiaomi,
				CredentialType: token.CredentialTypeMimoTokenPlan,
				TokenValue:     "tp-token-plan-secret",
			})
			if err != nil {
				t.Fatal(err)
			}
			paygo, err := manager.Add(token.UpsertRequest{
				Name:           "paygo",
				Provider:       token.ProviderXiaomi,
				CredentialType: token.CredentialTypeAPIKey,
				TokenValue:     "sk-paygo-secret",
			})
			if err != nil {
				t.Fatal(err)
			}
			recorder, err := history.NewRecorder(storage.NewJSONStore[[]history.Entry](filepath.Join(t.TempDir(), "history.json")), 100)
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
				XiaomiCredentialPriority:        tt.priority,
				GatewayRoutes: config.GatewayRoutes{
					Claude: config.GatewayRouteConfig{Provider: token.ProviderXiaomi},
				},
				SwitchThreshold: 15,
				MaxRetries:      0,
			}, manager, logs.NewRecorder(10), recorder)
			if err != nil {
				t.Fatal(err)
			}

			req := httptest.NewRequest(http.MethodPost, "/anthropic-router/v1/messages", stringsReader(`{"model":"mimo-v2.5","messages":[]}`))
			res := httptest.NewRecorder()
			service.ServeHTTP(res, req)
			if res.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
			}
			if strings.Join(gotPaths, ",") != strings.Join(tt.expectedPath, ",") || strings.Join(gotKeys, ",") != strings.Join(tt.expectedKey, ",") {
				t.Fatalf("unexpected MiMo fallback route paths=%#v keys=%#v", gotPaths, gotKeys)
			}

			firstID := tokenPlan.ID
			secondID := paygo.ID
			if tt.firstToken == "paygo" {
				firstID = paygo.ID
				secondID = tokenPlan.ID
			}
			firstState, err := manager.Get(firstID)
			if err != nil {
				t.Fatal(err)
			}
			if firstState.Status != token.StatusExhausted || firstState.CooldownUntil == nil || !firstState.CooldownUntil.After(time.Now()) {
				t.Fatalf("expected first MiMo credential to enter cooldown, got %#v", firstState)
			}
			secondState, err := manager.Get(secondID)
			if err != nil {
				t.Fatal(err)
			}
			if secondState.Status != token.StatusActive {
				t.Fatalf("expected fallback MiMo credential to stay active, got %s", secondState.Status)
			}
			entries := recorder.List(history.Filter{Limit: 10})
			if len(entries) != 1 || len(entries[0].RetryChain) != 2 {
				t.Fatalf("expected one history entry with two MiMo attempts, got %#v", entries)
			}
			if entries[0].RetryChain[0].Status != http.StatusPaymentRequired || !entries[0].RetryChain[0].CooldownTriggered {
				t.Fatalf("expected first MiMo attempt to record quota cooldown, got %#v", entries[0].RetryChain[0])
			}
			if entries[0].RetryChain[1].TokenName != tt.secondToken || entries[0].RetryChain[1].Status != http.StatusOK {
				t.Fatalf("expected second MiMo attempt to use fallback credential, got %#v", entries[0].RetryChain[1])
			}
		})
	}
}

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

func TestServiceRoutesNewAPIGeminiRequests(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodPost, "/newapi/gemini/v1beta/models/gemini-3-pro-preview:generateContent", stringsReader(`{"contents":[]}`))
	req.Header.Set("Authorization", "Bearer caller")
	req.Header.Set("X-Goog-Api-Key", "caller")
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	if upstreamPath != "/v1beta/models/gemini-3-pro-preview:generateContent" || geminiKey != "newapi-api-key-token" || authorization != "" {
		t.Fatalf("unexpected new-api gemini route path=%q key=%q authorization=%q", upstreamPath, geminiKey, authorization)
	}
}

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

func TestParseTokenConsumptionFromSSE(t *testing.T) {
	body := []byte(strings.Join([]string{
		`event: response.output_text.delta`,
		`data: {"type":"response.output_text.delta","delta":"hello"}`,
		``,
		`event: response.completed`,
		`data: {"type":"response.completed","response":{"model":"gpt-5.5","usage":{"input_tokens":20,"output_tokens":8,"total_tokens":28,"input_tokens_details":{"cached_tokens":12},"cache_creation_input_tokens":3}}}`,
		``,
		`data: [DONE]`,
	}, "\n"))

	usage := parseTokenConsumption(http.Header{"Content-Type": []string{"text/event-stream"}}, body)
	if usage.TotalTokens != 28 || usage.InputTokens != 20 || usage.OutputTokens != 8 {
		t.Fatalf("unexpected usage: %#v", usage)
	}
	if usage.CacheReadTokens != 12 || usage.CacheCreationTokens != 3 {
		t.Fatalf("unexpected cache usage: %#v", usage)
	}
	if model := parseResponseModel(http.Header{"Content-Type": []string{"text/event-stream"}}, body); model != "gpt-5.5" {
		t.Fatalf("expected response model gpt-5.5, got %q", model)
	}
}

func stringsReader(value string) io.Reader {
	return strings.NewReader(value)
}

func TestReadProxyRequestBodyDecodesZstd(t *testing.T) {
	var compressed bytes.Buffer
	encoder, err := zstd.NewWriter(&compressed)
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"model":"gpt-5.5","input":"hi"}`)
	if _, err := encoder.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := encoder.Close(); err != nil {
		t.Fatal(err)
	}

	body, decoded, err := readProxyRequestBody(io.NopCloser(bytes.NewReader(compressed.Bytes())), "zstd")
	if err != nil {
		t.Fatal(err)
	}
	if !decoded {
		t.Fatal("expected zstd request body to be decoded")
	}
	if !bytes.Equal(body, raw) {
		t.Fatalf("unexpected decoded body: %q", string(body))
	}
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
