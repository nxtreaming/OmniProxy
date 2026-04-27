package token

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)

func ExtractCodexEmail(authJSON string) (string, bool) {
	var data struct {
		Tokens struct {
			IDToken string `json:"id_token"`
		} `json:"tokens"`
	}
	if err := json.Unmarshal([]byte(authJSON), &data); err != nil {
		return "", false
	}
	if strings.TrimSpace(data.Tokens.IDToken) == "" {
		return "", false
	}

	payload, ok := decodeJWTPayload(data.Tokens.IDToken)
	if !ok {
		return "", false
	}

	if profile, ok := payload["https://api.openai.com/profile"].(map[string]any); ok {
		if email, ok := profile["email"].(string); ok && strings.TrimSpace(email) != "" {
			return strings.TrimSpace(email), true
		}
	}
	if email, ok := payload["email"].(string); ok && strings.TrimSpace(email) != "" {
		return strings.TrimSpace(email), true
	}
	return "", false
}

func decodeJWTPayload(jwt string) (map[string]any, bool) {
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		return nil, false
	}

	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		raw, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return nil, false
		}
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, false
	}
	return payload, true
}
