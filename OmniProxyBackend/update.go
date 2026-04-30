package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	goruntime "runtime"
	"strconv"
	"strings"
	"time"
	"unicode"
)

var appVersion = "dev"
var appStartedAt = time.Now()

var latestReleaseURL = "https://api.github.com/repos/mibgb65-cloud/OmniProxy/releases/latest"

const updateCheckTimeout = 5 * time.Second

type updateInfo struct {
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion,omitempty"`
	UpdateAvailable bool   `json:"updateAvailable"`
	ReleaseURL      string `json:"releaseUrl,omitempty"`
	DownloadURL     string `json:"downloadUrl,omitempty"`
	Name            string `json:"name,omitempty"`
	Body            string `json:"body,omitempty"`
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

func currentAppInfo() appInfo {
	current := strings.TrimSpace(appVersion)
	if current == "" {
		current = "dev"
	}
	executablePath, _ := os.Executable()
	return appInfo{
		Name:           "OmniProxy",
		Version:        current,
		IsDevelopment:  isDevelopmentVersion(current),
		UpdateEndpoint: latestReleaseURL,
		Platform:       goruntime.GOOS + "/" + goruntime.GOARCH,
		GoVersion:      goruntime.Version(),
		ExecutablePath: executablePath,
		StartedAt:      appStartedAt.Format(time.RFC3339),
	}
}

type githubRelease struct {
	TagName string               `json:"tag_name"`
	Name    string               `json:"name"`
	HTMLURL string               `json:"html_url"`
	Body    string               `json:"body"`
	Assets  []githubReleaseAsset `json:"assets"`
}

type githubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func checkForUpdates(ctx context.Context, client *http.Client) (updateInfo, error) {
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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestReleaseURL, nil)
	if err != nil {
		return info, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "OmniProxy/"+current)

	resp, err := client.Do(req)
	if err != nil {
		return info, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return info, fmt.Errorf("check latest release: github returned %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return info, err
	}
	latest := strings.TrimSpace(release.TagName)
	info.LatestVersion = latest
	info.ReleaseURL = release.HTMLURL
	info.DownloadURL = updateDownloadURL(release.Assets)
	info.Name = release.Name
	info.Body = release.Body
	info.UpdateAvailable = compareVersions(latest, current) > 0
	return info, nil
}

func isDevelopmentVersion(version string) bool {
	normalized := strings.ToLower(strings.TrimSpace(version))
	return normalized == "" || normalized == "dev" || normalized == "development"
}

func updateDownloadURL(assets []githubReleaseAsset) string {
	for _, asset := range assets {
		name := strings.ToLower(asset.Name)
		if strings.HasSuffix(name, ".exe") && strings.Contains(name, "windows") && strings.Contains(name, "amd64") {
			return asset.BrowserDownloadURL
		}
	}
	for _, asset := range assets {
		if strings.HasSuffix(strings.ToLower(asset.Name), ".exe") {
			return asset.BrowserDownloadURL
		}
	}
	return ""
}

func compareVersions(left string, right string) int {
	leftParts, leftOK := versionParts(left)
	rightParts, rightOK := versionParts(right)
	if !leftOK || !rightOK {
		return 0
	}
	maxLen := len(leftParts)
	if len(rightParts) > maxLen {
		maxLen = len(rightParts)
	}
	for i := 0; i < maxLen; i++ {
		leftValue := 0
		if i < len(leftParts) {
			leftValue = leftParts[i]
		}
		rightValue := 0
		if i < len(rightParts) {
			rightValue = rightParts[i]
		}
		if leftValue > rightValue {
			return 1
		}
		if leftValue < rightValue {
			return -1
		}
	}
	return 0
}

func versionParts(version string) ([]int, bool) {
	version = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(version), "v"))
	if version == "" {
		return nil, false
	}
	rawParts := strings.Split(version, ".")
	parts := make([]int, 0, len(rawParts))
	for _, raw := range rawParts {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return nil, false
		}
		digits := strings.Builder{}
		for _, char := range raw {
			if !unicode.IsDigit(char) {
				break
			}
			digits.WriteRune(char)
		}
		if digits.Len() == 0 {
			return nil, false
		}
		value, err := strconv.Atoi(digits.String())
		if err != nil {
			return nil, false
		}
		parts = append(parts, value)
	}
	return parts, len(parts) > 0
}
