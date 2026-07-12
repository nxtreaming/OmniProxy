package proxy

import (
	"testing"

	"omniproxy/internal/config"
	"omniproxy/internal/token"
)

func TestRouterDoesNotUseForgeModelRouteForCodexResponses(t *testing.T) {
	router := NewRouter(config.Config{
		GatewayRoutes: config.GatewayRoutes{
			Codex: config.GatewayRouteConfig{Provider: token.ProviderOpenAI, Model: "gpt-5.6-sol"},
		},
		ModelRoutes: config.ModelRoutes{
			"deepseek-r1": {Provider: token.ProviderForge, Model: "deepseek-r1"},
		},
	})

	route := router.Route(mustRouterTestURL(t, "/codex/v1/responses"), []byte(`{"model":"deepseek-r1","input":"hi"}`))
	if route.Provider == token.ProviderForge || route.Path != "/v1/responses" {
		t.Fatalf("expected Codex Responses to stay off Forge, got %#v", route)
	}

	route = router.Route(mustRouterTestURL(t, "/opencode-router/v1/chat/completions"), []byte(`{"model":"deepseek-r1","messages":[]}`))
	if route.Provider != token.ProviderForge || route.Path != "/v1/chat/completions" {
		t.Fatalf("expected OpenAI Chat to use Forge model route, got %#v", route)
	}
}

func TestRouterTargetsForgeVersionedBaseURL(t *testing.T) {
	router := NewRouter(config.Config{
		ForgeBaseURL: "https://forge-gateway-api.fly.dev/v1",
	})
	selected := token.Token{Provider: token.ProviderForge, CredentialType: token.CredentialTypeAPIKey}

	route := router.Route(mustRouterTestURL(t, "/forge/v1/chat/completions"), []byte(`{"model":"gpt-5.6-sol"}`))
	target, err := router.TargetURL(route, selected)
	if err != nil {
		t.Fatal(err)
	}
	if target != "https://forge-gateway-api.fly.dev/v1/chat/completions" {
		t.Fatalf("unexpected Forge OpenAI target url: %s", target)
	}

	route = router.Route(mustRouterTestURL(t, "/forge/anthropic/v1/messages"), []byte(`{"model":"claude-sonnet-5"}`))
	target, err = router.TargetURL(route, selected)
	if err != nil {
		t.Fatal(err)
	}
	if target != "https://forge-gateway-api.fly.dev/v1/messages" {
		t.Fatalf("unexpected Forge Anthropic target url: %s", target)
	}
}
