package main

import "testing"

func TestParseProviderCatalogModelsSupportsOpenAIAndGeminiShapes(t *testing.T) {
	body := []byte(`{
		"data": [
			{"id": "gpt-test", "name": "GPT Test", "context_length": 128000},
			{"id": ""}
		]
	}`)
	models, err := parseProviderCatalogModels(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].ID != "gpt-test" || models[0].ContextLength != 128000 {
		t.Fatalf("unexpected OpenAI-compatible models: %#v", models)
	}

	body = []byte(`{
		"models": [
			{"name": "models/gemini-test", "displayName": "Gemini Test", "inputTokenLimit": 1048576}
		]
	}`)
	models, err = parseProviderCatalogModels(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].ID != "gemini-test" || models[0].Name != "Gemini Test" {
		t.Fatalf("unexpected Gemini models: %#v", models)
	}
}
