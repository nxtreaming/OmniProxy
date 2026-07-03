package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"omniproxy/internal/history"
	"omniproxy/internal/logs"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *DesktopApp) Logs() []logResponse {
	return logResponses(a.server.logs.List())
}

func (a *DesktopApp) RequestHistory(filter history.Filter) []historyResponse {
	if a.server.history == nil {
		return []historyResponse{}
	}
	if filter.Limit <= 0 {
		filter.Limit = defaultHistoryLimit
	}
	return historyResponses(a.server.history.List(filter))
}

func (a *DesktopApp) RequestHistorySummary(filter history.Filter, days int) history.Summary {
	if a.server.history == nil {
		return history.Summary{}
	}
	return a.server.history.Summary(filter, days)
}

func (a *DesktopApp) BillingUsage(date string) []history.DailyUsage {
	if a.server.history == nil {
		return []history.DailyUsage{}
	}
	return a.server.history.DailyUsage(date)
}

func (a *DesktopApp) BillingDates(limit int) []string {
	if a.server.history == nil {
		return []string{}
	}
	return a.server.history.DailyUsageDates(limit)
}

func (a *DesktopApp) BillingSummary(days int) history.BillingSummary {
	if a.server.history == nil {
		return history.BillingSummary{}
	}
	return a.server.history.BillingSummary(days)
}

func (a *DesktopApp) ClearBillingUsage() error {
	if a.server.history == nil {
		return nil
	}
	if err := a.server.history.ClearDailyUsage(); err != nil {
		return err
	}
	a.server.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: "billing daily usage cleared"})
	return nil
}

func (a *DesktopApp) ClearRequestHistory() error {
	if a.server.history == nil {
		return nil
	}
	if err := a.server.history.ClearRequestHistory(); err != nil {
		return err
	}
	a.server.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: "request history cleared"})
	return nil
}

func (a *DesktopApp) ActiveProxyRequests() []activeRequestResponse {
	return activeRequestResponses(a.server.activeProxyRequests())
}

func (a *DesktopApp) ExportRequestHistory(format string, filter history.Filter) (string, error) {
	if a.ctx == nil {
		return "", errors.New("application is not ready")
	}
	if a.server.history == nil {
		return "", errors.New("request history is not ready")
	}

	format = strings.ToLower(strings.TrimSpace(format))
	if format != "csv" && format != "json" {
		return "", errors.New("export format must be csv or json")
	}
	if filter.Limit <= 0 {
		filter.Limit = defaultHistoryLimit
	}
	entries := a.server.history.List(filter)

	var (
		data       []byte
		err        error
		filterName runtime.FileFilter
	)
	switch format {
	case "csv":
		data, err = encodeHistoryCSV(entries)
		filterName = runtime.FileFilter{DisplayName: "CSV 文件 (*.csv)", Pattern: "*.csv"}
	case "json":
		data, err = json.MarshalIndent(entries, "", "  ")
		filterName = runtime.FileFilter{DisplayName: "JSON 文件 (*.json)", Pattern: "*.json"}
	}
	if err != nil {
		return "", err
	}

	filename := fmt.Sprintf("omniproxy-request-history-%s.%s", time.Now().Format("20060102-150405"), format)
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "导出请求历史",
		DefaultFilename: filename,
		Filters:         []runtime.FileFilter{filterName},
	})
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(path) == "" {
		return "", nil
	}
	if strings.ToLower(filepath.Ext(path)) != "."+format {
		path += "." + format
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}
