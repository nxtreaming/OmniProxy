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

func TestServicePreservesRetryableForgeResponseWhenNoNextToken(t *testing.T) {
	const upstreamBody = `{"error":{"message":"context length exceeded: sk-upstream-secret-12345678"}}`
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("expected Forge Responses path, got %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer fg-forge-api-key-token" {
			t.Fatalf("expected Forge bearer auth, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(upstreamBody))
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	forgeToken, err := manager.Add(token.UpsertRequest{
		Name:       "forge-primary",
		Provider:   token.ProviderForge,
		TokenValue: "fg-forge-api-key-token",
	})
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
		ForgeBaseURL:    upstream.URL + "/v1",
		SwitchThreshold: 15,
		MaxRetries:      1,
	}, manager, logs.NewRecorder(10), recorder)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/forge/v1/responses", stringsReader(`{"model":"gpt-5.6-sol","input":"hi"}`))
	res := httptest.NewRecorder()
	service.ServeHTTP(res, req)

	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected upstream status 503, got %d body=%q", res.Code, res.Body.String())
	}
	if got := res.Body.String(); got != upstreamBody {
		t.Fatalf("expected original upstream body, got %q", got)
	}
	if got := res.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected upstream content type, got %q", got)
	}
	if upstreamCalls != 1 {
		t.Fatalf("expected one upstream call with no duplicate token, got %d", upstreamCalls)
	}

	updated, err := manager.Get(forgeToken.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != token.StatusExhausted || updated.CooldownUntil == nil || !updated.CooldownUntil.After(time.Now()) {
		t.Fatalf("expected Forge token to enter transient cooldown, got %#v", updated)
	}
	entries := recorder.List(history.Filter{Limit: 10})
	if len(entries) != 1 {
		t.Fatalf("expected one history entry, got %#v", entries)
	}
	if entries[0].Status != http.StatusServiceUnavailable || !strings.Contains(entries[0].Message, "context length exceeded") {
		t.Fatalf("expected preserved upstream error summary, got %#v", entries[0])
	}
	if strings.Contains(entries[0].Message, "sk-upstream-secret-12345678") || !strings.Contains(entries[0].Message, "sk-***") {
		t.Fatalf("expected history error summary to redact credentials, got %q", entries[0].Message)
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

func TestServiceFallsBackToGatewayBackupProviderAfterRetryablePrimaryResponse(t *testing.T) {
	var authHeaders []string
	var providers []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		authHeaders = append(authHeaders, auth)
		if auth == "Bearer sk-openai-token" {
			providers = append(providers, token.ProviderOpenAI)
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"message":"primary rate limited"}}`))
			return
		}
		providers = append(providers, token.ProviderDeepSeek)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("backup-ok"))
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(token.UpsertRequest{Name: "openai-primary", Provider: token.ProviderOpenAI, TokenValue: "sk-openai-token"}); err != nil {
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
					{Provider: token.ProviderDeepSeek, CredentialType: token.CredentialTypeAPIKey, Model: "deepseek-route"},
				},
			},
		},
		SwitchThreshold: 15,
		MaxRetries:      2,
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
	if got := strings.Join(authHeaders, ","); got != "Bearer sk-openai-token,Bearer sk-deepseek-token" {
		t.Fatalf("expected primary then fallback auth headers, got %#v", authHeaders)
	}
	if got := strings.Join(providers, ","); got != token.ProviderOpenAI+","+token.ProviderDeepSeek {
		t.Fatalf("expected primary then fallback providers, got %#v", providers)
	}
	entries := recorder.List(history.Filter{Limit: 10})
	if len(entries) != 1 {
		t.Fatalf("expected one history entry, got %#v", entries)
	}
	if entries[0].Provider != token.ProviderDeepSeek || entries[0].Status != http.StatusOK {
		t.Fatalf("expected history to record fallback success, got %#v", entries[0])
	}
	if len(entries[0].RetryChain) != 3 {
		t.Fatalf("expected three retry chain entries, got %#v", entries[0].RetryChain)
	}
	if entries[0].RetryChain[0].Provider != token.ProviderOpenAI || entries[0].RetryChain[0].Status != http.StatusTooManyRequests {
		t.Fatalf("expected primary 429 attempt first, got %#v", entries[0].RetryChain[0])
	}
	if entries[0].RetryChain[1].Provider != token.ProviderOpenAI || entries[0].RetryChain[1].Status != 0 {
		t.Fatalf("expected primary miss before fallback, got %#v", entries[0].RetryChain[1])
	}
	if entries[0].RetryChain[2].Provider != token.ProviderDeepSeek || entries[0].RetryChain[2].Status != http.StatusOK {
		t.Fatalf("expected fallback success attempt third, got %#v", entries[0].RetryChain[2])
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
