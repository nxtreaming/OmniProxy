package config

import (
	"omniproxy/internal/token"
	"strings"
)

func normalizeGatewayRoutes(routes GatewayRoutes, defaults GatewayRoutes) GatewayRoutes {
	return GatewayRoutes{
		Codex:  normalizeGatewayRoute(routes.Codex, defaults.Codex, gatewayCodexProviders()),
		Claude: normalizeGatewayRoute(routes.Claude, defaults.Claude, gatewayClaudeProviders()),
		OpenAI: normalizeGatewayRoute(routes.OpenAI, defaults.OpenAI, gatewayOpenAIProviders()),
		Gemini: normalizeGatewayRoute(routes.Gemini, defaults.Gemini, gatewayGeminiProviders()),
	}
}

func normalizeGatewayRoute(route GatewayRouteConfig, defaults GatewayRouteConfig, allowed map[string]bool) GatewayRouteConfig {
	normalized := normalizeGatewayRouteTarget(route, defaults, allowed, true)
	normalized.Fallbacks = normalizeGatewayRouteFallbacks(route.Fallbacks, normalized, allowed)
	return normalized
}

func normalizeGatewayRouteTarget(route GatewayRouteConfig, defaults GatewayRouteConfig, allowed map[string]bool, useDefaults bool) GatewayRouteConfig {
	provider := strings.TrimSpace(strings.ToLower(route.Provider))
	if provider == "" {
		if !useDefaults {
			return GatewayRouteConfig{}
		}
		provider = defaults.Provider
	}
	credentialType := strings.TrimSpace(strings.ToLower(route.CredentialType))
	if useDefaults && credentialType == "" && strings.EqualFold(provider, defaults.Provider) {
		credentialType = defaults.CredentialType
	}
	credentialExplicit := credentialType != ""
	normalizedProvider, normalizedCredential, err := token.NormalizeProviderAndCredential(provider, credentialType)
	if err != nil || !allowed[normalizedProvider] {
		if !useDefaults {
			return GatewayRouteConfig{}
		}
		normalizedProvider = defaults.Provider
		normalizedCredential = defaults.CredentialType
	}
	if !credentialExplicit && (!useDefaults || !strings.EqualFold(normalizedProvider, defaults.Provider)) {
		normalizedCredential = ""
	}
	model := strings.TrimSpace(route.Model)
	if model == "" {
		model = defaults.Model
	}
	return GatewayRouteConfig{
		Provider:       normalizedProvider,
		CredentialType: normalizedCredential,
		Model:          model,
	}
}

func normalizeGatewayRouteFallbacks(fallbacks []GatewayRouteConfig, primary GatewayRouteConfig, allowed map[string]bool) []GatewayRouteConfig {
	if len(fallbacks) == 0 {
		return nil
	}
	seen := map[string]bool{gatewayRouteTargetKey(primary): true}
	out := make([]GatewayRouteConfig, 0, len(fallbacks))
	for _, fallback := range fallbacks {
		normalized := normalizeGatewayRouteTarget(fallback, primary, allowed, false)
		if normalized.Provider == "" {
			continue
		}
		key := gatewayRouteTargetKey(normalized)
		if seen[key] {
			continue
		}
		seen[key] = true
		normalized.Fallbacks = nil
		out = append(out, normalized)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func gatewayRouteTargetKey(route GatewayRouteConfig) string {
	return strings.ToLower(strings.TrimSpace(route.Provider)) + "\x00" + strings.ToLower(strings.TrimSpace(route.CredentialType))
}

func gatewayCodexProviders() map[string]bool {
	return gatewayProviderSet(
		token.ProviderOpenAI,
		token.ProviderDeepSeek,
		token.ProviderKimi,
		token.ProviderXiaomi,
		token.ProviderZhipu,
		token.ProviderMiniMax,
		token.ProviderOpenRouter,
		token.ProviderTokenRouter,
		token.ProviderSub2API,
		token.ProviderNewAPI,
		token.ProviderAnyRouter,
		token.ProviderZo,
		token.ProviderPrem,
		token.ProviderCustom,
	)
}

func gatewayClaudeProviders() map[string]bool {
	return gatewayProviderSet(
		token.ProviderAnthropic,
		token.ProviderDeepSeek,
		token.ProviderKimi,
		token.ProviderXiaomi,
		token.ProviderZhipu,
		token.ProviderMiniMax,
		token.ProviderSub2API,
		token.ProviderNewAPI,
		token.ProviderAnyRouter,
		token.ProviderZo,
		token.ProviderPrem,
		token.ProviderCustom,
	)
}

func gatewayOpenAIProviders() map[string]bool {
	return gatewayCodexProviders()
}

func gatewayGeminiProviders() map[string]bool {
	return gatewayProviderSet(
		token.ProviderGemini,
		token.ProviderSub2API,
		token.ProviderNewAPI,
	)
}

func gatewayProviderSet(providers ...string) map[string]bool {
	out := map[string]bool{}
	for _, provider := range providers {
		out[token.NormalizeProvider(provider)] = true
	}
	return out
}
