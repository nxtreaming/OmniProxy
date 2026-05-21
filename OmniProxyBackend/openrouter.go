package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"OmniProxyBackend/internal/config"
	"OmniProxyBackend/internal/logs"
	"OmniProxyBackend/internal/proxy"
	"OmniProxyBackend/internal/token"
)

const openRouterModelsCacheTTL = 6 * time.Hour

type openRouterModelResponse struct {
	ID                  string            `json:"id"`
	Name                string            `json:"name,omitempty"`
	Description         string            `json:"description,omitempty"`
	ContextLength       int               `json:"contextLength,omitempty"`
	Pricing             openRouterPricing `json:"pricing,omitempty"`
	Architecture        map[string]any    `json:"architecture,omitempty"`
	TopProvider         map[string]any    `json:"topProvider,omitempty"`
	SupportedParameters []string          `json:"supportedParameters,omitempty"`
}

type openRouterPricing struct {
	Prompt     string `json:"prompt,omitempty"`
	Completion string `json:"completion,omitempty"`
	Request    string `json:"request,omitempty"`
	Image      string `json:"image,omitempty"`
}

type openRouterModelsResponse struct {
	Models    []openRouterModelResponse `json:"models"`
	FetchedAt string                    `json:"fetchedAt,omitempty"`
	Source    string                    `json:"source"`
	Cached    bool                      `json:"cached"`
}

type openRouterChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openRouterChatRequest struct {
	Model       string                  `json:"model"`
	Messages    []openRouterChatMessage `json:"messages"`
	Temperature *float64                `json:"temperature,omitempty"`
	MaxTokens   int                     `json:"maxTokens,omitempty"`
}

type openRouterChatUsageResponse struct {
	InputTokens  int `json:"inputTokens"`
	OutputTokens int `json:"outputTokens"`
	TotalTokens  int `json:"totalTokens"`
}

type openRouterChatResponse struct {
	Model        string                      `json:"model"`
	Message      openRouterChatMessage       `json:"message"`
	Usage        openRouterChatUsageResponse `json:"usage"`
	FinishReason string                      `json:"finishReason,omitempty"`
}

type openRouterModelsCache struct {
	models    []openRouterModelResponse
	fetchedAt time.Time
	baseURL   string
	tokenID   string
}

type openRouterRequestError struct {
	message string
}

func (e openRouterRequestError) Error() string {
	return e.message
}

func (a *appServer) openRouterModels(ctx context.Context, refresh bool) (openRouterModelsResponse, error) {
	a.mu.Lock()
	cfg := a.cfg
	a.mu.Unlock()
	cfg = config.Normalize(cfg)

	selected, err := a.openRouterModelToken()
	if err != nil {
		if cached, ok := a.cachedOpenRouterModels(cfg.OpenRouterBaseURL, "", false); ok {
			return cached, nil
		}
		return openRouterModelsResponse{Source: "openrouter"}, err
	}

	if !refresh {
		if cached, ok := a.cachedOpenRouterModels(cfg.OpenRouterBaseURL, selected.ID, true); ok {
			return cached, nil
		}
	}

	client, err := proxy.NewTokenHTTPClient(cfg, selected, 0)
	if err != nil {
		return openRouterModelsResponse{Source: "openrouter"}, err
	}
	models, err := fetchOpenRouterModels(ctx, client, cfg.OpenRouterBaseURL, selected.TokenValue)
	if err != nil {
		if cached, ok := a.cachedOpenRouterModels(cfg.OpenRouterBaseURL, selected.ID, false); ok {
			return cached, nil
		}
		return openRouterModelsResponse{Source: "openrouter"}, err
	}

	now := time.Now()
	a.openRouterModelsMu.Lock()
	a.openRouterModelsCache = openRouterModelsCache{
		models:    append([]openRouterModelResponse(nil), models...),
		fetchedAt: now,
		baseURL:   strings.TrimSpace(cfg.OpenRouterBaseURL),
		tokenID:   selected.ID,
	}
	a.openRouterModelsMu.Unlock()

	return openRouterModelsResponse{
		Models:    models,
		FetchedAt: timeString(now),
		Source:    "openrouter",
		Cached:    false,
	}, nil
}

func (a *appServer) cachedOpenRouterModels(baseURL string, tokenID string, requireFresh bool) (openRouterModelsResponse, bool) {
	a.openRouterModelsMu.Lock()
	defer a.openRouterModelsMu.Unlock()

	cache := a.openRouterModelsCache
	if len(cache.models) == 0 || cache.fetchedAt.IsZero() {
		return openRouterModelsResponse{}, false
	}
	if strings.TrimSpace(cache.baseURL) != strings.TrimSpace(baseURL) {
		return openRouterModelsResponse{}, false
	}
	if tokenID != "" && cache.tokenID != "" && cache.tokenID != tokenID {
		return openRouterModelsResponse{}, false
	}
	if requireFresh && time.Since(cache.fetchedAt) > openRouterModelsCacheTTL {
		return openRouterModelsResponse{}, false
	}

	return openRouterModelsResponse{
		Models:    append([]openRouterModelResponse(nil), cache.models...),
		FetchedAt: timeString(cache.fetchedAt),
		Source:    "openrouter",
		Cached:    true,
	}, true
}

func (a *appServer) openRouterModelToken() (token.Token, error) {
	if a.tokens == nil {
		return token.Token{}, errors.New("token manager is not ready")
	}
	var fallback token.Token
	for _, item := range a.tokens.List() {
		if token.NormalizeProvider(item.Provider) != token.ProviderOpenRouter {
			continue
		}
		if strings.TrimSpace(item.TokenValue) == "" {
			continue
		}
		if fallback.ID == "" {
			fallback = item
		}
		if !item.Disabled {
			return item, nil
		}
	}
	if fallback.ID != "" {
		return fallback, nil
	}
	return token.Token{}, errors.New("请先添加 OpenRouter API Key")
}

func (a *appServer) openRouterChat(ctx context.Context, req openRouterChatRequest) (openRouterChatResponse, error) {
	a.mu.Lock()
	cfg := a.cfg
	a.mu.Unlock()
	cfg = config.Normalize(cfg)

	selected, err := a.openRouterModelToken()
	if err != nil {
		return openRouterChatResponse{}, err
	}
	if selected.Disabled {
		return openRouterChatResponse{}, openRouterRequestError{message: "OpenRouter API Key 已停用，请启用后再对话"}
	}

	model, messages, err := normalizeOpenRouterChatRequest(req)
	if err != nil {
		return openRouterChatResponse{}, err
	}
	target, err := joinExternalURLPath(cfg.OpenRouterBaseURL, "/chat/completions")
	if err != nil {
		return openRouterChatResponse{}, err
	}

	body := map[string]any{
		"model":    model,
		"messages": messages,
	}
	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return openRouterChatResponse{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	client, err := proxy.NewTokenHTTPClient(cfg, selected, 0)
	if err != nil {
		return openRouterChatResponse{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(encoded))
	if err != nil {
		return openRouterChatResponse{}, err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+strings.TrimSpace(selected.TokenValue))

	start := time.Now()
	resp, err := client.Do(httpReq)
	if err != nil {
		a.recordOpenRouterChatAttempt(selected, model, 0, start, token.TokenConsumption{}, logs.LevelWarn, "OpenRouter 对话请求失败")
		_ = a.tokens.RecordProxyUsage(selected.ID, token.TokenConsumption{})
		return openRouterChatResponse{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if err != nil {
		a.recordOpenRouterChatAttempt(selected, model, resp.StatusCode, start, token.TokenConsumption{}, logs.LevelWarn, "OpenRouter 对话响应读取失败")
		_ = a.tokens.RecordUsage(selected.ID, -1)
		_ = a.tokens.RecordProxyUsage(selected.ID, token.TokenConsumption{})
		return openRouterChatResponse{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		a.recordOpenRouterChatAttempt(selected, model, resp.StatusCode, start, token.TokenConsumption{}, logs.LevelWarn, "OpenRouter 对话请求未通过")
		_ = a.tokens.RecordUsage(selected.ID, -1)
		_ = a.tokens.RecordProxyUsage(selected.ID, token.TokenConsumption{})
		return openRouterChatResponse{}, fmt.Errorf("OpenRouter chat request returned %d: %s", resp.StatusCode, limitOpenRouterError(respBody))
	}

	result, err := parseOpenRouterChatResponse(respBody)
	if err != nil {
		a.recordOpenRouterChatAttempt(selected, model, resp.StatusCode, start, token.TokenConsumption{}, logs.LevelWarn, "OpenRouter 对话响应解析失败")
		_ = a.tokens.RecordUsage(selected.ID, -1)
		_ = a.tokens.RecordProxyUsage(selected.ID, token.TokenConsumption{})
		return openRouterChatResponse{}, err
	}
	if result.Model == "" {
		result.Model = model
	}
	consumption := result.Usage.tokenConsumption()
	_ = a.tokens.RecordUsage(selected.ID, -1)
	_ = a.tokens.RecordProxyUsage(selected.ID, consumption)
	a.recordOpenRouterChatAttempt(selected, result.Model, resp.StatusCode, start, consumption, logs.LevelInfo, "OpenRouter 对话完成")
	return result, nil
}

func normalizeOpenRouterChatRequest(req openRouterChatRequest) (string, []openRouterChatMessage, error) {
	model := strings.TrimSpace(req.Model)
	if model == "" {
		return "", nil, openRouterRequestError{message: "请选择 OpenRouter 模型"}
	}
	if len(req.Messages) == 0 {
		return "", nil, openRouterRequestError{message: "请输入要发送的消息"}
	}
	if len(req.Messages) > 64 {
		return "", nil, openRouterRequestError{message: "对话消息过多，请清空后重试"}
	}
	if req.Temperature != nil && (*req.Temperature < 0 || *req.Temperature > 2) {
		return "", nil, openRouterRequestError{message: "温度必须在 0 到 2 之间"}
	}
	if req.MaxTokens < 0 {
		return "", nil, openRouterRequestError{message: "输出 Token 上限不能为负数"}
	}

	out := make([]openRouterChatMessage, 0, len(req.Messages))
	totalChars := 0
	for _, item := range req.Messages {
		role := normalizeOpenRouterChatRole(item.Role)
		if role == "" {
			return "", nil, openRouterRequestError{message: "对话消息角色无效"}
		}
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		totalChars += len([]rune(content))
		if totalChars > 512*1024 {
			return "", nil, openRouterRequestError{message: "消息内容过长，请缩短后重试"}
		}
		out = append(out, openRouterChatMessage{Role: role, Content: content})
	}
	if len(out) == 0 {
		return "", nil, openRouterRequestError{message: "请输入要发送的消息"}
	}
	return model, out, nil
}

func normalizeOpenRouterChatRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "system", "user", "assistant":
		return strings.ToLower(strings.TrimSpace(role))
	default:
		return ""
	}
}

func parseOpenRouterChatResponse(body []byte) (openRouterChatResponse, error) {
	var payload struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Role    string `json:"role"`
				Content any    `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return openRouterChatResponse{}, err
	}
	if len(payload.Choices) == 0 {
		return openRouterChatResponse{}, errors.New("OpenRouter 响应没有返回消息")
	}

	choice := payload.Choices[0]
	content := openRouterContentText(choice.Message.Content)
	if content == "" {
		return openRouterChatResponse{}, errors.New("OpenRouter 响应没有返回文本内容")
	}
	role := normalizeOpenRouterChatRole(choice.Message.Role)
	if role == "" {
		role = "assistant"
	}
	usage := openRouterChatUsageResponse{
		InputTokens:  payload.Usage.PromptTokens,
		OutputTokens: payload.Usage.CompletionTokens,
		TotalTokens:  payload.Usage.TotalTokens,
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	}
	return openRouterChatResponse{
		Model: payload.Model,
		Message: openRouterChatMessage{
			Role:    role,
			Content: content,
		},
		Usage:        usage,
		FinishReason: strings.TrimSpace(choice.FinishReason),
	}, nil
}

func openRouterContentText(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			object := objectField(item)
			if object == nil {
				continue
			}
			if text := stringField(object["text"]); text != "" {
				parts = append(parts, text)
			}
		}
		if len(parts) > 0 {
			return strings.TrimSpace(strings.Join(parts, "\n"))
		}
	}
	if text := stringField(value); text != "" {
		return text
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(encoded))
}

func (usage openRouterChatUsageResponse) tokenConsumption() token.TokenConsumption {
	return token.TokenConsumption{
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
		TotalTokens:  usage.TotalTokens,
	}
}

func (a *appServer) recordOpenRouterChatAttempt(selected token.Token, model string, status int, start time.Time, consumption token.TokenConsumption, level logs.Level, message string) {
	if a.logs == nil {
		return
	}
	if consumption.TotalTokens > 0 {
		message = fmt.Sprintf("%s：%d tokens", message, consumption.TotalTokens)
	}
	a.logs.Add(logs.Entry{
		Level:     level,
		Method:    http.MethodPost,
		Path:      "/api/openrouter/chat",
		Model:     model,
		Status:    status,
		Duration:  time.Since(start).Milliseconds(),
		TokenName: selected.Name,
		Message:   message,
	})
}

func fetchOpenRouterModels(ctx context.Context, client *http.Client, baseURL string, apiKey string) ([]openRouterModelResponse, error) {
	target, err := joinExternalURLPath(baseURL, "/models")
	if err != nil {
		return nil, err
	}
	if client == nil {
		client = http.DefaultClient
	}

	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if secret := strings.TrimSpace(apiKey); secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("OpenRouter models request returned %d: %s", resp.StatusCode, limitOpenRouterError(body))
	}

	models, err := parseOpenRouterModels(body)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(models, func(i, j int) bool {
		return strings.ToLower(models[i].ID) < strings.ToLower(models[j].ID)
	})
	return models, nil
}

func parseOpenRouterModels(body []byte) ([]openRouterModelResponse, error) {
	var payload struct {
		Data []map[string]any `json:"data"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return nil, err
	}

	models := make([]openRouterModelResponse, 0, len(payload.Data))
	for _, raw := range payload.Data {
		id := stringField(raw["id"])
		if id == "" {
			continue
		}
		models = append(models, openRouterModelResponse{
			ID:                  id,
			Name:                stringField(raw["name"]),
			Description:         stringField(raw["description"]),
			ContextLength:       intField(raw["context_length"]),
			Pricing:             openRouterPricingFrom(raw["pricing"]),
			Architecture:        objectField(raw["architecture"]),
			TopProvider:         objectField(raw["top_provider"]),
			SupportedParameters: stringSliceField(raw["supported_parameters"]),
		})
	}
	return models, nil
}

func openRouterPricingFrom(value any) openRouterPricing {
	object := objectField(value)
	return openRouterPricing{
		Prompt:     stringField(object["prompt"]),
		Completion: stringField(object["completion"]),
		Request:    stringField(object["request"]),
		Image:      stringField(object["image"]),
	}
}

func joinExternalURLPath(baseURL string, path string) (string, error) {
	base, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", err
	}
	out := *base
	out.Path = joiningSlash(base.Path, path)
	out.RawQuery = ""
	return out.String(), nil
}

func joiningSlash(a string, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	default:
		return a + b
	}
}

func stringField(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	default:
		return ""
	}
}

func intField(value any) int {
	switch typed := value.(type) {
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil && parsed >= 0 {
			return int(parsed)
		}
	case float64:
		if typed >= 0 {
			return int(typed)
		}
	case int:
		if typed >= 0 {
			return typed
		}
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err == nil && parsed >= 0 {
			return parsed
		}
	}
	return 0
}

func objectField(value any) map[string]any {
	object, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	return object
}

func stringSliceField(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if text := stringField(item); text != "" {
			out = append(out, text)
		}
	}
	return out
}

func limitOpenRouterError(body []byte) string {
	text := strings.TrimSpace(string(body))
	if text == "" {
		return http.StatusText(http.StatusBadGateway)
	}
	const max = 240
	if len(text) > max {
		return text[:max] + "..."
	}
	return text
}
