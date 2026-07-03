package token

import (
	"encoding/base64"
	"encoding/json"
	"omniproxy/internal/storage"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSecureStoreProtectsTokenValuesAndMigratesPlaintext(t *testing.T) {
	useTestSecureStoreCodec(t)

	path := filepath.Join(t.TempDir(), "tokens.json")
	rawStore := storage.NewJSONStore[[]Token](path)
	store := NewSecureStore(rawStore)

	manager, err := NewManager(store, 15)
	if err != nil {
		t.Fatal(err)
	}
	item, err := manager.Add(UpsertRequest{Name: "primary", Provider: ProviderOpenAI, TokenValue: "sk-secure-token"})
	if err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "sk-secure-token") {
		t.Fatalf("stored token file leaked plaintext secret: %s", string(raw))
	}

	reloaded, err := NewManager(store, 15)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := reloaded.Get(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.TokenValue != "sk-secure-token" {
		t.Fatalf("expected decrypted token value, got %q", loaded.TokenValue)
	}

	if err := rawStore.Save([]Token{{
		ID:             "legacy",
		Name:           "legacy",
		Provider:       ProviderOpenAI,
		CredentialType: CredentialTypeAPIKey,
		TokenValue:     "sk-legacy-token",
		Remaining:      100,
		Status:         StatusActive,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}}); err != nil {
		t.Fatal(err)
	}
	migrated, err := NewManager(store, 15)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := migrated.Get("legacy")
	if err != nil {
		t.Fatal(err)
	}
	if legacy.TokenValue != "sk-legacy-token" {
		t.Fatalf("expected migrated legacy token to decrypt, got %q", legacy.TokenValue)
	}
	raw, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "sk-legacy-token") {
		t.Fatalf("legacy token was not migrated to protected storage: %s", string(raw))
	}
}

func useTestSecureStoreCodec(t *testing.T) {
	t.Helper()

	const prefix = "omniproxy-secret:v1:test:"
	oldProtect := protectTokenValue
	oldUnprotect := unprotectTokenValue

	protectTokenValue = func(value string) (string, error) {
		if value == "" || strings.HasPrefix(value, prefix) {
			return value, nil
		}
		return prefix + base64.RawURLEncoding.EncodeToString([]byte(value)), nil
	}
	unprotectTokenValue = func(value string) (string, error) {
		if !strings.HasPrefix(value, prefix) {
			return value, nil
		}
		plain, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(value, prefix))
		if err != nil {
			return "", err
		}
		return string(plain), nil
	}

	t.Cleanup(func() {
		protectTokenValue = oldProtect
		unprotectTokenValue = oldUnprotect
	})
}

func TestManagerHealthCooldownCandidatesAndRecovery(t *testing.T) {
	manager, err := NewManager(storage.NewJSONStore[[]Token](filepath.Join(t.TempDir(), "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	item, err := manager.Add(UpsertRequest{Name: "primary", Provider: ProviderOpenAI, TokenValue: "sk-primary-token"})
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	cooldownUntil := now.Add(time.Hour)
	if err := manager.MarkExhaustedUntil(item.ID, "upstream returned 429", &cooldownUntil); err != nil {
		t.Fatal(err)
	}
	if candidates := manager.HealthCheckCandidates(now, time.Minute, time.Minute); len(candidates) != 0 {
		t.Fatalf("expected active cooldown to skip health check, got %#v", candidates)
	}

	afterCooldown := cooldownUntil.Add(time.Second)
	candidates := manager.HealthCheckCandidates(afterCooldown, time.Minute, time.Minute)
	if len(candidates) != 1 || candidates[0].ID != item.ID {
		t.Fatalf("expected expired cooldown to be checked, got %#v", candidates)
	}

	if err := manager.RecordUsage(item.ID, 80); err != nil {
		t.Fatal(err)
	}
	if err := manager.RecordHealthCheck(item.ID, true, 200, "OK", nil); err != nil {
		t.Fatal(err)
	}
	updated, err := manager.Get(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != StatusActive || updated.CooldownUntil != nil || updated.Health.ConsecutiveErrors != 0 {
		t.Fatalf("expected health recovery to clear cooldown and restore active status, got %#v", updated)
	}
}

func codexAuthJSONForTest(t *testing.T, email string) string {
	t.Helper()

	payload, err := json.Marshal(map[string]any{
		"https://api.openai.com/profile": map[string]any{
			"email": email,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	jwt := "header." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
	data, err := json.Marshal(map[string]any{
		"auth_mode": "chatgpt",
		"tokens": map[string]any{
			"id_token":      jwt,
			"access_token":  "codex-access-token",
			"refresh_token": "codex-refresh-token",
			"account_id":    "account-123",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

type countingTokenStore struct {
	tokens []Token
	saves  int
}

func (s *countingTokenStore) Load() ([]Token, error) {
	out := make([]Token, len(s.tokens))
	copy(out, s.tokens)
	return out, nil
}

func (s *countingTokenStore) Save(tokens []Token) error {
	s.saves++
	s.tokens = make([]Token, len(tokens))
	copy(s.tokens, tokens)
	return nil
}
