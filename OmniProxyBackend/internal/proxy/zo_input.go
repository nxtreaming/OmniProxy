package proxy

import (
	"fmt"
	"net/url"
	"strings"
)

func joinZoURL(baseURL string, path string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	out := *base
	out.Path = singleJoiningSlash(base.Path, path)
	out.RawQuery = ""
	return out.String(), nil
}

func buildZoInputFromOpenAI(messages []any) string {
	if len(messages) == 0 {
		return ""
	}
	parts := make([]string, 0, len(messages))
	for _, item := range messages {
		msg, _ := item.(map[string]any)
		role := stringFromMap(msg, "role")
		if role == "" {
			role = "user"
		}
		parts = append(parts, fmt.Sprintf("[%s]: %s", role, extractZoText(msg["content"])))
	}
	return strings.Join(parts, "\n")
}

func buildZoInputFromAnthropic(system any, messages []any) string {
	parts := []string{}
	if system != nil {
		text := extractZoText(system)
		if strings.TrimSpace(text) != "" {
			parts = append(parts, "[system]: "+text)
		}
	}
	for _, item := range messages {
		msg, _ := item.(map[string]any)
		role := stringFromMap(msg, "role")
		if role == "" {
			role = "user"
		}
		parts = append(parts, fmt.Sprintf("[%s]: %s", role, extractZoText(msg["content"])))
	}
	return strings.Join(parts, "\n")
}

func buildZoInputFromResponses(instructions any, input any) string {
	parts := []string{}
	if text := strings.TrimSpace(extractZoResponsesText(instructions)); text != "" {
		parts = append(parts, "[instructions]: "+text)
	}
	if text := strings.TrimSpace(extractZoResponsesText(input)); text != "" {
		parts = append(parts, text)
	}
	return strings.Join(parts, "\n")
}

func extractZoResponsesText(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			text := extractZoResponsesText(item)
			if strings.TrimSpace(text) != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	case map[string]any:
		itemType := stringFromMap(typed, "type")
		switch itemType {
		case "message":
			role := stringFromMap(typed, "role")
			if role == "" {
				role = "user"
			}
			return fmt.Sprintf("[%s]: %s", role, extractZoResponsesText(typed["content"]))
		case "input_text", "output_text", "text":
			return stringFromMap(typed, "text")
		case "input_image", "image", "image_url":
			return "[Image]"
		case "function_call":
			return fmt.Sprintf("[Tool Use: %s(%s)]", stringFromMap(typed, "name"), mustJSONText(typed["arguments"]))
		case "function_call_output":
			return fmt.Sprintf("[Tool Result: %s]", anyString(typed["output"]))
		default:
			return extractZoText(value)
		}
	case nil:
		return ""
	default:
		return mustJSONText(value)
	}
}

func zoToolsList(value any) []any {
	switch typed := value.(type) {
	case nil:
		return nil
	case []any:
		return typed
	case map[string]any:
		if tools, ok := typed["tools"].([]any); ok {
			return tools
		}
		return []any{typed}
	default:
		return nil
	}
}

func truthyZoBool(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "1", "true", "yes", "on":
			return true
		default:
			return false
		}
	case float64:
		return typed != 0
	default:
		return false
	}
}

func extractZoText(content any) string {
	switch typed := content.(type) {
	case string:
		return typed
	case []any:
		parts := make([]string, 0, len(typed))
		for _, block := range typed {
			blockMap, ok := block.(map[string]any)
			if !ok {
				parts = append(parts, mustJSONText(block))
				continue
			}
			switch stringFromMap(blockMap, "type") {
			case "text":
				parts = append(parts, stringFromMap(blockMap, "text"))
			case "image", "image_url":
				parts = append(parts, "[Image]")
			case "tool_use":
				parts = append(parts, fmt.Sprintf("[Tool Use: %s(%s)]", stringFromMap(blockMap, "name"), mustJSONText(blockMap["input"])))
			case "tool_result":
				parts = append(parts, fmt.Sprintf("[Tool Result: %s]", mustJSONText(blockMap["content"])))
			default:
				parts = append(parts, mustJSONText(block))
			}
		}
		return strings.Join(parts, "\n")
	case nil:
		return ""
	default:
		return mustJSONText(content)
	}
}

func injectZoTools(input string, tools []any) (string, any) {
	if len(tools) == 0 {
		return input, nil
	}
	names := clientToolNames(tools)
	var builder strings.Builder
	builder.WriteString("You have access to the following tools. To use a tool, set tool_name to the tool name and tool_args to a JSON string of its arguments. If no tool is needed, leave tool_name and tool_args as empty strings and put your answer in text.\n\nAvailable tools:\n")
	for _, tool := range tools {
		spec := toolSpec(tool)
		name := stringFromMap(spec, "name")
		if name == "" {
			continue
		}
		builder.WriteString("\n  ")
		builder.WriteString(name)
		builder.WriteString(": ")
		builder.WriteString(stringFromMap(spec, "description"))
		builder.WriteString("\n")
		for _, param := range schemaParamNames(toolSchema(spec)) {
			builder.WriteString("    ")
			builder.WriteString(param)
			builder.WriteString("\n")
		}
	}
	builder.WriteString("\nResponse rules:\n")
	builder.WriteString("- If using a tool: set tool_name to one of [")
	for i, name := range names {
		if i > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(fmt.Sprintf("%q", name))
	}
	builder.WriteString("] and tool_args to a JSON string containing only the defined parameters.\n")
	builder.WriteString("- If not using a tool: leave tool_name and tool_args as empty strings, and put the full answer in text.\n")
	builder.WriteString("- Do not output anything outside the JSON structure.\n")
	builder.WriteString("\n---\nUser request:\n")
	builder.WriteString(input)

	return builder.String(), map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text":      map[string]string{"type": "string"},
			"tool_name": map[string]string{"type": "string"},
			"tool_args": map[string]string{"type": "string"},
		},
		"required": []string{"text", "tool_name", "tool_args"},
	}
}
