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
	if cfg.XiaomiCredentialPriority != MimoCredentialPriorityTokenPlan {
		t.Fatalf("expected default MiMo token plan priority, got %q", cfg.XiaomiCredentialPriority)
	}

	cfg = Normalize(Config{
		SchedulingMode:           "BALANCED",
		WebSocketMode:            "DISABLED",
		XiaomiCredentialPriority: "api",
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
}
