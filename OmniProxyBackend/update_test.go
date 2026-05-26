package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
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
				{
					"name":                 "OmniProxy-v1.0.3-darwin-universal.dmg",
					"browser_download_url": "https://example.com/installer.dmg",
					"size":                 456,
				},
				{
					"name":                 "OmniProxy-v1.0.3-darwin-universal.dmg.sha256",
					"browser_download_url": "https://example.com/installer.dmg.sha256",
				},
			},
		})
	}))
	defer server.Close()
	latestReleaseURL = server.URL

	info, err := checkForUpdates(context.Background(), server.Client(), false)
	if err != nil {
		t.Fatal(err)
	}
	if !info.UpdateAvailable {
		t.Fatalf("expected update to be available, got %#v", info)
	}
	wantName, wantURL, wantChecksum, wantSize := expectedUpdateAsset("v1.0.3", "installer")
	if info.CurrentVersion != "v1.0.2" || info.LatestVersion != "v1.0.3" || info.DownloadURL != wantURL {
		t.Fatalf("unexpected update info: %#v", info)
	}
	if info.ChecksumURL != wantChecksum || info.DownloadFileName != wantName || info.DownloadSize != wantSize {
		t.Fatalf("unexpected update info: %#v", info)
	}
}

func TestCheckForUpdatesReportsPrereleaseWhenEnabled(t *testing.T) {
	restore := overrideUpdateGlobals("v1.0.2", "")
	defer restore()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			t.Fatalf("unexpected releases path: %s", r.URL.String())
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"tag_name":   "v1.0.3",
				"name":       "OmniProxy v1.0.3",
				"html_url":   "https://github.com/mibgb65-cloud/OmniProxy/releases/tag/v1.0.3",
				"body":       "stable notes",
				"prerelease": false,
				"assets": []map[string]any{
					{
						"name":                 "OmniProxy-Setup-v1.0.3-windows-amd64.exe",
						"browser_download_url": "https://example.com/stable.exe",
					},
					{
						"name":                 "OmniProxy-Setup-v1.0.3-windows-amd64.exe.sha256",
						"browser_download_url": "https://example.com/stable.exe.sha256",
					},
					{
						"name":                 "OmniProxy-v1.0.3-darwin-universal.dmg",
						"browser_download_url": "https://example.com/stable.dmg",
					},
					{
						"name":                 "OmniProxy-v1.0.3-darwin-universal.dmg.sha256",
						"browser_download_url": "https://example.com/stable.dmg.sha256",
					},
				},
			},
			{
				"tag_name":   "v1.0.4-beta.1",
				"name":       "OmniProxy v1.0.4-beta.1",
				"html_url":   "https://github.com/mibgb65-cloud/OmniProxy/releases/tag/v1.0.4-beta.1",
				"body":       "beta notes",
				"prerelease": true,
				"assets": []map[string]any{
					{
						"name":                 "OmniProxy-Setup-v1.0.4-beta.1-windows-amd64.exe",
						"browser_download_url": "https://example.com/beta.exe",
						"size":                 456,
					},
					{
						"name":                 "OmniProxy-Setup-v1.0.4-beta.1-windows-amd64.exe.sha256",
						"browser_download_url": "https://example.com/beta.exe.sha256",
					},
					{
						"name":                 "OmniProxy-v1.0.4-beta.1-darwin-universal.dmg",
						"browser_download_url": "https://example.com/beta.dmg",
						"size":                 789,
					},
					{
						"name":                 "OmniProxy-v1.0.4-beta.1-darwin-universal.dmg.sha256",
						"browser_download_url": "https://example.com/beta.dmg.sha256",
					},
				},
			},
		})
	}))
	defer server.Close()
	releasesURL = server.URL

	info, err := checkForUpdates(context.Background(), server.Client(), true)
	if err != nil {
		t.Fatal(err)
	}
	if !info.UpdateAvailable || !info.Prerelease {
		t.Fatalf("expected prerelease update, got %#v", info)
	}
	_, wantURL, _, _ := expectedUpdateAsset("v1.0.4-beta.1", "beta")
	if info.LatestVersion != "v1.0.4-beta.1" || info.DownloadURL != wantURL {
		t.Fatalf("unexpected prerelease update info: %#v", info)
	}
}

func TestLatestVersionedReleaseSkipsPrereleaseWhenDisabled(t *testing.T) {
	releases := []githubRelease{
		{TagName: "v1.0.4-beta.1", Prerelease: true},
		{TagName: "v1.0.3"},
	}
	release, ok := latestVersionedRelease(releases, false)
	if !ok || release.TagName != "v1.0.3" {
		t.Fatalf("expected latest stable release, got ok=%v release=%#v", ok, release)
	}
	release, ok = latestVersionedRelease(releases, true)
	if !ok || release.TagName != "v1.0.4-beta.1" {
		t.Fatalf("expected prerelease candidate when enabled, got ok=%v release=%#v", ok, release)
	}
}

func TestUpdateDownloadAssetFromAssetsSelectsInstallerAndChecksum(t *testing.T) {
	asset := updateDownloadAssetFromAssetsForPlatform([]githubReleaseAsset{
		{Name: "source.zip", BrowserDownloadURL: "https://example.com/source.zip"},
		{Name: "OmniProxy-Setup-v1.2.3-windows-amd64.exe.sha256", BrowserDownloadURL: "https://example.com/setup.exe.sha256"},
		{Name: "OmniProxy-Setup-v1.2.3-windows-amd64.exe", BrowserDownloadURL: "https://example.com/setup.exe", Size: 42},
	}, "windows", "amd64")
	if asset.URL != "https://example.com/setup.exe" || asset.ChecksumURL != "https://example.com/setup.exe.sha256" || asset.FileName != "OmniProxy-Setup-v1.2.3-windows-amd64.exe" || asset.Size != 42 {
		t.Fatalf("unexpected asset selection: %#v", asset)
	}
}

func TestUpdateDownloadAssetFromAssetsSelectsDarwinUniversal(t *testing.T) {
	asset := updateDownloadAssetFromAssetsForPlatform([]githubReleaseAsset{
		{Name: "OmniProxy-Setup-v1.2.3-windows-amd64.exe", BrowserDownloadURL: "https://example.com/setup.exe"},
		{Name: "OmniProxy-v1.2.3-darwin-arm64.dmg", BrowserDownloadURL: "https://example.com/arm64.dmg"},
		{Name: "OmniProxy-v1.2.3-darwin-universal.dmg.sha256", BrowserDownloadURL: "https://example.com/universal.dmg.sha256"},
		{Name: "OmniProxy-v1.2.3-darwin-universal.dmg", BrowserDownloadURL: "https://example.com/universal.dmg", Size: 84},
	}, "darwin", "arm64")
	if asset.URL != "https://example.com/universal.dmg" || asset.ChecksumURL != "https://example.com/universal.dmg.sha256" || asset.FileName != "OmniProxy-v1.2.3-darwin-universal.dmg" || asset.Size != 84 {
		t.Fatalf("unexpected darwin asset selection: %#v", asset)
	}
}

func TestUpdateDownloaderDownloadsAndVerifiesInstaller(t *testing.T) {
	content := []byte("fake installer bytes")
	sum := sha256.Sum256(content)
	fileName := testInstallerFileName()

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

func TestUpdateDownloaderInstallStartsSilentAutoUpdate(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "OmniProxy-Setup-test.exe")
	if err := os.WriteFile(filePath, []byte("fake installer"), 0o600); err != nil {
		t.Fatal(err)
	}

	oldStart := updateInstallerStart
	defer func() {
		updateInstallerStart = oldStart
	}()

	var gotPath string
	var gotArgs []string
	updateInstallerStart = func(filePath string, args []string) error {
		gotPath = filePath
		gotArgs = append([]string(nil), args...)
		return nil
	}

	downloader := newUpdateDownloader()
	downloader.status = updateDownloadStatus{
		State:    "downloaded",
		FileName: filepath.Base(filePath),
		FilePath: filePath,
		Verified: true,
	}

	status, err := downloader.Install()
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "installing" {
		t.Fatalf("expected installing status, got %#v", status)
	}
	if gotPath != filePath {
		t.Fatalf("expected installer path %q, got %q", filePath, gotPath)
	}
	if !reflect.DeepEqual(gotArgs, updateInstallerArgs()) {
		t.Fatalf("unexpected installer args: %#v", gotArgs)
	}
}

func TestUpdateInstallerFileNameValidatesPlatformExtensions(t *testing.T) {
	if name, err := updateInstallerFileNameForPlatform(updateDownloadRequest{FileName: "OmniProxy-v1.2.3-darwin-universal.dmg"}, "darwin"); err != nil || name != "OmniProxy-v1.2.3-darwin-universal.dmg" {
		t.Fatalf("expected darwin dmg to be accepted, name=%q err=%v", name, err)
	}
	if _, err := updateInstallerFileNameForPlatform(updateDownloadRequest{FileName: "OmniProxy-v1.2.3-darwin-universal.dmg"}, "windows"); err == nil {
		t.Fatal("expected windows updater to reject dmg installer")
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

	info, err := checkForUpdates(context.Background(), server.Client(), false)
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
	oldReleasesURL := releasesURL
	appVersion = version
	if releaseURL != "" {
		latestReleaseURL = releaseURL
	}
	return func() {
		appVersion = oldVersion
		latestReleaseURL = oldReleaseURL
		releasesURL = oldReleasesURL
	}
}

func expectedUpdateAsset(version string, stem string) (string, string, string, int64) {
	if runtime.GOOS == "darwin" {
		name := fmt.Sprintf("OmniProxy-%s-darwin-universal.dmg", version)
		return name, fmt.Sprintf("https://example.com/%s.dmg", stem), fmt.Sprintf("https://example.com/%s.dmg.sha256", stem), 456
	}
	name := fmt.Sprintf("OmniProxy-Setup-%s-windows-amd64.exe", version)
	return name, fmt.Sprintf("https://example.com/%s.exe", stem), fmt.Sprintf("https://example.com/%s.exe.sha256", stem), 123
}

func testInstallerFileName() string {
	if runtime.GOOS == "darwin" {
		return fmt.Sprintf("OmniProxy-test-%d.dmg", time.Now().UnixNano())
	}
	return fmt.Sprintf("OmniProxy-Setup-test-%d.exe", time.Now().UnixNano())
}
