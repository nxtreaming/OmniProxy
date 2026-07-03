package main

import "omniproxy/internal/autostart"

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
