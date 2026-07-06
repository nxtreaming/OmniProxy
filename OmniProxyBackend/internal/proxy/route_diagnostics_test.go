package proxy

import (
	"path/filepath"
	"strings"
	"testing"

	"omniproxy/internal/config"
	"omniproxy/internal/storage"
	"omniproxy/internal/token"
)

func TestDiagnoseRouteUsesFallbackWhenPrimaryHasNoToken(t *testing.T) {
	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add(token.UpsertRequest{
		Name:       "deepseek",
		Provider:   token.ProviderDeepSeek,
		TokenValue: "sk-deepseek-test",
	}); err != nil {
		t.Fatal(err)
	}

	result := DiagnoseRoute(config.Config{
		DeepSeekAnthropicBaseURL: "https://deepseek.example/anthropic",
		GatewayRoutes: config.GatewayRoutes{
			Claude: config.GatewayRouteConfig{
				Provider: token.ProviderAnthropic,
				Model:    "sonnet",
				Fallbacks: []config.GatewayRouteConfig{
					{Provider: token.ProviderDeepSeek, Model: "deepseek-v4-pro"},
				},
			},
		},
	}, manager, RouteDiagnosticRequest{Client: "claude", Model: "sonnet"})

	if !result.OK || result.SelectedIndex != 1 {
		t.Fatalf("expected fallback route to be selected, got %#v", result)
	}
	if len(result.Chain) != 2 {
		t.Fatalf("expected primary and fallback diagnostics, got %#v", result.Chain)
	}
	if result.Chain[0].Available || !strings.Contains(result.Chain[0].Issue, "账号") {
		t.Fatalf("expected primary to report missing account, got %#v", result.Chain[0])
	}
	if result.Chain[1].Provider != token.ProviderDeepSeek || result.Chain[1].TokenName != "deepseek" {
		t.Fatalf("unexpected fallback diagnostic: %#v", result.Chain[1])
	}
	if !strings.Contains(result.Chain[1].TargetURL, "/anthropic/v1/messages") {
		t.Fatalf("expected anthropic fallback target path, got %q", result.Chain[1].TargetURL)
	}
}
