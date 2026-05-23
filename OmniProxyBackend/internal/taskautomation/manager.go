package taskautomation

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"OmniProxyBackend/internal/config"
	"OmniProxyBackend/internal/logs"
	"OmniProxyBackend/internal/proxy"
)

type windowHandle uintptr

type platformController interface {
	ForegroundWindow() windowHandle
	Launch(launchRequest) (launchResult, error)
	PressSpace() error
	Focus(windowHandle) error
}

type launchRequest struct {
	Mode              string
	Target            string
	FallbackURL       string
	Browser           string
	BrowserUserData   string
	BrowserProfile    string
	PauseBeforeReturn bool
}

type launchResult struct {
	Opened            string
	PauseBeforeReturn bool
}

type Manager struct {
	mu                 sync.Mutex
	cfg                config.Config
	logs               *logs.Recorder
	platform           platformController
	active             map[int64]proxy.ActiveRequest
	returnTo           windowHandle
	pauseBeforeReturn  bool
	pausedByAutomation bool
	pausedWindow       windowHandle
	linuxDOSessionKey  string
	linuxDOWindow      windowHandle
	idleTimer          *time.Timer
	idleSeq            int64
	resumeDelay        time.Duration
}

func NewManager(cfg config.Config, recorder *logs.Recorder) *Manager {
	return newManagerWithPlatform(cfg, recorder, defaultPlatformController())
}

func newManagerWithPlatform(cfg config.Config, recorder *logs.Recorder, platform platformController) *Manager {
	return &Manager{
		cfg:         config.Normalize(cfg),
		logs:        recorder,
		platform:    platform,
		active:      map[int64]proxy.ActiveRequest{},
		resumeDelay: 1200 * time.Millisecond,
	}
}

func (m *Manager) UpdateConfig(cfg config.Config) {
	cfg = config.Normalize(cfg)
	sessionKey := linuxDOSessionKey(launchRequestFromConfig(cfg))

	m.mu.Lock()
	m.cfg = cfg
	if !cfg.TaskAutomationEnabled {
		m.resetLocked()
	} else if m.linuxDOSessionKey != "" && m.linuxDOSessionKey != sessionKey {
		m.clearLinuxDOSessionLocked()
	}
	m.mu.Unlock()
}

func (m *Manager) Stop() {
	m.mu.Lock()
	m.resetLocked()
	m.mu.Unlock()
}

func (m *Manager) ActiveRequestStarted(req proxy.ActiveRequest) {
	cfg := m.snapshotConfig()
	if !requestMatchesConfig(req, cfg) {
		return
	}
	launchReq := launchRequestFromConfig(cfg)
	sessionKey := linuxDOSessionKey(launchReq)

	m.mu.Lock()
	if m.idleTimer != nil {
		m.idleTimer.Stop()
		m.idleTimer = nil
	}
	wasIdle := len(m.active) == 0
	hadPendingReturn := m.returnTo != 0
	m.active[req.ID] = req
	if !wasIdle {
		m.mu.Unlock()
		return
	}
	returnTo := m.returnTo
	if !hadPendingReturn {
		returnTo = m.platform.ForegroundWindow()
		m.returnTo = returnTo
	}
	m.pauseBeforeReturn = shouldPauseBeforeReturn(cfg)
	hasLinuxDOSession := sessionKey != "" && m.linuxDOSessionKey == sessionKey
	linuxDOWindow := m.linuxDOWindow
	m.mu.Unlock()

	if hadPendingReturn {
		m.resumePausedForeground(returnTo, 0)
		return
	}

	if hasLinuxDOSession {
		if linuxDOWindow != 0 && linuxDOWindow != returnTo {
			if err := m.platform.Focus(linuxDOWindow); err != nil {
				m.log(logs.LevelWarn, "任务开始，切回 Linux.do 浏览器失败：%v", err)
				if !isWindowUnavailableError(err) {
					return
				}
				m.mu.Lock()
				if m.linuxDOSessionKey == sessionKey {
					m.clearLinuxDOSessionLocked()
				}
				m.mu.Unlock()
			} else {
				m.log(logs.LevelInfo, "任务开始，已切回 Linux.do 浏览器")
				return
			}
		} else {
			m.log(logs.LevelInfo, "任务开始，Linux.do 浏览器已打开，跳过重复启动")
			return
		}
	}

	result, err := m.platform.Launch(launchReq)
	if err != nil {
		m.log(logs.LevelWarn, "放心刷打开目标失败：%v", err)
		return
	}
	m.mu.Lock()
	m.pauseBeforeReturn = result.PauseBeforeReturn
	m.mu.Unlock()
	opened := result.Opened
	if opened == "" {
		opened = "目标应用"
	}
	m.log(logs.LevelInfo, "任务开始，已打开%s", opened)
	if sessionKey != "" {
		m.rememberLinuxDOSession(sessionKey, returnTo, m.resumeDelay)
	} else {
		m.resumePausedForeground(returnTo, m.resumeDelay)
	}
}

func (m *Manager) ActiveRequestFinished(req proxy.ActiveRequest) {
	cfg := m.snapshotConfig()
	if !cfg.TaskAutomationEnabled {
		return
	}

	m.mu.Lock()
	if _, ok := m.active[req.ID]; !ok {
		m.mu.Unlock()
		return
	}
	delete(m.active, req.ID)
	if len(m.active) > 0 {
		m.mu.Unlock()
		return
	}
	if m.idleTimer != nil {
		m.idleTimer.Stop()
	}
	m.idleSeq++
	seq := m.idleSeq
	idle := time.Duration(cfg.TaskAutomationIdleSeconds) * time.Second
	m.idleTimer = time.AfterFunc(idle, func() {
		m.finishIdle(seq)
	})
	m.mu.Unlock()
}

func (m *Manager) finishIdle(seq int64) {
	cfg := m.snapshotConfig()

	m.mu.Lock()
	if seq != m.idleSeq || len(m.active) > 0 {
		m.mu.Unlock()
		return
	}
	if cfg.TaskAutomationEnabled && cfg.TaskAutomationReturnToClient && m.returnTo != 0 {
		handle := m.returnTo
		pauseBeforeReturn := m.pauseBeforeReturn
		delay := time.Duration(cfg.TaskAutomationReturnDelaySeconds) * time.Second
		m.idleTimer = time.AfterFunc(delay, func() {
			m.finishReturn(seq)
		})
		m.mu.Unlock()
		if pauseBeforeReturn {
			m.pauseForegroundBeforeReturn(handle)
		}
		m.log(logs.LevelInfo, "任务结束，等待 %d 秒后切回 CLI 窗口", cfg.TaskAutomationReturnDelaySeconds)
		return
	}
	m.idleTimer = nil
	m.mu.Unlock()

	m.finishReturn(seq)
}

func (m *Manager) finishReturn(seq int64) {
	cfg := m.snapshotConfig()

	m.mu.Lock()
	if seq != m.idleSeq || len(m.active) > 0 {
		m.mu.Unlock()
		return
	}
	handle := m.returnTo
	m.returnTo = 0
	m.idleTimer = nil
	m.mu.Unlock()

	if !cfg.TaskAutomationEnabled || !cfg.TaskAutomationReturnToClient || handle == 0 {
		return
	}
	if err := m.platform.Focus(handle); err != nil {
		m.log(logs.LevelWarn, "任务结束，切回 CLI 窗口失败：%v", err)
		return
	}
	m.log(logs.LevelInfo, "任务结束，已切回 CLI 窗口")
}

func (m *Manager) pauseForegroundBeforeReturn(handle windowHandle) {
	current := m.platform.ForegroundWindow()
	if current == 0 || current == handle {
		return
	}
	if err := m.platform.PressSpace(); err != nil {
		m.log(logs.LevelWarn, "任务结束，发送空格暂停当前窗口失败：%v", err)
		return
	}
	m.setPausedByAutomation(true, current)
	m.log(logs.LevelInfo, "任务结束，已发送空格暂停当前窗口")
}

func (m *Manager) resumePausedForeground(returnHandle windowHandle, delay time.Duration) {
	if paused, _ := m.pausedAutomationState(); !paused {
		return
	}
	resume := func() {
		paused, pausedWindow := m.pausedAutomationState()
		if !paused {
			return
		}
		if pausedWindow != 0 && m.platform.ForegroundWindow() != pausedWindow {
			if err := m.platform.Focus(pausedWindow); err != nil {
				m.setPausedByAutomation(false, 0)
				m.log(logs.LevelWarn, "任务开始，切回上次暂停窗口失败：%v", err)
				return
			}
		}
		foreground := m.platform.ForegroundWindow()
		if foreground == 0 || (returnHandle != 0 && foreground == returnHandle) {
			return
		}
		if err := m.platform.PressSpace(); err != nil {
			m.log(logs.LevelWarn, "任务开始，发送空格恢复当前窗口播放失败：%v", err)
			return
		}
		m.setPausedByAutomation(false, 0)
		m.log(logs.LevelInfo, "任务开始，已发送空格恢复当前窗口播放")
	}
	if delay <= 0 {
		resume()
		return
	}
	time.AfterFunc(delay, resume)
}

func (m *Manager) pausedAutomationState() (bool, windowHandle) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.pausedByAutomation, m.pausedWindow
}

func (m *Manager) setPausedByAutomation(paused bool, pausedWindow windowHandle) {
	m.mu.Lock()
	m.pausedByAutomation = paused
	m.pausedWindow = pausedWindow
	m.mu.Unlock()
}

func (m *Manager) rememberLinuxDOSession(sessionKey string, returnTo windowHandle, delay time.Duration) {
	m.mu.Lock()
	m.linuxDOSessionKey = sessionKey
	m.linuxDOWindow = 0
	m.mu.Unlock()

	capture := func() {
		foreground := m.platform.ForegroundWindow()
		if foreground == 0 || foreground == returnTo {
			return
		}
		m.mu.Lock()
		if m.linuxDOSessionKey == sessionKey {
			m.linuxDOWindow = foreground
		}
		m.mu.Unlock()
	}
	if delay <= 0 {
		capture()
		return
	}
	time.AfterFunc(delay, capture)
}

func (m *Manager) snapshotConfig() config.Config {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cfg
}

func (m *Manager) resetLocked() {
	if m.idleTimer != nil {
		m.idleTimer.Stop()
		m.idleTimer = nil
	}
	m.active = map[int64]proxy.ActiveRequest{}
	m.returnTo = 0
	m.pauseBeforeReturn = false
	m.pausedByAutomation = false
	m.pausedWindow = 0
	m.clearLinuxDOSessionLocked()
	m.idleSeq++
}

func (m *Manager) clearLinuxDOSessionLocked() {
	m.linuxDOSessionKey = ""
	m.linuxDOWindow = 0
}

func (m *Manager) log(level logs.Level, format string, args ...any) {
	if m.logs == nil {
		return
	}
	m.logs.Add(logs.Entry{
		Level:   level,
		Message: fmt.Sprintf(format, args...),
	})
}

func requestMatchesConfig(req proxy.ActiveRequest, cfg config.Config) bool {
	if !cfg.TaskAutomationEnabled {
		return false
	}
	client := strings.ToLower(strings.TrimSpace(req.ClientKey))
	if client == "" {
		return false
	}
	for _, allowed := range cfg.TaskAutomationClients {
		if strings.ToLower(strings.TrimSpace(allowed)) == client {
			return true
		}
	}
	return false
}

func launchRequestFromConfig(cfg config.Config) launchRequest {
	cfg = config.Normalize(cfg)
	pauseBeforeReturn := shouldPauseBeforeReturn(cfg)
	return launchRequest{
		Mode:              cfg.TaskAutomationLaunchMode,
		Target:            cfg.TaskAutomationLaunchTarget,
		FallbackURL:       cfg.TaskAutomationFallbackURL,
		Browser:           cfg.TaskAutomationBrowser,
		BrowserUserData:   cfg.TaskAutomationBrowserUserDataDir,
		BrowserProfile:    cfg.TaskAutomationBrowserProfile,
		PauseBeforeReturn: pauseBeforeReturn,
	}
}

func shouldPauseBeforeReturn(cfg config.Config) bool {
	cfg = config.Normalize(cfg)
	target := strings.ToLower(strings.TrimSpace(cfg.TaskAutomationLaunchTarget))
	return cfg.TaskAutomationLaunchMode != config.TaskAutomationLaunchModeLinuxDO &&
		target != "preset:linuxdo" &&
		target != "preset:linux-do" &&
		target != "preset:linux.do"
}

func isWindowUnavailableError(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "no longer available")
}

func linuxDOSessionKey(req launchRequest) string {
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	target := strings.ToLower(strings.TrimSpace(req.Target))
	if mode != config.TaskAutomationLaunchModeLinuxDO &&
		target != "preset:linuxdo" &&
		target != "preset:linux-do" &&
		target != "preset:linux.do" {
		return ""
	}
	if target == "" || target == "preset:linuxdo" || target == "preset:linux-do" || target == "preset:linux.do" {
		target = "https://linux.do/"
	}
	return strings.Join([]string{
		"linuxdo",
		target,
		strings.ToLower(strings.TrimSpace(req.Browser)),
		strings.ToLower(strings.TrimSpace(req.BrowserUserData)),
		strings.ToLower(strings.TrimSpace(req.BrowserProfile)),
	}, "\x00")
}
