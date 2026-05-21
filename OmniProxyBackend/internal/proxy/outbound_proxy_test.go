package proxy

import "testing"

func TestOutboundProxyMatchesModelPatterns(t *testing.T) {
	patterns := []string{"gpt-5.4", "claude-*", "*/*"}

	for _, model := range []string{"gpt-5.4", "CLAUDE-SONNET-4-6", "openai/gpt-5.5", "anthropic/claude-test"} {
		if !outboundProxyMatchesModel(model, patterns) {
			t.Fatalf("expected %q to match %#v", model, patterns)
		}
	}

	for _, model := range []string{"gpt-4.1", "gemini-3-pro-preview", ""} {
		if outboundProxyMatchesModel(model, patterns) {
			t.Fatalf("expected %q not to match %#v", model, patterns)
		}
	}
}
