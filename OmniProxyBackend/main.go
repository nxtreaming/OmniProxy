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
	"OmniProxyBackend/internal/token"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

type appServer struct {
	mu             sync.Mutex
	codexRefreshMu sync.Mutex
	dataDir        string
	cfg            config.Config
	configStore    *config.Store
	tokens         *token.Manager
	logs           *logs.Recorder
	history        *history.Recorder
	proxyServer    *http.Server
	control        *http.Server
	controlToken   string
	healthStop     context.CancelFunc
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
		MinWidth:          1040,
		MinHeight:         720,
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
	server := &appServer{
		cfg:          config.Default(),
		logs:         logs.NewRecorder(500),
		controlToken: controlToken,
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

	a.mu.Lock()
	a.dataDir = dataDir
	a.cfg = cfg
	a.configStore = cfgStore
	a.tokens = tokenManager
	a.history = historyRecorder
	a.mu.Unlock()
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
	mux.HandleFunc("/api/tokens/", a.handleTokenByID)
	mux.HandleFunc("/api/config", a.handleConfig)
	mux.HandleFunc("/api/logs", a.handleLogs)
	mux.HandleFunc("/api/history", a.handleHistory)
	mux.HandleFunc("/api/proxy/status", a.handleProxyStatus)
	mux.HandleFunc("/api/proxy/start", a.handleProxyStart)
	mux.HandleFunc("/api/proxy/stop", a.handleProxyStop)
	mux.HandleFunc("/api/app/info", a.handleAppInfo)
	mux.HandleFunc("/api/update/check", a.handleUpdateCheck)
	mux.HandleFunc("/api/data-directory", a.handleDataDirectory)
	mux.HandleFunc("/api/codex/configure", a.handleCodexConfigure)
	mux.HandleFunc("/api/codex/restore", a.handleCodexRestore)
	mux.HandleFunc("/api/mimo/claude/configure", a.handleMimoClaudeConfigure)
	mux.HandleFunc("/api/mimo/claude/restore", a.handleMimoClaudeRestore)
	mux.HandleFunc("/api/deepseek/claude/configure", a.handleDeepSeekClaudeConfigure)
	mux.HandleFunc("/api/deepseek/claude/restore", a.handleDeepSeekClaudeRestore)
	mux.HandleFunc("/api/kimi/claude/configure", a.handleKimiClaudeConfigure)
	mux.HandleFunc("/api/kimi/claude/restore", a.handleKimiClaudeRestore)
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
		item, err := a.tokens.Add(req)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		a.logs.Add(logs.Entry{Level: logs.LevelInfo, TokenName: item.Name, Message: "token added"})
		if isCodexToken(item) {
			result, err := a.validateAndRecordToken(r.Context(), item)
			a.recordTokenMaintenanceHistory(historyEventCodexRefreshAdd, item, result, err)
			if err != nil {
				a.logs.Add(logs.Entry{Level: logs.LevelWarn, TokenName: item.Name, Message: fmt.Sprintf("codex usage refresh failed after add: %v", err)})
			}
			if updated, err := a.tokens.Get(item.ID); err == nil {
				item = updated
			}
		}
		writeJSON(w, http.StatusCreated, tokenResponseFor(item))
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
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
		item, err := a.tokens.Update(id, req)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		a.logs.Add(logs.Entry{Level: logs.LevelInfo, TokenName: item.Name, Message: "token updated"})
		writeJSON(w, http.StatusOK, tokenResponseFor(item))
	case http.MethodDelete:
		if err := a.tokens.Delete(id); err != nil {
			writeDomainError(w, err)
			return
		}
		a.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: "token deleted"})
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *appServer) handleTokenValidate(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	selected, err := a.tokens.Get(id)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	result, err := a.validateAndRecordToken(r.Context(), selected)
	a.recordTokenMaintenanceHistory(historyEventManualValidation, selected, result, err)

	level := logs.LevelInfo
	if err != nil || !result.OK {
		level = logs.LevelWarn
	}
	a.logs.Add(logs.Entry{
		Level:     level,
		Status:    result.Status,
		Duration:  result.Duration,
		TokenName: selected.Name,
		Message:   "token validation completed",
	})

	if err != nil {
		writeJSON(w, http.StatusBadGateway, validationResponseFor(result))
		return
	}
	writeJSON(w, http.StatusOK, validationResponseFor(result))
}

func (a *appServer) validateAndRecordToken(ctx context.Context, selected token.Token) (proxy.ValidationResult, error) {
	a.mu.Lock()
	cfg := a.cfg
	a.mu.Unlock()

	refreshedSelected, _, refreshErr := a.refreshCodexTokenIfNeeded(ctx, selected, false)
	if refreshErr != nil {
		_ = a.tokens.MarkInvalid(selected.ID, fmt.Sprintf("codex token refresh failed: %v", refreshErr))
		return proxy.ValidationResult{}, refreshErr
	}
	selected = refreshedSelected

	validator, err := proxy.NewValidator(cfg)
	if err != nil {
		return proxy.ValidationResult{}, err
	}

	result, err := validator.Validate(ctx, selected)
	if err == nil && isCodexToken(selected) && (result.Status == http.StatusUnauthorized || result.Status == http.StatusForbidden) {
		refreshedSelected, refreshed, refreshErr := a.refreshCodexTokenIfNeeded(ctx, selected, true)
		if refreshErr != nil {
			_ = a.tokens.MarkInvalid(selected.ID, fmt.Sprintf("codex token refresh failed: %v", refreshErr))
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

func (a *appServer) refreshCodexTokenIfNeeded(ctx context.Context, selected token.Token, force bool) (token.Token, bool, error) {
	if !isCodexToken(selected) {
		return selected, false, nil
	}

	a.codexRefreshMu.Lock()
	defer a.codexRefreshMu.Unlock()

	if latest, err := a.tokens.Get(selected.ID); err == nil {
		selected = latest
	}

	client := &http.Client{Timeout: healthRequestTimeout}
	updatedValue, refreshed, err := proxy.RefreshCodexAuthJSON(ctx, client, selected.TokenValue, force, time.Now())
	if err != nil || !refreshed {
		return selected, refreshed, err
	}

	updated, err := a.tokens.UpdateTokenValue(selected.ID, updatedValue)
	if err != nil {
		return selected, true, err
	}
	if a.logs != nil {
		a.logs.Add(logs.Entry{Level: logs.LevelInfo, TokenName: updated.Name, Message: "codex access token refreshed"})
	}
	return updated, true, nil
}

func (a *appServer) refreshCodexUsageOnStartup(ctx context.Context) {
	items := a.tokens.List()
	total := 0
	failed := 0
	for _, item := range items {
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
		Level:    strings.TrimSpace(r.URL.Query().Get("level")),
		Status:   strings.TrimSpace(r.URL.Query().Get("status")),
		Model:    strings.TrimSpace(r.URL.Query().Get("model")),
		Token:    strings.TrimSpace(r.URL.Query().Get("token")),
		Search:   strings.TrimSpace(r.URL.Query().Get("search")),
		Limit:    limit,
	}
	writeJSON(w, http.StatusOK, historyResponses(recorder.List(filter)))
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
	info, err := checkForUpdates(r.Context(), http.DefaultClient)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, info)
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
	svc.SetTokenRefresher(a.refreshCodexTokenIfNeeded)

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

	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.logs.Add(logs.Entry{Level: logs.LevelError, Message: fmt.Sprintf("proxy stopped: %v", err)})
			log.Printf("proxy stopped: %v", err)
		}
		a.mu.Lock()
		if a.proxyServer == server {
			a.proxyServer = nil
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
	a.mu.Unlock()

	if server == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := server.Shutdown(ctx)
	a.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: "proxy stopped"})
	return err
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
	return oldCfg.ProxyPort != nextCfg.ProxyPort ||
		oldCfg.SchedulingMode != nextCfg.SchedulingMode ||
		oldCfg.WebSocketMode != nextCfg.WebSocketMode ||
		oldCfg.UpstreamBaseURL != nextCfg.UpstreamBaseURL ||
		oldCfg.OpenAIBaseURL != nextCfg.OpenAIBaseURL ||
		oldCfg.AnthropicBaseURL != nextCfg.AnthropicBaseURL ||
		oldCfg.DeepSeekBaseURL != nextCfg.DeepSeekBaseURL ||
		oldCfg.DeepSeekAnthropicBaseURL != nextCfg.DeepSeekAnthropicBaseURL ||
		oldCfg.KimiBaseURL != nextCfg.KimiBaseURL ||
		oldCfg.XiaomiBaseURL != nextCfg.XiaomiBaseURL ||
		oldCfg.XiaomiAPIBaseURL != nextCfg.XiaomiAPIBaseURL ||
		oldCfg.XiaomiAPIAnthropicBaseURL != nextCfg.XiaomiAPIAnthropicBaseURL ||
		oldCfg.XiaomiTokenPlanBaseURL != nextCfg.XiaomiTokenPlanBaseURL ||
		oldCfg.XiaomiTokenPlanAnthropicBaseURL != nextCfg.XiaomiTokenPlanAnthropicBaseURL ||
		oldCfg.XiaomiCredentialPriority != nextCfg.XiaomiCredentialPriority ||
		oldCfg.CodexBaseURL != nextCfg.CodexBaseURL ||
		oldCfg.MaxRetries != nextCfg.MaxRetries
}

func controlConfigChanged(oldCfg config.Config, nextCfg config.Config) bool {
	return oldCfg.ControlPort != nextCfg.ControlPort
}

func maskedTokenValue(item token.Token) string {
	if strings.TrimSpace(item.TokenValue) == "" {
		return ""
	}
	if item.CredentialType == token.CredentialTypeCodexAuthJSON {
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
