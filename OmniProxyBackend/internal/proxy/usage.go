package proxy

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"OmniProxyBackend/internal/token"
)

const maxUsageCaptureBytes = 2 * 1024 * 1024

type usageCapture struct {
	buf bytes.Buffer
}

func (c *usageCapture) Write(p []byte) (int, error) {
	if c.buf.Len() < maxUsageCaptureBytes {
		remaining := maxUsageCaptureBytes - c.buf.Len()
		if len(p) > remaining {
			_, _ = c.buf.Write(p[:remaining])
		} else {
			_, _ = c.buf.Write(p)
		}
	}
	return len(p), nil
}

func (c *usageCapture) Bytes() []byte {
	return c.buf.Bytes()
}

func parseTokenConsumption(header http.Header, body []byte) token.TokenConsumption {
	if len(body) == 0 {
		return token.TokenConsumption{}
	}

	contentType := strings.ToLower(header.Get("Content-Type"))
	if strings.Contains(contentType, "text/event-stream") || bytes.Contains(body, []byte("data:")) {
		if usage, ok := parseSSETokenConsumption(body); ok {
			return usage
		}
	}

	usage, _ := parseJSONTokenConsumption(body)
	return usage
}

func parseSSETokenConsumption(body []byte) (token.TokenConsumption, bool) {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 0, 64*1024), maxUsageCaptureBytes)

	var found token.TokenConsumption
	ok := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		if usage, usageOK := parseJSONTokenConsumption([]byte(data)); usageOK {
			found = usage
			ok = true
		}
	}
	return found, ok
}

func parseJSONTokenConsumption(body []byte) (token.TokenConsumption, bool) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()

	var payload any
	if err := decoder.Decode(&payload); err != nil {
		return token.TokenConsumption{}, false
	}
	return findTokenConsumption(payload)
}

func findTokenConsumption(value any) (token.TokenConsumption, bool) {
	switch typed := value.(type) {
	case map[string]any:
		if usageValue, ok := typed["usage"]; ok {
			if usage, usageOK := consumeUsageObject(usageValue); usageOK {
				return usage, true
			}
			if usage, usageOK := findTokenConsumption(usageValue); usageOK {
				return usage, true
			}
		}
		if usage, ok := consumeUsageMap(typed); ok {
			return usage, true
		}

		var found token.TokenConsumption
		ok := false
		for _, child := range typed {
			if usage, usageOK := findTokenConsumption(child); usageOK {
				found = usage
				ok = true
			}
		}
		return found, ok
	case []any:
		var found token.TokenConsumption
		ok := false
		for _, child := range typed {
			if usage, usageOK := findTokenConsumption(child); usageOK {
				found = usage
				ok = true
			}
		}
		return found, ok
	default:
		return token.TokenConsumption{}, false
	}
}

func consumeUsageObject(value any) (token.TokenConsumption, bool) {
	usageMap, ok := value.(map[string]any)
	if !ok {
		return token.TokenConsumption{}, false
	}
	return consumeUsageMap(usageMap)
}

func consumeUsageMap(value map[string]any) (token.TokenConsumption, bool) {
	input := intFromKeys(value, "input_tokens", "prompt_tokens")
	output := intFromKeys(value, "output_tokens", "completion_tokens")
	total := intFromKeys(value, "total_tokens")
	if total == 0 && (input > 0 || output > 0) {
		total = input + output
	}
	if input == 0 && output == 0 && total == 0 {
		return token.TokenConsumption{}, false
	}
	return token.TokenConsumption{
		InputTokens:  input,
		OutputTokens: output,
		TotalTokens:  total,
	}, true
}

func intFromKeys(value map[string]any, keys ...string) int {
	for _, key := range keys {
		if parsed, ok := intFromAny(value[key]); ok {
			return parsed
		}
	}
	return 0
}

func intFromAny(value any) (int, bool) {
	switch typed := value.(type) {
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil && parsed >= 0 {
			return int(parsed), true
		}
	case float64:
		if typed >= 0 {
			return int(typed), true
		}
	case int:
		if typed >= 0 {
			return typed, true
		}
	case int64:
		if typed >= 0 {
			return int(typed), true
		}
	}
	return 0, false
}
