package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"omniproxy/internal/config"
	"omniproxy/internal/history"
	"omniproxy/internal/proxy"
	"omniproxy/internal/sanitize"
)

type diagnosticsBundle struct {
	GeneratedAt     string                   `json:"generatedAt"`
	App             appInfo                  `json:"app"`
	DataDirectory   config.DataDirectoryInfo `json:"dataDirectory"`
	ProxyStatus     map[string]any           `json:"proxyStatus"`
	Config          config.Config            `json:"config"`
	Tokens          []tokenResponse          `json:"tokens"`
	Logs            []logResponse            `json:"logs"`
	History         []historyResponse        `json:"history"`
	HistorySummary  history.Summary          `json:"historySummary"`
	BillingSummary  history.BillingSummary   `json:"billingSummary"`
	ActiveRequests  []activeRequestResponse  `json:"activeRequests"`
	UpdateStatus    updateDownloadStatus     `json:"updateStatus"`
	UpdateDiagnosis updateDiagnostics        `json:"updateDiagnostics"`
}

type diagnosticsExportResult struct {
	Path     string `json:"path,omitempty"`
	FileName string `json:"fileName,omitempty"`
	Size     int64  `json:"size,omitempty"`
}

func (a *appServer) gatewayRouteDiagnostics(req proxy.RouteDiagnosticRequest) (proxy.RouteDiagnostic, error) {
	if err := proxy.ValidateRouteDiagnosticRequest(req); err != nil {
		return proxy.RouteDiagnostic{}, err
	}
	a.mu.Lock()
	cfg := a.cfg
	tokens := a.tokens
	a.mu.Unlock()
	return proxy.DiagnoseRoute(cfg, tokens, req), nil
}

func (a *appServer) diagnosticsBundleSnapshot() diagnosticsBundle {
	a.mu.Lock()
	cfg := a.cfg
	tokens := a.tokens
	recorder := a.history
	running := a.proxyServer != nil
	port := a.cfg.ProxyPort
	a.mu.Unlock()

	var tokenItems []tokenResponse
	if tokens != nil {
		tokenItems = tokenResponses(tokens.List())
	}
	var historyItems []historyResponse
	var historySummary history.Summary
	var billingSummary history.BillingSummary
	if recorder != nil {
		filter := history.Filter{Limit: 500}
		historyItems = historyResponses(recorder.List(filter))
		historySummary = recorder.Summary(history.Filter{}, 14)
		billingSummary = recorder.BillingSummary(30)
	}
	status := a.updateManager().Status()
	return diagnosticsBundle{
		GeneratedAt:     time.Now().Format(time.RFC3339),
		App:             currentAppInfo(),
		DataDirectory:   a.dataDirectoryInfo(),
		ProxyStatus:     map[string]any{"running": running, "port": port},
		Config:          config.Normalize(cfg),
		Tokens:          tokenItems,
		Logs:            logResponses(a.logs.List()),
		History:         historyItems,
		HistorySummary:  historySummary,
		BillingSummary:  billingSummary,
		ActiveRequests:  activeRequestResponses(a.activeProxyRequests()),
		UpdateStatus:    status,
		UpdateDiagnosis: currentUpdateDiagnostics(status),
	}
}

func encodeDiagnosticsBundleZip(bundle diagnosticsBundle) ([]byte, error) {
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	files := []struct {
		Name  string
		Value any
	}{
		{Name: "manifest.json", Value: map[string]any{
			"generatedAt": bundle.GeneratedAt,
			"app":         bundle.App,
		}},
		{Name: "config.json", Value: bundle.Config},
		{Name: "tokens.json", Value: bundle.Tokens},
		{Name: "logs.json", Value: bundle.Logs},
		{Name: "history.json", Value: bundle.History},
		{Name: "history-summary.json", Value: bundle.HistorySummary},
		{Name: "billing-summary.json", Value: bundle.BillingSummary},
		{Name: "runtime.json", Value: map[string]any{
			"dataDirectory":  bundle.DataDirectory,
			"proxyStatus":    bundle.ProxyStatus,
			"activeRequests": bundle.ActiveRequests,
		}},
		{Name: "update.json", Value: map[string]any{
			"status":      bundle.UpdateStatus,
			"diagnostics": bundle.UpdateDiagnosis,
		}},
	}
	for _, file := range files {
		if err := writeDiagnosticsJSONFile(writer, file.Name, file.Value); err != nil {
			_ = writer.Close()
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeDiagnosticsJSONFile(writer *zip.Writer, name string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	file, err := writer.Create(name)
	if err != nil {
		return err
	}
	_, err = file.Write([]byte(sanitize.Text(string(data))))
	return err
}

func writeDiagnosticsBundle(path string, bundle diagnosticsBundle) (diagnosticsExportResult, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return diagnosticsExportResult{}, errors.New("diagnostics path is empty")
	}
	if strings.ToLower(filepath.Ext(path)) != ".zip" {
		path += ".zip"
	}
	data, err := encodeDiagnosticsBundleZip(bundle)
	if err != nil {
		return diagnosticsExportResult{}, err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return diagnosticsExportResult{}, err
	}
	return diagnosticsExportResult{
		Path:     path,
		FileName: filepath.Base(path),
		Size:     int64(len(data)),
	}, nil
}

func diagnosticsBundleFilename() string {
	return fmt.Sprintf("omniproxy-diagnostics-%s.zip", time.Now().Format("20060102-150405"))
}
