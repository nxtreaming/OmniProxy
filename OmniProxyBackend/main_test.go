package main

import (
	"encoding/json"
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

func TestTokenExclusiveEndpointUsesOnlySelectedProviderAccount(t *testing.T) {
	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := manager.Add(token.UpsertRequest{Name: "selected", Provider: "openai", TokenValue: "sk-selected-token"})
	if err != nil {
		t.Fatal(err)
	}
	otherOpenAI, err := manager.Add(token.UpsertRequest{Name: "backup", Provider: "openai", TokenValue: "sk-backup-token"})
	if err != nil {
		t.Fatal(err)
	}
	otherProvider, err := manager.Add(token.UpsertRequest{Name: "anthropic", Provider: "anthropic", TokenValue: "sk-anthropic-token"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.SetDisabled(selected.ID, true); err != nil {
		t.Fatal(err)
	}
	app := &appServer{
		cfg:    config.Default(),
		tokens: manager,
		logs:   logs.NewRecorder(10),
	}

	req := httptest.NewRequest(http.MethodPut, "/api/tokens/"+selected.ID+"/exclusive", nil)
	res := httptest.NewRecorder()
	app.routes().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", res.Code, res.Body.String())
	}
	var payload []struct {
		ID       string `json:"id"`
		Disabled bool   `json:"disabled"`
		Selected bool   `json:"selected"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	disabledByID := map[string]bool{}
	selectedByID := map[string]bool{}
	for _, item := range payload {
		disabledByID[item.ID] = item.Disabled
		selectedByID[item.ID] = item.Selected
	}
	if disabledByID[selected.ID] {
		t.Fatal("expected selected token to be enabled")
	}
	if !selectedByID[selected.ID] {
		t.Fatal("expected selected token to be selected")
	}
	if disabledByID[otherOpenAI.ID] || selectedByID[otherOpenAI.ID] {
		t.Fatal("expected same provider backup token to keep enabled unselected state")
	}
	if disabledByID[otherProvider.ID] {
		t.Fatal("expected other provider token to remain enabled")
	}
	acquired, err := manager.Acquire(token.ProviderOpenAI, nil)
	if err != nil {
		t.Fatal(err)
	}
	if acquired.ID != selected.ID {
		t.Fatalf("expected selected account to be acquired, got %s", acquired.Name)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/tokens/"+selected.ID+"/exclusive", nil)
	res = httptest.NewRecorder()
	app.routes().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected cancel status 200, got %d body=%s", res.Code, res.Body.String())
	}
	payload = nil
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	disabledByID = map[string]bool{}
	selectedByID = map[string]bool{}
	for _, item := range payload {
		disabledByID[item.ID] = item.Disabled
		selectedByID[item.ID] = item.Selected
	}
	if disabledByID[selected.ID] || disabledByID[otherOpenAI.ID] {
		t.Fatal("expected selected provider tokens to be enabled after cancelling exclusive selection")
	}
	if selectedByID[selected.ID] || selectedByID[otherOpenAI.ID] {
		t.Fatal("expected selected provider selection to be cleared after cancelling exclusive selection")
	}
	if disabledByID[otherProvider.ID] {
		t.Fatal("expected other provider token to remain enabled after cancelling exclusive selection")
	}
}

func TestTokenSelectedEndpointSupportsProviderSelectionSet(t *testing.T) {
	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	first, err := manager.Add(token.UpsertRequest{Name: "first", Provider: "openai", TokenValue: "sk-first-token"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := manager.Add(token.UpsertRequest{Name: "second", Provider: "openai", TokenValue: "sk-second-token"})
	if err != nil {
		t.Fatal(err)
	}
	third, err := manager.Add(token.UpsertRequest{Name: "third", Provider: "openai", TokenValue: "sk-third-token"})
	if err != nil {
		t.Fatal(err)
	}
	app := &appServer{
		cfg:    config.Default(),
		tokens: manager,
		logs:   logs.NewRecorder(10),
	}

	for _, item := range []token.Token{first, second} {
		req := httptest.NewRequest(http.MethodPut, "/api/tokens/"+item.ID+"/selected", strings.NewReader(`{"selected":true}`))
		res := httptest.NewRecorder()
		app.routes().ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("expected selected status 200, got %d body=%s", res.Code, res.Body.String())
		}
	}

	acquired, err := manager.Acquire(token.ProviderOpenAI, map[string]bool{first.ID: true})
	if err != nil {
		t.Fatal(err)
	}
	if acquired.ID != second.ID {
		t.Fatalf("expected selected backup account, got %s", acquired.Name)
	}
	acquired, err = manager.Acquire(token.ProviderOpenAI, map[string]bool{first.ID: true, second.ID: true})
	if err != nil {
		t.Fatal(err)
	}
	if acquired.ID != third.ID {
		t.Fatalf("expected active unselected account to protect against unavailable selected set, got %s", acquired.Name)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/tokens/"+first.ID+"/selected", strings.NewReader(`{"selected":false}`))
	res := httptest.NewRecorder()
	app.routes().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected deselect status 200, got %d body=%s", res.Code, res.Body.String())
	}
}
