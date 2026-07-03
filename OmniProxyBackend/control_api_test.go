package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"omniproxy/internal/config"
	"omniproxy/internal/logs"
	"path/filepath"
	"strings"
	"testing"
)

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
