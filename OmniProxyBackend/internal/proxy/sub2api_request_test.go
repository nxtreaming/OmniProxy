package proxy

import (
	"strings"
	"testing"
)

func TestStripOpenAIImageGenerationToolsRecursively(t *testing.T) {
	body := []byte(`{
		"model":"gpt-5.5",
		"input":"user text mentions image_generation literally",
		"tools":[
			{"type":"image_generation"},
			{"type":"web_search_preview","config":{"tools":[{"type":"image_generation"}]}}
		],
		"tool_choice":{"type":"image_generation"},
		"include":["reasoning.encrypted_content","image_generation_call.partial_images"],
		"metadata":{"tool_choice":{"tool":{"type":"image_generation"}}}
	}`)

	updated, changed := stripOpenAIImageGenerationTools(body)
	if !changed {
		t.Fatal("expected body to be changed")
	}
	text := string(updated)
	if strings.Contains(text, `"type":"image_generation"`) {
		t.Fatalf("expected image_generation tools to be removed, got %s", text)
	}
	if strings.Contains(text, `"tool_choice"`) {
		t.Fatalf("expected image_generation tool_choice fields to be removed, got %s", text)
	}
	if strings.Contains(text, "image_generation_call.partial_images") {
		t.Fatalf("expected image_generation include entries to be removed, got %s", text)
	}
	if !strings.Contains(text, "web_search_preview") {
		t.Fatalf("expected non-image tools to be preserved, got %s", text)
	}
	if !strings.Contains(text, "user text mentions image_generation literally") {
		t.Fatalf("expected user text to be preserved, got %s", text)
	}
}

func TestStripOpenAIImageGenerationToolsLeavesTextRequestUnchanged(t *testing.T) {
	body := []byte(`{"model":"gpt-5.5","input":"hi","tools":[{"type":"web_search_preview"}],"include":["reasoning.encrypted_content"]}`)

	updated, changed := stripOpenAIImageGenerationTools(body)
	if changed {
		t.Fatalf("expected text request to be unchanged, got %s", string(updated))
	}
	if string(updated) != string(body) {
		t.Fatalf("expected original body to be returned")
	}
}
