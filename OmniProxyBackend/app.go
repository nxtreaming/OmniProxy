package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"OmniProxyBackend/internal/config"
	"OmniProxyBackend/internal/logs"
	"OmniProxyBackend/internal/proxy"
	"OmniProxyBackend/internal/token"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type DesktopApp struct {
	ctx    context.Context
	server *appServer
}

func NewDesktopApp(server *appServer) *DesktopApp {
	return &DesktopApp{server: server}
}

func (a *DesktopApp) startup(ctx context.Context) {
	a.ctx = ctx

	if err := a.ensureDataDirectory(ctx); err != nil {
		log.Printf("data directory not configured: %v", err)
		_, _ = runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
			Type:    runtime.ErrorDialog,
			Title:   "OmniProxy 数据目录",
			Message: err.Error(),
		})
		runtime.Quit(ctx)
		return
	}

	if err := a.server.startControl(); err != nil {
		log.Printf("control API not started: %v", err)
	}
	if err := a.server.startProxy(); err != nil {
		log.Printf("proxy not started: %v", err)
	}
	go a.server.refreshCodexUsageOnStartup(ctx)
}

func (a *DesktopApp) shutdown(ctx context.Context) {
	if err := a.server.stopProxy(); err != nil {
		log.Printf("proxy shutdown failed: %v", err)
	}
	if err := a.server.stopControl(); err != nil {
		log.Printf("control API shutdown failed: %v", err)
	}
	if a.server.tokens != nil {
		if err := a.server.tokens.Flush(); err != nil {
			log.Printf("token data flush failed: %v", err)
		}
	}
}

func (a *DesktopApp) ControlAPI() string {
	a.server.mu.Lock()
	defer a.server.mu.Unlock()
	return "http://127.0.0.1:" + intToString(a.server.cfg.ControlPort) + "/api"
}

func (a *DesktopApp) Tokens() []tokenResponse {
	return tokenResponses(a.server.tokens.List())
}

func (a *DesktopApp) CreateToken(req token.UpsertRequest) (tokenResponse, error) {
	item, err := a.server.tokens.Add(req)
	if err != nil {
		return tokenResponse{}, err
	}
	a.server.logs.Add(logs.Entry{Level: logs.LevelInfo, TokenName: item.Name, Message: "token added"})
	if isCodexToken(item) {
		if _, err := a.server.validateAndRecordToken(a.callContext(), item); err != nil {
			a.server.logs.Add(logs.Entry{Level: logs.LevelWarn, TokenName: item.Name, Message: fmt.Sprintf("codex usage refresh failed after add: %v", err)})
		}
		if updated, err := a.server.tokens.Get(item.ID); err == nil {
			item = updated
		}
	}
	return tokenResponseFor(item), nil
}

func (a *DesktopApp) UpdateToken(id string, req token.UpsertRequest) (tokenResponse, error) {
	item, err := a.server.tokens.Update(id, req)
	if err != nil {
		return tokenResponse{}, err
	}
	a.server.logs.Add(logs.Entry{Level: logs.LevelInfo, TokenName: item.Name, Message: "token updated"})
	return tokenResponseFor(item), nil
}

func (a *DesktopApp) DeleteToken(id string) error {
	if err := a.server.tokens.Delete(id); err != nil {
		return err
	}
	a.server.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: "token deleted"})
	return nil
}

func (a *DesktopApp) ValidateToken(id string) (proxy.ValidationResult, error) {
	selected, err := a.server.tokens.Get(id)
	if err != nil {
		return proxy.ValidationResult{}, err
	}

	result, err := a.server.validateAndRecordToken(a.callContext(), selected)
	level := logs.LevelInfo
	if err != nil || !result.OK {
		level = logs.LevelWarn
	}
	a.server.logs.Add(logs.Entry{
		Level:     level,
		Status:    result.Status,
		Duration:  result.Duration,
		TokenName: selected.Name,
		Message:   "token validation completed",
	})
	return result, err
}

func (a *DesktopApp) Config() config.Config {
	a.server.mu.Lock()
	defer a.server.mu.Unlock()
	return a.server.cfg
}

func (a *DesktopApp) SaveConfig(cfg config.Config) (config.Config, error) {
	return a.server.saveConfig(cfg)
}

func (a *DesktopApp) Logs() []logs.Entry {
	return a.server.logs.List()
}

func (a *DesktopApp) ProxyStatus() map[string]any {
	a.server.mu.Lock()
	defer a.server.mu.Unlock()
	return map[string]any{
		"running": a.server.proxyServer != nil,
		"port":    a.server.cfg.ProxyPort,
	}
}

func (a *DesktopApp) StartProxy() (map[string]any, error) {
	if err := a.server.startProxy(); err != nil {
		return nil, err
	}
	return map[string]any{"running": true}, nil
}

func (a *DesktopApp) StopProxy() (map[string]any, error) {
	if err := a.server.stopProxy(); err != nil {
		return nil, err
	}
	return map[string]any{"running": false}, nil
}

func (a *DesktopApp) ConfigureCodex() (codexConfigureResult, error) {
	return a.server.configureCodex()
}

func (a *DesktopApp) RestoreCodex() (codexConfigureResult, error) {
	return a.server.restoreCodexConfig()
}

func (a *DesktopApp) ConfigureMimoClaude() (mimoConfigureResult, error) {
	return a.server.configureMimoClaude()
}

func (a *DesktopApp) RestoreMimoClaude() (mimoConfigureResult, error) {
	return a.server.restoreMimoClaudeConfig()
}

func (a *DesktopApp) ConfigureDeepSeekClaude() (mimoConfigureResult, error) {
	return a.server.configureDeepSeekClaude()
}

func (a *DesktopApp) RestoreDeepSeekClaude() (mimoConfigureResult, error) {
	return a.server.restoreDeepSeekClaudeConfig()
}

func (a *DesktopApp) ConfigureKimiClaude() (mimoConfigureResult, error) {
	return a.server.configureKimiClaude()
}

func (a *DesktopApp) RestoreKimiClaude() (mimoConfigureResult, error) {
	return a.server.restoreKimiClaudeConfig()
}

func (a *DesktopApp) callContext() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
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
