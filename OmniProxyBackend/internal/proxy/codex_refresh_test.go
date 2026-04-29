package proxy

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestRefreshCodexAuthJSONUsesRefreshToken(t *testing.T) {
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	var form url.Values
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if got := r.Header.Get("Content-Type"); !strings.Contains(got, "application/x-www-form-urlencoded") {
			t.Fatalf("unexpected content type: %q", got)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		form = r.PostForm
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"access_token": "new-access-token",
			"id_token": "new-id-token",
			"refresh_token": "new-refresh-token",
			"token_type": "Bearer",
			"expires_in": 864000
		}`))
	}))
	defer upstream.Close()

	restore := replaceHTTPPostFormForTest(func(ctx context.Context, client *http.Client, endpoint string, values url.Values) (*http.Response, error) {
		if endpoint != codexOAuthTokenEndpoint {
			t.Fatalf("unexpected endpoint: %s", endpoint)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstream.URL, strings.NewReader(values.Encode()))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return upstream.Client().Do(req)
	})
	defer restore()

	raw := `{
		"auth_mode": "chatgpt",
		"tokens": {
			"access_token": "` + jwtWithExp(t, now.Add(time.Minute)) + `",
			"id_token": "` + jwtWithExp(t, now.Add(time.Hour)) + `",
			"refresh_token": "old-refresh-token",
			"account_id": "account-123"
		}
	}`
	updated, refreshed, err := RefreshCodexAuthJSON(context.Background(), upstream.Client(), raw, false, now)
	if err != nil {
		t.Fatal(err)
	}
	if !refreshed {
		t.Fatal("expected refresh")
	}
	if form.Get("grant_type") != "refresh_token" {
		t.Fatalf("unexpected grant_type: %q", form.Get("grant_type"))
	}
	if form.Get("client_id") != codexOAuthClientID {
		t.Fatalf("unexpected client_id: %q", form.Get("client_id"))
	}
	if form.Get("refresh_token") != "old-refresh-token" {
		t.Fatalf("unexpected refresh_token: %q", form.Get("refresh_token"))
	}

	var saved struct {
		Tokens struct {
			AccessToken  string `json:"access_token"`
			IDToken      string `json:"id_token"`
			RefreshToken string `json:"refresh_token"`
			AccountID    string `json:"account_id"`
		} `json:"tokens"`
		LastRefresh string `json:"last_refresh"`
	}
	if err := json.Unmarshal([]byte(updated), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Tokens.AccessToken != "new-access-token" || saved.Tokens.IDToken != "new-id-token" || saved.Tokens.RefreshToken != "new-refresh-token" {
		t.Fatalf("unexpected refreshed tokens: %#v", saved.Tokens)
	}
	if saved.Tokens.AccountID != "account-123" {
		t.Fatalf("account id should be preserved, got %q", saved.Tokens.AccountID)
	}
	if saved.LastRefresh != now.Format(time.RFC3339Nano) {
		t.Fatalf("unexpected last_refresh: %q", saved.LastRefresh)
	}
}

func TestRefreshCodexAuthJSONSkipsFreshAccessToken(t *testing.T) {
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	restore := replaceHTTPPostFormForTest(func(context.Context, *http.Client, string, url.Values) (*http.Response, error) {
		t.Fatal("refresh endpoint should not be called")
		return nil, nil
	})
	defer restore()

	raw := `{"tokens":{"access_token":"` + jwtWithExp(t, now.Add(2*time.Hour)) + `","refresh_token":"refresh-token"}}`
	updated, refreshed, err := RefreshCodexAuthJSON(context.Background(), nil, raw, false, now)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed {
		t.Fatal("expected no refresh")
	}
	if updated != raw {
		t.Fatalf("expected original auth JSON")
	}
}

func TestRefreshCodexAuthJSONFailsWhenExpiredAndRefreshTokenMissing(t *testing.T) {
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	raw := `{"tokens":{"access_token":"` + jwtWithExp(t, now.Add(-time.Minute)) + `"}}`
	if _, _, err := RefreshCodexAuthJSON(context.Background(), nil, raw, false, now); err == nil {
		t.Fatal("expected missing refresh token error")
	}
}

func replaceHTTPPostFormForTest(fn func(context.Context, *http.Client, string, url.Values) (*http.Response, error)) func() {
	previous := httpPostForm
	httpPostForm = fn
	return func() {
		httpPostForm = previous
	}
}

func jwtWithExp(t *testing.T, exp time.Time) string {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"exp": exp.Unix(),
		"https://api.openai.com/profile": map[string]string{
			"email": "coder@example.com",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return "header." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
}
