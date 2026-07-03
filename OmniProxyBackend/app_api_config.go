package main

import (
	"context"
	"errors"
	"os"

	"omniproxy/internal/config"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *DesktopApp) Config() config.Config {
	a.server.mu.Lock()
	defer a.server.mu.Unlock()
	return a.server.cfg
}

func (a *DesktopApp) SaveConfig(cfg config.Config) (config.Config, error) {
	return a.server.saveConfig(cfg)
}

func (a *DesktopApp) DataDirectory() (config.DataDirectoryInfo, error) {
	return a.server.dataDirectoryInfo(), nil
}

func (a *DesktopApp) ChooseDataDirectory(migrate bool) (config.DataDirectoryChangeResult, error) {
	if a.ctx == nil {
		return config.DataDirectoryChangeResult{}, errors.New("application is not ready")
	}
	info := a.server.dataDirectoryInfo()
	if info.EnvOverride {
		return config.DataDirectoryChangeResult{DataDir: info.DataDir, BootstrapPath: info.BootstrapPath, EnvOverride: true}, errors.New("data directory is controlled by OMNIPROXY_DATA_DIR")
	}

	defaultDir := info.DataDir
	if _, err := os.Stat(defaultDir); err != nil {
		defaultDir = ""
	}
	selected, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:                "选择 OmniProxy 数据目录",
		DefaultDirectory:     defaultDir,
		CanCreateDirectories: true,
	})
	if err != nil {
		return config.DataDirectoryChangeResult{}, err
	}
	if selected == "" {
		return config.DataDirectoryChangeResult{Cancelled: true, DataDir: info.DataDir, BootstrapPath: info.BootstrapPath}, nil
	}
	return a.server.changeDataDirectory(selected, migrate)
}

func (a *DesktopApp) ensureDataDirectory(ctx context.Context) error {
	if a.server.isLoaded() {
		return nil
	}

	info, configured, err := config.ResolveDataDir()
	if err != nil {
		return err
	}
	if configured {
		return a.server.loadData(info.DataDir)
	}

	defaultDir := config.DefaultDataDir()
	if _, err := os.Stat(defaultDir); err != nil {
		defaultDir = ""
	}
	selected, err := runtime.OpenDirectoryDialog(ctx, runtime.OpenDialogOptions{
		Title:                "首次启动：选择 OmniProxy 数据目录",
		DefaultDirectory:     defaultDir,
		CanCreateDirectories: true,
	})
	if err != nil {
		return err
	}
	if selected == "" {
		return errors.New("首次启动需要选择一个可写的数据目录")
	}
	if _, err := a.server.changeDataDirectory(selected, false); err != nil {
		return err
	}
	return a.server.loadData(selected)
}
