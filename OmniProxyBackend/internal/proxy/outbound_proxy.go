package proxy

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"OmniProxyBackend/internal/config"
)

func outboundProxyURL(cfg config.Config) (*url.URL, error) {
	cfg = config.Normalize(cfg)
	if !cfg.OutboundProxyEnabled {
		return nil, nil
	}
	raw := strings.TrimSpace(cfg.OutboundProxyURL)
	if raw == "" {
		return nil, nil
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid outbound proxy url: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid outbound proxy url %q", raw)
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https", "socks5", "socks5h":
		return parsed, nil
	default:
		return nil, fmt.Errorf("unsupported outbound proxy scheme %q", parsed.Scheme)
	}
}

func newHTTPClient(timeout time.Duration, proxyURL *url.URL) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxyURL != nil {
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}

func outboundProxyMatchesModel(model string, patterns []string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	if model == "" {
		return false
	}
	for _, pattern := range patterns {
		pattern = strings.ToLower(strings.TrimSpace(pattern))
		if pattern == "" {
			continue
		}
		if pattern == "*" || pattern == "all" {
			return true
		}
		if wildcardMatch(pattern, model) {
			return true
		}
	}
	return false
}

func wildcardMatch(pattern string, value string) bool {
	if !strings.Contains(pattern, "*") {
		return pattern == value
	}
	anchoredStart := !strings.HasPrefix(pattern, "*")
	anchoredEnd := !strings.HasSuffix(pattern, "*")
	parts := strings.Split(pattern, "*")
	offset := 0
	lastMatched := ""
	for index, part := range parts {
		if part == "" {
			continue
		}
		found := strings.Index(value[offset:], part)
		if found < 0 {
			return false
		}
		if index == 0 && anchoredStart && found != 0 {
			return false
		}
		offset += found + len(part)
		lastMatched = part
	}
	if anchoredEnd && lastMatched != "" && !strings.HasSuffix(value, lastMatched) {
		return false
	}
	return true
}
