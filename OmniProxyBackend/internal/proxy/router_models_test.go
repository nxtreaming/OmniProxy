package proxy

import (
	"testing"

	"omniproxy/internal/token"
)

func TestRouterProviderInferenceTables(t *testing.T) {
	claudeCases := []struct {
		model string
		want  string
	}{
		{"deepseek-v4-pro", token.ProviderDeepSeek},
		{"kimi-for-coding", token.ProviderKimi},
		{"mimo-v2.5-pro", token.ProviderXiaomi},
		{"glm-5.1", token.ProviderZhipu},
		{"MiniMax-M2.7", token.ProviderMiniMax},
		{"claude-sonnet-4-6", token.ProviderZo},
		{"claude-opus-4-7", token.ProviderZo},
		{"claude-sonnet-4-5", token.ProviderAnthropic},
		{"", token.ProviderAnthropic},
	}
	for _, tt := range claudeCases {
		t.Run("claude "+tt.model, func(t *testing.T) {
			if got := providerForModel(tt.model); got != tt.want {
				t.Fatalf("providerForModel(%q) = %q, want %q", tt.model, got, tt.want)
			}
		})
	}

	openAICases := []struct {
		model string
		want  string
	}{
		{"deepseek-v4-pro", token.ProviderDeepSeek},
		{"kimi-for-coding", token.ProviderKimi},
		{"mimo-v2.5-pro", token.ProviderXiaomi},
		{"glm-5.1", token.ProviderZhipu},
		{"MiniMax-M2.7", token.ProviderMiniMax},
		{"auto:balance", token.ProviderTokenRouter},
		{"tokenrouter/auto", token.ProviderTokenRouter},
		{"openai/gpt-5.4", token.ProviderOpenRouter},
		{"custom-model", token.ProviderCustom},
		{"gpt-5.4", token.ProviderOpenAI},
		{"", token.ProviderOpenAI},
	}
	for _, tt := range openAICases {
		t.Run("openai "+tt.model, func(t *testing.T) {
			if got := providerForOpenCodeModel(tt.model); got != tt.want {
				t.Fatalf("providerForOpenCodeModel(%q) = %q, want %q", tt.model, got, tt.want)
			}
		})
	}
}
