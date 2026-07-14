package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"omniproxy/internal/token"
	"strings"
	"time"
)

const (
	codexResetCreditsRefreshInterval = 5 * time.Minute
	codexResetCreditsUserAgent       = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36"
)

type CodexResetCreditsSnapshot struct {
	AvailableCount *int
	Credits        []token.CodexResetCredit
	NextExpiresAt  int64
	CheckedAt      int64
}

type CodexResetCreditsHTTPError struct {
	StatusCode int
	Message    string
}

func (e *CodexResetCreditsHTTPError) Error() string {
	return e.Message
}

func CodexResetCreditsAuthFailed(err error) bool {
	typed, ok := err.(*CodexResetCreditsHTTPError)
	return ok && (typed.StatusCode == http.StatusUnauthorized || typed.StatusCode == http.StatusForbidden)
}

func (v *Validator) FetchCodexResetCredits(ctx context.Context, selected token.Token) (CodexResetCreditsSnapshot, error) {
	target, err := codexResetCreditsEndpoint(v.cfg.CodexUsageEndpoint, false)
	if err != nil {
		return CodexResetCreditsSnapshot{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return CodexResetCreditsSnapshot{}, err
	}
	if err := applyCodexResetCreditsHeaders(req, selected); err != nil {
		return CodexResetCreditsSnapshot{}, err
	}

	body, status, err := v.doCodexResetCreditsRequest(req, selected)
	if err != nil {
		return CodexResetCreditsSnapshot{}, err
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return CodexResetCreditsSnapshot{}, codexResetCreditsStatusError(status, body)
	}

	snapshot, err := parseCodexResetCredits(body)
	if err != nil {
		return CodexResetCreditsSnapshot{}, err
	}
	snapshot.CheckedAt = time.Now().Unix()
	return snapshot, nil
}

func (v *Validator) ConsumeCodexResetCredit(ctx context.Context, selected token.Token, redeemRequestID string) error {
	redeemRequestID = strings.TrimSpace(redeemRequestID)
	if redeemRequestID == "" {
		return fmt.Errorf("redeem request id is required")
	}
	target, err := codexResetCreditsEndpoint(v.cfg.CodexUsageEndpoint, true)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]string{"redeem_request_id": redeemRequestID})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	if err := applyCodexResetCreditsHeaders(req, selected); err != nil {
		return err
	}

	body, status, err := v.doCodexResetCreditsRequest(req, selected)
	if err != nil {
		return err
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return codexResetCreditsStatusError(status, body)
	}
	return nil
}

func (v *Validator) doCodexResetCreditsRequest(req *http.Request, selected token.Token) ([]byte, int, error) {
	resp, err := v.clientForToken(selected).Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer closeBody(resp.Body)
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

func applyCodexResetCreditsHeaders(req *http.Request, selected token.Token) error {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Referer", "https://chatgpt.com/")
	req.Header.Set("User-Agent", codexResetCreditsUserAgent)
	req.Header.Set("OpenAI-Beta", "codex-1")
	req.Header.Set("Originator", "Codex Desktop")
	return applyAuth(req.Header, selected)
}

func codexResetCreditsEndpoint(usageEndpoint string, consume bool) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(usageEndpoint))
	if err != nil {
		return "", err
	}
	path := strings.TrimSuffix(parsed.Path, "/")
	if !strings.HasSuffix(path, "/usage") {
		return "", fmt.Errorf("codex usage endpoint does not end with /usage")
	}
	path = strings.TrimSuffix(path, "/usage") + "/rate-limit-reset-credits"
	if consume {
		path += "/consume"
	}
	parsed.Path = path
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func codexResetCreditsStatusError(status int, body []byte) error {
	message := fmt.Sprintf("Codex 额度刷新卡接口返回 %d", status)
	payload, err := decodeObject(body)
	if err == nil {
		if detail := codexResetCreditsErrorDetail(payload); detail != "" {
			message += "：" + detail
		}
	}
	return &CodexResetCreditsHTTPError{StatusCode: status, Message: message}
}

func codexResetCreditsErrorDetail(payload map[string]any) string {
	for _, key := range []string{"detail", "error"} {
		switch value := payload[key].(type) {
		case string:
			if text := strings.TrimSpace(value); text != "" {
				return text
			}
		case map[string]any:
			for _, childKey := range []string{"message", "code"} {
				if text, ok := stringFromAny(value[childKey]); ok {
					return text
				}
			}
		}
	}
	if text, ok := stringFromAny(payload["message"]); ok {
		return text
	}
	return ""
}

func parseCodexResetCredits(body []byte) (CodexResetCreditsSnapshot, error) {
	payload, err := decodeObject(body)
	if err != nil {
		return CodexResetCreditsSnapshot{}, fmt.Errorf("解析 Codex 额度刷新卡失败: %w", err)
	}
	source := payload
	if data, ok := payload["data"].(map[string]any); ok {
		source = data
	}

	countValue, hasCount := firstMapValue(source, "available_count", "availableCount")
	creditsValue, hasCredits := firstMapValue(source, "credits")
	if !hasCount && !hasCredits {
		return CodexResetCreditsSnapshot{}, fmt.Errorf("Codex 额度刷新卡响应缺少预期字段")
	}

	var availableCount *int
	if value, ok := codexResetCreditsIntFromAny(countValue); ok {
		if value < 0 {
			value = 0
		}
		availableCount = &value
	}

	credits := []token.CodexResetCredit{}
	if items, ok := creditsValue.([]any); ok {
		for _, item := range items {
			credit, ok := parseCodexResetCredit(item)
			if ok {
				credits = append(credits, credit)
			}
		}
	}

	now := time.Now().Unix()
	availableFromRecords := 0
	nextExpiresAt := int64(0)
	for _, credit := range credits {
		if !codexResetCreditAvailable(credit, now) {
			continue
		}
		availableFromRecords++
		if credit.ExpiresAt > 0 && (nextExpiresAt == 0 || credit.ExpiresAt < nextExpiresAt) {
			nextExpiresAt = credit.ExpiresAt
		}
	}
	if availableCount == nil {
		availableCount = &availableFromRecords
	}

	return CodexResetCreditsSnapshot{
		AvailableCount: availableCount,
		Credits:        credits,
		NextExpiresAt:  nextExpiresAt,
	}, nil
}

func parseCodexResetCredit(value any) (token.CodexResetCredit, bool) {
	record, ok := value.(map[string]any)
	if !ok {
		return token.CodexResetCredit{}, false
	}
	rawStatus := firstStringValue(record, "status", "state")
	expiresAt := firstUnixValue(record, "expires_at", "expire_at", "expiresAt")
	status := strings.ToLower(strings.TrimSpace(rawStatus))
	if status == "" {
		if expiresAt > 0 && expiresAt <= time.Now().Unix() {
			status = "expired"
		} else {
			status = "available"
		}
	}
	return token.CodexResetCredit{
		ID:         firstStringValue(record, "id", "credit_id", "creditId"),
		Status:     status,
		ResetType:  firstStringValue(record, "reset_type", "resetType", "type"),
		GrantedAt:  firstUnixValue(record, "granted_at", "created_at", "grantedAt"),
		ExpiresAt:  expiresAt,
		RedeemedAt: firstUnixValue(record, "redeemed_at", "used_at", "consumed_at", "redeemedAt"),
		RawStatus:  rawStatus,
	}, true
}

func codexResetCreditAvailable(credit token.CodexResetCredit, now int64) bool {
	status := strings.ToLower(strings.TrimSpace(credit.Status))
	switch status {
	case "redeemed", "used", "consumed", "expired":
		return false
	}
	return credit.ExpiresAt == 0 || credit.ExpiresAt > now
}

func firstMapValue(source map[string]any, keys ...string) (any, bool) {
	for _, key := range keys {
		value, ok := source[key]
		if ok {
			return value, true
		}
	}
	return nil, false
}

func firstStringValue(source map[string]any, keys ...string) string {
	value, ok := firstMapValue(source, keys...)
	if !ok {
		return ""
	}
	text, _ := stringFromAny(value)
	return text
}

func firstUnixValue(source map[string]any, keys ...string) int64 {
	value, ok := firstMapValue(source, keys...)
	if !ok {
		return 0
	}
	return unixSecondsFromAny(value)
}

func codexResetCreditsIntFromAny(value any) (int, bool) {
	number, ok := floatFromAny(value)
	if !ok {
		return 0, false
	}
	return int(number), true
}

func codexResetCreditsAvailableFromUsage(body []byte) *int {
	payload, err := decodeObject(body)
	if err != nil {
		return nil
	}
	value, ok := firstMapValue(payload, "rate_limit_reset_credits", "rateLimitResetCredits")
	if !ok {
		return nil
	}
	summary, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	countValue, ok := firstMapValue(summary, "available_count", "availableCount")
	if !ok {
		return nil
	}
	count, ok := codexResetCreditsIntFromAny(countValue)
	if !ok {
		return nil
	}
	if count < 0 {
		count = 0
	}
	return &count
}

func (v *Validator) hydrateCodexResetCredits(ctx context.Context, selected token.Token, usage *token.UsageInfo) {
	if usage == nil {
		return
	}
	embeddedCount := usage.CodexResetCreditsAvailable
	previous := selected.Usage
	if embeddedCount == nil && previous.CodexResetCreditsAvailable == nil {
		return
	}

	usage.CodexResetCreditsAvailable = previous.CodexResetCreditsAvailable
	usage.CodexResetCredits = previous.CodexResetCredits
	usage.CodexResetCreditsNextExpiresAt = previous.CodexResetCreditsNextExpiresAt
	usage.CodexResetCreditsError = previous.CodexResetCreditsError
	usage.CodexResetCreditsCheckedAt = previous.CodexResetCreditsCheckedAt
	if embeddedCount != nil {
		usage.CodexResetCreditsAvailable = embeddedCount
	}

	now := time.Now().Unix()
	countChanged := embeddedCount != nil && (previous.CodexResetCreditsAvailable == nil || *embeddedCount != *previous.CodexResetCreditsAvailable)
	cacheFresh := previous.CodexResetCreditsCheckedAt > 0 && now-previous.CodexResetCreditsCheckedAt < int64(codexResetCreditsRefreshInterval/time.Second)
	if cacheFresh && !countChanged {
		return
	}

	snapshot, err := v.FetchCodexResetCredits(ctx, selected)
	usage.CodexResetCreditsCheckedAt = now
	if err != nil {
		usage.CodexResetCreditsError = err.Error()
		return
	}
	usage.CodexResetCreditsAvailable = snapshot.AvailableCount
	usage.CodexResetCredits = snapshot.Credits
	usage.CodexResetCreditsNextExpiresAt = snapshot.NextExpiresAt
	usage.CodexResetCreditsError = ""
	usage.CodexResetCreditsCheckedAt = snapshot.CheckedAt
}
