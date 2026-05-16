package proxy

import (
	"bytes"
	"encoding/json"
	"strings"

	"OmniProxyBackend/internal/token"
)

func normalizeSub2APIRequestBody(originalPath string, route routeInfo, body []byte) ([]byte, bool) {
	if token.NormalizeProvider(route.Provider) != token.ProviderSub2API || route.Protocol != "openai" {
		return body, false
	}
	if !isSub2APIResponsesPath(originalPath) {
		return body, false
	}
	return stripOpenAIImageGenerationTools(body)
}

func isSub2APIResponsesPath(path string) bool {
	path = stripPathPrefix(path, "/sub2api")
	path = stripPathPrefix(path, "/v1")
	return path == "/responses" || strings.HasPrefix(path, "/responses/")
}

func stripOpenAIImageGenerationTools(body []byte) ([]byte, bool) {
	if len(bytes.TrimSpace(body)) == 0 {
		return body, false
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var payload map[string]any
	if err := decoder.Decode(&payload); err != nil {
		return body, false
	}

	changed := false
	if tools, ok := payload["tools"].([]any); ok {
		filtered := make([]any, 0, len(tools))
		for _, raw := range tools {
			if openAIToolType(raw) == "image_generation" {
				changed = true
				continue
			}
			filtered = append(filtered, raw)
		}
		if changed {
			if len(filtered) == 0 {
				delete(payload, "tools")
			} else {
				payload["tools"] = filtered
			}
		}
	}

	if openAIToolChoiceType(payload["tool_choice"]) == "image_generation" {
		delete(payload, "tool_choice")
		changed = true
	}

	if !changed {
		return body, false
	}
	updated, err := json.Marshal(payload)
	if err != nil {
		return body, false
	}
	return updated, true
}

func openAIToolType(value any) string {
	item, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	toolType, _ := stringFromAny(item["type"])
	return strings.ToLower(toolType)
}

func openAIToolChoiceType(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.ToLower(strings.TrimSpace(typed))
	case map[string]any:
		for _, key := range []string{"type", "tool.type", "function.name"} {
			if text, ok := stringFromAny(typed[key]); ok {
				return strings.ToLower(text)
			}
		}
		if tool, ok := typed["tool"].(map[string]any); ok {
			if text, ok := stringFromAny(tool["type"]); ok {
				return strings.ToLower(text)
			}
		}
		if function, ok := typed["function"].(map[string]any); ok {
			if text, ok := stringFromAny(function["name"]); ok {
				return strings.ToLower(text)
			}
		}
	}
	return ""
}
