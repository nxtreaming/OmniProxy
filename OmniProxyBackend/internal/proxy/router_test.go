package proxy

import (
	"net/url"
	"testing"

	"OmniProxyBackend/internal/config"
	"OmniProxyBackend/internal/token"
)

func TestRouterReadsRequestBodyModel(t *testing.T) {
	route := NewRouter(config.Config{}).Route(mustRouterTestURL(t, "/backend-api/codex/responses"), []byte(`{"model":"gpt-body","input":"hello"}`))

	if route.Model != "gpt-body" {
		t.Fatalf("expected body model, got %#v", route)
	}
}

func TestRouterReadsQueryModel(t *testing.T) {
	route := NewRouter(config.Config{}).Route(mustRouterTestURL(t, "/v1/responses?model=gpt-query"), []byte(`{}`))

	if route.Model != "gpt-query" {
		t.Fatalf("expected query model, got %#v", route)
	}
}

func TestPiRouterOpenAIModelsRequireAPIKeyCredential(t *testing.T) {
	route := NewRouter(config.Config{}).Route(mustRouterTestURL(t, "/pi-router/v1/chat/completions"), []byte(`{"model":"gpt-5.4"}`))

	if route.Provider != token.ProviderOpenAI || route.CredentialType != token.CredentialTypeAPIKey {
		t.Fatalf("expected Pi OpenAI model to require API key credential, got %#v", route)
	}
}

func TestRouterMapsNewProviderPrefixes(t *testing.T) {
	router := NewRouter(config.Config{})
	cases := []struct {
		name     string
		path     string
		body     string
		provider string
		protocol string
		outPath  string
	}{
		{
			name:     "anthropic router zhipu",
			path:     "/anthropic-router/v1/messages",
			body:     `{"model":"glm-5.1"}`,
			provider: token.ProviderZhipu,
			protocol: "anthropic",
			outPath:  "/v1/messages",
		},
		{
			name:     "anthropic router minimax",
			path:     "/anthropic-router/v1/messages",
			body:     `{"model":"MiniMax-M2.7"}`,
			provider: token.ProviderMiniMax,
			protocol: "anthropic",
			outPath:  "/v1/messages",
		},
		{
			name:     "opencode router defaults openai",
			path:     "/opencode-router/v1/chat/completions",
			body:     `{"model":"gpt-5.4"}`,
			provider: token.ProviderOpenAI,
			protocol: "openai",
			outPath:  "/v1/chat/completions",
		},
		{
			name:     "opencode router openrouter",
			path:     "/opencode-router/v1/chat/completions",
			body:     `{"model":"openai/gpt-test"}`,
			provider: token.ProviderOpenRouter,
			protocol: "openai",
			outPath:  "/v1/chat/completions",
		},
		{
			name:     "opencode router custom",
			path:     "/opencode-router/v1/chat/completions",
			body:     `{"model":"custom-model"}`,
			provider: token.ProviderCustom,
			protocol: "openai",
			outPath:  "/v1/chat/completions",
		},
		{
			name:     "pi router kimi",
			path:     "/pi-router/v1/chat/completions",
			body:     `{"model":"kimi-for-coding"}`,
			provider: token.ProviderKimi,
			protocol: "openai",
			outPath:  "/v1/chat/completions",
		},
		{
			name:     "gemini direct",
			path:     "/gemini/v1beta/models/gemini-3-pro-preview:generateContent",
			body:     `{}`,
			provider: token.ProviderGemini,
			protocol: "gemini",
			outPath:  "/v1beta/models/gemini-3-pro-preview:generateContent",
		},
		{
			name:     "openrouter direct",
			path:     "/openrouter/v1/chat/completions",
			body:     `{"model":"anthropic/claude-test"}`,
			provider: token.ProviderOpenRouter,
			protocol: "openai",
			outPath:  "/v1/chat/completions",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			route := router.Route(mustRouterTestURL(t, tt.path), []byte(tt.body))
			if route.Provider != tt.provider || route.Protocol != tt.protocol || route.Path != tt.outPath {
				t.Fatalf("unexpected route: %#v", route)
			}
		})
	}
}

func TestRouterAvoidsDuplicateOpenAIVersionForVersionedProviderBaseURL(t *testing.T) {
	router := NewRouter(config.Config{
		ZhipuBaseURL: "https://open.bigmodel.cn/api/paas/v4",
	})
	route := router.Route(mustRouterTestURL(t, "/opencode-router/v1/chat/completions"), []byte(`{"model":"glm-5.1"}`))
	target, err := router.TargetURL(route, token.Token{Provider: token.ProviderZhipu, CredentialType: token.CredentialTypeAPIKey})
	if err != nil {
		t.Fatal(err)
	}
	if target != "https://open.bigmodel.cn/api/paas/v4/chat/completions" {
		t.Fatalf("unexpected target url: %s", target)
	}
}

func TestRouterTargetsXiaomiTokenPlanAnthropicSGPBaseURL(t *testing.T) {
	router := NewRouter(config.Config{
		XiaomiTokenPlanBaseURL:             "https://token-plan-cn.xiaomimimo.com/v1",
		XiaomiTokenPlanSGPBaseURL:          "https://token-plan-sgp.xiaomimimo.com/v1",
		XiaomiTokenPlanAnthropicBaseURL:    "https://token-plan-cn.xiaomimimo.com/anthropic",
		XiaomiTokenPlanSGPAnthropicBaseURL: "https://token-plan-sgp.xiaomimimo.com/anthropic",
	})
	selected := token.Token{Provider: token.ProviderXiaomi, CredentialType: token.CredentialTypeMimoTokenPlan, Region: token.MimoRegionSGP}
	cases := []struct {
		name string
		path string
		body string
	}{
		{
			name: "direct xiaomi anthropic prefix",
			path: "/xiaomi/anthropic/v1/messages",
			body: `{}`,
		},
		{
			name: "anthropic model router",
			path: "/anthropic-router/v1/messages",
			body: `{"model":"mimo-v2.5"}`,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			route := router.Route(mustRouterTestURL(t, tt.path), []byte(tt.body))
			target, err := router.TargetURL(route, selected)
			if err != nil {
				t.Fatal(err)
			}
			if target != "https://token-plan-sgp.xiaomimimo.com/anthropic/v1/messages" {
				t.Fatalf("unexpected target url: %s", target)
			}
		})
	}

	route := router.Route(mustRouterTestURL(t, "/xiaomi/v1/chat/completions"), []byte(`{}`))
	target, err := router.TargetURL(route, selected)
	if err != nil {
		t.Fatal(err)
	}
	if target != "https://token-plan-sgp.xiaomimimo.com/v1/chat/completions" {
		t.Fatalf("unexpected openai-compatible target url: %s", target)
	}
}

func mustRouterTestURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
