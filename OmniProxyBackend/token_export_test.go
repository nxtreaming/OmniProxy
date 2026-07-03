package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"omniproxy/internal/token"
)

func TestEncodeTokenExportIncludesCredentials(t *testing.T) {
	items := []token.Token{{
		ID:             "tok-1",
		Name:           "primary",
		Provider:       token.ProviderOpenAI,
		CredentialType: token.CredentialTypeAPIKey,
		TokenValue:     "sk-export-token",
		Remaining:      88,
		Status:         token.StatusActive,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}}

	data, err := encodeTokenExport(items, time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "sk-export-token") {
		t.Fatalf("expected export to include credential for backup restore, got %s", string(data))
	}

	var payload tokenExportPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Version != "1" || len(payload.Tokens) != 1 || payload.Tokens[0].TokenValue != "sk-export-token" {
		t.Fatalf("unexpected token export payload: %#v", payload)
	}
}

func TestCodexAuthExportFilesUseSafeNames(t *testing.T) {
	items := []token.Token{
		{
			ID:             "codex-1",
			Name:           "coder@example.com",
			Provider:       token.ProviderOpenAI,
			CredentialType: token.CredentialTypeCodexAuthJSON,
			TokenValue:     `{"tokens":{"access_token":"first"}}`,
		},
		{
			ID:             "codex-2",
			Name:           "coder@example.com",
			Provider:       token.ProviderOpenAI,
			CredentialType: token.CredentialTypeCodexAuthJSON,
			TokenValue:     `{"tokens":{"access_token":"second"}}`,
		},
		{
			ID:             "api-key",
			Name:           "plain",
			Provider:       token.ProviderOpenAI,
			CredentialType: token.CredentialTypeAPIKey,
			TokenValue:     "sk-not-codex",
		},
	}

	files := codexAuthExportFiles(items, "20260429-100000")
	if len(files) != 2 {
		t.Fatalf("expected 2 codex auth files, got %#v", files)
	}
	if files[0].Name != "codex-auth-coder-example.com-20260429-100000.json" {
		t.Fatalf("unexpected first filename: %q", files[0].Name)
	}
	if files[1].Name != "codex-auth-coder-example.com-20260429-100000-2.json" {
		t.Fatalf("unexpected duplicate filename: %q", files[1].Name)
	}
	if !strings.HasSuffix(files[0].Content, "\n") || !strings.Contains(files[1].Content, "second") {
		t.Fatalf("unexpected codex auth file contents: %#v", files)
	}
}
