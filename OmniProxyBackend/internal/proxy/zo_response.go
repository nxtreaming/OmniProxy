package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

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
