package proxy

import (
	"encoding/json"
	"strings"
)

func parseZoOutput(output any) zoParsedOutput {
	switch typed := output.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		if strings.HasPrefix(trimmed, "{") {
			var obj map[string]any
			if err := json.Unmarshal([]byte(trimmed), &obj); err == nil {
				return parseZoOutput(obj)
			}
		}
		candidates := extractJSONObjectCandidates(trimmed)
		if len(candidates) > 0 {
			return parseZoOutput(candidates[len(candidates)-1])
		}
		return zoParsedOutput{Text: typed}
	case map[string]any:
		if text, ok := typed["text"].(string); ok {
			if candidates := extractJSONObjectCandidates(text); len(candidates) > 0 {
				inner := parseZoOutput(candidates[len(candidates)-1])
				if inner.Text != "" || len(inner.ToolCalls) > 0 {
					return inner
				}
			}
		}
		text := stringFromMap(typed, "text")
		if name := strings.TrimSpace(stringFromMap(typed, "tool_name")); name != "" {
			return zoParsedOutput{
				Text: text,
				ToolCalls: []zoToolCall{{
					Name:      name,
					Arguments: parseZoToolArgs(typed["tool_args"]),
				}},
			}
		}
		if toolCalls, ok := parseZoToolCalls(typed["tool_calls"]); ok {
			return zoParsedOutput{Text: text, ToolCalls: toolCalls}
		}
		if name := strings.TrimSpace(stringFromMap(typed, "name")); name != "" {
			return zoParsedOutput{
				Text: text,
				ToolCalls: []zoToolCall{{
					Name:      name,
					Arguments: parseZoToolArgs(typed["arguments"]),
				}},
			}
		}
		return zoParsedOutput{Text: text}
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, parseZoOutput(item).Text)
		}
		return zoParsedOutput{Text: strings.Join(parts, "\n")}
	default:
		return zoParsedOutput{Text: mustJSONText(output)}
	}
}

func parseZoToolCalls(value any) ([]zoToolCall, bool) {
	items, ok := value.([]any)
	if !ok {
		return nil, false
	}
	out := make([]zoToolCall, 0, len(items))
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := stringFromMap(obj, "name")
		args := parseZoToolArgs(obj["arguments"])
		if fn, ok := obj["function"].(map[string]any); ok {
			if name == "" {
				name = stringFromMap(fn, "name")
			}
			if len(args) == 0 {
				args = parseZoToolArgs(fn["arguments"])
			}
		}
		if name != "" {
			out = append(out, zoToolCall{Name: name, Arguments: args})
		}
	}
	return out, len(out) > 0
}

func parseZoToolArgs(value any) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		return typed
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return map[string]any{}
		}
		var parsed map[string]any
		if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
			return parsed
		}
		return map[string]any{}
	default:
		return map[string]any{}
	}
}

func normalizeZoParsedOutput(parsed zoParsedOutput, tools []any) zoParsedOutput {
	if len(parsed.ToolCalls) == 0 || len(tools) == 0 {
		return parsed
	}
	out := parsed
	out.ToolCalls = nil
	for _, call := range parsed.ToolCalls {
		name := mapZoToolName(call.Name, tools)
		if name == "" || !isAllowedZoTool(name, tools) {
			continue
		}
		out.ToolCalls = append(out.ToolCalls, zoToolCall{Name: name, Arguments: mapZoToolArgs(call.Arguments, name, tools)})
	}
	return out
}

func mapZoToolName(name string, tools []any) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	names := clientToolNames(tools)
	for _, candidate := range names {
		if name == candidate {
			return candidate
		}
	}
	lower := strings.ToLower(name)
	for _, candidate := range names {
		candidateLower := strings.ToLower(candidate)
		if strings.Contains(lower, candidateLower) || strings.Contains(candidateLower, lower) {
			return candidate
		}
	}
	return name
}

func isAllowedZoTool(name string, tools []any) bool {
	for _, candidate := range clientToolNames(tools) {
		if name == candidate {
			return true
		}
	}
	return false
}

func mapZoToolArgs(args map[string]any, toolName string, tools []any) map[string]any {
	if len(args) == 0 {
		return map[string]any{}
	}
	params := []string{}
	for _, tool := range tools {
		spec := toolSpec(tool)
		if stringFromMap(spec, "name") == toolName {
			params = schemaParamNames(toolSchema(spec))
			break
		}
	}
	if len(params) == 0 {
		return stripZoNoiseArgs(args)
	}

	filtered := map[string]any{}
	used := map[string]bool{}
	for _, param := range params {
		if value, ok := args[param]; ok {
			filtered[param] = value
			used[param] = true
		}
	}
	for _, param := range params {
		if _, ok := filtered[param]; ok {
			continue
		}
		paramLower := strings.ToLower(param)
		for key, value := range args {
			if used[key] {
				continue
			}
			keyLower := strings.ToLower(key)
			if strings.Contains(paramLower, keyLower) || strings.Contains(keyLower, paramLower) {
				filtered[param] = value
				used[key] = true
				break
			}
		}
	}
	if len(filtered) > 0 {
		return filtered
	}
	return stripZoNoiseArgs(args)
}

func stripZoNoiseArgs(args map[string]any) map[string]any {
	noise := map[string]bool{"description": true, "explanation": true, "reason": true, "note": true, "comment": true}
	out := map[string]any{}
	for key, value := range args {
		if !noise[strings.ToLower(key)] {
			out[key] = value
		}
	}
	return out
}
