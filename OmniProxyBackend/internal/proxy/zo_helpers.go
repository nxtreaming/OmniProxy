package proxy

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func extractJSONObjectCandidates(text string) []map[string]any {
	var out []map[string]any
	start := -1
	depth := 0
	inString := false
	escaped := false
	for i, char := range text {
		if inString {
			if escaped {
				escaped = false
			} else if char == '\\' {
				escaped = true
			} else if char == '"' {
				inString = false
			}
			continue
		}
		if char == '"' {
			inString = true
			continue
		}
		if char == '{' {
			if depth == 0 {
				start = i
			}
			depth++
		} else if char == '}' {
			depth--
			if depth == 0 && start >= 0 {
				var obj map[string]any
				if err := json.Unmarshal([]byte(text[start:i+1]), &obj); err == nil && isZoProxyOutputObject(obj) {
					out = append(out, obj)
				}
				start = -1
			}
			if depth < 0 {
				depth = 0
			}
		}
	}
	return out
}

func isZoProxyOutputObject(obj map[string]any) bool {
	_, hasText := obj["text"]
	_, hasToolName := obj["tool_name"]
	_, hasToolArgs := obj["tool_args"]
	_, hasName := obj["name"]
	_, hasArguments := obj["arguments"]
	return hasText || hasToolName || hasToolArgs || (hasName && hasArguments)
}

func toolSpec(tool any) map[string]any {
	obj, _ := tool.(map[string]any)
	if fn, ok := obj["function"].(map[string]any); ok {
		return fn
	}
	return obj
}

func toolSchema(spec map[string]any) map[string]any {
	if schema, ok := spec["parameters"].(map[string]any); ok {
		return schema
	}
	if schema, ok := spec["input_schema"].(map[string]any); ok {
		return schema
	}
	return map[string]any{}
}

func clientToolNames(tools []any) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		if name := stringFromMap(toolSpec(tool), "name"); name != "" {
			names = append(names, name)
		}
	}
	return names
}

func schemaParamNames(schema map[string]any) []string {
	properties, _ := schema["properties"].(map[string]any)
	names := make([]string, 0, len(properties))
	for name := range properties {
		names = append(names, name)
	}
	return names
}

func stringFromMap(value map[string]any, key string) string {
	if value == nil {
		return ""
	}
	return anyString(value[key])
}

func anyString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return ""
	}
}

func mustJSONText(value any) string {
	if value == nil {
		return ""
	}
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(data)
}

func zoTimestamp() int64 {
	return time.Now().Unix()
}

func zoShortID(prefix string) string {
	id := strings.ReplaceAll(zoUUID(), "-", "")
	if len(id) > 24 {
		id = id[:24]
	}
	return prefix + id
}

func zoUUID() string {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return hex.EncodeToString([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	}
	data[6] = (data[6] & 0x0f) | 0x40
	data[8] = (data[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", data[0:4], data[4:6], data[6:8], data[8:10], data[10:16])
}
