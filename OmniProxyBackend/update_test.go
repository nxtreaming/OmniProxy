package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckForUpdatesReportsAvailableRelease(t *testing.T) {
	restore := overrideUpdateGlobals("v1.0.2", "")
	defer restore()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "OmniProxy/v1.0.2" {
			t.Fatalf("unexpected user agent: %q", r.Header.Get("User-Agent"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v1.0.3",
			"name":     "OmniProxy v1.0.3",
			"html_url": "https://github.com/mibgb65-cloud/OmniProxy/releases/tag/v1.0.3",
			"body":     "release notes",
			"assets": []map[string]string{
				{
					"name":                 "OmniProxy-Setup-v1.0.3-windows-amd64.exe",
					"browser_download_url": "https://example.com/installer.exe",
				},
			},
		})
	}))
	defer server.Close()
	latestReleaseURL = server.URL

	info, err := checkForUpdates(context.Background(), server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if !info.UpdateAvailable {
		t.Fatalf("expected update to be available, got %#v", info)
	}
	if info.CurrentVersion != "v1.0.2" || info.LatestVersion != "v1.0.3" || info.DownloadURL != "https://example.com/installer.exe" {
		t.Fatalf("unexpected update info: %#v", info)
	}
}

func TestCheckForUpdatesSkipsDevelopmentVersion(t *testing.T) {
	restore := overrideUpdateGlobals("dev", "")
	defer restore()

	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	latestReleaseURL = server.URL

	info, err := checkForUpdates(context.Background(), server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("development builds should not call the release API")
	}
	if info.UpdateAvailable || info.CurrentVersion != "dev" {
		t.Fatalf("unexpected development update info: %#v", info)
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		left  string
		right string
		want  int
	}{
		{left: "v1.0.3", right: "v1.0.2", want: 1},
		{left: "v1.0.10", right: "v1.0.2", want: 1},
		{left: "v1.0.2", right: "v1.0.2", want: 0},
		{left: "v1.0.2", right: "v1.0.3", want: -1},
		{left: "dev", right: "v1.0.3", want: 0},
	}
	for _, tt := range tests {
		if got := compareVersions(tt.left, tt.right); got != tt.want {
			t.Fatalf("compareVersions(%q, %q) = %d, want %d", tt.left, tt.right, got, tt.want)
		}
	}
}

func overrideUpdateGlobals(version string, releaseURL string) func() {
	oldVersion := appVersion
	oldReleaseURL := latestReleaseURL
	appVersion = version
	if releaseURL != "" {
		latestReleaseURL = releaseURL
	}
	return func() {
		appVersion = oldVersion
		latestReleaseURL = oldReleaseURL
	}
}
