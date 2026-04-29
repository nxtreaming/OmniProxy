package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"OmniProxyBackend/internal/token"
)

type tokenExportResult struct {
	Path    string `json:"path,omitempty"`
	Count   int    `json:"count"`
	Message string `json:"message"`
}

type codexAuthExportResult struct {
	Directory string   `json:"directory,omitempty"`
	Files     []string `json:"files,omitempty"`
	Count     int      `json:"count"`
	Message   string   `json:"message"`
}

type tokenExportPayload struct {
	ExportedAt string        `json:"exportedAt"`
	Version    string        `json:"version"`
	Tokens     []token.Token `json:"tokens"`
}

type codexAuthExportFile struct {
	Name    string
	Content string
}

func encodeTokenExport(items []token.Token, exportedAt time.Time) ([]byte, error) {
	payload := tokenExportPayload{
		ExportedAt: exportedAt.Format(time.RFC3339Nano),
		Version:    "1",
		Tokens:     append([]token.Token(nil), items...),
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func codexAuthExportFiles(items []token.Token, stamp string) []codexAuthExportFile {
	files := []codexAuthExportFile{}
	seen := map[string]int{}
	for _, item := range items {
		if !isCodexToken(item) || strings.TrimSpace(item.TokenValue) == "" {
			continue
		}
		stem := safeExportFileStem(item.Name)
		base := "codex-auth-" + stem
		if stamp != "" {
			base += "-" + stamp
		}
		seen[base]++
		name := base
		if seen[base] > 1 {
			name += "-" + intToString(seen[base])
		}
		files = append(files, codexAuthExportFile{
			Name:    name + ".json",
			Content: strings.TrimSpace(item.TokenValue) + "\n",
		})
	}
	return files
}

func writeCodexAuthExportFiles(directory string, files []codexAuthExportFile) ([]string, error) {
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return nil, err
	}
	written := make([]string, 0, len(files))
	for _, file := range files {
		path := filepath.Join(directory, file.Name)
		if err := os.WriteFile(path, []byte(file.Content), 0o600); err != nil {
			return written, err
		}
		written = append(written, path)
	}
	return written, nil
}

func safeExportFileStem(value string) string {
	value = strings.TrimSpace(value)
	var builder strings.Builder
	lastDash := false
	for _, r := range value {
		valid := unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == '-'
		if valid {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	stem := strings.Trim(builder.String(), ".-_")
	if stem == "" {
		return "account"
	}
	return stem
}
