package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"OmniProxyBackend/internal/autostart"
	"OmniProxyBackend/internal/config"
	"OmniProxyBackend/internal/history"
	"OmniProxyBackend/internal/logs"
	"OmniProxyBackend/internal/token"
	"OmniProxyBackend/internal/tray"

	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type DesktopApp struct {
	ctx    context.Context
	server *appServer
	tray   *tray.Manager
}

const autoStartNameBase = "OmniProxy"

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
	a.server.startHealthMonitor()
	a.setupTray()
}

func (a *DesktopApp) shutdown(ctx context.Context) {
	if a.tray != nil {
		a.tray.Stop()
		a.tray = nil
	}
	a.server.stopHealthMonitor()
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
	if a.server.history != nil {
		if err := a.server.history.Close(); err != nil {
			log.Printf("request history flush failed: %v", err)
		}
	}
}

func (a *DesktopApp) secondInstanceLaunch(data options.SecondInstanceData) {
	log.Printf("second OmniProxy %s instance requested from %s with args: %v", appRuntimeMode(), data.WorkingDirectory, data.Args)
	a.showMainWindow()
}

func (a *DesktopApp) showMainWindow() {
	if a.ctx == nil {
		return
	}
	runtime.Show(a.ctx)
	runtime.WindowShow(a.ctx)
	runtime.WindowUnminimise(a.ctx)
}

func (a *DesktopApp) setupTray() {
	manager, err := tray.Start(tray.Options{
		Tooltip: appDisplayName(),
		StatusLabel: func() string {
			a.server.mu.Lock()
			running := a.server.proxyServer != nil
			a.server.mu.Unlock()
			if running {
				return "代理运行中"
			}
			return "代理已停止"
		},
		PortLabel: func() string {
			a.server.mu.Lock()
			cfg := a.server.cfg
			a.server.mu.Unlock()
			return fmt.Sprintf("代理端口 :%d / 控制端口 :%d", cfg.ProxyPort, cfg.ControlPort)
		},
		IsProxyRunning: func() bool {
			a.server.mu.Lock()
			defer a.server.mu.Unlock()
			return a.server.proxyServer != nil
		},
		StartProxy: func() error {
			return a.server.startProxy()
		},
		StopProxy: func() error {
			return a.server.stopProxy()
		},
		ShowWindow: func() {
			a.showMainWindow()
		},
		Quit: func() {
			if a.ctx != nil {
				runtime.Quit(a.ctx)
			}
		},
		Log: func(format string, args ...any) {
			message := fmt.Sprintf(format, args...)
			log.Print(message)
			if a.server.logs != nil {
				a.server.logs.Add(logs.Entry{Level: logs.LevelWarn, Message: message})
			}
		},
	})
	if err != nil {
		log.Printf("tray not started: %v", err)
		if a.server.logs != nil {
			a.server.logs.Add(logs.Entry{Level: logs.LevelWarn, Message: fmt.Sprintf("tray not started: %v", err)})
		}
		return
	}
	a.tray = manager
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
	return a.server.createToken(a.callContext(), req)
}

func (a *DesktopApp) UpdateToken(id string, req token.UpsertRequest) (tokenResponse, error) {
	return a.server.updateToken(id, req)
}

func (a *DesktopApp) DeleteToken(id string) error {
	return a.server.deleteToken(id)
}

func (a *DesktopApp) SetTokenDisabled(id string, disabled bool) (tokenResponse, error) {
	return a.server.setTokenDisabled(id, disabled)
}

func (a *DesktopApp) UseOnlyToken(id string) ([]tokenResponse, error) {
	return a.server.useOnlyToken(id)
}

func (a *DesktopApp) CancelUseOnlyToken(id string) ([]tokenResponse, error) {
	return a.server.cancelUseOnlyToken(id)
}

func (a *DesktopApp) SetTokenSelected(id string, selected bool) ([]tokenResponse, error) {
	return a.server.setTokenSelected(id, selected)
}

func (a *DesktopApp) ValidateToken(id string) (validationResponse, error) {
	return a.server.validateToken(a.callContext(), id)
}

func (a *DesktopApp) RefreshTokenAuth(id string) (tokenResponse, error) {
	return a.server.refreshAuthTokenResponse(a.callContext(), id)
}

func (a *DesktopApp) ImportAPIKeys(req apiKeyBatchImportRequest) (apiKeyBatchImportResult, error) {
	return a.server.importAPIKeys(req)
}

func (a *DesktopApp) Config() config.Config {
	a.server.mu.Lock()
	defer a.server.mu.Unlock()
	return a.server.cfg
}

func (a *DesktopApp) SaveConfig(cfg config.Config) (config.Config, error) {
	return a.server.saveConfig(cfg)
}

type mimoCookieImportResult struct {
	Path       string `json:"path"`
	MatchedURL string `json:"matchedUrl"`
	Length     int    `json:"length"`
	Message    string `json:"message"`
}

func (a *DesktopApp) ImportMimoCookieFromHAR() (mimoCookieImportResult, error) {
	if a.ctx == nil {
		return mimoCookieImportResult{}, errors.New("application is not ready")
	}

	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择 Xiaomi MiMo HAR 文件",
		Filters: []runtime.FileFilter{
			{DisplayName: "HAR 文件 (*.har)", Pattern: "*.har"},
			{DisplayName: "JSON 文件 (*.json)", Pattern: "*.json"},
		},
	})
	if err != nil {
		return mimoCookieImportResult{}, err
	}
	if strings.TrimSpace(path) == "" {
		return mimoCookieImportResult{}, errors.New("未选择 HAR 文件")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return mimoCookieImportResult{}, err
	}
	cookie, matchedURL, err := extractMimoCookieFromHAR(data)
	if err != nil {
		return mimoCookieImportResult{}, err
	}

	a.server.mu.Lock()
	cfg := a.server.cfg
	a.server.mu.Unlock()
	cfg.XiaomiPlatformCookie = cookie
	if _, err := a.server.saveConfig(cfg); err != nil {
		return mimoCookieImportResult{}, err
	}

	result := mimoCookieImportResult{
		Path:       path,
		MatchedURL: matchedURL,
		Length:     len(cookie),
		Message:    "MiMo 控制台 Cookie 已导入",
	}
	a.server.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: "mimo platform cookie imported from HAR"})
	return result, nil
}

func extractMimoCookieFromHAR(data []byte) (string, string, error) {
	var har struct {
		Log struct {
			Entries []struct {
				Request struct {
					URL     string `json:"url"`
					Headers []struct {
						Name  string `json:"name"`
						Value string `json:"value"`
					} `json:"headers"`
				} `json:"request"`
			} `json:"entries"`
		} `json:"log"`
	}
	if err := json.Unmarshal(data, &har); err != nil {
		return "", "", fmt.Errorf("无法解析 HAR：%w", err)
	}

	targets := []string{
		"platform.xiaomimimo.com/api/v1/balance",
		"platform.xiaomimimo.com/api/v1/tokenPlan/usage",
		"platform.xiaomimimo.com/api/v1/tokenPlan/detail",
	}
	for _, target := range targets {
		for _, entry := range har.Log.Entries {
			if !strings.Contains(entry.Request.URL, target) {
				continue
			}
			for _, header := range entry.Request.Headers {
				if !strings.EqualFold(strings.TrimSpace(header.Name), "cookie") {
					continue
				}
				cookie := strings.TrimSpace(header.Value)
				if cookie == "" {
					continue
				}
				return cookie, entry.Request.URL, nil
			}
		}
	}

	return "", "", errors.New("HAR 中未找到 MiMo 余额或 Token Plan 请求的 Cookie")
}

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

func (a *DesktopApp) ExportTokens() (tokenExportResult, error) {
	if a.ctx == nil {
		return tokenExportResult{}, errors.New("application is not ready")
	}
	if a.server.tokens == nil {
		return tokenExportResult{}, errors.New("token manager is not ready")
	}

	items := a.server.tokens.List()
	data, err := encodeTokenExport(items, time.Now())
	if err != nil {
		return tokenExportResult{}, err
	}

	filename := fmt.Sprintf("omniproxy-tokens-%s.json", time.Now().Format("20060102-150405"))
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "导出账号池备份",
		DefaultFilename: filename,
		Filters:         []runtime.FileFilter{{DisplayName: "JSON 文件 (*.json)", Pattern: "*.json"}},
	})
	if err != nil {
		return tokenExportResult{}, err
	}
	if strings.TrimSpace(path) == "" {
		return tokenExportResult{Count: len(items), Message: "已取消导出账号池"}, nil
	}
	if strings.ToLower(filepath.Ext(path)) != ".json" {
		path += ".json"
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return tokenExportResult{}, err
	}
	a.server.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: fmt.Sprintf("token backup exported: %d accounts", len(items))})
	return tokenExportResult{
		Path:    path,
		Count:   len(items),
		Message: fmt.Sprintf("账号池已导出：%d 个账号", len(items)),
	}, nil
}

func (a *DesktopApp) ExportCodexAuthFiles() (codexAuthExportResult, error) {
	if a.ctx == nil {
		return codexAuthExportResult{}, errors.New("application is not ready")
	}
	if a.server.tokens == nil {
		return codexAuthExportResult{}, errors.New("token manager is not ready")
	}

	files := codexAuthExportFiles(a.server.tokens.List(), time.Now().Format("20060102-150405"))
	if len(files) == 0 {
		return codexAuthExportResult{}, errors.New("没有可导出的 Codex auth.json 账号")
	}

	directory, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:                "选择 Codex auth.json 导出目录",
		CanCreateDirectories: true,
	})
	if err != nil {
		return codexAuthExportResult{}, err
	}
	if strings.TrimSpace(directory) == "" {
		return codexAuthExportResult{Count: len(files), Message: "已取消导出 Codex auth.json"}, nil
	}

	written, err := writeCodexAuthExportFiles(directory, files)
	if err != nil {
		return codexAuthExportResult{}, err
	}
	a.server.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: fmt.Sprintf("codex auth files exported: %d files", len(written))})
	return codexAuthExportResult{
		Directory: directory,
		Files:     written,
		Count:     len(written),
		Message:   fmt.Sprintf("Codex auth.json 已导出：%d 个文件", len(written)),
	}, nil
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

func (a *DesktopApp) CheckForUpdates() (updateInfo, error) {
	return checkForUpdates(a.callContext(), http.DefaultClient)
}

func (a *DesktopApp) DownloadUpdate(req updateDownloadRequest) (updateDownloadStatus, error) {
	return a.server.updateManager().Start(context.Background(), http.DefaultClient, req)
}

func (a *DesktopApp) UpdateDownloadStatus() updateDownloadStatus {
	return a.server.updateManager().Status()
}

func (a *DesktopApp) InstallDownloadedUpdate() (updateDownloadStatus, error) {
	return a.server.updateManager().Install()
}

func (a *DesktopApp) AppInfo() appInfo {
	return currentAppInfo()
}

func (a *DesktopApp) AutoStartStatus() (map[string]any, error) {
	enabled, err := autostart.Enabled(autoStartName())
	if err != nil {
		return nil, err
	}
	return map[string]any{"enabled": enabled}, nil
}

func (a *DesktopApp) SetAutoStart(enabled bool) (map[string]any, error) {
	if err := autostart.Set(autoStartName(), enabled, "--minimized"); err != nil {
		return nil, err
	}
	return map[string]any{"enabled": enabled}, nil
}

func autoStartName() string {
	if isDevInstance() {
		return autoStartNameBase + " Dev"
	}
	return autoStartNameBase
}

func (a *DesktopApp) ConfigureCodex() (codexConfigureResult, error) {
	return a.server.configureCodex()
}

func (a *DesktopApp) ConfigureCodexSub2API() (codexConfigureResult, error) {
	return a.server.configureCodexSub2API()
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

func (a *DesktopApp) ConfigureZhipuClaude() (mimoConfigureResult, error) {
	return a.server.configureZhipuClaude()
}

func (a *DesktopApp) ConfigureClaudeModels(req claudeModelsConfigureRequest) (mimoConfigureResult, error) {
	return a.server.configureClaudeModels(req)
}

func (a *DesktopApp) RestoreZhipuClaude() (mimoConfigureResult, error) {
	return a.server.restoreZhipuClaudeConfig()
}

func (a *DesktopApp) ConfigureDeepSeekTUI() (clientConfigureResult, error) {
	return a.server.configureDeepSeekTUI()
}

func (a *DesktopApp) RestoreDeepSeekTUI() (clientConfigureResult, error) {
	return a.server.restoreDeepSeekTUIConfig()
}

func (a *DesktopApp) ConfigureGemini() (clientConfigureResult, error) {
	return a.server.configureGemini()
}

func (a *DesktopApp) RestoreGemini() (clientConfigureResult, error) {
	return a.server.restoreGeminiConfig()
}

func (a *DesktopApp) ConfigureOpenCode() (clientConfigureResult, error) {
	return a.server.configureOpenCode()
}

func (a *DesktopApp) RestoreOpenCode() (clientConfigureResult, error) {
	return a.server.restoreOpenCodeConfig()
}

func (a *DesktopApp) ConfigurePi() (clientConfigureResult, error) {
	return a.server.configurePi()
}

func (a *DesktopApp) RestorePi() (clientConfigureResult, error) {
	return a.server.restorePiConfig()
}

func (a *DesktopApp) OpenRouterModels(refresh bool) (openRouterModelsResponse, error) {
	return a.server.openRouterModels(a.callContext(), refresh)
}

func (a *DesktopApp) OpenRouterChat(req openRouterChatRequest) (openRouterChatResponse, error) {
	return a.server.openRouterChat(a.callContext(), req)
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

func encodeHistoryCSV(entries []history.Entry) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF")
	writer := csv.NewWriter(&buf)
	if err := writer.Write([]string{
		"时间",
		"级别",
		"方法",
		"路径",
		"路由厂商",
		"协议",
		"编程工具",
		"模型",
		"状态码",
		"耗时(ms)",
		"账号",
		"输入Token",
		"输出Token",
		"总Token",
		"触发冷却",
		"错误摘要",
		"重试链路",
	}); err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if err := writer.Write([]string{
			entry.Time.Format(time.RFC3339),
			entry.Level,
			entry.Method,
			entry.Path,
			entry.Provider,
			entry.Protocol,
			entry.ClientName,
			entry.Model,
			fmt.Sprintf("%d", entry.Status),
			fmt.Sprintf("%d", entry.Duration),
			entry.TokenName,
			fmt.Sprintf("%d", entry.InputTokens),
			fmt.Sprintf("%d", entry.OutputTokens),
			fmt.Sprintf("%d", entry.TotalTokens),
			formatBoolCN(entry.CooldownTriggered),
			entry.Message,
			formatRetryChain(entry.RetryChain),
		}); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	return buf.Bytes(), writer.Error()
}

func formatBoolCN(value bool) string {
	if value {
		return "是"
	}
	return "否"
}

func formatRetryChain(chain []history.RetryAttempt) string {
	if len(chain) == 0 {
		return ""
	}
	parts := make([]string, 0, len(chain))
	for _, attempt := range chain {
		label := fmt.Sprintf("#%d %s", attempt.Attempt, attempt.Provider)
		if attempt.TokenName != "" {
			label += " " + attempt.TokenName
		}
		if attempt.Status != 0 {
			label += fmt.Sprintf(" %d", attempt.Status)
		}
		if attempt.Duration > 0 {
			label += fmt.Sprintf(" %dms", attempt.Duration)
		}
		if attempt.CooldownTriggered {
			label += " 冷却"
		}
		if attempt.Message != "" {
			label += " " + attempt.Message
		}
		parts = append(parts, label)
	}
	return strings.Join(parts, " | ")
}
