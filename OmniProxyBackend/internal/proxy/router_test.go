package proxy

import (
	"net/url"
	"testing"

	"OmniProxyBackend/internal/config"
)

func TestRouterReadsRequestBodyModel(t *testing.T) {
	route := NewRouter(config.Config{}).Route(mustRouterTestURL(t, "/backend-api/codex/responses"), []byte(`{"model":"gpt-body","input":"hello"}`))

	if route.Model != "gpt-body" {
		t.Fatalf("expected body model, got %#v", route)
	}
}

func TestRouterReadsQueryModel(t *testing.T) {
	route := NewRouter(config.Config{}).Route(mustRouterTestURL(t, "/v1/responses?model=gpt-query"), []byte(`{}`))

	if route.Model != "gpt-query" {
		t.Fatalf("expected query model, got %#v", route)
	}
}

func mustRouterTestURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
