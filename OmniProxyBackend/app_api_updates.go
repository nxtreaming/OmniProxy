package main

import (
	"context"
	"net/http"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *DesktopApp) CheckForUpdates() (updateInfo, error) {
	return checkForUpdates(a.callContext(), http.DefaultClient, a.server.includePrereleaseUpdates())
}

func (a *DesktopApp) DownloadUpdate(req updateDownloadRequest) (updateDownloadStatus, error) {
	return a.server.updateManager().Start(context.Background(), http.DefaultClient, req)
}

func (a *DesktopApp) UpdateDownloadStatus() updateDownloadStatus {
	return a.server.updateManager().Status()
}

func (a *DesktopApp) UpdateDiagnostics() updateDiagnostics {
	return currentUpdateDiagnostics(a.server.updateManager().Status())
}

func (a *DesktopApp) InstallDownloadedUpdate() (updateDownloadStatus, error) {
	status, err := a.server.updateManager().Install()
	if err != nil {
		return status, err
	}
	if a.ctx != nil {
		ctx := a.ctx
		if shouldQuitAfterUpdateInstall() {
			appendUpdateLog("application quit requested for update install version=%s", status.Version)
			go func() {
				time.Sleep(300 * time.Millisecond)
				runtime.Quit(ctx)
			}()
		}
	}
	return status, nil
}

func (a *DesktopApp) AppInfo() appInfo {
	return currentAppInfo()
}
