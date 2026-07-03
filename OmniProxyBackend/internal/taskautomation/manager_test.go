package taskautomation

import (
	"errors"
	"testing"

	"omniproxy/internal/config"
	"omniproxy/internal/proxy"
)

type fakePlatform struct {
	foreground       windowHandle
	launchForeground windowHandle
	opened           []string
	spaces           int
	focused          []windowHandle
	focusErr         error
}

func (f *fakePlatform) ForegroundWindow() windowHandle {
	return f.foreground
}

func (f *fakePlatform) Launch(req launchRequest) (launchResult, error) {
	target := req.Target
	if target == "" {
		target = req.FallbackURL
	}
	f.opened = append(f.opened, target)
	if f.launchForeground != 0 {
		f.foreground = f.launchForeground
	} else {
		f.foreground = 99
	}
	return launchResult{Opened: target, PauseBeforeReturn: req.PauseBeforeReturn}, nil
}

func (f *fakePlatform) PressSpace() error {
	f.spaces++
	return nil
}

func (f *fakePlatform) Focus(handle windowHandle) error {
	if f.focusErr != nil {
		return f.focusErr
	}
	f.focused = append(f.focused, handle)
	f.foreground = handle
	return nil
}

func finishIdleAndReturnForTest(t *testing.T, manager *Manager) {
	t.Helper()
	manager.mu.Lock()
	seq := manager.idleSeq
	if manager.idleTimer != nil {
		manager.idleTimer.Stop()
	}
	manager.mu.Unlock()
	manager.finishIdle(seq)
	manager.mu.Lock()
	if manager.idleTimer != nil {
		manager.idleTimer.Stop()
	}
	manager.mu.Unlock()
	manager.finishReturn(seq)
}

func TestManagerAggregatesRequestsIntoOneTask(t *testing.T) {
	platform := &fakePlatform{foreground: 42}
	cfg := config.Default()
	cfg.TaskAutomationEnabled = true
	cfg.TaskAutomationClients = []string{"codex"}
	cfg.TaskAutomationLaunchTarget = "notepad.exe"
	cfg.TaskAutomationIdleSeconds = 1

	manager := newManagerWithPlatform(cfg, nil, platform)
	manager.resumeDelay = 0
	manager.ActiveRequestStarted(proxy.ActiveRequest{ID: 1, ClientKey: "codex"})
	manager.ActiveRequestStarted(proxy.ActiveRequest{ID: 2, ClientKey: "codex"})
	if len(platform.opened) != 1 {
		t.Fatalf("expected one launch for aggregated task, got %d", len(platform.opened))
	}

	manager.ActiveRequestFinished(proxy.ActiveRequest{ID: 1, ClientKey: "codex"})
	if len(platform.focused) != 0 {
		t.Fatalf("expected no focus while a matching request is active, got %d", len(platform.focused))
	}

	manager.ActiveRequestFinished(proxy.ActiveRequest{ID: 2, ClientKey: "codex"})
	manager.mu.Lock()
	seq := manager.idleSeq
	if manager.idleTimer != nil {
		manager.idleTimer.Stop()
	}
	manager.mu.Unlock()
	manager.finishIdle(seq)
	if platform.spaces != 1 {
		t.Fatalf("expected one space press before return, got %d", platform.spaces)
	}
	if len(platform.focused) != 0 {
		t.Fatalf("expected no focus before return delay elapses, got %#v", platform.focused)
	}
	manager.mu.Lock()
	if manager.idleTimer != nil {
		manager.idleTimer.Stop()
	}
	manager.mu.Unlock()
	manager.finishReturn(seq)

	if len(platform.focused) != 1 || platform.focused[0] != 42 {
		t.Fatalf("expected focus back to captured CLI window, got %#v", platform.focused)
	}

	platform.launchForeground = 77
	manager.ActiveRequestStarted(proxy.ActiveRequest{ID: 3, ClientKey: "codex"})
	if platform.spaces != 2 {
		t.Fatalf("expected second space press to resume playback, got %d", platform.spaces)
	}
	if len(platform.focused) != 2 || platform.focused[1] != 99 {
		t.Fatalf("expected focus back to the paused playback window, got %#v", platform.focused)
	}
}

func TestManagerIgnoresUnselectedClient(t *testing.T) {
	platform := &fakePlatform{foreground: 42}
	cfg := config.Default()
	cfg.TaskAutomationEnabled = true
	cfg.TaskAutomationClients = []string{"claude"}

	manager := newManagerWithPlatform(cfg, nil, platform)
	manager.ActiveRequestStarted(proxy.ActiveRequest{ID: 1, ClientKey: "codex"})
	manager.ActiveRequestFinished(proxy.ActiveRequest{ID: 1, ClientKey: "codex"})

	if len(platform.opened) != 0 || len(platform.focused) != 0 {
		t.Fatalf("expected no automation for unselected client, opened=%#v focused=%#v", platform.opened, platform.focused)
	}
}

func TestManagerSkipsPauseForLinuxDOMode(t *testing.T) {
	platform := &fakePlatform{foreground: 42}
	cfg := config.Default()
	cfg.TaskAutomationEnabled = true
	cfg.TaskAutomationClients = []string{"codex"}
	cfg.TaskAutomationLaunchMode = config.TaskAutomationLaunchModeLinuxDO
	cfg.TaskAutomationLaunchTarget = "preset:linuxdo"
	cfg.TaskAutomationIdleSeconds = 1

	manager := newManagerWithPlatform(cfg, nil, platform)
	manager.ActiveRequestStarted(proxy.ActiveRequest{ID: 1, ClientKey: "codex"})
	manager.ActiveRequestFinished(proxy.ActiveRequest{ID: 1, ClientKey: "codex"})
	manager.mu.Lock()
	seq := manager.idleSeq
	if manager.idleTimer != nil {
		manager.idleTimer.Stop()
	}
	manager.mu.Unlock()
	manager.finishIdle(seq)

	if platform.spaces != 0 {
		t.Fatalf("expected linux.do mode not to press space, got %d", platform.spaces)
	}
	manager.mu.Lock()
	if manager.idleTimer != nil {
		manager.idleTimer.Stop()
	}
	manager.mu.Unlock()
	manager.finishReturn(seq)
	if len(platform.focused) != 1 || platform.focused[0] != 42 {
		t.Fatalf("expected linux.do mode to still return focus, got %#v", platform.focused)
	}
}

func TestManagerReusesLinuxDOBrowserSessionAcrossRounds(t *testing.T) {
	platform := &fakePlatform{foreground: 42}
	cfg := config.Default()
	cfg.TaskAutomationEnabled = true
	cfg.TaskAutomationClients = []string{"codex"}
	cfg.TaskAutomationLaunchMode = config.TaskAutomationLaunchModeLinuxDO
	cfg.TaskAutomationLaunchTarget = "preset:linuxdo"
	cfg.TaskAutomationIdleSeconds = 1

	manager := newManagerWithPlatform(cfg, nil, platform)
	manager.resumeDelay = 0

	manager.ActiveRequestStarted(proxy.ActiveRequest{ID: 1, ClientKey: "codex"})
	if len(platform.opened) != 1 {
		t.Fatalf("expected first round to launch linux.do once, got %d", len(platform.opened))
	}
	manager.ActiveRequestFinished(proxy.ActiveRequest{ID: 1, ClientKey: "codex"})
	finishIdleAndReturnForTest(t, manager)

	manager.ActiveRequestStarted(proxy.ActiveRequest{ID: 2, ClientKey: "codex"})
	if len(platform.opened) != 1 {
		t.Fatalf("expected second round to reuse linux.do browser without relaunching, got %d launches", len(platform.opened))
	}
	if len(platform.focused) != 2 || platform.focused[1] != 99 {
		t.Fatalf("expected second round to focus captured linux.do browser window, got %#v", platform.focused)
	}
}

func TestManagerReopensLinuxDOWhenCapturedWindowClosed(t *testing.T) {
	platform := &fakePlatform{foreground: 42}
	cfg := config.Default()
	cfg.TaskAutomationEnabled = true
	cfg.TaskAutomationClients = []string{"codex"}
	cfg.TaskAutomationLaunchMode = config.TaskAutomationLaunchModeLinuxDO
	cfg.TaskAutomationLaunchTarget = "preset:linuxdo"

	manager := newManagerWithPlatform(cfg, nil, platform)
	manager.resumeDelay = 0

	manager.ActiveRequestStarted(proxy.ActiveRequest{ID: 1, ClientKey: "codex"})
	if len(platform.opened) != 1 {
		t.Fatalf("expected first round to launch linux.do once, got %d", len(platform.opened))
	}
	manager.ActiveRequestFinished(proxy.ActiveRequest{ID: 1, ClientKey: "codex"})
	finishIdleAndReturnForTest(t, manager)

	platform.foreground = 42
	platform.focusErr = errors.New("window is no longer available")
	manager.ActiveRequestStarted(proxy.ActiveRequest{ID: 2, ClientKey: "codex"})
	if len(platform.opened) != 2 {
		t.Fatalf("expected closed linux.do window to be reopened, got %d launches", len(platform.opened))
	}
}
