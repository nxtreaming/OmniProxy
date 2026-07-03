//go:build windows

package taskautomation

import (
	"fmt"
	"strings"
	"syscall"

	"omniproxy/internal/config"
)

const (
	swShow    = 5
	swRestore = 9

	processQueryLimitedInformation = 0x1000

	keyeventfKeyup = 0x0002
	swpNoSize      = 0x0001
	swpNoMove      = 0x0002
	swpShowWindow  = 0x0040
	vkMenu         = 0x12
	vkSpace        = 0x20
)

var (
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	user32                  = syscall.NewLazyDLL("user32.dll")
	shell32                 = syscall.NewLazyDLL("shell32.dll")
	procGetCurrentThreadID  = kernel32.NewProc("GetCurrentThreadId")
	procGetForegroundWindow = user32.NewProc("GetForegroundWindow")
	procGetWindowThreadID   = user32.NewProc("GetWindowThreadProcessId")
	procOpenProcess         = kernel32.NewProc("OpenProcess")
	procQueryProcessImage   = kernel32.NewProc("QueryFullProcessImageNameW")
	procAttachThreadInput   = user32.NewProc("AttachThreadInput")
	procIsWindow            = user32.NewProc("IsWindow")
	procIsIconic            = user32.NewProc("IsIconic")
	procIsWindowVisible     = user32.NewProc("IsWindowVisible")
	procEnumWindows         = user32.NewProc("EnumWindows")
	procShowWindow          = user32.NewProc("ShowWindow")
	procSetWindowPos        = user32.NewProc("SetWindowPos")
	procBringWindowToTop    = user32.NewProc("BringWindowToTop")
	procSetActiveWindow     = user32.NewProc("SetActiveWindow")
	procSetFocus            = user32.NewProc("SetFocus")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procKeybdEvent          = user32.NewProc("keybd_event")
	procShellExecuteW       = shell32.NewProc("ShellExecuteW")
)

type windowsPlatform struct{}

func defaultPlatformController() platformController {
	return windowsPlatform{}
}

func (windowsPlatform) ForegroundWindow() windowHandle {
	hwnd, _, _ := procGetForegroundWindow.Call()
	return windowHandle(hwnd)
}

func (windowsPlatform) Launch(req launchRequest) (launchResult, error) {
	req = normalizeLaunchRequest(req)
	if req.Mode == config.TaskAutomationLaunchModeLinuxDO || isLinuxDOPreset(req.Target) {
		opened, err := openLinuxDOInBrowser(req)
		if err != nil {
			return launchResult{}, err
		}
		return launchResult{Opened: opened, PauseBeforeReturn: false}, nil
	}

	target := strings.TrimSpace(req.Target)
	fallbackURL := strings.TrimSpace(req.FallbackURL)
	if fallbackURL == "" {
		fallbackURL = "https://www.douyin.com"
	}

	if target == "" {
		preset, _ := launchPresetFor("douyin")
		opened, err := openLaunchPreset(preset, fallbackURL)
		return launchResult{Opened: opened, PauseBeforeReturn: req.PauseBeforeReturn}, err
	}

	opened, err := openConfiguredTarget(target)
	if err == nil {
		return launchResult{Opened: opened, PauseBeforeReturn: req.PauseBeforeReturn}, nil
	}
	if fallbackURL == "" {
		return launchResult{}, err
	}
	if fallbackErr := shellOpen(fallbackURL); fallbackErr != nil {
		return launchResult{}, fmt.Errorf("%w; fallback failed: %v", err, fallbackErr)
	}
	return launchResult{Opened: "备用地址", PauseBeforeReturn: req.PauseBeforeReturn}, nil
}

func (windowsPlatform) PressSpace() error {
	procKeybdEvent.Call(vkSpace, 0, 0, 0)
	procKeybdEvent.Call(vkSpace, 0, keyeventfKeyup, 0)
	return nil
}

func (windowsPlatform) Focus(hwnd windowHandle) error {
	if hwnd == 0 {
		return fmt.Errorf("CLI window handle is empty")
	}
	valid, _, _ := procIsWindow.Call(uintptr(hwnd))
	if valid == 0 {
		return fmt.Errorf("CLI window is no longer available")
	}
	target := uintptr(hwnd)
	restoreWindowIfMinimized(target)
	if !forceForegroundWindow(target) {
		unlockForegroundWithAlt()
	}
	if !forceForegroundWindow(target) {
		return fmt.Errorf("Windows rejected foreground activation")
	}
	return nil
}
