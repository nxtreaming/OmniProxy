package config

import "testing"

func TestNormalizeSchedulingAndWebSocketModes(t *testing.T) {
	cfg := Normalize(Config{})
	if cfg.SchedulingMode != SchedulingModeQueue {
		t.Fatalf("expected default queue scheduling, got %q", cfg.SchedulingMode)
	}
	if cfg.WebSocketMode != WebSocketModeEnabled {
		t.Fatalf("expected websocket enabled by default, got %q", cfg.WebSocketMode)
	}
	if cfg.OutboundProxyEnabled {
		t.Fatal("expected outbound proxy disabled by default")
	}
	if cfg.OutboundProxyURL != "http://127.0.0.1:10808" {
		t.Fatalf("expected default outbound proxy URL, got %q", cfg.OutboundProxyURL)
	}
	if len(cfg.OutboundProxyModels) != len(defaultOutboundProxyModels) {
		t.Fatalf("expected default outbound proxy models, got %#v", cfg.OutboundProxyModels)
	}
	if len(cfg.OutboundProxyProviders) != len(defaultOutboundProxyProviders) {
		t.Fatalf("expected default outbound proxy providers, got %#v", cfg.OutboundProxyProviders)
	}
	if cfg.XiaomiCredentialPriority != MimoCredentialPriorityTokenPlan {
		t.Fatalf("expected default MiMo token plan priority, got %q", cfg.XiaomiCredentialPriority)
	}
	if cfg.TaskAutomationEnabled {
		t.Fatal("expected task automation disabled by default")
	}
	if len(cfg.TaskAutomationClients) != 3 || cfg.TaskAutomationClients[0] != "codex" || cfg.TaskAutomationClients[1] != "claude" || cfg.TaskAutomationClients[2] != "claude-desktop" {
		t.Fatalf("expected default task automation clients, got %#v", cfg.TaskAutomationClients)
	}
	if cfg.TaskAutomationFallbackURL != "https://www.douyin.com" || cfg.TaskAutomationIdleSeconds != 5 || cfg.TaskAutomationReturnDelaySeconds != 3 {
		t.Fatalf("expected default task automation timing, got fallback=%q idle=%d delay=%d", cfg.TaskAutomationFallbackURL, cfg.TaskAutomationIdleSeconds, cfg.TaskAutomationReturnDelaySeconds)
	}
	if cfg.ZhipuBaseURL == "" || cfg.MiniMaxBaseURL == "" || cfg.GeminiBaseURL == "" || cfg.OpenRouterBaseURL == "" || cfg.TokenRouterBaseURL == "" || cfg.Sub2APIBaseURL == "" || cfg.ZoBaseURL == "" {
		t.Fatalf("expected new provider default base urls, got zhipu=%q minimax=%q gemini=%q openrouter=%q tokenrouter=%q sub2api=%q zo=%q", cfg.ZhipuBaseURL, cfg.MiniMaxBaseURL, cfg.GeminiBaseURL, cfg.OpenRouterBaseURL, cfg.TokenRouterBaseURL, cfg.Sub2APIBaseURL, cfg.ZoBaseURL)
	}

	cfg = Normalize(Config{
		SchedulingMode:                   "BALANCED",
		WebSocketMode:                    "DISABLED",
		OutboundProxyEnabled:             true,
		OutboundProxyURL:                 "mixed:10808",
		OutboundProxyProviders:           []string{"codex", " ", "OPENROUTER", "zo-computer", "codex"},
		OutboundProxyModels:              []string{"gpt-5.4", " ", "GPT-5.4", "claude-*"},
		XiaomiCredentialPriority:         "api",
		TaskAutomationClients:            []string{"Codex", "claudecode", "claude-code-desktop", "codex", "unknown"},
		TaskAutomationIdleSeconds:        900,
		TaskAutomationReturnDelaySeconds: 900,
	})
	if cfg.SchedulingMode != SchedulingModeBalanced {
		t.Fatalf("expected balanced scheduling, got %q", cfg.SchedulingMode)
	}
	if cfg.WebSocketMode != WebSocketModeDisabled {
		t.Fatalf("expected websocket disabled, got %q", cfg.WebSocketMode)
	}
	if cfg.XiaomiCredentialPriority != MimoCredentialPriorityAPIKey {
		t.Fatalf("expected MiMo API priority, got %q", cfg.XiaomiCredentialPriority)
	}
	if !cfg.OutboundProxyEnabled || cfg.OutboundProxyURL != "http://127.0.0.1:10808" {
		t.Fatalf("expected normalized outbound proxy, enabled=%v url=%q", cfg.OutboundProxyEnabled, cfg.OutboundProxyURL)
	}
	if len(cfg.OutboundProxyModels) != 2 || cfg.OutboundProxyModels[0] != "gpt-5.4" || cfg.OutboundProxyModels[1] != "claude-*" {
		t.Fatalf("expected normalized outbound proxy models, got %#v", cfg.OutboundProxyModels)
	}
	if len(cfg.OutboundProxyProviders) != 3 || cfg.OutboundProxyProviders[0] != "openai" || cfg.OutboundProxyProviders[1] != "openrouter" || cfg.OutboundProxyProviders[2] != "zo" {
		t.Fatalf("expected normalized outbound proxy providers, got %#v", cfg.OutboundProxyProviders)
	}
	if len(cfg.TaskAutomationClients) != 3 || cfg.TaskAutomationClients[0] != "codex" || cfg.TaskAutomationClients[1] != "claude" || cfg.TaskAutomationClients[2] != "claude-desktop" {
		t.Fatalf("expected normalized task automation clients, got %#v", cfg.TaskAutomationClients)
	}
	if cfg.TaskAutomationIdleSeconds != 600 {
		t.Fatalf("expected capped task automation idle seconds, got %d", cfg.TaskAutomationIdleSeconds)
	}
	if cfg.TaskAutomationReturnDelaySeconds != 600 {
		t.Fatalf("expected capped task automation return delay seconds, got %d", cfg.TaskAutomationReturnDelaySeconds)
	}
}
