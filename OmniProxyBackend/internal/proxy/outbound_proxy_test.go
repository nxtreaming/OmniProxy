package proxy

import (
	"testing"

	"OmniProxyBackend/internal/config"
	"OmniProxyBackend/internal/token"
)

func TestOutboundProxyMatchesRouteProviders(t *testing.T) {
	cfg := config.Config{OutboundProxyProviders: []string{"openai", "anthropic", "gemini", "openrouter", "zo", "prem"}}

	for _, route := range []routeInfo{
		{Provider: token.ProviderOpenAI, CredentialType: token.CredentialTypeCodexAuthJSON, Path: "/backend-api/codex/models"},
		{Provider: token.ProviderOpenAI, Path: "/v1/models"},
		{Provider: token.ProviderAnthropic, Path: "/v1/models"},
		{Provider: token.ProviderGemini, Path: "/v1beta/models"},
		{Provider: token.ProviderOpenRouter, Path: "/models"},
		{Provider: token.ProviderZo, Path: "/models/available"},
		{Provider: token.ProviderPrem, Path: "/v1/models"},
	} {
		if !outboundProxyMatchesRoute(route, cfg) {
			t.Fatalf("expected route %#v to match proxy providers", route)
		}
	}

	for _, route := range []routeInfo{
		{Provider: token.ProviderDeepSeek, Path: "/models"},
		{Provider: token.ProviderKimi, Path: "/models"},
		{Provider: token.ProviderCustom, Path: "/models"},
	} {
		if outboundProxyMatchesRoute(route, cfg) {
			t.Fatalf("expected route %#v to bypass proxy providers", route)
		}
	}
}

func TestOutboundProxyProviderSelectionIsIndependentFromModelName(t *testing.T) {
	route := routeInfo{Provider: token.ProviderOpenAI, Model: "deepseek-v4-pro", Path: "/v1/chat/completions"}
	if !outboundProxyMatchesRoute(route, config.Config{OutboundProxyProviders: []string{"openai"}}) {
		t.Fatalf("expected selected provider to use proxy even when model name has another prefix")
	}
	if outboundProxyMatchesRoute(route, config.Config{OutboundProxyProviders: []string{"deepseek"}}) {
		t.Fatalf("expected unselected route provider to bypass proxy")
	}
}
