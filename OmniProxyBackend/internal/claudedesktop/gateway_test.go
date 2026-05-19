package claudedesktop

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestRewriteRequestBodyMapsDesktopRouteToUpstreamModel(t *testing.T) {
	updated, model, err := RewriteRequestBody([]byte(`{"model":"claude-sonnet-4-6","messages":[],"temperature":0.2}`), []ModelRoute{
		{RouteID: "claude-sonnet-4-6", UpstreamModel: "deepseek-v4-pro[1m]"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if model != "deepseek-v4-pro[1m]" {
		t.Fatalf("unexpected upstream model %q", model)
	}
	var payload map[string]any
	if err := json.Unmarshal(updated, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["model"] != "deepseek-v4-pro[1m]" || payload["temperature"].(float64) != 0.2 {
		t.Fatalf("unexpected rewritten body: %#v", payload)
	}
}

func TestValidateGatewayAuthRequiresConfiguredBearerToken(t *testing.T) {
	header := http.Header{}
	header.Set("Authorization", "Bearer "+GatewayToken)
	if err := ValidateGatewayAuth(header); err != nil {
		t.Fatal(err)
	}
	header.Set("Authorization", "Bearer wrong")
	if err := ValidateGatewayAuth(header); err == nil {
		t.Fatal("expected invalid token to fail")
	}
}
