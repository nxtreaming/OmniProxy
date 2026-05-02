package proxy

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	codexOAuthClientID       = "app_EMoamEEZ73f0CkXaXp7hrann"
	codexOAuthTokenEndpoint  = "https://auth.openai.com/oauth/token"
	codexAccessRefreshMargin = 30 * time.Minute
)

type codexRefreshResponse struct {
	AccessToken  string `json:"access_token"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

type codexRefreshError struct {
	ErrorName        string `json:"error"`
	ErrorDescription string `json:"error_description"`
	Message          string `json:"message"`
}

var httpPostForm = func(ctx context.Context, client *http.Client, endpoint string, values url.Values) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return client.Do(req)
}

func RefreshCodexAuthJSON(ctx context.Context, client *http.Client, raw string, force bool, now time.Time) (string, bool, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if now.IsZero() {
		now = time.Now()
	}

	auth, tokens, topLevelTokens, err := parseCodexAuth(raw)
	if err != nil {
		return "", false, err
	}
	accessToken := stringMapValue(tokens, "access_token")
	refreshToken := stringMapValue(tokens, "refresh_token")
	accessExpiredOrExpiring := codexAccessTokenExpiredOrExpiring(accessToken, now)
	if !accessExpiredOrExpiring {
		accessExpiredOrExpiring = codexAuthExpiredOrExpiring(auth, now)
	}
	if strings.TrimSpace(refreshToken) == "" {
		if force || accessExpiredOrExpiring {
			return "", false, errors.New("codex auth.json does not contain refresh_token")
		}
		return raw, false, nil
	}
	if !force && !accessExpiredOrExpiring {
		return raw, false, nil
	}

	values := url.Values{}
	values.Set("grant_type", "refresh_token")
	values.Set("client_id", codexOAuthClientID)
	values.Set("refresh_token", refreshToken)

	resp, err := httpPostForm(ctx, client, codexOAuthTokenEndpoint, values)
	if err != nil {
		return "", false, fmt.Errorf("refresh codex token: %w", err)
	}
	defer closeBody(resp.Body)

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", false, fmt.Errorf("refresh codex token returned %d: %s", resp.StatusCode, codexRefreshErrorMessage(body, resp.Status))
	}

	var refresh codexRefreshResponse
	if err := json.Unmarshal(body, &refresh); err != nil {
		return "", false, fmt.Errorf("decode codex refresh response: %w", err)
	}
	if strings.TrimSpace(refresh.AccessToken) == "" {
		return "", false, errors.New("codex refresh response did not include access_token")
	}

	tokens["access_token"] = strings.TrimSpace(refresh.AccessToken)
	if strings.TrimSpace(refresh.IDToken) != "" {
		tokens["id_token"] = strings.TrimSpace(refresh.IDToken)
	}
	if strings.TrimSpace(refresh.RefreshToken) != "" {
		tokens["refresh_token"] = strings.TrimSpace(refresh.RefreshToken)
	}
	if !topLevelTokens {
		auth["tokens"] = tokens
	}
	if topLevelTokens && refresh.ExpiresIn > 0 {
		auth["expired"] = now.UTC().Add(time.Duration(refresh.ExpiresIn) * time.Second).Format(time.RFC3339)
	}
	auth["last_refresh"] = now.UTC().Format(time.RFC3339Nano)

	updated, err := json.MarshalIndent(auth, "", "  ")
	if err != nil {
		return "", false, err
	}
	return string(updated), true, nil
}

func parseCodexAuth(raw string) (map[string]any, map[string]any, bool, error) {
	var auth map[string]any
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&auth); err != nil {
		return nil, nil, false, fmt.Errorf("codex auth.json must be valid JSON: %w", err)
	}
	tokensValue, ok := auth["tokens"]
	if !ok {
		if hasTopLevelCodexTokenFields(auth) {
			return auth, auth, true, nil
		}
		return nil, nil, false, errors.New("codex auth.json does not contain tokens or top-level token fields")
	}
	tokens, ok := tokensValue.(map[string]any)
	if !ok {
		return nil, nil, false, errors.New("codex auth.json tokens must be an object")
	}
	return auth, tokens, false, nil
}

func hasTopLevelCodexTokenFields(auth map[string]any) bool {
	if auth == nil {
		return false
	}
	for _, key := range []string{"access_token", "refresh_token", "id_token", "OPENAI_API_KEY"} {
		if stringMapValue(auth, key) != "" {
			return true
		}
	}
	return false
}

func codexAccessTokenExpiredOrExpiring(accessToken string, now time.Time) bool {
	expiresAt, ok := jwtExpiresAt(accessToken)
	if !ok {
		return false
	}
	return !expiresAt.After(now.Add(codexAccessRefreshMargin))
}

func codexAuthExpiredOrExpiring(auth map[string]any, now time.Time) bool {
	expiresAt, ok := codexAuthExpiresAt(auth)
	if !ok {
		return false
	}
	return !expiresAt.After(now.Add(codexAccessRefreshMargin))
}

func codexAuthExpiresAt(auth map[string]any) (time.Time, bool) {
	if auth == nil {
		return time.Time{}, false
	}
	for _, key := range []string{"expired", "expires_at", "expiresAt"} {
		if value, ok := auth[key]; ok {
			if expiresAt, ok := parseCodexExpiryValue(value); ok {
				return expiresAt, true
			}
		}
	}
	return time.Time{}, false
}

func parseCodexExpiryValue(value any) (time.Time, bool) {
	switch typed := value.(type) {
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return time.Time{}, false
		}
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
			if parsed, err := time.Parse(layout, text); err == nil {
				return parsed, true
			}
		}
	case json.Number:
		seconds, err := typed.Int64()
		if err == nil && seconds > 0 {
			return time.Unix(seconds, 0), true
		}
	case float64:
		if typed > 0 {
			return time.Unix(int64(typed), 0), true
		}
	}
	return time.Time{}, false
}

func jwtExpiresAt(jwt string) (time.Time, bool) {
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		return time.Time{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		payload, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return time.Time{}, false
		}
	}
	var data struct {
		Exp json.Number `json:"exp"`
	}
	if err := json.NewDecoder(bytes.NewReader(payload)).Decode(&data); err != nil {
		return time.Time{}, false
	}
	if data.Exp == "" {
		return time.Time{}, false
	}
	seconds, err := data.Exp.Int64()
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(seconds, 0), true
}

func stringMapValue(values map[string]any, key string) string {
	value, ok := values[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func codexRefreshErrorMessage(body []byte, fallback string) string {
	var payload codexRefreshError
	if err := json.Unmarshal(body, &payload); err == nil {
		switch {
		case strings.TrimSpace(payload.ErrorDescription) != "":
			return strings.TrimSpace(payload.ErrorDescription)
		case strings.TrimSpace(payload.Message) != "":
			return strings.TrimSpace(payload.Message)
		case strings.TrimSpace(payload.ErrorName) != "":
			return strings.TrimSpace(payload.ErrorName)
		}
	}
	text := strings.TrimSpace(string(body))
	if text != "" {
		if len(text) > 240 {
			text = text[:240]
		}
		return text
	}
	return fallback
}
