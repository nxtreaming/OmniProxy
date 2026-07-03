package proxy

import (
	"strings"
	"unicode"
)

func mapZoModel(clientModel string, models []zoModel) string {
	clientModel = strings.TrimSpace(clientModel)
	if clientModel == "" {
		return ""
	}
	if strings.HasPrefix(clientModel, "zo:") {
		return clientModel
	}
	clientKey := normalizeZoModelKey(clientModel)
	for _, model := range models {
		if clientModel == model.ModelName || clientModel == model.Label || strings.EqualFold(clientModel, model.ModelName) || strings.EqualFold(clientModel, model.Label) {
			return model.ModelName
		}
	}
	if clientKey != "" {
		if modelName := matchZoModelByKey(clientKey, models); modelName != "" {
			return modelName
		}
	}
	lower := strings.ToLower(clientModel)
	vendor := ""
	switch {
	case strings.Contains(lower, "claude"):
		vendor = "anthropic"
	case strings.Contains(lower, "gpt"), strings.Contains(lower, "o1"), strings.Contains(lower, "o3"), strings.Contains(lower, "openai"):
		vendor = "openai"
	case strings.Contains(lower, "deepseek"):
		vendor = "deepseek"
	case strings.Contains(lower, "gemini"):
		vendor = "google"
	case strings.Contains(lower, "glm"):
		vendor = "zai"
	case strings.Contains(lower, "minimax"):
		vendor = "minimax"
	}
	if vendor == "" {
		return ""
	}
	for _, model := range models {
		if zoModelMatchesVendor(model, vendor) {
			return model.ModelName
		}
	}
	return ""
}

func matchZoModelByKey(clientKey string, models []zoModel) string {
	for _, model := range models {
		for _, key := range zoModelKeys(model) {
			if clientKey == key {
				return model.ModelName
			}
		}
	}
	bestModel := ""
	bestScore := 0
	for _, model := range models {
		for _, key := range zoModelKeys(model) {
			score := 0
			switch {
			case strings.Contains(key, clientKey):
				score = len(clientKey)*2 - len(key)/1000
			case strings.Contains(clientKey, key) && len(key) >= 5:
				score = len(key)
			}
			if score > bestScore {
				bestScore = score
				bestModel = model.ModelName
			}
		}
	}
	return bestModel
}

func zoModelKeys(model zoModel) []string {
	values := []string{model.ModelName, model.Label}
	if _, suffix, ok := strings.Cut(model.ModelName, "/"); ok {
		values = append(values, suffix)
	}
	keys := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		key := normalizeZoModelKey(value)
		if key == "" || seen[key] {
			continue
		}
		keys = append(keys, key)
		seen[key] = true
	}
	return keys
}

func normalizeZoModelKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func zoModelMatchesVendor(model zoModel, vendor string) bool {
	vendor = strings.ToLower(strings.TrimSpace(vendor))
	if vendor == "" {
		return false
	}
	return strings.Contains(strings.ToLower(model.ModelName), vendor) ||
		strings.Contains(strings.ToLower(model.Label), vendor) ||
		strings.EqualFold(strings.TrimSpace(model.Vendor), vendor)
}
