package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"OmniProxyBackend/internal/config"
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
	mu          sync.Mutex
	dataDir     string
	cfg         config.Config
	configStore *config.Store
	tokens      *token.Manager
	logs        *logs.Recorder
	proxyServer *http.Server
	control     *http.Server
}

func main() {
	server, err := newAppServer()
	if err != nil {
		log.Fatalf("initialise app: %v", err)
	}

	desktop := NewDesktopApp(server)

	err = wails.Run(&options.App{
		Title:     "OmniProxy",
		Width:     1280,
		Height:    860,
		MinWidth:  1040,
		MinHeight: 720,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 238, G: 242, B: 247, A: 1},
		OnStartup:        desktop.startup,
		OnShutdown:       desktop.shutdown,
		Bind: []interface{}{
			desktop,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}

func newAppServer() (*appServer, error) {
	server := &appServer{
		cfg:  config.Default(),
		logs: logs.NewRecorder(500),
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

	tokenStore := storage.NewJSONStore[[]token.Token](filepath.Join(dataDir, "tokens.json"))
	tokenManager, err := token.NewManager(tokenStore, cfg.SwitchThreshold)
	if err != nil {
		return fmt.Errorf("load tokens: %w", err)
	}

	a.mu.Lock()
	a.dataDir = dataDir
	a.cfg = cfg
	a.configStore = cfgStore
	a.tokens = tokenManager
	a.mu.Unlock()
	return nil
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
	mux.HandleFunc("/api/tokens", a.handleTokens)
	mux.HandleFunc("/api/tokens/", a.handleTokenByID)
	mux.HandleFunc("/api/config", a.handleConfig)
	mux.HandleFunc("/api/logs", a.handleLogs)
	mux.HandleFunc("/api/proxy/status", a.handleProxyStatus)
	mux.HandleFunc("/api/proxy/start", a.handleProxyStart)
	mux.HandleFunc("/api/proxy/stop", a.handleProxyStop)
	mux.HandleFunc("/api/codex/configure", a.handleCodexConfigure)
	mux.HandleFunc("/api/codex/restore", a.handleCodexRestore)
	mux.HandleFunc("/api/mimo/claude/configure", a.handleMimoClaudeConfigure)
	mux.HandleFunc("/api/mimo/claude/restore", a.handleMimoClaudeRestore)
	mux.HandleFunc("/api/deepseek/claude/configure", a.handleDeepSeekClaudeConfigure)
	mux.HandleFunc("/api/deepseek/claude/restore", a.handleDeepSeekClaudeRestore)
	mux.HandleFunc("/api/kimi/claude/configure", a.handleKimiClaudeConfigure)
	mux.HandleFunc("/api/kimi/claude/restore", a.handleKimiClaudeRestore)
	return mux
}

func (a *appServer) handleTokens(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, a.tokens.List())
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
			if _, err := a.validateAndRecordToken(r.Context(), item); err != nil {
				a.logs.Add(logs.Entry{Level: logs.LevelWarn, TokenName: item.Name, Message: fmt.Sprintf("codex usage refresh failed after add: %v", err)})
			}
			if updated, err := a.tokens.Get(item.ID); err == nil {
				item = updated
			}
		}
		writeJSON(w, http.StatusCreated, item)
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
		writeJSON(w, http.StatusOK, item)
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
		writeJSON(w, http.StatusBadGateway, result)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *appServer) validateAndRecordToken(ctx context.Context, selected token.Token) (proxy.ValidationResult, error) {
	a.mu.Lock()
	cfg := a.cfg
	a.mu.Unlock()

	validator, err := proxy.NewValidator(cfg)
	if err != nil {
		return proxy.ValidationResult{}, err
	}

	result, err := validator.Validate(ctx, selected)
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
		_ = a.tokens.MarkExhausted(selected.ID, "validation returned 429")
	}
	return result, err
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
		if _, err := a.validateAndRecordToken(ctx, item); err != nil {
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

func isCodexToken(item token.Token) bool {
	return token.NormalizeProvider(item.Provider) == token.ProviderOpenAI &&
		item.CredentialType == token.CredentialTypeCodexAuthJSON
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
		cfg = config.Normalize(cfg)
		if err := a.configStore.Save(cfg); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		a.mu.Lock()
		oldCfg := a.cfg
		shouldRestartProxy := a.proxyServer != nil && proxyConfigChanged(oldCfg, cfg)
		a.cfg = cfg
		a.mu.Unlock()
		a.tokens.SetThreshold(cfg.SwitchThreshold)

		if shouldRestartProxy {
			if err := a.restartProxy(); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}

		a.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: "configuration updated"})
		writeJSON(w, http.StatusOK, cfg)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *appServer) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, a.logs.List())
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

	svc, err := proxy.NewService(a.cfg, a.tokens, a.logs)
	if err != nil {
		return err
	}

	server := &http.Server{
		Addr:              fmt.Sprintf("127.0.0.1:%d", a.cfg.ProxyPort),
		Handler:           svc,
		ReadHeaderTimeout: 30 * time.Second,
	}
	a.proxyServer = server

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
	defer a.mu.Unlock()

	if a.control != nil {
		return nil
	}

	server := &http.Server{
		Addr:              fmt.Sprintf("127.0.0.1:%d", a.cfg.ControlPort),
		Handler:           withCORS(a.routes()),
		ReadHeaderTimeout: 5 * time.Second,
	}
	a.control = server

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.logs.Add(logs.Entry{Level: logs.LevelError, Message: fmt.Sprintf("control API stopped: %v", err)})
			log.Printf("control API stopped: %v", err)
		}
		a.mu.Lock()
		if a.control == server {
			a.control = nil
		}
		a.mu.Unlock()
	}()

	a.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: fmt.Sprintf("control API started on port %d", a.cfg.ControlPort)})
	log.Printf("OmniProxy control API listening on http://%s", server.Addr)
	return nil
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
		oldCfg.CodexBaseURL != nextCfg.CodexBaseURL ||
		oldCfg.MaxRetries != nextCfg.MaxRetries
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
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
