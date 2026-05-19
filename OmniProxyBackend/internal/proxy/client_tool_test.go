package proxy

import (
	"net/http/httptest"
	"testing"
)

func TestClientInfoDistinguishesClaudeDesktop(t *testing.T) {
	req := httptest.NewRequest("POST", "/claude-desktop/v1/messages", nil)
	info := clientInfoForRequest(req, routeInfo{})
	if info.Key != clientClaudeDesktop || info.Name != "Claude Code Desktop" {
		t.Fatalf("unexpected desktop client info: %#v", info)
	}

	info = clientInfoFromLabel("Claude Code Desktop")
	if info.Key != clientClaudeDesktop || info.Name != "Claude Code Desktop" {
		t.Fatalf("unexpected desktop label client info: %#v", info)
	}
}
