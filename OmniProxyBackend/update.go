package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

var appVersion = "dev"
var appStartedAt = time.Now()

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
	installer := githubReleaseAsset{}
	for _, asset := range assets {
		name := strings.ToLower(asset.Name)
		if strings.HasSuffix(name, ".exe") && strings.Contains(name, "windows") && strings.Contains(name, "amd64") {
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

func newUpdateDownloader() *updateDownloader {
	return &updateDownloader{status: updateDownloadStatus{State: "idle"}}
}

func (d *updateDownloader) Status() updateDownloadStatus {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.status
}

func (d *updateDownloader) Start(ctx context.Context, client *http.Client, req updateDownloadRequest) (updateDownloadStatus, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if strings.TrimSpace(req.DownloadURL) == "" {
		return updateDownloadStatus{}, fmt.Errorf("download URL is required")
	}
	if strings.TrimSpace(req.ChecksumURL) == "" {
		return updateDownloadStatus{}, fmt.Errorf("checksum URL is required")
	}
	fileName, err := updateInstallerFileName(req)
	if err != nil {
		return updateDownloadStatus{}, err
	}
	started := time.Now().Format(time.RFC3339)
	status := updateDownloadStatus{
		State:       "downloading",
		Version:     strings.TrimSpace(req.Version),
		FileName:    fileName,
		DownloadURL: strings.TrimSpace(req.DownloadURL),
		ChecksumURL: strings.TrimSpace(req.ChecksumURL),
		TotalBytes:  req.ExpectedSize,
		StartedAt:   started,
		UpdatedAt:   started,
	}

	d.mu.Lock()
	if d.status.State == "downloading" {
		current := d.status
		d.mu.Unlock()
		return current, fmt.Errorf("update download is already in progress")
	}
	d.status = status
	d.mu.Unlock()

	go d.download(ctx, client, req, fileName)
	return status, nil
}

func (d *updateDownloader) Install() (updateDownloadStatus, error) {
	d.mu.Lock()
	status := d.status
	d.mu.Unlock()
	if status.State != "downloaded" || status.FilePath == "" {
		return status, fmt.Errorf("no downloaded update installer is ready")
	}
	if !status.Verified {
		return status, fmt.Errorf("downloaded update has not been verified")
	}
	if _, err := os.Stat(status.FilePath); err != nil {
		return status, fmt.Errorf("update installer is unavailable: %w", err)
	}

	if err := updateInstallerStart(status.FilePath, updateInstallerArgs()); err != nil {
		d.fail(fmt.Sprintf("start update installer: %v", err))
		return d.Status(), err
	}

	now := time.Now().Format(time.RFC3339)
	d.mu.Lock()
	d.status.State = "installing"
	d.status.Error = ""
	d.status.UpdatedAt = now
	status = d.status
	d.mu.Unlock()
	return status, nil
}

func updateInstallerArgs() []string {
	return []string{"/S", "/OMNIPROXY_AUTOUPDATE=1"}
}

func (d *updateDownloader) download(ctx context.Context, client *http.Client, req updateDownloadRequest, fileName string) {
	if ctx == nil {
		ctx = context.Background()
	}
	dir := filepath.Join(os.TempDir(), "OmniProxy", "updates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		d.fail(fmt.Sprintf("create update directory: %v", err))
		return
	}

	filePath := filepath.Join(dir, fileName)
	tmpPath := filePath + ".download"
	if err := downloadFile(ctx, client, strings.TrimSpace(req.DownloadURL), tmpPath, req.ExpectedSize, d.setProgress); err != nil {
		_ = os.Remove(tmpPath)
		d.fail(err.Error())
		return
	}
	_ = os.Remove(filePath)
	if err := os.Rename(tmpPath, filePath); err != nil {
		_ = os.Remove(tmpPath)
		d.fail(fmt.Sprintf("finish update download: %v", err))
		return
	}

	if err := verifyDownloadedInstaller(ctx, client, filePath, strings.TrimSpace(req.ChecksumURL)); err != nil {
		_ = os.Remove(filePath)
		d.fail(err.Error())
		return
	}

	now := time.Now().Format(time.RFC3339)
	size := fileSize(filePath)
	d.mu.Lock()
	d.status.State = "downloaded"
	d.status.FilePath = filePath
	d.status.BytesReceived = size
	if d.status.TotalBytes <= 0 {
		d.status.TotalBytes = size
	}
	d.status.Percent = 100
	d.status.Verified = true
	d.status.Error = ""
	d.status.UpdatedAt = now
	d.status.CompletedAt = now
	d.mu.Unlock()
}

func downloadFile(ctx context.Context, client *http.Client, rawURL string, filePath string, expectedSize int64, progress func(received int64, total int64)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "OmniProxy/"+strings.TrimSpace(appVersion))
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download update: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download update: server returned %d", resp.StatusCode)
	}

	total := resp.ContentLength
	if total <= 0 {
		total = expectedSize
	}
	out, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("create update installer: %w", err)
	}
	defer out.Close()

	var received int64
	buffer := make([]byte, 64*1024)
	for {
		n, readErr := resp.Body.Read(buffer)
		if n > 0 {
			if _, err := out.Write(buffer[:n]); err != nil {
				return fmt.Errorf("write update installer: %w", err)
			}
			received += int64(n)
			if progress != nil {
				progress(received, total)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("read update installer: %w", readErr)
		}
	}
	return nil
}

func verifyDownloadedInstaller(ctx context.Context, client *http.Client, filePath string, checksumURL string) error {
	expected, err := downloadSHA256(ctx, client, checksumURL)
	if err != nil {
		return err
	}
	actual, err := fileSHA256(filePath)
	if err != nil {
		return err
	}
	if !strings.EqualFold(expected, actual) {
		return fmt.Errorf("verify update installer: checksum mismatch")
	}
	return nil
}

func downloadSHA256(ctx context.Context, client *http.Client, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "OmniProxy/"+strings.TrimSpace(appVersion))
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download update checksum: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("download update checksum: server returned %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", fmt.Errorf("read update checksum: %w", err)
	}
	for _, field := range strings.Fields(string(body)) {
		candidate := strings.TrimSpace(field)
		if len(candidate) != sha256.Size*2 {
			continue
		}
		if _, err := strconv.ParseUint(candidate[:16], 16, 64); err == nil && isHexString(candidate) {
			return strings.ToLower(candidate), nil
		}
	}
	return "", fmt.Errorf("download update checksum: no SHA256 hash found")
}

func fileSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open update installer: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("hash update installer: %w", err)
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func isHexString(value string) bool {
	for _, char := range value {
		if !unicode.IsDigit(char) && (char < 'a' || char > 'f') && (char < 'A' || char > 'F') {
			return false
		}
	}
	return true
}

func updateInstallerFileName(req updateDownloadRequest) (string, error) {
	name := strings.TrimSpace(req.FileName)
	if name == "" {
		name = fileNameFromURL(req.DownloadURL)
	}
	if name == "" {
		name = "OmniProxy-Setup.exe"
	}
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	if !strings.HasSuffix(strings.ToLower(name), ".exe") {
		return "", fmt.Errorf("update installer must be an .exe file")
	}
	if strings.ContainsAny(name, `<>:"|?*`) {
		return "", fmt.Errorf("update installer file name contains invalid characters")
	}
	return name, nil
}

func fileNameFromURL(rawURL string) string {
	parts := strings.Split(strings.TrimSpace(rawURL), "/")
	if len(parts) == 0 {
		return ""
	}
	name := parts[len(parts)-1]
	if question := strings.IndexByte(name, '?'); question >= 0 {
		name = name[:question]
	}
	return strings.TrimSpace(name)
}

func (d *updateDownloader) setProgress(received int64, total int64) {
	now := time.Now().Format(time.RFC3339)
	d.mu.Lock()
	d.status.BytesReceived = received
	if total > 0 {
		d.status.TotalBytes = total
		d.status.Percent = int((received * 100) / total)
		if d.status.Percent > 99 {
			d.status.Percent = 99
		}
	}
	d.status.UpdatedAt = now
	d.mu.Unlock()
}

func (d *updateDownloader) fail(message string) {
	now := time.Now().Format(time.RFC3339)
	d.mu.Lock()
	d.status.State = "failed"
	d.status.Error = message
	d.status.UpdatedAt = now
	d.status.CompletedAt = now
	d.mu.Unlock()
}

func fileSize(filePath string) int64 {
	info, err := os.Stat(filePath)
	if err != nil {
		return 0
	}
	return info.Size()
}

func compareVersions(left string, right string) int {
	leftVersion, leftOK := parseVersion(left)
	rightVersion, rightOK := parseVersion(right)
	if !leftOK || !rightOK {
		return 0
	}
	maxLen := len(leftVersion.parts)
	if len(rightVersion.parts) > maxLen {
		maxLen = len(rightVersion.parts)
	}
	for i := 0; i < maxLen; i++ {
		leftValue := 0
		if i < len(leftVersion.parts) {
			leftValue = leftVersion.parts[i]
		}
		rightValue := 0
		if i < len(rightVersion.parts) {
			rightValue = rightVersion.parts[i]
		}
		if leftValue > rightValue {
			return 1
		}
		if leftValue < rightValue {
			return -1
		}
	}
	if leftVersion.prerelease == "" && rightVersion.prerelease != "" {
		return 1
	}
	if leftVersion.prerelease != "" && rightVersion.prerelease == "" {
		return -1
	}
	if leftVersion.prerelease > rightVersion.prerelease {
		return 1
	}
	if leftVersion.prerelease < rightVersion.prerelease {
		return -1
	}
	return 0
}

type parsedVersion struct {
	parts      []int
	prerelease string
}

func parseVersion(version string) (parsedVersion, bool) {
	version = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(version), "v"))
	if version == "" {
		return parsedVersion{}, false
	}
	prerelease := ""
	if core, suffix, ok := strings.Cut(version, "-"); ok {
		version = core
		prerelease = strings.TrimSpace(suffix)
	}
	rawParts := strings.Split(version, ".")
	parts := make([]int, 0, len(rawParts))
	for _, raw := range rawParts {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return parsedVersion{}, false
		}
		digits := strings.Builder{}
		for _, char := range raw {
			if !unicode.IsDigit(char) {
				break
			}
			digits.WriteRune(char)
		}
		if digits.Len() == 0 {
			return parsedVersion{}, false
		}
		value, err := strconv.Atoi(digits.String())
		if err != nil {
			return parsedVersion{}, false
		}
		parts = append(parts, value)
	}
	return parsedVersion{parts: parts, prerelease: prerelease}, len(parts) > 0
}

func versionParts(version string) ([]int, bool) {
	parsed, ok := parseVersion(version)
	return parsed.parts, ok
}
