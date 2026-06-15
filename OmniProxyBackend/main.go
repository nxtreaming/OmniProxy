package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"OmniProxyBackend/internal/config"
	"OmniProxyBackend/internal/history"
	"OmniProxyBackend/internal/logs"
	"OmniProxyBackend/internal/proxy"
	"OmniProxyBackend/internal/storage"
	"OmniProxyBackend/internal/taskautomation"
	"OmniProxyBackend/internal/token"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

type appServer struct {
	mu                    sync.Mutex
	codexRefreshMu        sync.Mutex
	dataDir               string
	cfg                   config.Config
	configStore           *config.Store
	tokens                *token.Manager
	logs                  *logs.Recorder
	history               *history.Recorder
	proxyServer           *http.Server
	proxyService          *proxy.Service
	premProxy             *premProxyManager
	taskAutomation        *taskautomation.Manager
	control               *http.Server
	controlToken          string
	updates               *updateDownloader
	healthStop            context.CancelFunc
	openRouterModelsMu    sync.Mutex
	openRouterModelsCache openRouterModelsCache
}

const (
	healthCheckTick       = time.Minute
	activeHealthInterval  = 15 * time.Minute
	retryHealthInterval   = time.Minute
	currentQuotaInterval  = 30 * time.Second
	healthRequestTimeout  = 15 * time.Second
	failedHealthRetryWait = 5 * time.Minute
	controlTokenHeader    = "X-OmniProxy-Control-Token"
	requestHistoryMax     = 50000
	defaultHistoryLimit   = 10000
)

const (
	historyEventManualValidation  = "manual-validation"
	historyEventCodexRefreshAdd   = "codex-refresh-after-add"
	historyEventStartupCodexUsage = "startup-codex-usage-refresh"
	historyEventCurrentQuota      = "current-quota-refresh"
	historyEventHealthCheck       = "health-check"
)

func main() {
	config.SetRuntimeProfile(appRuntimeMode())

	server, err := newAppServer()
	if err != nil {
		log.Fatalf("initialise app: %v", err)
	}

	desktop := NewDesktopApp(server)

	err = wails.Run(&options.App{
		Title:             appDisplayName(),
		Width:             1280,
		Height:            860,
		MinWidth:          1240,
		MinHeight:         720,
		Frameless:         true,
		StartHidden:       startHiddenFromArgs(),
		HideWindowOnClose: true,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 238, G: 242, B: 247, A: 1},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               singleInstanceUniqueID(),
			OnSecondInstanceLaunch: desktop.secondInstanceLaunch,
		},
		OnStartup:  desktop.startup,
		OnShutdown: desktop.shutdown,
		Bind: []interface{}{
			desktop,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}

func startHiddenFromArgs() bool {
	for _, arg := range os.Args[1:] {
		switch strings.ToLower(strings.TrimSpace(arg)) {
		case "--minimized", "--hidden", "/minimized", "/hidden":
			return true
		}
	}
	return false
}

func newControlToken() (string, error) {
	var data [32]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(data[:]), nil
}

func newAppServer() (*appServer, error) {
	controlToken, err := newControlToken()
	if err != nil {
		return nil, fmt.Errorf("generate control token: %w", err)
	}
	recorder := logs.NewRecorder(500)
	defaultCfg := config.Default()
	server := &appServer{
		cfg:            defaultCfg,
		logs:           recorder,
		premProxy:      newPremProxyManager(recorder),
		taskAutomation: taskautomation.NewManager(defaultCfg, recorder),
		controlToken:   controlToken,
		updates:        newUpdateDownloader(),
	}
	info, configured, err := config.ResolveDataDir()
	if err != nil {
		return nil, fmt.Errorf("resolve data directory: %w", err)
	}
	if configured {
		if err := server.loadData(info.DataDir); err != nil {
			return nil, err
		}
	} else {
		server.dataDir = info.DataDir
	}
	return server, nil
}

func (a *appServer) loadData(dataDir string) error {
	dataDir, err := config.PrepareDataDir(dataDir)
	if err != nil {
		return fmt.Errorf("prepare data directory: %w", err)
	}

	cfgStore := config.NewStore(filepath.Join(dataDir, "config.json"))
	cfg, err := cfgStore.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	tokenStore := token.NewSecureStore(storage.NewJSONStore[[]token.Token](filepath.Join(dataDir, "tokens.json")))
	tokenManager, err := token.NewManager(tokenStore, cfg.SwitchThreshold)
	if err != nil {
		return fmt.Errorf("load tokens: %w", err)
	}
	historyStore, err := history.NewSQLiteStore(filepath.Join(dataDir, "request_history.db"))
	if err != nil {
		return fmt.Errorf("open request history database: %w", err)
	}
	if err := migrateLegacyRequestHistory(historyStore, filepath.Join(dataDir, "request_history.json")); err != nil {
		_ = historyStore.Close()
		return fmt.Errorf("migrate request history: %w", err)
	}
	historyRecorder, err := history.NewRecorder(historyStore, requestHistoryMax)
	if err != nil {
		_ = historyStore.Close()
		return fmt.Errorf("load request history: %w", err)
	}
	if err := historyRecorder.SetRetentionDays(cfg.HistoryRetentionDays); err != nil {
		_ = historyStore.Close()
		return fmt.Errorf("apply request history retention: %w", err)
	}

	a.mu.Lock()
	a.dataDir = dataDir
	a.cfg = cfg
	a.configStore = cfgStore
	a.tokens = tokenManager
	a.history = historyRecorder
	a.mu.Unlock()
	if a.taskAutomation != nil {
		a.taskAutomation.UpdateConfig(cfg)
	}
	return nil
}

func migrateLegacyRequestHistory(store history.Store, legacyPath string) error {
	existing, err := store.Load()
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return nil
	}

	legacyStore := storage.NewJSONStore[[]history.Entry](legacyPath)
	entries, err := legacyStore.Load()
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}
	return store.Save(entries)
}

func (a *appServer) isLoaded() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.configStore != nil && a.tokens != nil
}

func (a *appServer) updateManager() *updateDownloader {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.updates == nil {
		a.updates = newUpdateDownloader()
	}
	return a.updates
}

func (a *appServer) includePrereleaseUpdates() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cfg.CheckBetaUpdates
}

func (a *appServer) dataDirectoryInfo() config.DataDirectoryInfo {
	a.mu.Lock()
	dataDir := a.dataDir
	a.mu.Unlock()

	info, _, err := config.ResolveDataDir()
	if err == nil && info.DataDir != "" {
		if dataDir != "" {
			info.DataDir = dataDir
		}
		return info
	}
	return config.DataDirectoryInfo{
		DataDir:       dataDir,
		BootstrapPath: config.BootstrapPath(),
		Source:        "unknown",
	}
}

func (a *appServer) changeDataDirectory(dataDir string, migrate bool) (config.DataDirectoryChangeResult, error) {
	if info, configured, err := config.ResolveDataDir(); err == nil && configured && info.EnvOverride {
		return config.DataDirectoryChangeResult{DataDir: info.DataDir, BootstrapPath: info.BootstrapPath, EnvOverride: true}, errors.New("data directory is controlled by OMNIPROXY_DATA_DIR")
	}

	nextDir, err := config.PrepareDataDir(dataDir)
	if err != nil {
		return config.DataDirectoryChangeResult{}, err
	}

	a.mu.Lock()
	previousDir := a.dataDir
	a.mu.Unlock()

	var copied []string
	var skipped []string
	if migrate && previousDir != "" {
		copied, skipped, err = config.CopyDataFiles(previousDir, nextDir)
		if err != nil {
			return config.DataDirectoryChangeResult{}, err
		}
	}
	if err := config.SaveBootstrap(nextDir); err != nil {
		return config.DataDirectoryChangeResult{}, err
	}

	return config.DataDirectoryChangeResult{
		DataDir:         nextDir,
		PreviousDataDir: previousDir,
		BootstrapPath:   config.BootstrapPath(),
		MigratedFiles:   copied,
		SkippedFiles:    skipped,
		RestartRequired: previousDir != "" && !strings.EqualFold(filepath.Clean(previousDir), filepath.Clean(nextDir)),
	}, nil
}

func (a *appServer) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/control-token", a.handleControlToken)
	mux.HandleFunc("/api/tokens", a.handleTokens)
	mux.HandleFunc("/api/tokens/import-api-keys", a.handleImportAPIKeys)
	mux.HandleFunc("/api/tokens/", a.handleTokenByID)
	mux.HandleFunc("/api/config", a.handleConfig)
	mux.HandleFunc("/api/logs", a.handleLogs)
	mux.HandleFunc("/api/history", a.handleHistory)
	mux.HandleFunc("/api/history/summary", a.handleHistorySummary)
	mux.HandleFunc("/api/history/clear", a.handleHistoryClear)
	mux.HandleFunc("/api/billing/summary", a.handleBillingSummary)
	mux.HandleFunc("/api/billing/usage", a.handleBillingUsage)
	mux.HandleFunc("/api/billing/dates", a.handleBillingDates)
	mux.HandleFunc("/api/billing/clear", a.handleBillingClear)
	mux.HandleFunc("/api/proxy/status", a.handleProxyStatus)
	mux.HandleFunc("/api/proxy/active-requests", a.handleActiveProxyRequests)
	mux.HandleFunc("/api/proxy/start", a.handleProxyStart)
	mux.HandleFunc("/api/proxy/stop", a.handleProxyStop)
	mux.HandleFunc("/api/app/info", a.handleAppInfo)
	mux.HandleFunc("/api/update/check", a.handleUpdateCheck)
	mux.HandleFunc("/api/update/download", a.handleUpdateDownload)
	mux.HandleFunc("/api/update/download/status", a.handleUpdateDownloadStatus)
	mux.HandleFunc("/api/update/install", a.handleUpdateInstall)
	mux.HandleFunc("/api/data-directory", a.handleDataDirectory)
	mux.HandleFunc("/api/codex/configure", a.handleCodexConfigure)
	mux.HandleFunc("/api/codex/sub2api/configure", a.handleCodexSub2APIConfigure)
	mux.HandleFunc("/api/codex/newapi/configure", a.handleCodexNewAPIConfigure)
	mux.HandleFunc("/api/codex/anyrouter/configure", a.handleCodexAnyRouterConfigure)
	mux.HandleFunc("/api/codex/zo/configure", a.handleCodexZoConfigure)
	mux.HandleFunc("/api/codex/prem/configure", a.handleCodexPremConfigure)
	mux.HandleFunc("/api/codex/restore", a.handleCodexRestore)
	mux.HandleFunc("/api/mimo/claude/configure", a.handleMimoClaudeConfigure)
	mux.HandleFunc("/api/mimo/claude/restore", a.handleMimoClaudeRestore)
	mux.HandleFunc("/api/deepseek/claude/configure", a.handleDeepSeekClaudeConfigure)
	mux.HandleFunc("/api/deepseek/claude/restore", a.handleDeepSeekClaudeRestore)
	mux.HandleFunc("/api/kimi/claude/configure", a.handleKimiClaudeConfigure)
	mux.HandleFunc("/api/kimi/claude/restore", a.handleKimiClaudeRestore)
	mux.HandleFunc("/api/zhipu/claude/configure", a.handleZhipuClaudeConfigure)
	mux.HandleFunc("/api/zhipu/claude/restore", a.handleZhipuClaudeRestore)
	mux.HandleFunc("/api/anyrouter/claude/configure", a.handleAnyRouterClaudeConfigure)
	mux.HandleFunc("/api/zo/claude/configure", a.handleZoClaudeConfigure)
	mux.HandleFunc("/api/claude/models/configure", a.handleClaudeModelsConfigure)
	mux.HandleFunc("/api/claude/desktop/models/configure", a.handleClaudeDesktopModelsConfigure)
	mux.HandleFunc("/api/claude/desktop/restore", a.handleClaudeDesktopRestore)
	mux.HandleFunc("/api/deepseek-tui/configure", a.handleDeepSeekTUIConfigure)
	mux.HandleFunc("/api/deepseek-tui/restore", a.handleDeepSeekTUIRestore)
	mux.HandleFunc("/api/gemini/configure", a.handleGeminiConfigure)
	mux.HandleFunc("/api/gemini/restore", a.handleGeminiRestore)
	mux.HandleFunc("/api/openrouter/models", a.handleOpenRouterModels)
	mux.HandleFunc("/api/openrouter/chat", a.handleOpenRouterChat)
	mux.HandleFunc("/api/opencode/configure", a.handleOpenCodeConfigure)
	mux.HandleFunc("/api/opencode/restore", a.handleOpenCodeRestore)
	mux.HandleFunc("/api/pi/configure", a.handlePiConfigure)
	mux.HandleFunc("/api/pi/restore", a.handlePiRestore)
	return withControlTokenAuth(a.controlToken, mux)
}

func (a *appServer) handleControlToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !isTrustedControlTokenOrigin(r.Header.Get("Origin")) {
		w.Header().Set("Cache-Control", "no-store")
		writeError(w, http.StatusForbidden, "control token is only available to the desktop app")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]string{
		"header": controlTokenHeader,
		"token":  a.controlToken,
	})
}

func (a *appServer) handleTokens(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, tokenResponses(a.tokens.List()))
	case http.MethodPost:
		var req token.UpsertRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		item, err := a.createToken(r.Context(), req)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, item)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *appServer) handleImportAPIKeys(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req apiKeyBatchImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	result, err := a.importAPIKeys(req)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	status := http.StatusOK
	if result.CreatedCount > 0 {
		status = http.StatusCreated
	}
	writeJSON(w, status, result)
}

func (a *appServer) handleTokenByID(w http.ResponseWriter, r *http.Request) {
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/tokens/"), "/")
	parts := strings.Split(rest, "/")
	id := parts[0]
	if id == "" {
		writeError(w, http.StatusNotFound, "token not found")
		return
	}

	if len(parts) == 2 && parts[1] == "validate" {
		a.handleTokenValidate(w, r, id)
		return
	}
	if len(parts) == 2 && parts[1] == "refresh" {
		a.handleTokenRefresh(w, r, id)
		return
	}
	if len(parts) == 2 && parts[1] == "disabled" {
		a.handleTokenDisabled(w, r, id)
		return
	}
	if len(parts) == 2 && parts[1] == "exclusive" {
		a.handleTokenExclusive(w, r, id)
		return
	}
	if len(parts) == 2 && parts[1] == "selected" {
		a.handleTokenSelected(w, r, id)
		return
	}
	if len(parts) > 1 {
		writeError(w, http.StatusNotFound, "token not found")
		return
	}

	switch r.Method {
	case http.MethodPut:
		var req token.UpsertRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		item, err := a.updateToken(id, req)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, item)
	case http.MethodDelete:
		if err := a.deleteToken(id); err != nil {
			writeDomainError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

type tokenDisabledRequest struct {
	Disabled bool `json:"disabled"`
}

type tokenSelectedRequest struct {
	Selected bool `json:"selected"`
}

func (a *appServer) handleTokenDisabled(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPut {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req tokenDisabledRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	item, err := a.setTokenDisabled(id, req.Disabled)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *appServer) handleTokenExclusive(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPut && r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var (
		items []tokenResponse
		err   error
	)
	if r.Method == http.MethodDelete {
		items, err = a.cancelUseOnlyToken(id)
	} else {
		items, err = a.useOnlyToken(id)
	}
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (a *appServer) handleTokenSelected(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPut {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req tokenSelectedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	items, err := a.setTokenSelected(id, req.Selected)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (a *appServer) handleTokenValidate(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	result, err := a.validateToken(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, result)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *appServer) handleTokenRefresh(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	item, err := a.refreshAuthTokenResponse(r.Context(), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *appServer) validateAndRecordToken(ctx context.Context, selected token.Token) (proxy.ValidationResult, error) {
	a.mu.Lock()
	cfg := a.cfg
	a.mu.Unlock()

	refreshedSelected, _, refreshErr := a.refreshAuthTokenIfNeeded(ctx, selected, false)
	if refreshErr != nil {
		_ = a.tokens.MarkInvalid(selected.ID, fmt.Sprintf("OAuth token refresh failed: %v", refreshErr))
		return proxy.ValidationResult{}, refreshErr
	}
	selected = refreshedSelected

	validator, err := proxy.NewValidator(cfg)
	if err != nil {
		return proxy.ValidationResult{}, err
	}

	result, err := validator.Validate(ctx, selected)
	if err == nil && isRefreshableAuthToken(selected) && (result.Status == http.StatusUnauthorized || result.Status == http.StatusForbidden) {
		refreshedSelected, refreshed, refreshErr := a.refreshAuthTokenIfNeeded(ctx, selected, true)
		if refreshErr != nil {
			_ = a.tokens.MarkInvalid(selected.ID, fmt.Sprintf("OAuth token refresh failed: %v", refreshErr))
			return result, refreshErr
		}
		if refreshed {
			selected = refreshedSelected
			result, err = validator.Validate(ctx, selected)
		}
	}
	if result.Remaining != nil {
		_ = a.tokens.RecordUsage(selected.ID, *result.Remaining)
	}
	if result.Usage != nil {
		_ = a.tokens.RecordUsageInfo(selected.ID, *result.Usage)
	}
	if result.Status == http.StatusUnauthorized || result.Status == http.StatusForbidden {
		_ = a.tokens.MarkInvalid(selected.ID, fmt.Sprintf("validation returned %d", result.Status))
	}
	if result.Status == http.StatusTooManyRequests {
		_ = a.tokens.MarkExhaustedUntil(selected.ID, "validation returned 429", validationCooldownUntil(result))
	}
	if result.OK && result.Remaining == nil && result.Usage == nil {
		_ = a.tokens.MarkActive(selected.ID)
	}
	return result, err
}

func (a *appServer) refreshStoredAuthToken(ctx context.Context, id string) (token.Token, error) {
	selected, err := a.tokens.Get(id)
	if err != nil {
		return token.Token{}, err
	}
	if !isRefreshableAuthToken(selected) {
		return token.Token{}, errors.New("token credential does not support refresh")
	}

	updated, _, err := a.refreshAuthTokenIfNeeded(ctx, selected, true)
	if err != nil {
		_ = a.tokens.MarkInvalid(selected.ID, fmt.Sprintf("OAuth token refresh failed: %v", err))
		return token.Token{}, err
	}
	if a.logs != nil {
		a.logs.Add(logs.Entry{Level: logs.LevelInfo, TokenName: updated.Name, Message: "manual OAuth token refresh completed"})
	}
	return updated, nil
}

type parsedAPIKeyLine struct {
	line  int
	value string
}

func (a *appServer) importAPIKeys(req apiKeyBatchImportRequest) (apiKeyBatchImportResult, error) {
	provider, credentialType, err := token.NormalizeProviderAndCredential(req.Provider, token.CredentialTypeAPIKey)
	if err != nil {
		return apiKeyBatchImportResult{}, err
	}
	if credentialType != token.CredentialTypeAPIKey {
		return apiKeyBatchImportResult{}, errors.New("批量导入仅支持 API Key")
	}
	region, err := token.NormalizeRegion(provider, credentialType, req.Region)
	if err != nil {
		return apiKeyBatchImportResult{}, err
	}
	baseURL, err := token.NormalizeBaseURL(provider, req.BaseURL, true)
	if err != nil {
		return apiKeyBatchImportResult{}, err
	}

	lines := parseAPIKeyBatchLines(req.TokenText)
	if len(lines) == 0 {
		return apiKeyBatchImportResult{}, errors.New("未找到可导入的 API Key")
	}

	usedNames := map[string]bool{}
	existingKeys := map[string]bool{}
	for _, item := range a.tokens.List() {
		if token.NormalizeProvider(item.Provider) != provider || item.CredentialType != credentialType {
			continue
		}
		usedNames[strings.ToLower(strings.TrimSpace(item.Name))] = true
		if strings.TrimSpace(item.BaseURL) == baseURL {
			existingKeys[strings.TrimSpace(item.TokenValue)] = true
		}
	}

	result := apiKeyBatchImportResult{
		Skipped: []apiKeyBatchImportSkipped{},
	}
	seenKeys := map[string]bool{}
	for _, line := range lines {
		if seenKeys[line.value] {
			result.Skipped = append(result.Skipped, apiKeyBatchImportSkipped{Line: line.line, Reason: "本次导入中重复"})
			continue
		}
		seenKeys[line.value] = true
		if existingKeys[line.value] {
			result.Skipped = append(result.Skipped, apiKeyBatchImportSkipped{Line: line.line, Reason: "账号池中已存在"})
			continue
		}

		name := uniqueAPIKeyImportName(line.value, usedNames)
		if _, err := a.tokens.Add(token.UpsertRequest{
			Name:           name,
			Provider:       provider,
			CredentialType: credentialType,
			Region:         region,
			BaseURL:        baseURL,
			TokenValue:     line.value,
		}); err != nil {
			delete(usedNames, strings.ToLower(name))
			result.Skipped = append(result.Skipped, apiKeyBatchImportSkipped{Line: line.line, Reason: err.Error()})
			continue
		}
		existingKeys[line.value] = true
		result.CreatedCount++
	}

	if a.logs != nil && result.CreatedCount > 0 {
		a.logs.Add(logs.Entry{
			Level:   logs.LevelInfo,
			Message: fmt.Sprintf("%s batch imported API keys: %d created, %d skipped", provider, result.CreatedCount, len(result.Skipped)),
		})
	}
	return result, nil
}

func parseAPIKeyBatchLines(text string) []parsedAPIKeyLine {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	lines := strings.Split(normalized, "\n")
	out := []parsedAPIKeyLine{}
	for index, raw := range lines {
		line := strings.TrimSpace(raw)
		if before, _, ok := strings.Cut(line, "#"); ok {
			line = strings.TrimSpace(before)
		}
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		out = append(out, parsedAPIKeyLine{line: index + 1, value: strings.TrimSpace(fields[0])})
	}
	return out
}

func uniqueAPIKeyImportName(value string, used map[string]bool) string {
	base := apiKeyImportName(value)
	name := base
	for suffix := 2; used[strings.ToLower(name)]; suffix++ {
		name = fmt.Sprintf("%s-%d", base, suffix)
	}
	used[strings.ToLower(name)] = true
	return name
}

func apiKeyImportName(value string) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) == 0 {
		return "api-key"
	}
	if len(runes) > 8 {
		runes = runes[:8]
	}
	return string(runes)
}

func (a *appServer) recordTokenMaintenanceHistory(event string, selected token.Token, result proxy.ValidationResult, err error) {
	a.mu.Lock()
	recorder := a.history
	a.mu.Unlock()
	if recorder == nil {
		return
	}

	path, protocol, label := tokenMaintenanceEventMeta(event)
	level := logs.LevelInfo
	if err != nil || !result.OK {
		level = logs.LevelWarn
	}
	recorder.Add(history.Entry{
		Level:     string(level),
		Method:    "CHECK",
		Path:      path,
		Provider:  token.NormalizeProvider(selected.Provider),
		Protocol:  protocol,
		Model:     selected.CredentialType,
		Status:    result.Status,
		Duration:  result.Duration,
		TokenID:   selected.ID,
		TokenName: selected.Name,
		Message:   tokenMaintenanceHistoryMessage(label, result, err),
	})
}

func tokenMaintenanceEventMeta(event string) (string, string, string) {
	switch event {
	case historyEventCodexRefreshAdd:
		return "/maintenance/codex-usage-refresh", "quota-refresh", "codex usage refresh after add completed"
	case historyEventStartupCodexUsage:
		return "/maintenance/startup-codex-usage-refresh", "quota-refresh", "startup codex quota refresh completed"
	case historyEventCurrentQuota:
		return "/maintenance/current-token-quota-refresh", "quota-refresh", "current token quota refresh completed"
	case historyEventHealthCheck:
		return "/maintenance/token-health-check", "health-check", "token health check completed"
	default:
		return "/maintenance/token-validation", "token-validation", "manual token validation completed"
	}
}

func tokenMaintenanceHistoryMessage(label string, result proxy.ValidationResult, err error) string {
	parts := []string{label}
	if err != nil {
		parts = append(parts, err.Error())
	} else if strings.TrimSpace(result.Message) != "" {
		parts = append(parts, result.Message)
	}
	if result.Remaining != nil {
		parts = append(parts, fmt.Sprintf("remaining=%d%%", *result.Remaining))
	}
	if result.Usage != nil && result.Usage.SubscriptionQuotaAvailable {
		parts = append(parts, fmt.Sprintf("primary=%d%%", result.Usage.PrimaryRemainingPercent))
		parts = append(parts, fmt.Sprintf("secondary=%d%%", result.Usage.SecondaryRemainingPercent))
	}
	return strings.Join(parts, " · ")
}

func (a *appServer) refreshAuthTokenIfNeeded(ctx context.Context, selected token.Token, force bool) (token.Token, bool, error) {
	if !isRefreshableAuthToken(selected) {
		return selected, false, nil
	}

	a.codexRefreshMu.Lock()
	defer a.codexRefreshMu.Unlock()

	if latest, err := a.tokens.Get(selected.ID); err == nil {
		selected = latest
	}

	a.mu.Lock()
	cfg := a.cfg
	a.mu.Unlock()
	client, err := proxy.NewTokenHTTPClient(cfg, selected, healthRequestTimeout)
	if err != nil {
		return selected, false, err
	}
	var updatedValue string
	var refreshed bool
	switch {
	case isCodexToken(selected):
		updatedValue, refreshed, err = proxy.RefreshCodexAuthJSON(ctx, client, selected.TokenValue, force, time.Now())
	case isClaudeOAuthToken(selected):
		updatedValue, refreshed, err = proxy.RefreshClaudeOAuthJSON(ctx, client, selected.TokenValue, force, time.Now())
	default:
		return selected, false, nil
	}
	if err != nil || !refreshed {
		return selected, refreshed, err
	}

	updated, err := a.tokens.UpdateTokenValue(selected.ID, updatedValue)
	if err != nil {
		return selected, true, err
	}
	if a.logs != nil {
		a.logs.Add(logs.Entry{Level: logs.LevelInfo, TokenName: updated.Name, Message: "OAuth access token refreshed"})
	}
	return updated, true, nil
}

func (a *appServer) refreshCodexUsageOnStartup(ctx context.Context) {
	items := a.tokens.List()
	total := 0
	failed := 0
	for _, item := range items {
		if item.Disabled {
			continue
		}
		if !isCodexToken(item) {
			continue
		}
		total++
		result, err := a.validateAndRecordToken(ctx, item)
		a.recordTokenMaintenanceHistory(historyEventStartupCodexUsage, item, result, err)
		if err != nil {
			failed++
			a.logs.Add(logs.Entry{Level: logs.LevelWarn, TokenName: item.Name, Message: fmt.Sprintf("startup codex usage refresh failed: %v", err)})
		}
	}
	if total == 0 {
		return
	}
	message := fmt.Sprintf("startup codex usage refresh completed: %d accounts", total)
	level := logs.LevelInfo
	if failed > 0 {
		level = logs.LevelWarn
		message = fmt.Sprintf("startup codex usage refresh completed: %d accounts, %d failed", total, failed)
	}
	a.logs.Add(logs.Entry{Level: level, Message: message})
}

func (a *appServer) startHealthMonitor() {
	a.mu.Lock()
	if a.healthStop != nil || a.tokens == nil {
		a.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.healthStop = cancel
	a.mu.Unlock()

	go a.healthMonitor(ctx)
}

func (a *appServer) stopHealthMonitor() {
	a.mu.Lock()
	cancel := a.healthStop
	a.healthStop = nil
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *appServer) healthMonitor(ctx context.Context) {
	healthTimer := time.NewTimer(30 * time.Second)
	defer healthTimer.Stop()
	currentQuotaTimer := time.NewTimer(currentQuotaInterval)
	defer currentQuotaTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-healthTimer.C:
			a.runDueHealthChecks(ctx)
			healthTimer.Reset(healthCheckTick)
		case <-currentQuotaTimer.C:
			a.refreshCurrentTokenUsage(ctx)
			currentQuotaTimer.Reset(currentQuotaInterval)
		}
	}
}

func (a *appServer) refreshCurrentTokenUsage(ctx context.Context) {
	a.mu.Lock()
	manager := a.tokens
	a.mu.Unlock()
	if manager == nil {
		return
	}

	selected, ok := currentQuotaRefreshCandidate(manager.List(), time.Now())
	if !ok {
		return
	}

	checkCtx, cancel := context.WithTimeout(ctx, healthRequestTimeout)
	result, err := a.validateAndRecordToken(checkCtx, selected)
	cancel()
	a.recordTokenMaintenanceHistory(historyEventCurrentQuota, selected, result, err)

	message := result.Message
	if err != nil {
		message = err.Error()
	}
	_ = manager.RecordHealthCheck(selected.ID, result.OK, result.Status, message, nextHealthCheckAt(result, err))

	if err != nil || !result.OK {
		a.logs.Add(logs.Entry{
			Level:     logs.LevelWarn,
			Status:    result.Status,
			Duration:  result.Duration,
			TokenName: selected.Name,
			Message:   fmt.Sprintf("current token quota refresh failed: %v", message),
		})
	}
}

func currentQuotaRefreshCandidate(items []token.Token, now time.Time) (token.Token, bool) {
	var selected token.Token
	found := false
	for _, item := range items {
		if item.Disabled {
			continue
		}
		if strings.TrimSpace(item.TokenValue) == "" || item.Stats.UpdatedAt == nil {
			continue
		}
		if item.Status == token.StatusInvalid {
			continue
		}
		if item.CooldownUntil != nil && now.Before(*item.CooldownUntil) {
			continue
		}
		if item.Health.NextCheckAt != nil && now.Before(*item.Health.NextCheckAt) {
			continue
		}
		if !found || item.Stats.UpdatedAt.After(*selected.Stats.UpdatedAt) {
			selected = item
			found = true
		}
	}
	return selected, found
}

func (a *appServer) runDueHealthChecks(ctx context.Context) {
	a.mu.Lock()
	manager := a.tokens
	a.mu.Unlock()
	if manager == nil {
		return
	}

	candidates := manager.HealthCheckCandidates(time.Now(), activeHealthInterval, retryHealthInterval)
	if len(candidates) == 0 {
		return
	}

	a.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: fmt.Sprintf("health check started: %d accounts", len(candidates))})
	for _, item := range candidates {
		select {
		case <-ctx.Done():
			return
		default:
		}

		checkCtx, cancel := context.WithTimeout(ctx, healthRequestTimeout)
		result, err := a.validateAndRecordToken(checkCtx, item)
		cancel()
		a.recordTokenMaintenanceHistory(historyEventHealthCheck, item, result, err)

		message := result.Message
		if err != nil {
			message = err.Error()
		}
		nextCheck := nextHealthCheckAt(result, err)
		_ = manager.RecordHealthCheck(item.ID, result.OK, result.Status, message, nextCheck)

		level := logs.LevelInfo
		if err != nil || !result.OK {
			level = logs.LevelWarn
		}
		a.logs.Add(logs.Entry{
			Level:     level,
			Status:    result.Status,
			Duration:  result.Duration,
			TokenName: item.Name,
			Message:   "health check completed",
		})
	}
}

func nextHealthCheckAt(result proxy.ValidationResult, err error) *time.Time {
	if result.Status == http.StatusTooManyRequests {
		return validationCooldownUntil(result)
	}
	now := time.Now()
	wait := activeHealthInterval
	if err != nil || !result.OK {
		wait = failedHealthRetryWait
	}
	next := now.Add(wait)
	return &next
}

func isCodexToken(item token.Token) bool {
	return token.NormalizeProvider(item.Provider) == token.ProviderOpenAI &&
		item.CredentialType == token.CredentialTypeCodexAuthJSON
}

func isClaudeOAuthToken(item token.Token) bool {
	return token.NormalizeProvider(item.Provider) == token.ProviderAnthropic &&
		item.CredentialType == token.CredentialTypeClaudeOAuth
}

func isRefreshableAuthToken(item token.Token) bool {
	return isCodexToken(item) || isClaudeOAuthToken(item)
}

func validationCooldownUntil(result proxy.ValidationResult) *time.Time {
	now := time.Now()
	if result.Usage != nil && result.Usage.PrimaryResetAt > now.Unix() {
		until := time.Unix(result.Usage.PrimaryResetAt, 0)
		return &until
	}
	until := now.Add(5 * time.Minute)
	return &until
}

func (a *appServer) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.mu.Lock()
		cfg := a.cfg
		a.mu.Unlock()
		writeJSON(w, http.StatusOK, cfg)
	case http.MethodPut:
		var cfg config.Config
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		saved, err := a.saveConfig(cfg)
		if err != nil {
			if isConfigValidationError(err) {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, saved)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *appServer) saveConfig(cfg config.Config) (config.Config, error) {
	cfg = config.Normalize(cfg)
	if err := a.validateConfigForSave(cfg); err != nil {
		return config.Config{}, err
	}

	a.mu.Lock()
	oldCfg := a.cfg
	shouldRestartProxy := a.proxyServer != nil && proxyConfigChanged(oldCfg, cfg)
	shouldRestartControl := a.control != nil && controlConfigChanged(oldCfg, cfg)
	a.cfg = cfg
	a.mu.Unlock()
	if a.tokens != nil {
		a.tokens.SetThreshold(cfg.SwitchThreshold)
	}
	if a.history != nil {
		if err := a.history.SetRetentionDays(cfg.HistoryRetentionDays); err != nil {
			return config.Config{}, err
		}
	}
	if a.taskAutomation != nil {
		a.taskAutomation.UpdateConfig(cfg)
	}

	if shouldRestartProxy {
		if err := a.restartProxy(); err != nil {
			return config.Config{}, err
		}
	}
	if shouldRestartControl {
		if err := a.restartControl(); err != nil {
			return config.Config{}, err
		}
	}
	if a.configStore != nil {
		if err := a.configStore.Save(cfg); err != nil {
			return config.Config{}, err
		}
	}
	a.syncPremProxyAfterConfigChange(oldCfg, cfg)

	a.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: "configuration updated"})
	return cfg, nil
}

func (a *appServer) validateConfigForSave(cfg config.Config) error {
	if err := validateConfiguredPorts(cfg); err != nil {
		return err
	}
	if _, err := proxy.NewService(cfg, a.tokens, a.logs, a.history); err != nil {
		return err
	}
	if _, err := proxy.NewValidator(cfg); err != nil {
		return err
	}
	return nil
}

func (a *appServer) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, logResponses(a.logs.List()))
}

func (a *appServer) handleOpenRouterModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	refresh := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("refresh")), "true") ||
		strings.TrimSpace(r.URL.Query().Get("refresh")) == "1"
	result, err := a.openRouterModels(r.Context(), refresh)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *appServer) handleOpenRouterChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)
	var req openRouterChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	result, err := a.openRouterChat(r.Context(), req)
	if err != nil {
		var requestErr openRouterRequestError
		if errors.As(err, &requestErr) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *appServer) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	a.mu.Lock()
	recorder := a.history
	a.mu.Unlock()
	if recorder == nil {
		writeJSON(w, http.StatusOK, []historyResponse{})
		return
	}

	limit := defaultHistoryLimit
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	filter := history.Filter{
		Provider: strings.TrimSpace(r.URL.Query().Get("provider")),
		Client:   strings.TrimSpace(r.URL.Query().Get("client")),
		Level:    strings.TrimSpace(r.URL.Query().Get("level")),
		Status:   strings.TrimSpace(r.URL.Query().Get("status")),
		Model:    strings.TrimSpace(r.URL.Query().Get("model")),
		Token:    strings.TrimSpace(r.URL.Query().Get("token")),
		Search:   strings.TrimSpace(r.URL.Query().Get("search")),
		Limit:    limit,
	}
	writeJSON(w, http.StatusOK, historyResponses(recorder.List(filter)))
}

func (a *appServer) handleHistorySummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	a.mu.Lock()
	recorder := a.history
	a.mu.Unlock()
	if recorder == nil {
		writeJSON(w, http.StatusOK, history.Summary{})
		return
	}

	days := 14
	if raw := strings.TrimSpace(r.URL.Query().Get("days")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			days = parsed
		}
	}
	filter := history.Filter{
		Provider: strings.TrimSpace(r.URL.Query().Get("provider")),
		Client:   strings.TrimSpace(r.URL.Query().Get("client")),
		Level:    strings.TrimSpace(r.URL.Query().Get("level")),
		Status:   strings.TrimSpace(r.URL.Query().Get("status")),
		Model:    strings.TrimSpace(r.URL.Query().Get("model")),
		Token:    strings.TrimSpace(r.URL.Query().Get("token")),
		Search:   strings.TrimSpace(r.URL.Query().Get("search")),
	}
	writeJSON(w, http.StatusOK, recorder.Summary(filter, days))
}

func (a *appServer) handleHistoryClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	a.mu.Lock()
	recorder := a.history
	a.mu.Unlock()
	if recorder == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err := recorder.ClearRequestHistory(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: "request history cleared"})
	w.WriteHeader(http.StatusNoContent)
}

func (a *appServer) handleBillingUsage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	a.mu.Lock()
	recorder := a.history
	a.mu.Unlock()
	if recorder == nil {
		writeJSON(w, http.StatusOK, []history.DailyUsage{})
		return
	}
	date := strings.TrimSpace(r.URL.Query().Get("date"))
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	writeJSON(w, http.StatusOK, recorder.DailyUsage(date))
}

func (a *appServer) handleBillingSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	a.mu.Lock()
	recorder := a.history
	a.mu.Unlock()
	if recorder == nil {
		writeJSON(w, http.StatusOK, history.BillingSummary{})
		return
	}
	days := 30
	if raw := strings.TrimSpace(r.URL.Query().Get("days")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			days = parsed
		}
	}
	writeJSON(w, http.StatusOK, recorder.BillingSummary(days))
}

func (a *appServer) handleBillingDates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	a.mu.Lock()
	recorder := a.history
	a.mu.Unlock()
	if recorder == nil {
		writeJSON(w, http.StatusOK, []string{})
		return
	}
	limit := 30
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	writeJSON(w, http.StatusOK, recorder.DailyUsageDates(limit))
}

func (a *appServer) handleBillingClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	a.mu.Lock()
	recorder := a.history
	a.mu.Unlock()
	if recorder == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err := recorder.ClearDailyUsage(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: "billing daily usage cleared"})
	w.WriteHeader(http.StatusNoContent)
}

func (a *appServer) handleProxyStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"running": a.proxyServer != nil,
		"port":    a.cfg.ProxyPort,
	})
}

func (a *appServer) handleActiveProxyRequests(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, activeRequestResponses(a.activeProxyRequests()))
}

func (a *appServer) handleProxyStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err := a.startProxy(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"running": true})
}

func (a *appServer) handleProxyStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err := a.stopProxy(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"running": false})
}

func (a *appServer) handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	info, err := checkForUpdates(r.Context(), http.DefaultClient, a.includePrereleaseUpdates())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (a *appServer) handleUpdateDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req updateDownloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	status, err := a.updateManager().Start(context.Background(), http.DefaultClient, req)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, status)
}

func (a *appServer) handleUpdateDownloadStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, a.updateManager().Status())
}

func (a *appServer) handleUpdateInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	status, err := a.updateManager().Install()
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (a *appServer) handleAppInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, currentAppInfo())
}

func (a *appServer) handleDataDirectory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, a.dataDirectoryInfo())
}

func (a *appServer) handleCodexConfigure(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	result, err := a.configureCodex()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *appServer) handleCodexSub2APIConfigure(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	result, err := a.configureCodexSub2API()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *appServer) handleCodexNewAPIConfigure(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	result, err := a.configureCodexNewAPI()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *appServer) handleCodexAnyRouterConfigure(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	result, err := a.configureCodexAnyRouter()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *appServer) handleCodexZoConfigure(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	result, err := a.configureCodexZo()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *appServer) handleCodexPremConfigure(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	result, err := a.configureCodexPrem()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *appServer) handleCodexRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	result, err := a.restoreCodexConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *appServer) handleMimoClaudeConfigure(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	result, err := a.configureMimoClaude()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *appServer) handleMimoClaudeRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	result, err := a.restoreMimoClaudeConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *appServer) handleDeepSeekClaudeConfigure(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	result, err := a.configureDeepSeekClaude()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *appServer) handleDeepSeekClaudeRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	result, err := a.restoreDeepSeekClaudeConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *appServer) handleKimiClaudeConfigure(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	result, err := a.configureKimiClaude()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *appServer) handleKimiClaudeRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	result, err := a.restoreKimiClaudeConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *appServer) handleZhipuClaudeConfigure(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	result, err := a.configureZhipuClaude()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *appServer) handleZhipuClaudeRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	result, err := a.restoreZhipuClaudeConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *appServer) handleAnyRouterClaudeConfigure(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	result, err := a.configureAnyRouterClaude()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *appServer) handleZoClaudeConfigure(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	result, err := a.configureZoClaude()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *appServer) handleClaudeModelsConfigure(w http.ResponseWriter, r *http.Request) {
	a.handleClaudeModelsConfigureWith(w, r, a.configureClaudeModels)
}

func (a *appServer) handleClaudeDesktopModelsConfigure(w http.ResponseWriter, r *http.Request) {
	a.handleClaudeModelsConfigureWith(w, r, a.configureClaudeDesktopModels)
}

func (a *appServer) handleClaudeDesktopRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	result, err := a.restoreClaudeDesktopConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *appServer) handleClaudeModelsConfigureWith(w http.ResponseWriter, r *http.Request, configure func(claudeModelsConfigureRequest) (mimoConfigureResult, error)) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var req claudeModelsConfigureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	result, err := configure(req)
	if err != nil {
		var selectionErr *claudeModelSelectionError
		if errors.As(err, &selectionErr) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *appServer) handleDeepSeekTUIConfigure(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	result, err := a.configureDeepSeekTUI()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *appServer) handleDeepSeekTUIRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	result, err := a.restoreDeepSeekTUIConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *appServer) handleGeminiConfigure(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	result, err := a.configureGemini()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *appServer) handleGeminiRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	result, err := a.restoreGeminiConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *appServer) handleOpenCodeConfigure(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	result, err := a.configureOpenCode()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *appServer) handleOpenCodeRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	result, err := a.restoreOpenCodeConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *appServer) handlePiConfigure(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	result, err := a.configurePi()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *appServer) handlePiRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	result, err := a.restorePiConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *appServer) startProxy() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.proxyServer != nil {
		return nil
	}
	if err := validateConfiguredPorts(a.cfg); err != nil {
		return err
	}

	svc, err := proxy.NewService(a.cfg, a.tokens, a.logs, a.history)
	if err != nil {
		return err
	}
	svc.SetTokenRefresher(a.refreshAuthTokenIfNeeded)
	if a.taskAutomation != nil {
		svc.SetActivityObserver(a.taskAutomation)
	}

	addr := fmt.Sprintf("127.0.0.1:%d", a.cfg.ProxyPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("start proxy listener on %s: %w", addr, err)
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           svc,
		ReadHeaderTimeout: 30 * time.Second,
	}
	a.proxyServer = server
	a.proxyService = svc

	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.logs.Add(logs.Entry{Level: logs.LevelError, Message: fmt.Sprintf("proxy stopped: %v", err)})
			log.Printf("proxy stopped: %v", err)
		}
		a.mu.Lock()
		if a.proxyServer == server {
			a.proxyServer = nil
			a.proxyService = nil
		}
		a.mu.Unlock()
	}()

	a.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: fmt.Sprintf("proxy started on port %d", a.cfg.ProxyPort)})
	return nil
}

func (a *appServer) startControl() error {
	a.mu.Lock()
	if a.control != nil {
		a.mu.Unlock()
		return nil
	}
	cfg := a.cfg
	a.mu.Unlock()

	server, listener, err := a.newControlServer(cfg)
	if err != nil {
		return err
	}

	a.mu.Lock()
	if a.control != nil {
		a.mu.Unlock()
		_ = listener.Close()
		return nil
	}
	a.control = server
	a.mu.Unlock()

	a.serveControl(server, listener)
	a.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: fmt.Sprintf("control API started on port %d", cfg.ControlPort)})
	log.Printf("OmniProxy control API listening on http://%s", server.Addr)
	return nil
}

func (a *appServer) newControlServer(cfg config.Config) (*http.Server, net.Listener, error) {
	if err := validateConfiguredPorts(cfg); err != nil {
		return nil, nil, err
	}

	addr := fmt.Sprintf("127.0.0.1:%d", cfg.ControlPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, nil, fmt.Errorf("start control API listener on %s: %w", addr, err)
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           withCORS(a.routes()),
		ReadHeaderTimeout: 5 * time.Second,
	}
	return server, listener, nil
}

func (a *appServer) serveControl(server *http.Server, listener net.Listener) {
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.logs.Add(logs.Entry{Level: logs.LevelError, Message: fmt.Sprintf("control API stopped: %v", err)})
			log.Printf("control API stopped: %v", err)
		}
		a.mu.Lock()
		if a.control == server {
			a.control = nil
		}
		a.mu.Unlock()
	}()
}

func (a *appServer) stopProxy() error {
	a.mu.Lock()
	server := a.proxyServer
	a.proxyServer = nil
	a.proxyService = nil
	a.mu.Unlock()
	if a.taskAutomation != nil {
		a.taskAutomation.Stop()
	}

	if server == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := server.Shutdown(ctx)
	a.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: "proxy stopped"})
	return err
}

func (a *appServer) activeProxyRequests() []proxy.ActiveRequest {
	a.mu.Lock()
	svc := a.proxyService
	a.mu.Unlock()
	if svc == nil {
		return []proxy.ActiveRequest{}
	}
	return svc.ActiveRequests()
}

func (a *appServer) stopControl() error {
	a.mu.Lock()
	server := a.control
	a.control = nil
	a.mu.Unlock()

	if server == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := server.Shutdown(ctx)
	a.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: "control API stopped"})
	return err
}

func (a *appServer) restartProxy() error {
	if err := a.stopProxy(); err != nil {
		return err
	}
	return a.startProxy()
}

func (a *appServer) restartControl() error {
	a.mu.Lock()
	old := a.control
	cfg := a.cfg
	a.mu.Unlock()

	if old == nil {
		return a.startControl()
	}

	server, listener, err := a.newControlServer(cfg)
	if err != nil {
		return err
	}

	a.mu.Lock()
	old = a.control
	a.control = server
	a.mu.Unlock()

	a.serveControl(server, listener)
	a.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: fmt.Sprintf("control API started on port %d", cfg.ControlPort)})
	log.Printf("OmniProxy control API listening on http://%s", server.Addr)

	if old != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := old.Shutdown(ctx); err != nil {
				a.logs.Add(logs.Entry{Level: logs.LevelWarn, Message: fmt.Sprintf("control API restart shutdown failed: %v", err)})
				return
			}
			a.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: "control API stopped"})
		}()
	}
	return nil
}

func proxyConfigChanged(oldCfg config.Config, nextCfg config.Config) bool {
	return proxy.ProxyConfigChanged(oldCfg, nextCfg)
}

func controlConfigChanged(oldCfg config.Config, nextCfg config.Config) bool {
	return oldCfg.ControlPort != nextCfg.ControlPort
}

func maskedTokenValue(item token.Token) string {
	if strings.TrimSpace(item.TokenValue) == "" {
		return ""
	}
	if item.CredentialType == token.CredentialTypeCodexAuthJSON || item.CredentialType == token.CredentialTypeClaudeOAuth {
		return "auth.json"
	}
	value := strings.TrimSpace(item.TokenValue)
	if len(value) <= 12 {
		return value[:3] + "..."
	}
	return value[:7] + "..." + value[len(value)-4:]
}

func validateConfiguredPorts(cfg config.Config) error {
	if err := validateTCPPort("proxy port", cfg.ProxyPort); err != nil {
		return err
	}
	if err := validateTCPPort("control port", cfg.ControlPort); err != nil {
		return err
	}
	if cfg.ProxyPort == cfg.ControlPort {
		return errors.New("proxy port and control port must be different")
	}
	return nil
}

func validateTCPPort(name string, port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("%s must be between 1 and 65535", name)
	}
	return nil
}

func isConfigValidationError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "port") ||
		strings.Contains(message, "url") ||
		strings.Contains(message, "invalid")
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin != "" {
			if !isAllowedControlOrigin(origin) {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization,"+controlTokenHeader)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func withControlTokenAuth(expected string, next http.Handler) http.Handler {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions || isControlTokenEndpoint(r) {
			next.ServeHTTP(w, r)
			return
		}
		if !validControlToken(r, expected) {
			w.Header().Set("Cache-Control", "no-store")
			writeError(w, http.StatusUnauthorized, "control token required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isControlTokenEndpoint(r *http.Request) bool {
	return r.URL != nil && r.URL.Path == "/api/control-token"
}

func validControlToken(r *http.Request, expected string) bool {
	value := strings.TrimSpace(r.Header.Get(controlTokenHeader))
	if value == "" {
		auth := strings.TrimSpace(r.Header.Get("Authorization"))
		const bearerPrefix = "bearer "
		if len(auth) > len(bearerPrefix) && strings.EqualFold(auth[:len(bearerPrefix)], bearerPrefix) {
			value = strings.TrimSpace(auth[len(bearerPrefix):])
		}
	}
	if value == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(value), []byte(expected)) == 1
}

func isAllowedControlOrigin(origin string) bool {
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	scheme := strings.ToLower(parsed.Scheme)
	if scheme == "wails" && host == "wails.localhost" {
		return true
	}
	if host == "wails.localhost" {
		return true
	}
	if scheme != "http" && scheme != "https" {
		return false
	}
	return host == "127.0.0.1" || host == "localhost" || host == "::1"
}

func isTrustedControlTokenOrigin(origin string) bool {
	parsed, err := url.Parse(strings.TrimSpace(origin))
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	scheme := strings.ToLower(parsed.Scheme)
	return host == "wails.localhost" && scheme == "wails"
}

func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, token.ErrDuplicateName):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, token.ErrTokenNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		writeError(w, http.StatusBadRequest, err.Error())
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func intToString(value int) string {
	return fmt.Sprintf("%d", value)
}
