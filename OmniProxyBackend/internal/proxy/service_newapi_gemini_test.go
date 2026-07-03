package proxy

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"omniproxy/internal/config"
	"omniproxy/internal/logs"
	"omniproxy/internal/storage"
	"omniproxy/internal/token"
)

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
