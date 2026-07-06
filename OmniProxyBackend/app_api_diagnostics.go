package main

import (
	"errors"

	"omniproxy/internal/proxy"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *DesktopApp) GatewayRouteDiagnostics(req proxy.RouteDiagnosticRequest) (proxy.RouteDiagnostic, error) {
	return a.server.gatewayRouteDiagnostics(req)
}

func (a *DesktopApp) ExportDiagnosticsBundle() (diagnosticsExportResult, error) {
	if a.ctx == nil {
		return diagnosticsExportResult{}, errors.New("application is not ready")
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "导出 OmniProxy 诊断包",
		DefaultFilename: diagnosticsBundleFilename(),
		Filters: []runtime.FileFilter{
			{DisplayName: "ZIP 文件 (*.zip)", Pattern: "*.zip"},
		},
	})
	if err != nil {
		return diagnosticsExportResult{}, err
	}
	if path == "" {
		return diagnosticsExportResult{}, nil
	}
	return writeDiagnosticsBundle(path, a.server.diagnosticsBundleSnapshot())
}
