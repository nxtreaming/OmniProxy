package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"omniproxy/internal/token"
)

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

func TestFetchForgeCatalogModelsUsesVersionedModelsEndpoint(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("unexpected models path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer fg-forge-api-key-token" {
			t.Fatalf("unexpected Authorization header: %q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"claude-sonnet-5","context_length":200000}]}`))
	}))
	defer upstream.Close()

	models, err := fetchOpenAICompatibleCatalogModels(
		context.Background(),
		upstream.Client(),
		token.ProviderForge,
		upstream.URL+"/v1",
		"fg-forge-api-key-token",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].ID != "claude-sonnet-5" || models[0].ContextLength != 200000 {
		t.Fatalf("unexpected Forge models: %#v", models)
	}
}
