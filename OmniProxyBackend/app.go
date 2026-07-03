package main

import (
	"context"
	"fmt"
	"log"

	"omniproxy/internal/logs"
	"omniproxy/internal/tray"

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
	a.server.ensurePremProxyForConfiguredAccounts("startup")
	a.setupTray()
}

func (a *DesktopApp) shutdown(ctx context.Context) {
	if a.tray != nil {
		a.tray.Stop()
		a.tray = nil
	}
	a.server.stopHealthMonitor()
	a.server.stopPremProxy()
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

func (a *DesktopApp) callContext() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}
