package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"omniproxy/internal/token"
	"strings"
)

type codexChatRequest struct {
	Model               string          `json:"model"`
	Messages            []codexChatMsg  `json:"messages"`
	Stream              bool            `json:"stream,omitempty"`
	MaxTokens           *int            `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int            `json:"max_completion_tokens,omitempty"`
	ReasoningEffort     string          `json:"reasoning_effort,omitempty"`
	Tools               json.RawMessage `json:"tools,omitempty"`
	ToolChoice          json.RawMessage `json:"tool_choice,omitempty"`
	Functions           json.RawMessage `json:"functions,omitempty"`
	FunctionCall        json.RawMessage `json:"function_call,omitempty"`
	ServiceTier         string          `json:"service_tier,omitempty"`
	Instructions        string          `json:"instructions,omitempty"`
}

type codexChatMsg struct {
	Role       string              `json:"role"`
	Content    json.RawMessage     `json:"content"`
	Name       string              `json:"name,omitempty"`
	ToolCallID string              `json:"tool_call_id,omitempty"`
	ToolCalls  []codexChatToolCall `json:"tool_calls,omitempty"`
}

type codexChatToolCall struct {
	ID       string                `json:"id,omitempty"`
	Type     string                `json:"type,omitempty"`
	Function codexChatToolFunction `json:"function,omitempty"`
}

type codexChatToolFunction struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type codexResponsesEvent struct {
	Type        string                 `json:"type"`
	Delta       string                 `json:"delta,omitempty"`
	OutputIndex int                    `json:"output_index,omitempty"`
	Response    *codexResponsesPayload `json:"response,omitempty"`
	Item        *codexResponsesOutput  `json:"item,omitempty"`
}

type codexResponsesPayload struct {
	ID                string                 `json:"id,omitempty"`
	Model             string                 `json:"model,omitempty"`
	Status            string                 `json:"status,omitempty"`
	Output            []codexResponsesOutput `json:"output,omitempty"`
	Usage             *codexResponsesUsage   `json:"usage,omitempty"`
	IncompleteDetails *struct {
		Reason string `json:"reason,omitempty"`
	} `json:"incomplete_details,omitempty"`
	Error *struct {
		Message string `json:"message,omitempty"`
	} `json:"error,omitempty"`
}

type codexResponsesOutput struct {
	Type      string               `json:"type,omitempty"`
	Role      string               `json:"role,omitempty"`
	Content   []codexResponsesPart `json:"content,omitempty"`
	CallID    string               `json:"call_id,omitempty"`
	Name      string               `json:"name,omitempty"`
	Arguments string               `json:"arguments,omitempty"`
	Summary   []codexResponsesPart `json:"summary,omitempty"`
}

type codexResponsesPart struct {
	Type string `json:"type,omitempty"`
	Text string `json:"text,omitempty"`
}

type codexResponsesUsage struct {
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
	TotalTokens  int `json:"total_tokens,omitempty"`
}

type codexChatCompletion struct {
	ID      string            `json:"id"`
	Object  string            `json:"object"`
	Created int64             `json:"created"`
	Model   string            `json:"model"`
	Choices []codexChatChoice `json:"choices"`
	Usage   *codexChatUsage   `json:"usage,omitempty"`
}

type codexChatChoice struct {
	Index        int             `json:"index"`
	Message      codexChatOutput `json:"message"`
	FinishReason string          `json:"finish_reason"`
}

type codexChatOutput struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type codexChatUsage struct {
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`
}

type codexChatChunk struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []codexChunkChoice `json:"choices"`
	Usage   *codexChatUsage    `json:"usage,omitempty"`
}

type codexChunkChoice struct {
	Index        int            `json:"index"`
	Delta        codexChatDelta `json:"delta"`
	FinishReason *string        `json:"finish_reason"`
}

type codexChatDelta struct {
	Role    string  `json:"role,omitempty"`
	Content *string `json:"content,omitempty"`
}

func isCodexChatCompletionsRoute(route routeInfo, selected token.Token) bool {
	if !isCodexCredential(selected) {
		return false
	}
	path := stripPathPrefix(route.Path, "/backend-api/codex")
	path = stripPathPrefix(path, "/codex")
	path = stripPathPrefix(path, "/v1")
	return path == "/chat/completions"
}

func (s *Service) forwardCodexChatCompletions(ctx context.Context, original *http.Request, route routeInfo, body []byte, selected token.Token) (*http.Response, error) {
	if original.Method != http.MethodPost {
		return codexJSONProxyResponse(http.StatusMethodNotAllowed, []byte(`{"error":{"message":"method not allowed"}}`)), nil
	}

	upstreamBody, clientStream, err := buildCodexResponsesRequestBody(body)
	if err != nil {
		return codexJSONProxyResponse(http.StatusBadRequest, []byte(fmt.Sprintf(`{"error":{"message":%q}}`, err.Error()))), nil
	}

	upstreamRoute := route
	upstreamRoute.Path = "/responses"
	upstreamRoute.RawQuery = ""
	targetURL, err := s.router.TargetURL(upstreamRoute, selected)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(upstreamBody))
	if err != nil {
		return nil, err
	}
	copyHeader(req.Header, original.Header)
	removeHopHeaders(req.Header)
	removeClientIdentificationHeaders(req.Header)
	if err := applyRouteAuth(req.Header, selected, upstreamRoute); err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if strings.TrimSpace(req.Header.Get("OpenAI-Beta")) == "" {
		req.Header.Set("OpenAI-Beta", "responses=experimental")
	}
	if strings.TrimSpace(req.Header.Get("originator")) == "" {
		req.Header.Set("originator", "codex_cli_rs")
	}
	req.Host = req.URL.Host

	resp, err := s.clientForRoute(route).Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp, nil
	}

	converted, err := convertCodexResponsesToChat(resp, route.Model, clientStream)
	if err != nil {
		closeBody(resp.Body)
		return nil, err
	}
	return converted, nil
}

func buildCodexResponsesRequestBody(body []byte) ([]byte, bool, error) {
	var req codexChatRequest
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&req); err != nil {
		return nil, false, fmt.Errorf("invalid chat completions request: %w", err)
	}
	if strings.TrimSpace(req.Model) == "" {
		return nil, false, errors.New("missing model")
	}
	if len(req.Messages) == 0 {
		return nil, false, errors.New("missing messages")
	}

	input := make([]any, 0, len(req.Messages))
	instructions := strings.TrimSpace(req.Instructions)
	for _, msg := range req.Messages {
		items, systemText, err := codexChatMessageToResponsesInput(msg)
		if err != nil {
			return nil, false, err
		}
		if systemText != "" {
			if instructions == "" {
				instructions = systemText
			} else {
				instructions = systemText + "\n\n" + instructions
			}
		}
		input = append(input, items...)
	}
	if instructions == "" {
		instructions = "You are a helpful coding assistant."
	}

	out := map[string]any{
		"model":        normalizeCodexChatModel(req.Model),
		"input":        input,
		"instructions": instructions,
		"stream":       true,
		"store":        false,
	}
	if maxTokens := codexMaxOutputTokens(req); maxTokens > 0 {
		out["max_output_tokens"] = maxTokens
	}
	if effort := strings.TrimSpace(req.ReasoningEffort); effort != "" {
		out["reasoning"] = map[string]any{"effort": effort, "summary": "auto"}
	}
	if tier := strings.TrimSpace(req.ServiceTier); tier != "" {
		out["service_tier"] = tier
	}
	if tools, ok := codexConvertChatTools(req.Tools, req.Functions); ok {
		out["tools"] = tools
	}
	if choice, ok := codexConvertToolChoice(req.ToolChoice, req.FunctionCall); ok {
		out["tool_choice"] = choice
	}

	data, err := json.Marshal(out)
	if err != nil {
		return nil, false, err
	}
	return data, req.Stream, nil
}

func codexChatMessageToResponsesInput(msg codexChatMsg) ([]any, string, error) {
	role := strings.ToLower(strings.TrimSpace(msg.Role))
	switch role {
	case "system", "developer":
		return nil, codexPlainTextContent(msg.Content), nil
	case "assistant":
		items := []any{}
		content, err := codexChatContentToResponses(msg.Content, true)
		if err != nil {
			return nil, "", err
		}
		if !codexEmptyContent(content) {
			items = append(items, map[string]any{
				"type":    "message",
				"role":    "assistant",
				"content": content,
			})
		}
		for _, call := range msg.ToolCalls {
			args := strings.TrimSpace(call.Function.Arguments)
			if args == "" {
				args = "{}"
			}
			items = append(items, map[string]any{
				"type":      "function_call",
				"call_id":   strings.TrimSpace(call.ID),
				"name":      strings.TrimSpace(call.Function.Name),
				"arguments": args,
			})
		}
		return items, "", nil
	case "tool", "function":
		output := codexPlainTextContent(msg.Content)
		if output == "" {
			output = "(empty)"
		}
		callID := strings.TrimSpace(msg.ToolCallID)
		if callID == "" {
			callID = strings.TrimSpace(msg.Name)
		}
		return []any{map[string]any{
			"type":    "function_call_output",
			"call_id": callID,
			"output":  output,
		}}, "", nil
	default:
		content, err := codexChatContentToResponses(msg.Content, false)
		if err != nil {
			return nil, "", err
		}
		return []any{map[string]any{
			"type":    "message",
			"role":    "user",
			"content": content,
		}}, "", nil
	}
}

func codexChatContentToResponses(raw json.RawMessage, assistant bool) (any, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text, nil
	}
	var parts []map[string]any
	if err := json.Unmarshal(raw, &parts); err != nil {
		return nil, fmt.Errorf("unsupported message content")
	}
	out := make([]any, 0, len(parts))
	for _, part := range parts {
		switch strings.TrimSpace(codexStringValue(part["type"])) {
		case "text", "input_text", "output_text":
			text := codexStringValue(part["text"])
			if text == "" {
				continue
			}
			partType := "input_text"
			if assistant {
				partType = "output_text"
			}
			out = append(out, map[string]any{"type": partType, "text": text})
		case "image_url":
			if assistant {
				continue
			}
			imageURL := ""
			if image, ok := part["image_url"].(map[string]any); ok {
				imageURL = codexStringValue(image["url"])
			} else {
				imageURL = codexStringValue(part["image_url"])
			}
			if imageURL != "" {
				out = append(out, map[string]any{"type": "input_image", "image_url": imageURL})
			}
		}
	}
	return out, nil
}

func codexPlainTextContent(raw json.RawMessage) string {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text)
	}
	var parts []map[string]any
	if err := json.Unmarshal(raw, &parts); err != nil {
		return ""
	}
	var out strings.Builder
	for _, part := range parts {
		switch strings.TrimSpace(codexStringValue(part["type"])) {
		case "text", "input_text", "output_text":
			out.WriteString(codexStringValue(part["text"]))
		}
	}
	return strings.TrimSpace(out.String())
}

func codexEmptyContent(value any) bool {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed) == ""
	case []any:
		return len(typed) == 0
	default:
		return value == nil
	}
}

func codexMaxOutputTokens(req codexChatRequest) int {
	if req.MaxCompletionTokens != nil && *req.MaxCompletionTokens > 0 {
		return *req.MaxCompletionTokens
	}
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		return *req.MaxTokens
	}
	return 0
}

func codexConvertChatTools(rawTools json.RawMessage, rawFunctions json.RawMessage) ([]any, bool) {
	if len(rawTools) == 0 && len(rawFunctions) != 0 {
		var functions []map[string]any
		if err := json.Unmarshal(rawFunctions, &functions); err != nil || len(functions) == 0 {
			return nil, false
		}
		tools := make([]any, 0, len(functions))
		for _, fn := range functions {
			tools = append(tools, map[string]any{
				"type":        "function",
				"name":        codexStringValue(fn["name"]),
				"description": codexStringValue(fn["description"]),
				"parameters":  fn["parameters"],
			})
		}
		return tools, true
	}
	if len(rawTools) == 0 {
		return nil, false
	}
	var tools []map[string]any
	if err := json.Unmarshal(rawTools, &tools); err != nil || len(tools) == 0 {
		return nil, false
	}
	out := make([]any, 0, len(tools))
	for _, tool := range tools {
		if strings.TrimSpace(codexStringValue(tool["type"])) != "function" {
			out = append(out, tool)
			continue
		}
		if fn, ok := tool["function"].(map[string]any); ok {
			out = append(out, map[string]any{
				"type":        "function",
				"name":        codexStringValue(fn["name"]),
				"description": codexStringValue(fn["description"]),
				"parameters":  fn["parameters"],
				"strict":      fn["strict"],
			})
			continue
		}
		out = append(out, tool)
	}
	return out, true
}

func codexConvertToolChoice(rawChoice json.RawMessage, rawFunctionCall json.RawMessage) (any, bool) {
	if len(rawChoice) == 0 && len(rawFunctionCall) != 0 {
		var nameChoice struct {
			Name string `json:"name"`
		}
		var text string
		if err := json.Unmarshal(rawFunctionCall, &text); err == nil {
			return text, strings.TrimSpace(text) != ""
		}
		if err := json.Unmarshal(rawFunctionCall, &nameChoice); err == nil && strings.TrimSpace(nameChoice.Name) != "" {
			return map[string]any{"type": "function", "name": strings.TrimSpace(nameChoice.Name)}, true
		}
		return nil, false
	}
	if len(rawChoice) == 0 {
		return nil, false
	}
	var text string
	if err := json.Unmarshal(rawChoice, &text); err == nil {
		return text, strings.TrimSpace(text) != ""
	}
	var choice map[string]any
	if err := json.Unmarshal(rawChoice, &choice); err != nil {
		return nil, false
	}
	if strings.TrimSpace(codexStringValue(choice["type"])) == "function" {
		name := codexStringValue(choice["name"])
		if name == "" {
			if fn, ok := choice["function"].(map[string]any); ok {
				name = codexStringValue(fn["name"])
			}
		}
		if name != "" {
			return map[string]any{"type": "function", "name": name}, true
		}
		return "auto", true
	}
	return choice, true
}
