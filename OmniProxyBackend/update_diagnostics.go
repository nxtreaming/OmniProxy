package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

const updateDiagnosticsLogTailBytes int64 = 32 * 1024

type updateDiagnostics struct {
	Directory      string               `json:"directory"`
	StatusPath     string               `json:"statusPath"`
	LogPath        string               `json:"logPath"`
	Status         updateDownloadStatus `json:"status"`
	StatusExists   bool                 `json:"statusExists"`
	LogExists      bool                 `json:"logExists"`
	LogSize        int64                `json:"logSize"`
	LogTail        string               `json:"logTail"`
	InstallerCount int                  `json:"installerCount"`
	PartialCount   int                  `json:"partialCount"`
	Error          string               `json:"error,omitempty"`
}

func currentUpdateDiagnostics(status updateDownloadStatus) updateDiagnostics {
	diagnostics := updateDiagnostics{
		Directory:  updateDirectory(),
		StatusPath: updateStatusPath(),
		LogPath:    filepath.Join(updateDirectory(), updateLogFileName),
		Status:     status,
	}

	if _, err := os.Stat(diagnostics.StatusPath); err == nil {
		diagnostics.StatusExists = true
	} else if err != nil && !os.IsNotExist(err) {
		diagnostics.Error = err.Error()
	}

	logTail, logExists, logSize, err := readUpdateLogTail(diagnostics.LogPath, updateDiagnosticsLogTailBytes)
	diagnostics.LogTail = logTail
	diagnostics.LogExists = logExists
	diagnostics.LogSize = logSize
	if err != nil && diagnostics.Error == "" {
		diagnostics.Error = err.Error()
	}

	installerCount, partialCount, err := countUpdateFiles(diagnostics.Directory)
	diagnostics.InstallerCount = installerCount
	diagnostics.PartialCount = partialCount
	if err != nil && diagnostics.Error == "" {
		diagnostics.Error = err.Error()
	}
	return diagnostics
}

func readUpdateLogTail(path string, maxBytes int64) (string, bool, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, 0, nil
		}
		return "", false, 0, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return "", true, 0, err
	}
	size := info.Size()
	offset := int64(0)
	if maxBytes > 0 && size > maxBytes {
		offset = size - maxBytes
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return "", true, size, err
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return "", true, size, err
	}
	text := string(data)
	if offset > 0 {
		if newline := strings.IndexByte(text, '\n'); newline >= 0 && newline+1 < len(text) {
			text = text[newline+1:]
		}
	}
	return strings.TrimRight(text, "\r\n"), true, size, nil
}

func countUpdateFiles(dir string) (int, int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil
		}
		return 0, 0, err
	}
	installerCount := 0
	partialCount := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		if strings.HasSuffix(name, ".download") {
			partialCount++
		} else if isUpdateInstallerName(name) {
			installerCount++
		}
	}
	return installerCount, partialCount, nil
}
