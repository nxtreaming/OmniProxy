package proxy

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"OmniProxyBackend/internal/token"
)

const zoModelCacheTTL = 10 * time.Minute

type zoModelCacheEntry struct {
	models    []zoModel
	fetchedAt time.Time
}

type zoModel struct {
	ModelName string `json:"model_name"`
	Label     string `json:"label"`
	Vendor    string `json:"vendor"`
}

type zoModelsPayload struct {
	Models []zoModel `json:"models"`
}

type zoAskRequest struct {
	Input        string `json:"input"`
	Stream       bool   `json:"stream"`
	ModelName    string `json:"model_name,omitempty"`
	OutputFormat any    `json:"output_format,omitempty"`
}

type zoAskResponse struct {
	Output any `json:"output"`
}

type zoOpenAIRequest struct {
	Model     string `json:"model"`
	Messages  []any  `json:"messages"`
	Stream    bool   `json:"stream"`
	Tools     []any  `json:"tools"`
	Functions []any  `json:"functions"`
}

type zoAnthropicRequest struct {
	Model    string `json:"model"`
	System   any    `json:"system"`
	Messages []any  `json:"messages"`
	Stream   bool   `json:"stream"`
	Tools    []any  `json:"tools"`
}

type zoResponsesRequest struct {
	Model        string `json:"model"`
	Input        any    `json:"input"`
	Instructions any    `json:"instructions"`
	Stream       any    `json:"stream"`
	Tools        any    `json:"tools"`
}

type zoParsedOutput struct {
	Text      string
	ToolCalls []zoToolCall
}

type zoToolCall struct {
	Name      string
	Arguments map[string]any
}

func (s *Service) forwardZo(ctx context.Context, original *http.Request, route routeInfo, body []byte, selected token.Token) (*http.Response, error) {
	path := normalizeZoClientPath(route.Path)
	switch {
	case original.Method == http.MethodGet && path == "/v1/models":
		return s.forwardZoModels(ctx, original, route, selected)
	case original.Method == http.MethodPost && path == "/v1/chat/completions":
		return s.forwardZoOpenAIChat(ctx, original, route, body, selected)
	case original.Method == http.MethodPost && path == "/v1/messages":
		return s.forwardZoAnthropicMessages(ctx, original, route, body, selected)
	case original.Method == http.MethodPost && path == "/v1/responses":
		return s.forwardZoResponses(ctx, original, route, body, selected)
	default:
		return zoErrorResponse(original, http.StatusNotFound, fmt.Sprintf("Not found: %s %s", original.Method, route.Path), route.Protocol), nil
	}
}

func normalizeZoClientPath(path string) string {
	path = strings.TrimSpace(path)
	switch path {
	case "", "/":
		return "/"
	case "/v1/v1/messages", "/messages":
		return "/v1/messages"
	case "/chat/completions":
		return "/v1/chat/completions"
	case "/models":
		return "/v1/models"
	case "/responses":
		return "/v1/responses"
	default:
		return path
	}
}

func (s *Service) forwardZoModels(ctx context.Context, original *http.Request, route routeInfo, selected token.Token) (*http.Response, error) {
	models, status, header, body, err := s.fetchZoModels(ctx, route, selected, false)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return zoUpstreamErrorResponse(original, status, header, body, route.Protocol), nil
	}

	data := make([]map[string]any, 0, len(models))
	for _, model := range models {
		id := strings.TrimSpace(model.ModelName)
		if id == "" {
			id = strings.TrimSpace(model.Label)
		}
		if id == "" {
			continue
		}
		ownedBy := strings.TrimSpace(model.Vendor)
		if ownedBy == "" {
			ownedBy = "zo"
		}
		data = append(data, map[string]any{
			"id":       id,
			"object":   "model",
			"created":  zoTimestamp(),
			"owned_by": ownedBy,
		})
	}
	return jsonResponse(original, http.StatusOK, map[string]any{
		"object": "list",
		"data":   data,
	}, nil), nil
}

func (s *Service) forwardZoOpenAIChat(ctx context.Context, original *http.Request, route routeInfo, body []byte, selected token.Token) (*http.Response, error) {
	var reqBody zoOpenAIRequest
	if err := json.Unmarshal(body, &reqBody); err != nil {
		return zoErrorResponse(original, http.StatusBadRequest, "Invalid JSON body", "openai"), nil
	}

	models, _, _, _, _ := s.fetchZoModels(ctx, route, selected, true)
	input := buildZoInputFromOpenAI(reqBody.Messages)
	tools := reqBody.Tools
	if len(tools) == 0 {
		tools = reqBody.Functions
	}
	finalInput, outputFormat := injectZoTools(input, tools)

	zoBody := zoAskRequest{Input: finalInput, Stream: false}
	if model := mapZoModel(reqBody.Model, models); model != "" {
		zoBody.ModelName = model
	}
	if outputFormat != nil {
		zoBody.OutputFormat = outputFormat
	}

	status, header, responseBody, err := s.postZoAsk(ctx, route, selected, zoBody, original.Header)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return zoUpstreamErrorResponse(original, status, header, responseBody, "openai"), nil
	}

	var zoResp zoAskResponse
	if err := json.Unmarshal(responseBody, &zoResp); err != nil {
		return zoErrorResponse(original, http.StatusBadGateway, "Invalid Zo API response", "openai"), nil
	}
	parsed := normalizeZoParsedOutput(parseZoOutput(zoResp.Output), tools)
	if reqBody.Stream {
		return openAIStreamResponse(original, reqBody.Model, parsed, header), nil
	}
	return jsonResponse(original, http.StatusOK, openAIResponseFromZo(reqBody.Model, parsed), forwardedZoHeaders(header)), nil
}

func (s *Service) forwardZoAnthropicMessages(ctx context.Context, original *http.Request, route routeInfo, body []byte, selected token.Token) (*http.Response, error) {
	var reqBody zoAnthropicRequest
	if err := json.Unmarshal(body, &reqBody); err != nil {
		return zoErrorResponse(original, http.StatusBadRequest, "Invalid JSON body", "anthropic"), nil
	}

	models, _, _, _, _ := s.fetchZoModels(ctx, route, selected, true)
	input := buildZoInputFromAnthropic(reqBody.System, reqBody.Messages)
	finalInput, outputFormat := injectZoTools(input, reqBody.Tools)

	zoBody := zoAskRequest{Input: finalInput, Stream: false}
	if model := mapZoModel(reqBody.Model, models); model != "" {
		zoBody.ModelName = model
	}
	if outputFormat != nil {
		zoBody.OutputFormat = outputFormat
	}

	status, header, responseBody, err := s.postZoAsk(ctx, route, selected, zoBody, original.Header)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return zoUpstreamErrorResponse(original, status, header, responseBody, "anthropic"), nil
	}

	var zoResp zoAskResponse
	if err := json.Unmarshal(responseBody, &zoResp); err != nil {
		return zoErrorResponse(original, http.StatusBadGateway, "Invalid Zo API response", "anthropic"), nil
	}
	parsed := normalizeZoParsedOutput(parseZoOutput(zoResp.Output), reqBody.Tools)
	if reqBody.Stream {
		return anthropicStreamResponse(original, reqBody.Model, parsed, header), nil
	}
	return jsonResponse(original, http.StatusOK, anthropicResponseFromZo(reqBody.Model, parsed), forwardedZoHeaders(header)), nil
}

func (s *Service) forwardZoResponses(ctx context.Context, original *http.Request, route routeInfo, body []byte, selected token.Token) (*http.Response, error) {
	var reqBody zoResponsesRequest
	if err := json.Unmarshal(body, &reqBody); err != nil {
		return zoErrorResponse(original, http.StatusBadRequest, "Invalid JSON body", "openai"), nil
	}

	models, _, _, _, _ := s.fetchZoModels(ctx, route, selected, true)
	input := buildZoInputFromResponses(reqBody.Instructions, reqBody.Input)
	tools := zoToolsList(reqBody.Tools)
	finalInput, outputFormat := injectZoTools(input, tools)

	zoBody := zoAskRequest{Input: finalInput, Stream: false}
	if model := mapZoModel(reqBody.Model, models); model != "" {
		zoBody.ModelName = model
	}
	if outputFormat != nil {
		zoBody.OutputFormat = outputFormat
	}

	status, header, responseBody, err := s.postZoAsk(ctx, route, selected, zoBody, original.Header)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return zoUpstreamErrorResponse(original, status, header, responseBody, "openai"), nil
	}

	var zoResp zoAskResponse
	if err := json.Unmarshal(responseBody, &zoResp); err != nil {
		return zoErrorResponse(original, http.StatusBadGateway, "Invalid Zo API response", "openai"), nil
	}
	parsed := normalizeZoParsedOutput(parseZoOutput(zoResp.Output), tools)
	if truthyZoBool(reqBody.Stream) {
		return openAIResponsesStreamResponse(original, reqBody.Model, parsed, header), nil
	}
	return jsonResponse(original, http.StatusOK, openAIResponsesFromZo(reqBody.Model, parsed), forwardedZoHeaders(header)), nil
}

func (s *Service) fetchZoModels(ctx context.Context, route routeInfo, selected token.Token, allowCached bool) ([]zoModel, int, http.Header, []byte, error) {
	baseURL := s.router.BaseURL(route, selected)
	if baseURL == "" {
		return nil, 0, nil, nil, fmt.Errorf("%s upstream base url is not configured", route.Provider)
	}
	cacheKey := strings.TrimRight(baseURL, "/") + "|" + selected.ID
	if allowCached {
		s.zoModelsMu.Lock()
		if s.zoModelsCache != nil {
			if entry, ok := s.zoModelsCache[cacheKey]; ok && time.Since(entry.fetchedAt) < zoModelCacheTTL {
				models := append([]zoModel(nil), entry.models...)
				s.zoModelsMu.Unlock()
				return models, http.StatusOK, http.Header{}, nil, nil
			}
		}
		s.zoModelsMu.Unlock()
	}

	status, header, body, err := s.zoFetch(ctx, http.MethodGet, route, selected, "/models/available", nil, nil)
	if err != nil {
		return nil, 0, nil, nil, err
	}
	if status != http.StatusOK {
		return nil, status, header, body, nil
	}
	var payload zoModelsPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, http.StatusBadGateway, header, body, nil
	}
	models := append([]zoModel(nil), payload.Models...)

	s.zoModelsMu.Lock()
	if s.zoModelsCache == nil {
		s.zoModelsCache = map[string]zoModelCacheEntry{}
	}
	s.zoModelsCache[cacheKey] = zoModelCacheEntry{models: models, fetchedAt: time.Now()}
	s.zoModelsMu.Unlock()

	return models, status, header, body, nil
}

func (s *Service) postZoAsk(ctx context.Context, route routeInfo, selected token.Token, body zoAskRequest, clientHeader http.Header) (int, http.Header, []byte, error) {
	return s.zoFetch(ctx, http.MethodPost, route, selected, "/zo/ask", body, clientHeader)
}

func (s *Service) zoFetch(ctx context.Context, method string, route routeInfo, selected token.Token, path string, body any, clientHeader http.Header) (int, http.Header, []byte, error) {
	target, err := joinZoURL(s.router.BaseURL(route, selected), path)
	if err != nil {
		return 0, nil, nil, err
	}
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return 0, nil, nil, err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return 0, nil, nil, err
	}
	secret, err := credentialSecret(selected)
	if err != nil {
		return 0, nil, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+secret)
	req.Header.Set("Content-Type", "application/json")
	if clientHeader != nil {
		if conversationID := strings.TrimSpace(clientHeader.Get("x-conversation-id")); conversationID != "" {
			req.Header.Set("x-conversation-id", conversationID)
		}
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer closeBody(resp.Body)
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, resp.Header, nil, err
	}
	return resp.StatusCode, resp.Header, data, nil
}

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

func openAIResponseFromZo(model string, parsed zoParsedOutput) map[string]any {
	message := map[string]any{"role": "assistant"}
	if strings.TrimSpace(parsed.Text) == "" && len(parsed.ToolCalls) > 0 {
		message["content"] = nil
	} else {
		message["content"] = parsed.Text
	}
	if len(parsed.ToolCalls) > 0 {
		toolCalls := make([]map[string]any, 0, len(parsed.ToolCalls))
		for _, call := range parsed.ToolCalls {
			toolCalls = append(toolCalls, map[string]any{
				"id":   zoShortID("call_"),
				"type": "function",
				"function": map[string]any{
					"name":      call.Name,
					"arguments": mustJSONText(call.Arguments),
				},
			})
		}
		message["tool_calls"] = toolCalls
	}
	finishReason := "stop"
	if len(parsed.ToolCalls) > 0 {
		finishReason = "tool_calls"
	}
	return map[string]any{
		"id":      "chatcmpl-" + zoUUID(),
		"object":  "chat.completion",
		"created": zoTimestamp(),
		"model":   model,
		"choices": []map[string]any{{
			"index":         0,
			"message":       message,
			"finish_reason": finishReason,
		}},
		"usage": map[string]int{"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0},
	}
}

func openAIResponsesFromZo(model string, parsed zoParsedOutput) map[string]any {
	id := "resp_" + strings.ReplaceAll(zoUUID(), "-", "")
	return map[string]any{
		"id":         id,
		"object":     "response",
		"created_at": zoTimestamp(),
		"status":     "completed",
		"model":      model,
		"output":     zoResponsesOutput(parsed),
		"usage":      map[string]int{"input_tokens": 0, "output_tokens": 0, "total_tokens": 0},
	}
}

func zoResponsesOutput(parsed zoParsedOutput) []map[string]any {
	output := []map[string]any{}
	if strings.TrimSpace(parsed.Text) != "" {
		output = append(output, map[string]any{
			"id":     zoShortID("msg_"),
			"type":   "message",
			"role":   "assistant",
			"status": "completed",
			"content": []map[string]any{{
				"type": "output_text",
				"text": parsed.Text,
			}},
		})
	}
	for _, call := range parsed.ToolCalls {
		output = append(output, map[string]any{
			"id":        zoShortID("fc_"),
			"type":      "function_call",
			"call_id":   zoShortID("call_"),
			"name":      call.Name,
			"arguments": mustJSONText(call.Arguments),
			"status":    "completed",
		})
	}
	if len(output) == 0 {
		output = append(output, map[string]any{
			"id":      zoShortID("msg_"),
			"type":    "message",
			"role":    "assistant",
			"status":  "completed",
			"content": []map[string]any{},
		})
	}
	return output
}

func anthropicResponseFromZo(model string, parsed zoParsedOutput) map[string]any {
	content := []map[string]any{}
	if strings.TrimSpace(parsed.Text) != "" {
		content = append(content, map[string]any{"type": "text", "text": parsed.Text})
	}
	for _, call := range parsed.ToolCalls {
		content = append(content, map[string]any{
			"type":  "tool_use",
			"id":    zoShortID("toolu_"),
			"name":  call.Name,
			"input": call.Arguments,
		})
	}
	stopReason := "end_turn"
	if len(parsed.ToolCalls) > 0 {
		stopReason = "tool_use"
	}
	return map[string]any{
		"id":            "msg_" + zoUUID(),
		"type":          "message",
		"role":          "assistant",
		"model":         model,
		"content":       content,
		"stop_reason":   stopReason,
		"stop_sequence": nil,
		"usage":         map[string]int{"input_tokens": 0, "output_tokens": 0},
	}
}

func openAIStreamResponse(req *http.Request, model string, parsed zoParsedOutput, upstreamHeader http.Header) *http.Response {
	id := "chatcmpl-" + zoUUID()
	created := zoTimestamp()
	var buf bytes.Buffer
	writeSSEJSON(&buf, map[string]any{
		"id": id, "object": "chat.completion.chunk", "created": created, "model": model,
		"choices": []map[string]any{{"index": 0, "delta": map[string]any{"role": "assistant"}, "finish_reason": nil}},
	})
	if parsed.Text != "" {
		writeSSEJSON(&buf, map[string]any{
			"id": id, "object": "chat.completion.chunk", "created": created, "model": model,
			"choices": []map[string]any{{"index": 0, "delta": map[string]any{"content": parsed.Text}, "finish_reason": nil}},
		})
	}
	for index, call := range parsed.ToolCalls {
		writeSSEJSON(&buf, map[string]any{
			"id": id, "object": "chat.completion.chunk", "created": created, "model": model,
			"choices": []map[string]any{{
				"index": 0,
				"delta": map[string]any{
					"tool_calls": []map[string]any{{
						"index": index,
						"id":    zoShortID("call_"),
						"type":  "function",
						"function": map[string]any{
							"name":      call.Name,
							"arguments": mustJSONText(call.Arguments),
						},
					}},
				},
				"finish_reason": nil,
			}},
		})
	}
	reason := "stop"
	if len(parsed.ToolCalls) > 0 {
		reason = "tool_calls"
	}
	writeSSEJSON(&buf, map[string]any{
		"id": id, "object": "chat.completion.chunk", "created": created, "model": model,
		"choices": []map[string]any{{"index": 0, "delta": map[string]any{}, "finish_reason": reason}},
	})
	buf.WriteString("data: [DONE]\n\n")
	header := forwardedZoHeaders(upstreamHeader)
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-cache")
	return rawResponse(req, http.StatusOK, header, buf.Bytes())
}

func openAIResponsesStreamResponse(req *http.Request, model string, parsed zoParsedOutput, upstreamHeader http.Header) *http.Response {
	response := openAIResponsesFromZo(model, parsed)
	var buf bytes.Buffer
	writeEventJSON(&buf, "response.created", map[string]any{
		"type":     "response.created",
		"response": response,
	})
	for index, item := range response["output"].([]map[string]any) {
		writeEventJSON(&buf, "response.output_item.added", map[string]any{
			"type":         "response.output_item.added",
			"output_index": index,
			"item":         item,
		})
		if item["type"] == "message" {
			if content, ok := item["content"].([]map[string]any); ok && len(content) > 0 {
				part := content[0]
				writeEventJSON(&buf, "response.content_part.added", map[string]any{
					"type":          "response.content_part.added",
					"item_id":       item["id"],
					"output_index":  index,
					"content_index": 0,
					"part":          part,
				})
				if text, ok := part["text"].(string); ok && strings.TrimSpace(text) != "" {
					writeEventJSON(&buf, "response.output_text.delta", map[string]any{
						"type":          "response.output_text.delta",
						"item_id":       item["id"],
						"output_index":  index,
						"content_index": 0,
						"delta":         text,
					})
					writeEventJSON(&buf, "response.output_text.done", map[string]any{
						"type":          "response.output_text.done",
						"item_id":       item["id"],
						"output_index":  index,
						"content_index": 0,
						"text":          text,
					})
				}
				writeEventJSON(&buf, "response.content_part.done", map[string]any{
					"type":          "response.content_part.done",
					"item_id":       item["id"],
					"output_index":  index,
					"content_index": 0,
					"part":          part,
				})
			}
		}
		writeEventJSON(&buf, "response.output_item.done", map[string]any{
			"type":         "response.output_item.done",
			"output_index": index,
			"item":         item,
		})
	}
	writeEventJSON(&buf, "response.completed", map[string]any{
		"type":     "response.completed",
		"response": response,
	})
	buf.WriteString("data: [DONE]\n\n")
	header := forwardedZoHeaders(upstreamHeader)
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-cache")
	return rawResponse(req, http.StatusOK, header, buf.Bytes())
}

func anthropicStreamResponse(req *http.Request, model string, parsed zoParsedOutput, upstreamHeader http.Header) *http.Response {
	msgID := "msg_" + zoUUID()
	var buf bytes.Buffer
	writeAnthropicEvent(&buf, "message_start", map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id": msgID, "type": "message", "role": "assistant", "model": model,
			"content": []any{}, "stop_reason": nil, "stop_sequence": nil,
			"usage": map[string]int{"input_tokens": 0, "output_tokens": 0},
		},
	})
	index := 0
	if parsed.Text != "" {
		writeAnthropicEvent(&buf, "content_block_start", map[string]any{
			"type": "content_block_start", "index": index,
			"content_block": map[string]any{"type": "text", "text": ""},
		})
		writeAnthropicEvent(&buf, "content_block_delta", map[string]any{
			"type": "content_block_delta", "index": index,
			"delta": map[string]any{"type": "text_delta", "text": parsed.Text},
		})
		writeAnthropicEvent(&buf, "content_block_stop", map[string]any{"type": "content_block_stop", "index": index})
		index++
	}
	for _, call := range parsed.ToolCalls {
		writeAnthropicEvent(&buf, "content_block_start", map[string]any{
			"type": "content_block_start", "index": index,
			"content_block": map[string]any{"type": "tool_use", "id": zoShortID("toolu_"), "name": call.Name, "input": map[string]any{}},
		})
		if args := mustJSONText(call.Arguments); args != "{}" {
			writeAnthropicEvent(&buf, "content_block_delta", map[string]any{
				"type": "content_block_delta", "index": index,
				"delta": map[string]any{"type": "input_json_delta", "partial_json": args},
			})
		}
		writeAnthropicEvent(&buf, "content_block_stop", map[string]any{"type": "content_block_stop", "index": index})
		index++
	}
	stopReason := "end_turn"
	if len(parsed.ToolCalls) > 0 {
		stopReason = "tool_use"
	}
	writeAnthropicEvent(&buf, "message_delta", map[string]any{
		"type":  "message_delta",
		"delta": map[string]any{"stop_reason": stopReason, "stop_sequence": nil},
		"usage": map[string]int{"output_tokens": 0},
	})
	writeAnthropicEvent(&buf, "message_stop", map[string]any{"type": "message_stop"})

	header := forwardedZoHeaders(upstreamHeader)
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-cache")
	return rawResponse(req, http.StatusOK, header, buf.Bytes())
}

func zoUpstreamErrorResponse(req *http.Request, status int, upstreamHeader http.Header, body []byte, format string) *http.Response {
	message := "Zo API error"
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err == nil {
		if text := anyString(payload["detail"]); text != "" {
			message = text
		} else if text := anyString(payload["error"]); text != "" {
			message = text
		}
	}
	header := forwardedZoHeaders(upstreamHeader)
	return jsonResponse(req, status, zoErrorPayload(message, status, format), header)
}

func zoErrorResponse(req *http.Request, status int, message string, format string) *http.Response {
	return jsonResponse(req, status, zoErrorPayload(message, status, format), nil)
}

func zoErrorPayload(message string, status int, format string) map[string]any {
	if format == "anthropic" {
		return map[string]any{"type": "error", "error": map[string]any{"type": "api_error", "message": message}}
	}
	return map[string]any{"error": map[string]any{"message": message, "type": "api_error", "code": fmt.Sprintf("%d", status)}}
}

func jsonResponse(req *http.Request, status int, payload any, header http.Header) *http.Response {
	data, _ := json.Marshal(payload)
	if header == nil {
		header = http.Header{}
	}
	header.Set("Content-Type", "application/json")
	return rawResponse(req, status, header, data)
}

func rawResponse(req *http.Request, status int, header http.Header, body []byte) *http.Response {
	if header == nil {
		header = http.Header{}
	}
	return &http.Response{
		StatusCode:    status,
		Status:        fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:        header.Clone(),
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       req,
	}
}

func forwardedZoHeaders(header http.Header) http.Header {
	out := http.Header{}
	if conversationID := strings.TrimSpace(header.Get("x-conversation-id")); conversationID != "" {
		out.Set("x-conversation-id", conversationID)
	}
	return out
}

func writeSSEJSON(buf *bytes.Buffer, payload any) {
	buf.WriteString("data: ")
	data, _ := json.Marshal(payload)
	buf.Write(data)
	buf.WriteString("\n\n")
}

func writeEventJSON(buf *bytes.Buffer, event string, payload any) {
	buf.WriteString("event: ")
	buf.WriteString(event)
	buf.WriteString("\n")
	writeSSEJSON(buf, payload)
}

func writeAnthropicEvent(buf *bytes.Buffer, event string, payload any) {
	buf.WriteString("event: ")
	buf.WriteString(event)
	buf.WriteString("\n")
	writeSSEJSON(buf, payload)
}

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
