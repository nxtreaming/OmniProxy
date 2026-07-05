package proxy

import (
	"bytes"
	"encoding/json"
	"strings"
)

func normalizeUpstreamModelID(model string) string {
	model = strings.TrimSpace(model)
	switch strings.ToLower(model) {
	case "deepseek-v4-pro[1m]":
		return "deepseek-v4-pro"
	default:
		return model
	}
}

func normalizeRequestBodyModel(body []byte) ([]byte, bool) {
	var payload map[string]any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return body, false
	}

	model, ok := payload["model"].(string)
	if !ok {
		return body, false
	}
	normalized := normalizeUpstreamModelID(model)
	if normalized == "" || normalized == strings.TrimSpace(model) {
		return body, false
	}
	payload["model"] = normalized
	updated, err := json.Marshal(payload)
	if err != nil {
		return body, false
	}
	return updated, true
}
