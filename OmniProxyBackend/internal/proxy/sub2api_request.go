package proxy

import (
	"bytes"
	"encoding/json"
	"strings"

	"omniproxy/internal/token"
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
	var payload any
	if err := decoder.Decode(&payload); err != nil {
		return body, false
	}

	changed := sanitizeOpenAIImageGenerationFields(payload)
	if !changed {
		return body, false
	}
	updated, err := json.Marshal(payload)
	if err != nil {
		return body, false
	}
	return updated, true
}

func sanitizeOpenAIImageGenerationFields(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		return sanitizeOpenAIImageGenerationObject(typed)
	case []any:
		changed := false
		for _, item := range typed {
			if sanitizeOpenAIImageGenerationFields(item) {
				changed = true
			}
		}
		return changed
	default:
		return false
	}
}

func sanitizeOpenAIImageGenerationObject(payload map[string]any) bool {
	changed := false
	for key, raw := range payload {
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "tools":
			if tools, ok := raw.([]any); ok {
				filtered := make([]any, 0, len(tools))
				for _, tool := range tools {
					if openAIToolType(tool) == "image_generation" {
						changed = true
						continue
					}
					if sanitizeOpenAIImageGenerationFields(tool) {
						changed = true
					}
					filtered = append(filtered, tool)
				}
				if len(filtered) == 0 {
					delete(payload, key)
				} else {
					payload[key] = filtered
				}
			}
		case "tool_choice":
			if openAIToolChoiceType(raw) == "image_generation" {
				delete(payload, key)
				changed = true
			} else if sanitizeOpenAIImageGenerationFields(raw) {
				changed = true
			}
		case "include":
			if includes, ok := raw.([]any); ok {
				filtered := make([]any, 0, len(includes))
				for _, include := range includes {
					if openAIIncludeSelectsImageGeneration(include) {
						changed = true
						continue
					}
					filtered = append(filtered, include)
				}
				if len(filtered) == 0 {
					delete(payload, key)
				} else {
					payload[key] = filtered
				}
			}
		default:
			if sanitizeOpenAIImageGenerationFields(raw) {
				changed = true
			}
		}
	}
	return changed
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

func openAIIncludeSelectsImageGeneration(value any) bool {
	text, ok := stringFromAny(value)
	if !ok {
		return false
	}
	return strings.Contains(strings.ToLower(text), "image_generation")
}
