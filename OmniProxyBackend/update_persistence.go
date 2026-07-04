package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"sort"
	"strings"
	"time"
)

const (
	updateStatusFileName       = "status.json"
	updateLogFileName          = "update.log"
	updateInstallerFilesToKeep = 2
)

var updateTempDir = defaultUpdateTempDir

func defaultUpdateTempDir() string {
	return filepath.Join(os.TempDir(), "OmniProxy", "updates")
}

func updateDirectory() string {
	return updateTempDir()
}

func loadUpdateStatus() updateDownloadStatus {
	path := updateStatusPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return updateDownloadStatus{State: "idle"}
	}
	var status updateDownloadStatus
	if err := json.Unmarshal(data, &status); err != nil {
		appendUpdateLog("discard invalid update status: %v", err)
		return updateDownloadStatus{State: "idle"}
	}
	return normalizePersistedUpdateStatus(status)
}

func normalizePersistedUpdateStatus(status updateDownloadStatus) updateDownloadStatus {
	switch status.State {
	case "downloaded", "installing":
		if status.FilePath == "" || !status.Verified {
			status.State = "failed"
			status.Error = "previous update package is not available"
			return status
		}
		if !isPersistedUpdateInstallerPath(status.FilePath) {
			status.State = "failed"
			status.Error = "previous update package is outside the update directory"
			status.Verified = false
			status.FilePath = ""
			return status
		}
		if _, err := os.Stat(status.FilePath); err != nil {
			status.State = "failed"
			status.Error = fmt.Sprintf("previous update package is unavailable: %v", err)
			status.Verified = false
			status.FilePath = ""
			return status
		}
		if status.State == "installing" {
			status.State = "downloaded"
			status.Error = ""
		}
	case "downloading":
		status.State = "failed"
		status.Error = "previous update download was interrupted"
		status.Verified = false
	}
	if status.State == "" {
		status.State = "idle"
	}
	return status
}

func saveUpdateStatus(status updateDownloadStatus) {
	dir := updateDirectory()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		appendUpdateLog("save update status failed: %v", err)
		return
	}
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		appendUpdateLog("encode update status failed: %v", err)
		return
	}
	path := updateStatusPath()
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, append(data, '\n'), 0o600); err != nil {
		appendUpdateLog("write update status failed: %v", err)
		return
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		appendUpdateLog("finish update status failed: %v", err)
	}
}

func updateStatusPath() string {
	return filepath.Join(updateDirectory(), updateStatusFileName)
}

func appendUpdateLog(format string, args ...any) {
	dir := updateDirectory()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	message := fmt.Sprintf(format, args...)
	line := fmt.Sprintf("%s %s\n", time.Now().Format(time.RFC3339), message)
	file, err := os.OpenFile(filepath.Join(dir, updateLogFileName), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = file.WriteString(line)
}

func cleanupUpdateDirectory(preservePath string) {
	dir := updateDirectory()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	preservePath = cleanComparablePath(preservePath)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".download") {
			continue
		}
		_ = os.Remove(filepath.Join(dir, entry.Name()))
	}

	type candidate struct {
		path    string
		modTime time.Time
	}
	var installers []candidate
	for _, entry := range entries {
		if entry.IsDir() || !isUpdateInstallerName(entry.Name()) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if cleanComparablePath(path) == preservePath {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		installers = append(installers, candidate{path: path, modTime: info.ModTime()})
	}
	sort.Slice(installers, func(i, j int) bool {
		return installers[i].modTime.After(installers[j].modTime)
	})
	for index, installer := range installers {
		if index < updateInstallerFilesToKeep {
			continue
		}
		_ = os.Remove(installer.path)
	}
}

func isUpdateInstallerName(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	return strings.HasSuffix(lower, ".exe") || strings.HasSuffix(lower, ".dmg")
}

func isPersistedUpdateInstallerPath(path string) bool {
	if !isUpdateInstallerName(filepath.Base(path)) {
		return false
	}
	relative, err := filepath.Rel(cleanComparablePath(updateDirectory()), cleanComparablePath(path))
	if err != nil {
		return false
	}
	return relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator)) && !filepath.IsAbs(relative)
}

func cleanComparablePath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	cleaned := filepath.Clean(abs)
	if goruntime.GOOS == "windows" {
		cleaned = strings.ToLower(cleaned)
	}
	return cleaned
}
