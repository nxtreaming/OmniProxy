package proxy

import (
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
