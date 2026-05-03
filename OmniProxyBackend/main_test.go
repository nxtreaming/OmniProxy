package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"OmniProxyBackend/internal/config"
	"OmniProxyBackend/internal/history"
	"OmniProxyBackend/internal/logs"
	"OmniProxyBackend/internal/storage"
	"OmniProxyBackend/internal/token"
)

func TestTokenValidationMarksInvalidToken(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("unexpected validation path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	item, err := manager.Add(token.UpsertRequest{Name: "bad", Provider: "openai", TokenValue: "sk-invalid-token"})
	if err != nil {
		t.Fatal(err)
	}
	historyRecorder, err := history.NewRecorder(storage.NewJSONStore[[]history.Entry](filepath.Join(t.TempDir(), "history.json")), 100)
	if err != nil {
		t.Fatal(err)
	}

	app := &appServer{
		cfg: config.Config{
			ProxyPort:       3000,
			ControlPort:     3890,
			UpstreamBaseURL: upstream.URL,
			SwitchThreshold: 15,
			MaxRetries:      1,
		},
		tokens:  manager,
		logs:    logs.NewRecorder(10),
		history: historyRecorder,
	}

	req := httptest.NewRequest(http.MethodPost, "/api/tokens/"+item.ID+"/validate", nil)
	res := httptest.NewRecorder()
	app.routes().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}

	updated, err := manager.Get(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != token.StatusInvalid {
		t.Fatalf("expected invalid token status, got %s", updated.Status)
	}
	entries := historyRecorder.List(history.Filter{Limit: 10})
	if len(entries) != 1 {
		t.Fatalf("expected validation history entry, got %#v", entries)
	}
	if entries[0].Path != "/maintenance/token-validation" || entries[0].TokenName != "bad" || entries[0].Status != http.StatusUnauthorized {
		t.Fatalf("unexpected validation history entry: %#v", entries[0])
	}
}

func TestTokenListDoesNotExposeTokenValue(t *testing.T) {
	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(token.UpsertRequest{Name: "primary", Provider: "openai", TokenValue: "sk-primary-token"}); err != nil {
		t.Fatal(err)
	}
	app := &appServer{
		cfg:    config.Default(),
		tokens: manager,
		logs:   logs.NewRecorder(10),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/tokens", nil)
	res := httptest.NewRecorder()
	app.routes().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	if strings.Contains(res.Body.String(), "tokenValue") || strings.Contains(res.Body.String(), "sk-primary-token") {
		t.Fatalf("token list leaked secret: %s", res.Body.String())
	}
	var payload []struct {
		HasTokenValue    bool   `json:"hasTokenValue"`
		MaskedTokenValue string `json:"maskedTokenValue"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload) != 1 || !payload[0].HasTokenValue || payload[0].MaskedTokenValue != "sk-prim...oken" {
		t.Fatalf("unexpected sanitized token payload: %#v", payload)
	}
}

func TestTokenDisabledEndpointTogglesAccount(t *testing.T) {
	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	item, err := manager.Add(token.UpsertRequest{Name: "primary", Provider: "openai", TokenValue: "sk-primary-token"})
	if err != nil {
		t.Fatal(err)
	}
	app := &appServer{
		cfg:    config.Default(),
		tokens: manager,
		logs:   logs.NewRecorder(10),
	}

	req := httptest.NewRequest(http.MethodPut, "/api/tokens/"+item.ID+"/disabled", strings.NewReader(`{"disabled":true}`))
	res := httptest.NewRecorder()
	app.routes().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	var payload struct {
		Disabled bool `json:"disabled"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.Disabled {
		t.Fatal("expected disabled response")
	}
	if selected, err := manager.Acquire(token.ProviderOpenAI, nil); err != token.ErrNoActiveToken {
		t.Fatalf("expected disabled token to be unavailable, got selected=%#v err=%v", selected, err)
	}
}

func TestControlCORSRejectsNonLocalOrigin(t *testing.T) {
	handler := withCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/tokens", nil)
	req.Header.Set("Origin", "https://evil.example")
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", res.Code)
	}
}

func TestControlCORSAllowsControlTokenHeader(t *testing.T) {
	handler := withCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodOptions, "/api/tokens", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", res.Code)
	}
	if !strings.Contains(res.Header().Get("Access-Control-Allow-Headers"), controlTokenHeader) {
		t.Fatalf("expected CORS headers to allow %s, got %q", controlTokenHeader, res.Header().Get("Access-Control-Allow-Headers"))
	}
}

func TestControlAPIRequiresControlTokenWhenConfigured(t *testing.T) {
	app := &appServer{
		cfg:          config.Default(),
		logs:         logs.NewRecorder(10),
		controlToken: "test-control-token",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/logs", nil)
	res := httptest.NewRecorder()
	app.routes().ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 without token, got %d", res.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/logs", nil)
	req.Header.Set(controlTokenHeader, "test-control-token")
	res = httptest.NewRecorder()
	app.routes().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200 with token header, got %d body=%s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/logs", nil)
	req.Header.Set("Authorization", "Bearer test-control-token")
	res = httptest.NewRecorder()
	app.routes().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200 with bearer token, got %d body=%s", res.Code, res.Body.String())
	}
}

func TestControlTokenEndpointRequiresTrustedDesktopOrigin(t *testing.T) {
	app := &appServer{
		cfg:          config.Default(),
		logs:         logs.NewRecorder(10),
		controlToken: "test-control-token",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/control-token", nil)
	res := httptest.NewRecorder()
	app.routes().ServeHTTP(res, req)
	if res.Code != http.StatusForbidden {
		t.Fatalf("expected status 403 without trusted origin, got %d body=%s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/control-token", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	res = httptest.NewRecorder()
	app.routes().ServeHTTP(res, req)
	if res.Code != http.StatusForbidden {
		t.Fatalf("expected status 403 for local browser origin, got %d body=%s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/control-token", nil)
	req.Header.Set("Origin", "wails://wails.localhost")
	res = httptest.NewRecorder()
	app.routes().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	if res.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("expected no-store cache header, got %q", res.Header().Get("Cache-Control"))
	}
	var payload struct {
		Header string `json:"header"`
		Token  string `json:"token"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Header != controlTokenHeader || payload.Token != "test-control-token" {
		t.Fatalf("unexpected control token payload: %#v", payload)
	}
}

func TestDataDirectoryEndpointReturnsCurrentInfo(t *testing.T) {
	dataDir := t.TempDir()
	app := &appServer{
		dataDir: dataDir,
		cfg:     config.Default(),
		logs:    logs.NewRecorder(10),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/data-directory", nil)
	res := httptest.NewRecorder()
	app.routes().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	var payload config.DataDirectoryInfo
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(filepath.Clean(payload.DataDir), filepath.Clean(dataDir)) {
		t.Fatalf("expected data dir %q, got %#v", dataDir, payload)
	}
}

func TestAppInfoEndpointReturnsVersionMetadata(t *testing.T) {
	restore := overrideUpdateGlobals("v1.2.3", "https://example.com/releases/latest")
	defer restore()

	app := &appServer{
		cfg:  config.Default(),
		logs: logs.NewRecorder(10),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/app/info", nil)
	res := httptest.NewRecorder()
	app.routes().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	var payload appInfo
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Name != "OmniProxy" || payload.Version != "v1.2.3" || payload.IsDevelopment {
		t.Fatalf("unexpected app info: %#v", payload)
	}
	if payload.UpdateEndpoint != "https://example.com/releases/latest" || payload.Platform == "" || payload.GoVersion == "" || payload.StartedAt == "" {
		t.Fatalf("missing app metadata: %#v", payload)
	}
}

func TestRuntimeModeControlsSingleInstanceID(t *testing.T) {
	restore := overrideUpdateGlobals("v1.2.3", "")
	wantReleaseMode := "production"
	if appInstanceMode == "dev" {
		wantReleaseMode = "dev"
	}
	if got := appRuntimeMode(); got != wantReleaseMode {
		restore()
		t.Fatalf("expected %s runtime mode for release version, got %q", wantReleaseMode, got)
	}
	if got := singleInstanceUniqueID(); got != singleInstanceIDBase+"."+wantReleaseMode {
		restore()
		t.Fatalf("expected %s single instance ID, got %q", wantReleaseMode, got)
	}
	restore()

	restore = overrideUpdateGlobals("dev", "")
	defer restore()
	if got := appRuntimeMode(); got != "dev" {
		t.Fatalf("expected dev runtime mode for dev version, got %q", got)
	}
	if got := singleInstanceUniqueID(); got != singleInstanceIDBase+".dev" {
		t.Fatalf("expected dev single instance ID, got %q", got)
	}
	if got := appDisplayName(); got != "OmniProxy Dev" {
		t.Fatalf("expected dev display name, got %q", got)
	}
}

func TestAddCodexTokenRefreshesUsage(t *testing.T) {
	var validationCalled bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		validationCalled = true
		if got := r.Header.Get("Authorization"); got != "Bearer codex-access-token" {
			t.Fatalf("unexpected auth header: %q", got)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"plan_type": "team",
			"rate_limit": {
				"primary_window": {"used_percent": 23, "reset_at": 1777299888},
				"secondary_window": {"used_percent": 41, "reset_at": 1777798105}
			}
		}`))
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	app := &appServer{
		cfg: config.Config{
			ProxyPort:          3000,
			ControlPort:        3890,
			UpstreamBaseURL:    "https://api.openai.com",
			SwitchThreshold:    15,
			MaxRetries:         1,
			CodexUsageEndpoint: upstream.URL,
		},
		tokens: manager,
		logs:   logs.NewRecorder(10),
	}

	payload, err := json.Marshal(token.UpsertRequest{
		Provider:       token.ProviderOpenAI,
		CredentialType: token.CredentialTypeCodexAuthJSON,
		TokenValue:     codexAuthJSONForMainTest(t, "coder@example.com"),
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/tokens", strings.NewReader(string(payload)))
	res := httptest.NewRecorder()
	app.routes().ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d body=%s", res.Code, res.Body.String())
	}
	if !validationCalled {
		t.Fatal("expected codex usage validation to be called after add")
	}

	items := manager.List()
	if len(items) != 1 {
		t.Fatalf("expected 1 token, got %d", len(items))
	}
	if items[0].Remaining != 77 || items[0].Usage.PrimaryRemainingPercent != 77 || items[0].Usage.SecondaryRemainingPercent != 59 {
		t.Fatalf("expected usage to be refreshed after add, got remaining=%d usage=%#v", items[0].Remaining, items[0].Usage)
	}
}

func TestStartupRefreshesCodexUsage(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"plan_type": "team",
			"rate_limit": {
				"primary_window": {"used_percent": 35, "reset_at": 1777299888},
				"secondary_window": {"used_percent": 12, "reset_at": 1777798105}
			}
		}`))
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	item, err := manager.Add(token.UpsertRequest{
		Provider:       token.ProviderOpenAI,
		CredentialType: token.CredentialTypeCodexAuthJSON,
		TokenValue:     codexAuthJSONForMainTest(t, "startup@example.com"),
	})
	if err != nil {
		t.Fatal(err)
	}
	app := &appServer{
		cfg: config.Config{
			ProxyPort:          3000,
			ControlPort:        3890,
			UpstreamBaseURL:    "https://api.openai.com",
			SwitchThreshold:    15,
			MaxRetries:         1,
			CodexUsageEndpoint: upstream.URL,
		},
		tokens: manager,
		logs:   logs.NewRecorder(10),
	}

	app.refreshCodexUsageOnStartup(context.Background())

	updated, err := manager.Get(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Remaining != 65 || updated.Usage.PrimaryRemainingPercent != 65 || updated.Usage.SecondaryRemainingPercent != 88 {
		t.Fatalf("expected startup usage refresh, got remaining=%d usage=%#v", updated.Remaining, updated.Usage)
	}
}

func TestCurrentTokenQuotaRefreshUsesLatestProxyUsage(t *testing.T) {
	var seenAuth []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = append(seenAuth, r.Header.Get("Authorization"))
		switch r.Header.Get("Authorization") {
		case "Bearer sk-current-token":
			w.Header().Set("x-ratelimit-remaining-tokens", "64")
		case "Bearer sk-older-token":
			w.Header().Set("x-ratelimit-remaining-tokens", "12")
		default:
			t.Fatalf("unexpected auth header: %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer upstream.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	older, err := manager.Add(token.UpsertRequest{Name: "older", Provider: token.ProviderOpenAI, TokenValue: "sk-older-token"})
	if err != nil {
		t.Fatal(err)
	}
	current, err := manager.Add(token.UpsertRequest{Name: "current", Provider: token.ProviderOpenAI, TokenValue: "sk-current-token"})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.RecordProxyUsage(older.ID, token.TokenConsumption{TotalTokens: 1}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
	if err := manager.RecordProxyUsage(current.ID, token.TokenConsumption{TotalTokens: 1}); err != nil {
		t.Fatal(err)
	}
	historyRecorder, err := history.NewRecorder(storage.NewJSONStore[[]history.Entry](filepath.Join(t.TempDir(), "history.json")), 100)
	if err != nil {
		t.Fatal(err)
	}

	app := &appServer{
		cfg: config.Config{
			ProxyPort:       3000,
			ControlPort:     3890,
			OpenAIBaseURL:   upstream.URL,
			SwitchThreshold: 15,
			MaxRetries:      1,
		},
		tokens:  manager,
		logs:    logs.NewRecorder(10),
		history: historyRecorder,
	}

	app.refreshCurrentTokenUsage(context.Background())

	if len(seenAuth) != 1 || seenAuth[0] != "Bearer sk-current-token" {
		t.Fatalf("expected only current token to be refreshed, got %#v", seenAuth)
	}
	updatedCurrent, err := manager.Get(current.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updatedCurrent.Remaining != 64 || updatedCurrent.Usage.APIRemaining != 64 {
		t.Fatalf("expected current token quota to refresh, got remaining=%d usage=%#v", updatedCurrent.Remaining, updatedCurrent.Usage)
	}
	updatedOlder, err := manager.Get(older.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updatedOlder.Remaining != 100 || updatedOlder.Usage.APIRemaining != 0 {
		t.Fatalf("older token should not be refreshed, got remaining=%d usage=%#v", updatedOlder.Remaining, updatedOlder.Usage)
	}
	entries := historyRecorder.List(history.Filter{Limit: 10})
	if len(entries) != 1 {
		t.Fatalf("expected quota refresh history entry, got %#v", entries)
	}
	if entries[0].Path != "/maintenance/current-token-quota-refresh" || entries[0].Protocol != "quota-refresh" || entries[0].TokenName != "current" || entries[0].Status != http.StatusOK {
		t.Fatalf("unexpected quota refresh history entry: %#v", entries[0])
	}
	if !strings.Contains(entries[0].Message, "remaining=64%") {
		t.Fatalf("expected remaining quota in history message, got %q", entries[0].Message)
	}
}

func TestCurrentQuotaRefreshCandidateSkipsValidationOnlyUsage(t *testing.T) {
	now := time.Now()
	validationOnly := token.Token{
		ID:         "validation-only",
		TokenValue: "sk-validation-only",
		LastUsedAt: &now,
		Status:     token.StatusActive,
	}
	if selected, ok := currentQuotaRefreshCandidate([]token.Token{validationOnly}, now); ok {
		t.Fatalf("expected validation-only token to be skipped, got %#v", selected)
	}
}

func TestCurrentQuotaRefreshCandidateSkipsBackoff(t *testing.T) {
	now := time.Now()
	nextCheckAt := now.Add(time.Minute)
	usedAt := now.Add(-time.Second)
	item := token.Token{
		ID:         "backoff",
		TokenValue: "sk-backoff-token",
		Status:     token.StatusActive,
		Health: token.HealthInfo{
			NextCheckAt: &nextCheckAt,
		},
		Stats: token.TokenStats{
			UpdatedAt: &usedAt,
		},
	}
	if selected, ok := currentQuotaRefreshCandidate([]token.Token{item}, now); ok {
		t.Fatalf("expected backoff token to be skipped, got %#v", selected)
	}
}

func TestConfigureCodexSyncsExistingAuthJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	codexDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codexDir, "auth.json"), []byte(codexAuthJSONForMainTestWithCredentials(t, "coder@example.com", "new-account", "new-access-token")), 0o600); err != nil {
		t.Fatal(err)
	}

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	item, err := manager.Add(token.UpsertRequest{
		Provider:       token.ProviderOpenAI,
		CredentialType: token.CredentialTypeCodexAuthJSON,
		TokenValue:     codexAuthJSONForMainTestWithCredentials(t, "coder@example.com", "old-account", "old-access-token"),
	})
	if err != nil {
		t.Fatal(err)
	}

	app := &appServer{
		cfg: config.Config{
			ProxyPort:       3000,
			ControlPort:     3890,
			UpstreamBaseURL: "https://api.openai.com",
			SwitchThreshold: 15,
			MaxRetries:      1,
		},
		tokens: manager,
		logs:   logs.NewRecorder(10),
	}

	result, err := app.configureCodex()
	if err != nil {
		t.Fatal(err)
	}
	if !result.AuthUpdated || result.ImportedAuth {
		t.Fatalf("expected existing auth to be synced, got %#v", result)
	}

	updated, err := manager.Get(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(updated.TokenValue, "new-access-token") || !strings.Contains(updated.TokenValue, "new-account") {
		t.Fatalf("expected stored auth.json to be refreshed, got %s", updated.TokenValue)
	}
}

func TestConfigureCodexReportsAlreadyImportedAuthJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	codexDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		t.Fatal(err)
	}
	authValue := codexAuthJSONForMainTestWithCredentials(t, "coder@example.com", "same-account", "same-access-token")
	if err := os.WriteFile(filepath.Join(codexDir, "auth.json"), []byte(authValue), 0o600); err != nil {
		t.Fatal(err)
	}

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(token.UpsertRequest{
		Provider:       token.ProviderOpenAI,
		CredentialType: token.CredentialTypeCodexAuthJSON,
		TokenValue:     authValue,
	}); err != nil {
		t.Fatal(err)
	}

	app := &appServer{
		cfg: config.Config{
			ProxyPort:       3000,
			ControlPort:     3890,
			UpstreamBaseURL: "https://api.openai.com",
			SwitchThreshold: 15,
			MaxRetries:      1,
		},
		tokens: manager,
		logs:   logs.NewRecorder(10),
	}

	result, err := app.configureCodex()
	if err != nil {
		t.Fatal(err)
	}
	if !result.AuthAlreadyAdded || result.AuthUpdated || result.ImportedAuth {
		t.Fatalf("expected auth to be reported as already imported, got %#v", result)
	}
	if !strings.Contains(result.Message, "auth.json 账号已存在") {
		t.Fatalf("expected already-added message, got %q", result.Message)
	}
	if items := manager.List(); len(items) != 1 {
		t.Fatalf("expected no duplicate token to be created, got %d", len(items))
	}
}

func TestMigrateLegacyRequestHistoryAssignsIDsForZeroIDEntries(t *testing.T) {
	dataDir := t.TempDir()
	legacyPath := filepath.Join(dataDir, "request_history.json")
	now := time.Now()
	entries := []history.Entry{
		{
			Time:     now.Add(-time.Minute),
			Level:    "info",
			Provider: "openai",
			Status:   http.StatusOK,
			Message:  "first legacy entry",
		},
		{
			Time:     now,
			Level:    "warn",
			Provider: "openai",
			Status:   http.StatusTooManyRequests,
			Message:  "second legacy entry",
		},
	}
	raw, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	store, err := history.NewSQLiteStore(filepath.Join(dataDir, "request_history.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := migrateLegacyRequestHistory(store, legacyPath); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.List(history.Filter{}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected both legacy entries to be imported, got %#v", loaded)
	}
	if loaded[0].ID == 0 || loaded[1].ID == 0 || loaded[0].ID == loaded[1].ID {
		t.Fatalf("expected generated unique IDs, got %#v", loaded)
	}
}

func TestProxyConfigChanged(t *testing.T) {
	base := config.Default()

	if proxyConfigChanged(base, base) {
		t.Fatal("same config should not require proxy restart")
	}

	thresholdOnly := base
	thresholdOnly.SwitchThreshold = 30
	if proxyConfigChanged(base, thresholdOnly) {
		t.Fatal("threshold-only change should not require proxy restart")
	}

	next := base
	next.UpstreamBaseURL = "https://example.com"
	if !proxyConfigChanged(base, next) {
		t.Fatal("upstream change should require proxy restart")
	}

	next = base
	next.KimiBaseURL = "https://example.com/coding"
	if !proxyConfigChanged(base, next) {
		t.Fatal("kimi base url change should require proxy restart")
	}

	next = base
	next.SchedulingMode = config.SchedulingModeBalanced
	if !proxyConfigChanged(base, next) {
		t.Fatal("scheduling mode change should require proxy restart")
	}

	next = base
	next.WebSocketMode = config.WebSocketModeDisabled
	if !proxyConfigChanged(base, next) {
		t.Fatal("websocket mode change should require proxy restart")
	}
}

func TestValidateConfiguredPortsRejectsInvalidPorts(t *testing.T) {
	cfg := config.Default()
	cfg.ControlPort = cfg.ProxyPort
	if err := validateConfiguredPorts(cfg); err == nil {
		t.Fatal("expected matching proxy and control ports to fail")
	}

	cfg = config.Default()
	cfg.ProxyPort = 70000
	if err := validateConfiguredPorts(cfg); err == nil {
		t.Fatal("expected out-of-range proxy port to fail")
	}
}

func TestStartProxyReturnsErrorWhenPortOccupied(t *testing.T) {
	listener, port := listenOnLocalhost(t)
	defer listener.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}

	cfg := config.Default()
	cfg.ProxyPort = port
	app := &appServer{
		cfg:    cfg,
		tokens: manager,
		logs:   logs.NewRecorder(10),
	}

	err = app.startProxy()
	if err == nil {
		t.Fatal("expected occupied proxy port to fail")
	}
	if !strings.Contains(err.Error(), "start proxy listener") {
		t.Fatalf("expected listener error, got %v", err)
	}
	if app.proxyServer != nil {
		t.Fatal("proxy server should not be marked running after listen failure")
	}
}

func TestStartControlReturnsErrorWhenPortOccupied(t *testing.T) {
	listener, port := listenOnLocalhost(t)
	defer listener.Close()

	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}

	cfg := config.Default()
	cfg.ControlPort = port
	app := &appServer{
		cfg:    cfg,
		tokens: manager,
		logs:   logs.NewRecorder(10),
	}

	err = app.startControl()
	if err == nil {
		t.Fatal("expected occupied control API port to fail")
	}
	if !strings.Contains(err.Error(), "start control API listener") {
		t.Fatalf("expected listener error, got %v", err)
	}
	if app.control != nil {
		t.Fatal("control server should not be marked running after listen failure")
	}
}

func TestSaveConfigRejectsInvalidURLBeforePersisting(t *testing.T) {
	dataDir := t.TempDir()
	store := config.NewStore(filepath.Join(dataDir, "config.json"))
	initial := config.Default()
	if err := store.Save(initial); err != nil {
		t.Fatal(err)
	}
	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(dataDir, "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	app := &appServer{
		cfg:         initial,
		configStore: store,
		tokens:      manager,
		logs:        logs.NewRecorder(10),
	}

	bad := initial
	bad.OpenAIBaseURL = "://bad-url"
	if _, err := app.saveConfig(bad); err == nil {
		t.Fatal("expected invalid url to be rejected")
	}

	reloaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.OpenAIBaseURL != initial.OpenAIBaseURL {
		t.Fatalf("invalid url should not be persisted, got %q", reloaded.OpenAIBaseURL)
	}
	if app.cfg.OpenAIBaseURL != initial.OpenAIBaseURL {
		t.Fatalf("invalid url should not update runtime config, got %q", app.cfg.OpenAIBaseURL)
	}
}

func TestSaveConfigRestartsControlAPIWhenPortChanges(t *testing.T) {
	firstListener, firstPort := listenOnLocalhost(t)
	secondListener, secondPort := listenOnLocalhost(t)
	proxyListener, proxyPort := listenOnLocalhost(t)
	firstListener.Close()
	secondListener.Close()
	proxyListener.Close()

	dataDir := t.TempDir()
	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(dataDir, "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.ProxyPort = proxyPort
	cfg.ControlPort = firstPort
	app := &appServer{
		cfg:          cfg,
		configStore:  config.NewStore(filepath.Join(dataDir, "config.json")),
		tokens:       manager,
		logs:         logs.NewRecorder(10),
		controlToken: "test-control-token",
	}
	if err := app.startControl(); err != nil {
		t.Fatal(err)
	}
	defer app.stopControl()

	next := cfg
	next.ControlPort = secondPort
	if _, err := app.saveConfig(next); err != nil {
		t.Fatal(err)
	}

	app.mu.Lock()
	addr := ""
	if app.control != nil {
		addr = app.control.Addr
	}
	app.mu.Unlock()
	if !strings.HasSuffix(addr, intToString(secondPort)) {
		t.Fatalf("expected control API to move to port %d, got %q", secondPort, addr)
	}

	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:"+intToString(secondPort)+"/api/logs", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set(controlTokenHeader, "test-control-token")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected new control API port to respond, got %d", res.StatusCode)
	}
}

func TestChangeDataDirectoryMigratesFilesAndSavesBootstrap(t *testing.T) {
	home := t.TempDir()
	appData := t.TempDir()
	t.Setenv("OMNIPROXY_DATA_DIR", "")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("APPDATA", appData)
	t.Setenv("XDG_CONFIG_HOME", appData)

	currentDir := filepath.Join(t.TempDir(), "current")
	nextDir := filepath.Join(t.TempDir(), "next")
	if err := os.MkdirAll(currentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(currentDir, "config.json"), []byte(`{"proxyPort":3000}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(currentDir, "tokens.json"), []byte(`[]`), 0o600); err != nil {
		t.Fatal(err)
	}

	app := &appServer{
		dataDir: currentDir,
		cfg:     config.Default(),
		logs:    logs.NewRecorder(10),
	}
	result, err := app.changeDataDirectory(nextDir, true)
	if err != nil {
		t.Fatal(err)
	}
	if !result.RestartRequired {
		t.Fatal("expected data directory change to require restart")
	}
	for _, name := range []string{"config.json", "tokens.json"} {
		if _, err := os.Stat(filepath.Join(nextDir, name)); err != nil {
			t.Fatalf("expected migrated %s: %v", name, err)
		}
	}
	bootstrap, err := os.ReadFile(config.BootstrapPath())
	if err != nil {
		t.Fatal(err)
	}
	var saved struct {
		DataDir string `json:"dataDir"`
	}
	if err := json.Unmarshal(bootstrap, &saved); err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(filepath.Clean(saved.DataDir), filepath.Clean(nextDir)) {
		t.Fatalf("expected bootstrap to point at next data dir, got %s", string(bootstrap))
	}
}

func listenOnLocalhost(t *testing.T) (net.Listener, int) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		_ = listener.Close()
		t.Fatalf("expected TCP listener, got %T", listener.Addr())
	}
	return listener, addr.Port
}

func codexAuthJSONForMainTest(t *testing.T, email string) string {
	return codexAuthJSONForMainTestWithCredentials(t, email, "account-123", "codex-access-token")
}

func codexAuthJSONForMainTestWithCredentials(t *testing.T, email string, accountID string, accessToken string) string {
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

func TestWriteCodexOmniProxyConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	initial := strings.Join([]string{
		`model = "gpt-5.5"`,
		`model_provider = "openai"`,
		`chatgpt_base_url = "http://127.0.0.1:3000/backend-api/"`,
		``,
		`[projects.'E:\go\OmniProxy']`,
		`trust_level = "trusted"`,
		``,
		`[model_providers.omniproxy]`,
		`base_url = "http://old.example/v1"`,
	}, "\n")
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := writeCodexOmniProxyConfig(path, "http://127.0.0.1:3000/backend-api/codex"); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, expected := range []string{
		`model_provider = "openai"`,
		`openai_base_url = "http://127.0.0.1:3000/backend-api/codex"`,
		`[projects.'E:\go\OmniProxy']`,
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected config to contain %q, got:\n%s", expected, text)
		}
	}
	if strings.Contains(text, "old.example") {
		t.Fatalf("old omniproxy section was not removed:\n%s", text)
	}
	if strings.Contains(text, "[model_providers.omniproxy]") {
		t.Fatalf("legacy omniproxy section was not removed:\n%s", text)
	}
	if strings.Contains(text, "[model_providers.openai]") {
		t.Fatalf("reserved openai provider section was not removed:\n%s", text)
	}
	if strings.Contains(text, "chatgpt_base_url") {
		t.Fatalf("chatgpt_base_url should not be used for the model proxy:\n%s", text)
	}
	if _, err := os.Stat(path + ".omniproxy.bak"); err != nil {
		t.Fatalf("expected backup file: %v", err)
	}
}

func TestWriteCodexOmniProxyConfigKeepsOriginalBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	first := `model_provider = "openai"` + "\n"
	if err := os.WriteFile(path, []byte(first), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeCodexOmniProxyConfig(path, "http://127.0.0.1:3000/backend-api/codex"); err != nil {
		t.Fatal(err)
	}
	if err := writeCodexOmniProxyConfig(path, "http://127.0.0.1:3001/backend-api/codex"); err != nil {
		t.Fatal(err)
	}

	backup, err := os.ReadFile(path + ".omniproxy.bak")
	if err != nil {
		t.Fatal(err)
	}
	if string(backup) != first {
		t.Fatalf("backup should keep original config, got:\n%s", string(backup))
	}
}

func TestWriteClaudeRouterSettingsUsesSingleModelOption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	initial := `{"env":{"ANTHROPIC_BASE_URL":"https://api.anthropic.com","ANTHROPIC_MODEL":"deepseek-v4-pro[1m]","ANTHROPIC_DEFAULT_OPUS_MODEL":"deepseek-v4-pro[1m]","ANTHROPIC_DEFAULT_OPUS_MODEL_NAME":"Old DeepSeek","CLAUDE_CODE_EFFORT_LEVEL":"max","OTHER":"keep"},"availableModels":["custom-existing-model","claude-opus-4-7","kimi-for-coding"],"modelOverrides":{"claude-sonnet-4-0":"custom-existing","claude-opus-4-7":"mimo-v2.5-pro"}}` + "\n"
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := writeMimoClaudeSettings(path, "http://127.0.0.1:3000/anthropic-router"); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, expected := range []string{
		`"ANTHROPIC_BASE_URL": "http://127.0.0.1:3000/anthropic-router"`,
		`"ANTHROPIC_AUTH_TOKEN": "omniproxy"`,
		`"ANTHROPIC_MODEL": "mimo-v2.5-pro"`,
		`"ANTHROPIC_DEFAULT_OPUS_MODEL": "mimo-v2.5-pro"`,
		`"ANTHROPIC_DEFAULT_OPUS_MODEL_NAME": "MiMo-V2.5-Pro"`,
		`"ANTHROPIC_DEFAULT_SONNET_MODEL": "mimo-v2.5"`,
		`"ANTHROPIC_DEFAULT_SONNET_MODEL_NAME": "MiMo-V2.5"`,
		`"ANTHROPIC_DEFAULT_HAIKU_MODEL": "mimo-v2.5"`,
		`"ANTHROPIC_DEFAULT_HAIKU_MODEL_NAME": "MiMo-V2.5"`,
		`"CLAUDE_CODE_SUBAGENT_MODEL": "mimo-v2.5"`,
		`"ANTHROPIC_CUSTOM_MODEL_OPTION": "mimo-v2.5"`,
		`"ANTHROPIC_CUSTOM_MODEL_OPTION_NAME": "MiMo-V2.5"`,
		`"OTHER": "keep"`,
		`"availableModels": [`,
		`"custom-existing-model"`,
		`"claude-sonnet-4-0": "custom-existing"`,
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected settings to contain %q, got:\n%s", expected, text)
		}
	}
	for _, unwanted := range []string{
		`"Old DeepSeek"`,
		`"CLAUDE_CODE_EFFORT_LEVEL"`,
		`"claude-opus-4-7"`,
		`"kimi-for-coding"`,
		`"deepseek-v4-flash"`,
	} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("expected settings not to contain %q, got:\n%s", unwanted, text)
		}
	}
	if _, err := os.Stat(path + ".omniproxy.bak"); err != nil {
		t.Fatalf("expected backup file: %v", err)
	}
}

func TestWriteClaudeRouterSettingsCanSelectEachProvider(t *testing.T) {
	cases := []struct {
		name             string
		write            func(string, string) error
		defaultModel     string
		opusModel        string
		sonnetModel      string
		haikuModel       string
		subagentModel    string
		label            string
		unwanted         string
		expectEffortMax  bool
		expectCustomName bool
	}{
		{
			name:             "mimo",
			write:            writeMimoClaudeSettings,
			defaultModel:     "mimo-v2.5-pro",
			opusModel:        "mimo-v2.5-pro",
			sonnetModel:      "mimo-v2.5",
			haikuModel:       "mimo-v2.5",
			subagentModel:    "mimo-v2.5",
			label:            "MiMo-V2.5",
			unwanted:         "deepseek-v4-pro[1m]",
			expectCustomName: true,
		},
		{
			name:            "deepseek",
			write:           writeDeepSeekClaudeSettings,
			defaultModel:    "deepseek-v4-pro[1m]",
			opusModel:       "deepseek-v4-pro[1m]",
			sonnetModel:     "deepseek-v4-pro[1m]",
			haikuModel:      "deepseek-v4-flash",
			subagentModel:   "deepseek-v4-flash",
			label:           "DeepSeek V4 Flash",
			unwanted:        "mimo-v2.5-pro",
			expectEffortMax: true,
		},
		{
			name:             "kimi",
			write:            writeKimiClaudeSettings,
			defaultModel:     "kimi-for-coding",
			opusModel:        "kimi-for-coding",
			sonnetModel:      "kimi-for-coding",
			haikuModel:       "kimi-for-coding",
			subagentModel:    "kimi-for-coding",
			label:            "Kimi for Coding",
			unwanted:         "mimo-v2.5-pro",
			expectCustomName: true,
		},
		{
			name:             "zhipu",
			write:            writeZhipuClaudeSettings,
			defaultModel:     "glm-5.1",
			opusModel:        "glm-5.1",
			sonnetModel:      "glm-5.1",
			haikuModel:       "glm-5.1",
			subagentModel:    "glm-5.1",
			label:            "GLM-5.1",
			unwanted:         "mimo-v2.5-pro",
			expectCustomName: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			if err := os.WriteFile(path, []byte(`{"env":{"OTHER":"keep"}}`+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := tc.write(path, "http://127.0.0.1:3000/anthropic-router"); err != nil {
				t.Fatal(err)
			}
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			text := string(content)
			for _, expected := range []string{
				`"ANTHROPIC_MODEL": "` + tc.defaultModel + `"`,
				`"ANTHROPIC_DEFAULT_OPUS_MODEL": "` + tc.opusModel + `"`,
				`"ANTHROPIC_DEFAULT_SONNET_MODEL": "` + tc.sonnetModel + `"`,
				`"ANTHROPIC_DEFAULT_HAIKU_MODEL": "` + tc.haikuModel + `"`,
				`"CLAUDE_CODE_SUBAGENT_MODEL": "` + tc.subagentModel + `"`,
			} {
				if !strings.Contains(text, expected) {
					t.Fatalf("expected settings to contain %q, got:\n%s", expected, text)
				}
			}
			if tc.expectCustomName && !strings.Contains(text, `"ANTHROPIC_CUSTOM_MODEL_OPTION_NAME": "`+tc.label+`"`) {
				t.Fatalf("expected settings to contain custom label %q, got:\n%s", tc.label, text)
			}
			if tc.expectEffortMax && !strings.Contains(text, `"CLAUDE_CODE_EFFORT_LEVEL": "max"`) {
				t.Fatalf("expected deepseek settings to set max effort, got:\n%s", text)
			}
			if strings.Contains(text, tc.unwanted) {
				t.Fatalf("expected settings not to contain %q, got:\n%s", tc.unwanted, text)
			}
		})
	}
}

func TestWriteClaudeRouterSettingsAcceptsUTF8BOM(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	raw := append([]byte{0xEF, 0xBB, 0xBF}, []byte(`{"env":{"OTHER":"keep"}}`+"\n")...)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := writeMimoClaudeSettings(path, "http://127.0.0.1:3000/anthropic-router"); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(string(content), "\ufeff") {
		t.Fatalf("expected rewritten settings to omit UTF-8 BOM, got:\n%s", string(content))
	}
	if !strings.Contains(string(content), `"ANTHROPIC_BASE_URL": "http://127.0.0.1:3000/anthropic-router"`) {
		t.Fatalf("expected router base url, got:\n%s", string(content))
	}
}

func TestExtractMimoCookieFromHAR(t *testing.T) {
	data := []byte(`{
		"log": {
			"entries": [
				{
					"request": {
						"url": "https://platform.xiaomimimo.com/api/v1/balance",
						"headers": [
							{"name": "accept", "value": "*/*"},
							{"name": "cookie", "value": "serviceToken=session; userId=123"}
						]
					}
				}
			]
		}
	}`)

	cookie, matchedURL, err := extractMimoCookieFromHAR(data)
	if err != nil {
		t.Fatal(err)
	}
	if cookie != "serviceToken=session; userId=123" {
		t.Fatalf("unexpected cookie %q", cookie)
	}
	if matchedURL != "https://platform.xiaomimimo.com/api/v1/balance" {
		t.Fatalf("unexpected matched URL %q", matchedURL)
	}
}

func TestWriteGeminiConfigPreservesExistingSettings(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	settingsPath := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(envPath, []byte("OTHER=keep\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath, []byte(`{"mcpServers":{"demo":{}},"security":{"auth":{"old":"keep"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := writeGeminiEnv(envPath, "http://127.0.0.1:3000/gemini", "omniproxy-local", "gemini-3-pro-preview"); err != nil {
		t.Fatal(err)
	}
	if err := writeGeminiSettings(settingsPath, "gemini-api-key"); err != nil {
		t.Fatal(err)
	}

	env, err := readEnvFile(envPath)
	if err != nil {
		t.Fatal(err)
	}
	if env["OTHER"] != "keep" || env["GOOGLE_GEMINI_BASE_URL"] != "http://127.0.0.1:3000/gemini" || env["GEMINI_MODEL"] != "gemini-3-pro-preview" {
		t.Fatalf("unexpected gemini env: %#v", env)
	}
	settings, err := readJSONObject(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if settings["mcpServers"] == nil {
		t.Fatalf("expected existing settings to be preserved: %#v", settings)
	}
	security := settings["security"].(map[string]any)
	auth := security["auth"].(map[string]any)
	if auth["old"] != "keep" || auth["selectedType"] != "gemini-api-key" {
		t.Fatalf("unexpected auth settings: %#v", auth)
	}
	if _, err := os.Stat(envPath + ".omniproxy.bak"); err != nil {
		t.Fatalf("expected env backup: %v", err)
	}
	if _, err := os.Stat(settingsPath + ".omniproxy.bak"); err != nil {
		t.Fatalf("expected settings backup: %v", err)
	}
}

func TestWriteOpenCodeConfigAddsOmniProxyProviders(t *testing.T) {
	path := filepath.Join(t.TempDir(), "opencode.json")
	if err := os.WriteFile(path, []byte(`{"provider":{"existing":{"npm":"@ai-sdk/openai-compatible","models":{}}}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	openRouterModels := map[string]any{
		"openai/gpt-test": map[string]any{"name": "GPT Test"},
	}
	if err := writeOpenCodeConfig(path, "http://127.0.0.1:3000/opencode-router/v1", "http://127.0.0.1:3000/gemini", "http://127.0.0.1:3000/openrouter/v1", "http://127.0.0.1:3000/custom/v1", openRouterModels); err != nil {
		t.Fatal(err)
	}

	data, err := readJSONObject(path)
	if err != nil {
		t.Fatal(err)
	}
	providers := data["provider"].(map[string]any)
	for _, id := range []string{"existing", opencodeOmniProviderID, opencodeGeminiProviderID, opencodeOpenRouterProviderID, opencodeCustomProviderID} {
		if providers[id] == nil {
			t.Fatalf("expected provider %s in %#v", id, providers)
		}
	}
	routerProvider := providers[opencodeOmniProviderID].(map[string]any)
	options := routerProvider["options"].(map[string]any)
	if options["baseURL"] != "http://127.0.0.1:3000/opencode-router/v1" {
		t.Fatalf("unexpected router baseURL: %#v", options)
	}
	openRouterProvider := providers[opencodeOpenRouterProviderID].(map[string]any)
	openRouterOptions := openRouterProvider["options"].(map[string]any)
	if openRouterOptions["baseURL"] != "http://127.0.0.1:3000/openrouter/v1" {
		t.Fatalf("unexpected openrouter baseURL: %#v", openRouterOptions)
	}
	openRouterProviderModels := openRouterProvider["models"].(map[string]any)
	if openRouterProviderModels["openai/gpt-test"] == nil {
		t.Fatalf("expected openrouter models in %#v", openRouterProviderModels)
	}
	if _, err := os.Stat(path + ".omniproxy.bak"); err != nil {
		t.Fatalf("expected opencode backup: %v", err)
	}
}
