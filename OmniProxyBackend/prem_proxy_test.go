package main

import "testing"

func TestPremProxyTargetFromBaseURL(t *testing.T) {
	target, err := premProxyTargetFromBaseURL("http://127.0.0.1:3100/v1")
	if err != nil {
		t.Fatalf("parse local Prem Base URL: %v", err)
	}
	if target.Host != "127.0.0.1" || target.Port != "3100" || target.Address != "127.0.0.1:3100" || !target.Loopback {
		t.Fatalf("unexpected local target: %#v", target)
	}

	target, err = premProxyTargetFromBaseURL("https://gateway.prem.io/v1")
	if err != nil {
		t.Fatalf("parse remote Prem Base URL: %v", err)
	}
	if target.Port != "443" || target.Loopback {
		t.Fatalf("expected remote https target without auto-start, got %#v", target)
	}
}

func TestPremProxyTargetRejectsInvalidBaseURL(t *testing.T) {
	if _, err := premProxyTargetFromBaseURL("127.0.0.1:3100/v1"); err == nil {
		t.Fatal("expected missing scheme to be rejected")
	}
	if _, err := premProxyTargetFromBaseURL("ftp://127.0.0.1:3100/v1"); err == nil {
		t.Fatal("expected unsupported scheme to be rejected")
	}
}
