package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	goruntime "runtime"
	"strings"
	"sync"
	"time"
)

var latestReleaseURL = "https://api.github.com/repos/mibgb65-cloud/OmniProxy/releases/latest"
var releasesURL = "https://api.github.com/repos/mibgb65-cloud/OmniProxy/releases"
var updateInstallerStart = defaultStartUpdateInstaller

const updateCheckTimeout = 5 * time.Second

type updateInfo struct {
	CurrentVersion   string `json:"currentVersion"`
	LatestVersion    string `json:"latestVersion,omitempty"`
	UpdateAvailable  bool   `json:"updateAvailable"`
	ReleaseURL       string `json:"releaseUrl,omitempty"`
	DownloadURL      string `json:"downloadUrl,omitempty"`
	ChecksumURL      string `json:"checksumUrl,omitempty"`
	DownloadFileName string `json:"downloadFileName,omitempty"`
	DownloadSize     int64  `json:"downloadSize,omitempty"`
	Name             string `json:"name,omitempty"`
	Body             string `json:"body,omitempty"`
	Prerelease       bool   `json:"prerelease,omitempty"`
}

type appInfo struct {
	Name           string `json:"name"`
	Version        string `json:"version"`
	IsDevelopment  bool   `json:"isDevelopment"`
	UpdateEndpoint string `json:"updateEndpoint"`
	Platform       string `json:"platform"`
	GoVersion      string `json:"goVersion"`
	ExecutablePath string `json:"executablePath,omitempty"`
	StartedAt      string `json:"startedAt"`
}

type githubRelease struct {
	TagName    string               `json:"tag_name"`
	Name       string               `json:"name"`
	HTMLURL    string               `json:"html_url"`
	Body       string               `json:"body"`
	Prerelease bool                 `json:"prerelease"`
	Draft      bool                 `json:"draft"`
	Assets     []githubReleaseAsset `json:"assets"`
}

type githubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

type updateDownloadRequest struct {
	Version      string `json:"version,omitempty"`
	DownloadURL  string `json:"downloadUrl"`
	ChecksumURL  string `json:"checksumUrl,omitempty"`
	FileName     string `json:"fileName,omitempty"`
	ExpectedSize int64  `json:"expectedSize,omitempty"`
}

type updateDownloadStatus struct {
	State         string `json:"state"`
	Version       string `json:"version,omitempty"`
	FileName      string `json:"fileName,omitempty"`
	FilePath      string `json:"filePath,omitempty"`
	DownloadURL   string `json:"downloadUrl,omitempty"`
	ChecksumURL   string `json:"checksumUrl,omitempty"`
	BytesReceived int64  `json:"bytesReceived"`
	TotalBytes    int64  `json:"totalBytes,omitempty"`
	Percent       int    `json:"percent"`
	Verified      bool   `json:"verified"`
	Error         string `json:"error,omitempty"`
	StartedAt     string `json:"startedAt,omitempty"`
	UpdatedAt     string `json:"updatedAt,omitempty"`
	CompletedAt   string `json:"completedAt,omitempty"`
}

type updateDownloadAsset struct {
	URL         string
	ChecksumURL string
	FileName    string
	Size        int64
}

type updateDownloader struct {
	mu     sync.Mutex
	status updateDownloadStatus
}

func checkForUpdates(ctx context.Context, client *http.Client, includePrereleases bool) (updateInfo, error) {
	current := strings.TrimSpace(appVersion)
	if current == "" {
		current = "dev"
	}
	info := updateInfo{CurrentVersion: current}
	if isDevelopmentVersion(current) {
		return info, nil
	}

	if client == nil {
		client = http.DefaultClient
	}
	ctx, cancel := context.WithTimeout(ctx, updateCheckTimeout)
	defer cancel()

	var (
		release githubRelease
		found   bool
		err     error
	)
	if includePrereleases {
		release, found, err = fetchCandidateRelease(ctx, client)
	} else {
		release, err = fetchLatestRelease(ctx, client)
		found = err == nil
	}
	if err != nil {
		return info, err
	}
	if !found {
		return info, nil
	}

	return updateInfoFromRelease(info, release), nil
}

func fetchLatestRelease(ctx context.Context, client *http.Client) (githubRelease, error) {
	req, err := newGitHubReleaseRequest(ctx, latestReleaseURL)
	if err != nil {
		return githubRelease{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return githubRelease{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return githubRelease{}, fmt.Errorf("check latest release: github returned %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return githubRelease{}, err
	}
	return release, nil
}

func fetchCandidateRelease(ctx context.Context, client *http.Client) (githubRelease, bool, error) {
	req, err := newGitHubReleaseRequest(ctx, releasesListURL())
	if err != nil {
		return githubRelease{}, false, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return githubRelease{}, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return githubRelease{}, false, fmt.Errorf("check releases: github returned %d", resp.StatusCode)
	}

	var releases []githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return githubRelease{}, false, err
	}
	release, ok := latestVersionedRelease(releases, true)
	return release, ok, nil
}

func newGitHubReleaseRequest(ctx context.Context, rawURL string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "OmniProxy/"+strings.TrimSpace(appVersion))
	return req, nil
}

func releasesListURL() string {
	separator := "?"
	if strings.Contains(releasesURL, "?") {
		separator = "&"
	}
	return releasesURL + separator + "per_page=20"
}

func latestVersionedRelease(releases []githubRelease, includePrereleases bool) (githubRelease, bool) {
	var latest githubRelease
	found := false
	for _, release := range releases {
		tag := strings.TrimSpace(release.TagName)
		if tag == "" || release.Draft || (release.Prerelease && !includePrereleases) {
			continue
		}
		if _, ok := parseVersion(tag); !ok {
			continue
		}
		if !found || compareVersions(tag, latest.TagName) > 0 {
			latest = release
			found = true
		}
	}
	return latest, found
}

func updateInfoFromRelease(info updateInfo, release githubRelease) updateInfo {
	latest := strings.TrimSpace(release.TagName)
	info.LatestVersion = latest
	info.ReleaseURL = release.HTMLURL
	asset := updateDownloadAssetFromAssets(release.Assets)
	info.DownloadURL = asset.URL
	info.ChecksumURL = asset.ChecksumURL
	info.DownloadFileName = asset.FileName
	info.DownloadSize = asset.Size
	info.Name = release.Name
	info.Body = release.Body
	info.Prerelease = release.Prerelease
	info.UpdateAvailable = compareVersions(latest, info.CurrentVersion) > 0
	return info
}

func isDevelopmentVersion(version string) bool {
	normalized := strings.ToLower(strings.TrimSpace(version))
	return normalized == "" || normalized == "dev" || normalized == "development"
}

func updateDownloadURL(assets []githubReleaseAsset) string {
	return updateDownloadAssetFromAssets(assets).URL
}

func updateDownloadAssetFromAssets(assets []githubReleaseAsset) updateDownloadAsset {
	return updateDownloadAssetFromAssetsForPlatform(assets, goruntime.GOOS, goruntime.GOARCH)
}

func updateDownloadAssetFromAssetsForPlatform(assets []githubReleaseAsset, goos string, goarch string) updateDownloadAsset {
	installer := selectUpdateInstallerAsset(assets, goos, goarch)
	if installer.BrowserDownloadURL == "" {
		return updateDownloadAsset{}
	}

	checksumURL := ""
	wantChecksumName := strings.ToLower(installer.Name + ".sha256")
	for _, asset := range assets {
		if strings.ToLower(asset.Name) == wantChecksumName {
			checksumURL = asset.BrowserDownloadURL
			break
		}
	}
	if checksumURL == "" {
		for _, asset := range assets {
			if updateChecksumMatchesPlatform(asset.Name, goos, goarch) {
				checksumURL = asset.BrowserDownloadURL
				break
			}
		}
	}
	if checksumURL == "" {
		for _, asset := range assets {
			if strings.HasSuffix(strings.ToLower(asset.Name), ".sha256") {
				checksumURL = asset.BrowserDownloadURL
				break
			}
		}
	}

	return updateDownloadAsset{
		URL:         installer.BrowserDownloadURL,
		ChecksumURL: checksumURL,
		FileName:    installer.Name,
		Size:        installer.Size,
	}
}

func selectUpdateInstallerAsset(assets []githubReleaseAsset, goos string, goarch string) githubReleaseAsset {
	switch strings.ToLower(strings.TrimSpace(goos)) {
	case "darwin":
		for _, asset := range assets {
			name := strings.ToLower(asset.Name)
			if strings.HasSuffix(name, ".dmg") && (strings.Contains(name, "darwin") || strings.Contains(name, "macos")) && strings.Contains(name, "universal") {
				return asset
			}
		}
		for _, asset := range assets {
			name := strings.ToLower(asset.Name)
			if strings.HasSuffix(name, ".dmg") && (strings.Contains(name, "darwin") || strings.Contains(name, "macos")) && updateAssetNameContainsArch(name, goarch) {
				return asset
			}
		}
		for _, asset := range assets {
			name := strings.ToLower(asset.Name)
			if strings.HasSuffix(name, ".dmg") && (strings.Contains(name, "darwin") || strings.Contains(name, "macos")) {
				return asset
			}
		}
		for _, asset := range assets {
			if strings.HasSuffix(strings.ToLower(asset.Name), ".dmg") {
				return asset
			}
		}
	case "windows":
		return selectWindowsUpdateInstallerAsset(assets, goarch)
	default:
		for _, asset := range assets {
			name := strings.ToLower(asset.Name)
			if strings.Contains(name, strings.ToLower(goos)) && updateAssetNameContainsArch(name, goarch) {
				return asset
			}
		}
	}
	return githubReleaseAsset{}
}

func selectWindowsUpdateInstallerAsset(assets []githubReleaseAsset, goarch string) githubReleaseAsset {
	installer := githubReleaseAsset{}
	for _, asset := range assets {
		name := strings.ToLower(asset.Name)
		if strings.HasSuffix(name, ".exe") && strings.Contains(name, "windows") && updateAssetNameContainsArch(name, goarch) {
			installer = asset
			break
		}
	}
	if installer.BrowserDownloadURL == "" {
		for _, asset := range assets {
			if strings.HasSuffix(strings.ToLower(asset.Name), ".exe") {
				installer = asset
				break
			}
		}
	}
	return installer
}

func updateChecksumMatchesPlatform(name string, goos string, goarch string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	if !strings.HasSuffix(lower, ".sha256") {
		return false
	}
	lower = strings.TrimSuffix(lower, ".sha256")
	switch strings.ToLower(strings.TrimSpace(goos)) {
	case "darwin":
		return strings.HasSuffix(lower, ".dmg") &&
			(strings.Contains(lower, "darwin") || strings.Contains(lower, "macos")) &&
			(strings.Contains(lower, "universal") || updateAssetNameContainsArch(lower, goarch))
	case "windows":
		return strings.HasSuffix(lower, ".exe") &&
			strings.Contains(lower, "windows") &&
			updateAssetNameContainsArch(lower, goarch)
	default:
		return strings.Contains(lower, strings.ToLower(goos)) && updateAssetNameContainsArch(lower, goarch)
	}
}

func updateAssetNameContainsArch(name string, goarch string) bool {
	name = strings.ToLower(name)
	switch strings.ToLower(strings.TrimSpace(goarch)) {
	case "amd64":
		return strings.Contains(name, "amd64") || strings.Contains(name, "x64") || strings.Contains(name, "x86_64")
	case "arm64":
		return strings.Contains(name, "arm64") || strings.Contains(name, "aarch64") || strings.Contains(name, "universal")
	case "":
		return true
	default:
		return strings.Contains(name, strings.ToLower(goarch))
	}
}
