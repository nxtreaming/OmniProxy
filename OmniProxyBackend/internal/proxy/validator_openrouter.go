package proxy

import "omniproxy/internal/token"

func parseOpenRouterKeyUsage(body []byte) (token.UsageInfo, *int, bool) {
	payload, err := decodeObject(body)
	if err != nil {
		return token.UsageInfo{}, nil, false
	}

	data := payload
	if nested, ok := payload["data"].(map[string]any); ok {
		data = nested
	}

	usageValue, usageOK := floatFromAny(data["usage"])
	limitValue, limitOK := floatFromAny(data["limit"])
	remainingValue, remainingOK := floatFromAny(data["limit_remaining"])
	if !remainingOK && limitOK && usageOK {
		remainingValue = limitValue - usageValue
		if remainingValue < 0 {
			remainingValue = 0
		}
		remainingOK = true
	}

	planType := "OpenRouter API Key"
	if boolFromAny(data["is_free_tier"], false) {
		planType = "OpenRouter Free Tier"
	}
	usage := token.UsageInfo{
		Source:   token.ProviderOpenRouter,
		PlanType: planType,
		Message:  "OpenRouter key usage",
	}
	if usageOK {
		usage.BalanceUsed = usageValue
	}
	if usageOK || limitOK || remainingOK {
		usage.BalanceUnit = "USD"
	}
	if limitOK {
		usage.BalanceTotal = limitValue
	}
	if remainingOK {
		usage.BalanceRemaining = remainingValue
	}
	if usageOK && !limitOK && !remainingOK {
		usage.BalanceUnlimited = true
	}

	var remaining *int
	if usage.BalanceUnlimited {
		value := 100
		remaining = &value
	} else if remainingOK {
		value := 100
		if limitOK && limitValue > 0 {
			value = percent((remainingValue / limitValue) * 100)
		} else if remainingValue <= 0 {
			value = 0
		}
		remaining = &value
	}

	return usage, remaining, usageOK || limitOK || remainingOK
}
