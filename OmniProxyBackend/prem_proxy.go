package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"OmniProxyBackend/internal/config"
	"OmniProxyBackend/internal/logs"
	"OmniProxyBackend/internal/token"
)

const (
	premPCCIProxyPackage = "@premai/api-sdk"
	premPCCIProxyCommand = "confidential-proxy"
	premPCCIProxyURL     = "https://gateway.prem.io"
	premPCCIEnclaveURL   = "https://conf-engine.prem.io"
	premPCCIStartupWait  = 60 * time.Second
)

type premProxyTarget struct {
	BaseURL  string
	Host     string
	Port     string
	Address  string
	Loopback bool
}

type premProxyManager struct {
	mu          sync.Mutex
	logs        *logs.Recorder
	cmd         *exec.Cmd
	cancel      context.CancelFunc
	startedAddr string
}

func newPremProxyManager(recorder *logs.Recorder) *premProxyManager {
	return &premProxyManager{logs: recorder}
}

func (a *appServer) ensurePremProxyForConfiguredAccounts(reason string) {
	a.mu.Lock()
	cfg := a.cfg
	manager := a.premProxy
	a.mu.Unlock()

	apiKey := a.firstEnabledPremAPIKey()
	if manager == nil || !cfg.PremAutoStartPCCIProxy || apiKey == "" {
		return
	}
	manager.EnsureAsync(cfg.PremBaseURL, apiKey, reason)
}

func (a *appServer) syncPremProxyAfterConfigChange(oldCfg config.Config, nextCfg config.Config) {
	a.mu.Lock()
	manager := a.premProxy
	a.mu.Unlock()
	if manager == nil {
		return
	}

	if !nextCfg.PremAutoStartPCCIProxy || oldCfg.PremBaseURL != nextCfg.PremBaseURL {
		manager.Stop()
	}
	if nextCfg.PremAutoStartPCCIProxy {
		a.ensurePremProxyForConfiguredAccounts("configuration updated")
	}
}

func (a *appServer) ensurePremProxyForToken(item token.Token, reason string) {
	if item.Provider != token.ProviderPrem || item.Disabled {
		return
	}
	a.ensurePremProxyForConfiguredAccounts(reason)
}

func (a *appServer) stopPremProxyIfUnused() {
	if a.hasEnabledPremToken() {
		return
	}
	a.stopPremProxy()
}

func (a *appServer) stopPremProxy() {
	a.mu.Lock()
	manager := a.premProxy
	a.mu.Unlock()
	if manager != nil {
		manager.Stop()
	}
}

func (a *appServer) hasEnabledPremToken() bool {
	return a.firstEnabledPremAPIKey() != ""
}

func (a *appServer) firstEnabledPremAPIKey() string {
	if a.tokens == nil {
		return ""
	}
	for _, item := range a.tokens.List() {
		if item.Provider == token.ProviderPrem && !item.Disabled {
			return strings.TrimSpace(item.TokenValue)
		}
	}
	return ""
}

func (m *premProxyManager) EnsureAsync(baseURL string, apiKey string, reason string) {
	go func() {
		if err := m.Ensure(baseURL, apiKey); err != nil {
			m.record(logs.LevelWarn, fmt.Sprintf("Prem confidential-proxy auto-start failed (%s): %v", reason, err))
		}
	}()
}

func (m *premProxyManager) Ensure(baseURL string, apiKey string) error {
	target, err := premProxyTargetFromBaseURL(baseURL)
	if err != nil {
		return err
	}
	if !target.Loopback {
		m.record(logs.LevelInfo, fmt.Sprintf("Prem confidential-proxy auto-start skipped for non-local Base URL %s", target.BaseURL))
		return nil
	}
	if premProxyTCPListening(target) {
		m.record(logs.LevelInfo, fmt.Sprintf("Prem confidential-proxy already listening on %s", target.Address))
		return nil
	}

	m.mu.Lock()
	if m.isRunningLocked() {
		if m.startedAddr == target.Address {
			m.mu.Unlock()
			return nil
		}
		runningAddr := m.startedAddr
		m.mu.Unlock()
		return fmt.Errorf("managed pcci-proxy is already running on %s", runningAddr)
	}
	if premProxyTCPListening(target) {
		m.mu.Unlock()
		m.record(logs.LevelInfo, fmt.Sprintf("Prem confidential-proxy already listening on %s", target.Address))
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd, err := newPremPCCIProxyCommand(ctx, target.Port, apiKey)
	if err != nil {
		cancel()
		m.mu.Unlock()
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		m.mu.Unlock()
		return fmt.Errorf("open pcci-proxy stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		m.mu.Unlock()
		return fmt.Errorf("open pcci-proxy stderr: %w", err)
	}
	if err := cmd.Start(); err != nil {
		cancel()
		m.mu.Unlock()
		return fmt.Errorf("start pcci-proxy: %w", err)
	}

	m.cmd = cmd
	m.cancel = cancel
	m.startedAddr = target.Address
	m.mu.Unlock()

	m.record(logs.LevelInfo, fmt.Sprintf("Prem confidential-proxy starting on %s via npx -y -p %s %s --no-attest --compat both --host 127.0.0.1 --port %s", target.Address, premPCCIProxyPackage, premPCCIProxyCommand, target.Port))
	go m.forwardOutput("stdout", logs.LevelInfo, stdout)
	go m.forwardOutput("stderr", logs.LevelWarn, stderr)
	go m.waitForExit(ctx, cmd)

	if !waitForPremProxyTCP(target, premPCCIStartupWait) {
		return fmt.Errorf("confidential-proxy process started but %s did not open within %s", target.Address, premPCCIStartupWait)
	}
	m.record(logs.LevelInfo, fmt.Sprintf("Prem confidential-proxy ready on %s", target.Address))
	return nil
}

func (m *premProxyManager) Stop() {
	m.mu.Lock()
	cmd := m.cmd
	cancel := m.cancel
	m.cmd = nil
	m.cancel = nil
	m.startedAddr = ""
	m.mu.Unlock()

	if cmd == nil {
		return
	}
	if cancel != nil {
		cancel()
	}
	killPremProxyProcess(cmd)
	m.record(logs.LevelInfo, "Prem confidential-proxy stop requested")
}

func (m *premProxyManager) isRunningLocked() bool {
	return m.cmd != nil && m.cmd.Process != nil && m.cmd.ProcessState == nil
}

func (m *premProxyManager) waitForExit(ctx context.Context, cmd *exec.Cmd) {
	err := cmd.Wait()
	m.mu.Lock()
	if m.cmd == cmd {
		m.cmd = nil
		m.cancel = nil
		m.startedAddr = ""
	}
	m.mu.Unlock()

	if ctx.Err() != nil {
		m.record(logs.LevelInfo, "Prem confidential-proxy stopped")
		return
	}
	if err != nil {
		m.record(logs.LevelWarn, fmt.Sprintf("Prem confidential-proxy exited: %v", err))
		return
	}
	m.record(logs.LevelInfo, "Prem confidential-proxy exited")
}

func (m *premProxyManager) forwardOutput(stream string, level logs.Level, reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 1024), 64*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		m.record(level, fmt.Sprintf("Prem confidential-proxy %s: %s", stream, line))
	}
	if err := scanner.Err(); err != nil {
		m.record(logs.LevelWarn, fmt.Sprintf("Prem confidential-proxy %s read failed: %v", stream, err))
	}
}

func (m *premProxyManager) record(level logs.Level, message string) {
	if m == nil || m.logs == nil {
		return
	}
	m.logs.Add(logs.Entry{Level: level, Message: message})
}

func newPremPCCIProxyCommand(ctx context.Context, port string, apiKey string) (*exec.Cmd, error) {
	if _, err := exec.LookPath("npx"); err != nil {
		return nil, fmt.Errorf("npx not found; install Node.js/npm or start confidential-proxy manually")
	}

	args := []string{
		"-y",
		"-p", premPCCIProxyPackage,
		premPCCIProxyCommand,
		"--no-attest",
		"--compat", "both",
		"--host", "127.0.0.1",
		"--port", port,
		"--proxy-url", premPCCIProxyURL,
		"--enclave-url", premPCCIEnclaveURL,
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", append([]string{"/c", "npx"}, args...)...)
	} else {
		cmd = exec.CommandContext(ctx, "npx", args...)
	}
	cmd.Env = append(os.Environ(),
		"PORT="+port,
		"HOST=127.0.0.1",
		"PROXY_URL="+premPCCIProxyURL,
		"ENCLAVE_URL="+premPCCIEnclaveURL,
		"CI=1",
		"NO_COLOR=1",
	)
	if strings.TrimSpace(apiKey) != "" {
		cmd.Env = append(cmd.Env, "PREM_API_KEY="+strings.TrimSpace(apiKey))
	}
	hidePremProxyWindow(cmd)
	return cmd, nil
}

func premProxyTargetFromBaseURL(baseURL string) (premProxyTarget, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return premProxyTarget{}, fmt.Errorf("Prem Base URL is empty")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return premProxyTarget{}, fmt.Errorf("parse Prem Base URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return premProxyTarget{}, fmt.Errorf("Prem Base URL must use http or https")
	}
	host := parsed.Hostname()
	if host == "" {
		return premProxyTarget{}, fmt.Errorf("Prem Base URL must include a host")
	}
	port := parsed.Port()
	if port == "" {
		if parsed.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	return premProxyTarget{
		BaseURL:  baseURL,
		Host:     host,
		Port:     port,
		Address:  net.JoinHostPort(host, port),
		Loopback: isLoopbackHost(host),
	}, nil
}

func isLoopbackHost(host string) bool {
	normalized := strings.Trim(strings.ToLower(host), "[]")
	if normalized == "localhost" {
		return true
	}
	ip := net.ParseIP(normalized)
	return ip != nil && ip.IsLoopback()
}

func premProxyTCPListening(target premProxyTarget) bool {
	conn, err := net.DialTimeout("tcp", target.Address, 500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func waitForPremProxyTCP(target premProxyTarget, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if premProxyTCPListening(target) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(500 * time.Millisecond)
	}
}
