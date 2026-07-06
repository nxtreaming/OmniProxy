package main

import (
	"errors"
	"os"
	"strings"

	"omniproxy/internal/config"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *DesktopApp) ConfigSnapshots() ([]configSnapshotSummary, error) {
	return a.server.listConfigSnapshots()
}

func (a *DesktopApp) CreateConfigSnapshot(name string) (configSnapshotSummary, error) {
	return a.server.createConfigSnapshot(name)
}

func (a *DesktopApp) RestoreConfigSnapshot(id string) (config.Config, error) {
	return a.server.restoreConfigSnapshot(id)
}

func (a *DesktopApp) DeleteConfigSnapshot(id string) error {
	return a.server.deleteConfigSnapshot(id)
}

func (a *DesktopApp) ExportConfigBundle() (configExportResult, error) {
	if a.ctx == nil {
		return configExportResult{}, errors.New("application is not ready")
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "导出 OmniProxy 配置",
		DefaultFilename: configExportFilename(),
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON 文件 (*.json)", Pattern: "*.json"},
		},
	})
	if err != nil {
		return configExportResult{}, err
	}
	if strings.TrimSpace(path) == "" {
		return configExportResult{}, nil
	}
	return a.server.writeConfigExportBundle(path)
}

func (a *DesktopApp) ImportConfigBundle() (configImportResult, error) {
	if a.ctx == nil {
		return configImportResult{}, errors.New("application is not ready")
	}
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "导入 OmniProxy 配置",
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON 文件 (*.json)", Pattern: "*.json"},
		},
	})
	if err != nil {
		return configImportResult{}, err
	}
	if strings.TrimSpace(path) == "" {
		return configImportResult{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return configImportResult{}, err
	}
	return a.server.importConfigBundleBytes(data)
}
