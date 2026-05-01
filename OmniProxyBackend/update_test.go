package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
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
			"assets": []map[string]any{
				{
					"name":                 "OmniProxy-Setup-v1.0.3-windows-amd64.exe",
					"browser_download_url": "https://example.com/installer.exe",
					"size":                 123,
				},
				{
					"name":                 "OmniProxy-Setup-v1.0.3-windows-amd64.exe.sha256",
					"browser_download_url": "https://example.com/installer.exe.sha256",
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
	if info.ChecksumURL != "https://example.com/installer.exe.sha256" || info.DownloadFileName != "OmniProxy-Setup-v1.0.3-windows-amd64.exe" || info.DownloadSize != 123 {
		t.Fatalf("unexpected update info: %#v", info)
	}
}

func TestUpdateDownloadAssetFromAssetsSelectsInstallerAndChecksum(t *testing.T) {
	asset := updateDownloadAssetFromAssets([]githubReleaseAsset{
		{Name: "source.zip", BrowserDownloadURL: "https://example.com/source.zip"},
		{Name: "OmniProxy-Setup-v1.2.3-windows-amd64.exe.sha256", BrowserDownloadURL: "https://example.com/setup.exe.sha256"},
		{Name: "OmniProxy-Setup-v1.2.3-windows-amd64.exe", BrowserDownloadURL: "https://example.com/setup.exe", Size: 42},
	})
	if asset.URL != "https://example.com/setup.exe" || asset.ChecksumURL != "https://example.com/setup.exe.sha256" || asset.FileName != "OmniProxy-Setup-v1.2.3-windows-amd64.exe" || asset.Size != 42 {
		t.Fatalf("unexpected asset selection: %#v", asset)
	}
}

func TestUpdateDownloaderDownloadsAndVerifiesInstaller(t *testing.T) {
	content := []byte("fake installer bytes")
	sum := sha256.Sum256(content)
	fileName := fmt.Sprintf("OmniProxy-Setup-test-%d.exe", time.Now().UnixNano())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/installer.exe":
			w.Header().Set("Content-Length", fmt.Sprint(len(content)))
			_, _ = w.Write(content)
		case "/installer.exe.sha256":
			_, _ = fmt.Fprintf(w, "%x  %s\n", sum, fileName)
		default:
			t.Fatalf("unexpected download path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	downloader := newUpdateDownloader()
	status, err := downloader.Start(context.Background(), server.Client(), updateDownloadRequest{
		Version:      "v9.9.9",
		DownloadURL:  server.URL + "/installer.exe",
		ChecksumURL:  server.URL + "/installer.exe.sha256",
		FileName:     fileName,
		ExpectedSize: int64(len(content)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "downloading" {
		t.Fatalf("expected downloading status, got %#v", status)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		status = downloader.Status()
		if status.State == "downloaded" || status.State == "failed" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	defer func() {
		if status.FilePath != "" {
			_ = os.Remove(status.FilePath)
		}
	}()
	if status.State != "downloaded" {
		t.Fatalf("expected downloaded status, got %#v", status)
	}
	if !status.Verified || status.Percent != 100 || status.BytesReceived != int64(len(content)) {
		t.Fatalf("unexpected download status: %#v", status)
	}
	saved, err := os.ReadFile(status.FilePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(saved) != string(content) {
		t.Fatalf("unexpected saved content: %q", saved)
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
		{left: "v1.0.9", right: "v1.0.9-beta.1", want: 1},
		{left: "v1.0.9-beta.2", right: "v1.0.9-beta.1", want: 1},
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
