//go:build windows

package taskautomation

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

const (
	swShownormal = 1
	swRestore    = 9

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
	procAttachThreadInput   = user32.NewProc("AttachThreadInput")
	procIsWindow            = user32.NewProc("IsWindow")
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

type launchPreset struct {
	key         string
	desktopName string
	webName     string
	fallbackURL string
	candidates  []string
}

func defaultPlatformController() platformController {
	return windowsPlatform{}
}

func (windowsPlatform) ForegroundWindow() windowHandle {
	hwnd, _, _ := procGetForegroundWindow.Call()
	return windowHandle(hwnd)
}

func (windowsPlatform) Launch(target string, fallbackURL string) (string, error) {
	target = strings.TrimSpace(target)
	fallbackURL = strings.TrimSpace(fallbackURL)
	if fallbackURL == "" {
		fallbackURL = "https://www.douyin.com"
	}

	if target == "" {
		preset, _ := launchPresetFor("douyin")
		return openLaunchPreset(preset, fallbackURL)
	}

	opened, err := openConfiguredTarget(target)
	if err == nil {
		return opened, nil
	}
	if fallbackURL == "" {
		return "", err
	}
	if fallbackErr := shellOpen(fallbackURL); fallbackErr != nil {
		return "", fmt.Errorf("%w; fallback failed: %v", err, fallbackErr)
	}
	return "备用地址", nil
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
	procShowWindow.Call(target, swRestore)
	if !forceForegroundWindow(target) {
		unlockForegroundWithAlt()
	}
	if !forceForegroundWindow(target) {
		return fmt.Errorf("Windows rejected foreground activation")
	}
	return nil
}

func forceForegroundWindow(hwnd uintptr) bool {
	if hwnd == 0 {
		return false
	}

	currentThread, _, _ := procGetCurrentThreadID.Call()
	foreground, _, _ := procGetForegroundWindow.Call()
	foregroundThread := uintptr(0)
	if foreground != 0 {
		foregroundThread, _, _ = procGetWindowThreadID.Call(foreground, 0)
	}
	targetThread, _, _ := procGetWindowThreadID.Call(hwnd, 0)

	attachedForeground := attachThreadInput(currentThread, foregroundThread)
	attachedTarget := false
	if targetThread != foregroundThread {
		attachedTarget = attachThreadInput(currentThread, targetThread)
	}
	defer detachThreadInput(currentThread, foregroundThread, attachedForeground)
	defer detachThreadInput(currentThread, targetThread, attachedTarget)

	procShowWindow.Call(hwnd, swRestore)
	procSetWindowPos.Call(hwnd, ^uintptr(0), 0, 0, 0, 0, swpNoMove|swpNoSize|swpShowWindow)
	procSetWindowPos.Call(hwnd, ^uintptr(1), 0, 0, 0, 0, swpNoMove|swpNoSize|swpShowWindow)
	procBringWindowToTop.Call(hwnd)
	procSetActiveWindow.Call(hwnd)
	procSetFocus.Call(hwnd)
	procSetForegroundWindow.Call(hwnd)
	return isForegroundWindow(hwnd)
}

func attachThreadInput(currentThread uintptr, targetThread uintptr) bool {
	if currentThread == 0 || targetThread == 0 || currentThread == targetThread {
		return false
	}
	ok, _, _ := procAttachThreadInput.Call(currentThread, targetThread, 1)
	return ok != 0
}

func detachThreadInput(currentThread uintptr, targetThread uintptr, attached bool) {
	if attached {
		procAttachThreadInput.Call(currentThread, targetThread, 0)
	}
}

func unlockForegroundWithAlt() {
	procKeybdEvent.Call(vkMenu, 0, 0, 0)
	procKeybdEvent.Call(vkMenu, 0, keyeventfKeyup, 0)
}

func isForegroundWindow(hwnd uintptr) bool {
	foreground, _, _ := procGetForegroundWindow.Call()
	return foreground == hwnd
}

func openConfiguredTarget(target string) (string, error) {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(target)), "preset:") {
		preset, ok := launchPresetFromTarget(target)
		if !ok {
			return "", fmt.Errorf("unknown preset target: %s", target)
		}
		return openLaunchPreset(preset, "")
	}
	if isURLTarget(target) {
		return target, shellOpen(target)
	}
	resolved, ok := resolveLocalTarget(target)
	if !ok {
		return "", fmt.Errorf("target not found: %s", target)
	}
	return resolved, shellOpen(resolved)
}

func isURLTarget(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme != "" && len(parsed.Scheme) > 1
}

func resolveLocalTarget(target string) (string, bool) {
	if target == "" {
		return "", false
	}
	if exists(target) {
		return target, true
	}
	if resolved, err := exec.LookPath(target); err == nil && resolved != "" {
		return resolved, true
	}
	return "", false
}

func launchPresetFromTarget(target string) (launchPreset, bool) {
	key, ok := strings.CutPrefix(strings.ToLower(strings.TrimSpace(target)), "preset:")
	if !ok {
		return launchPreset{}, false
	}
	return launchPresetFor(key)
}

func launchPresetFor(key string) (launchPreset, bool) {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "douyin":
		return launchPreset{
			key:         "douyin",
			desktopName: "桌面抖音",
			webName:     "抖音官网",
			fallbackURL: "https://www.douyin.com",
			candidates: []string{
				envPath("LOCALAPPDATA", "Programs", "Douyin", "Douyin.exe"),
				envPath("LOCALAPPDATA", "Douyin", "Douyin.exe"),
				envPath("LOCALAPPDATA", "douyin", "Douyin.exe"),
				envPath("ProgramFiles", "Douyin", "Douyin.exe"),
				envPath("ProgramFiles(x86)", "Douyin", "Douyin.exe"),
				envPath("APPDATA", "Microsoft", "Windows", "Start Menu", "Programs", "Douyin.lnk"),
				envPath("APPDATA", "Microsoft", "Windows", "Start Menu", "Programs", "\u6296\u97f3.lnk"),
				envPath("ProgramData", "Microsoft", "Windows", "Start Menu", "Programs", "Douyin.lnk"),
				envPath("ProgramData", "Microsoft", "Windows", "Start Menu", "Programs", "\u6296\u97f3.lnk"),
				envPath("USERPROFILE", "Desktop", "Douyin.lnk"),
				envPath("USERPROFILE", "Desktop", "\u6296\u97f3.lnk"),
				envPath("PUBLIC", "Desktop", "Douyin.lnk"),
				envPath("PUBLIC", "Desktop", "\u6296\u97f3.lnk"),
			},
		}, true
	case "bilibili":
		return launchPreset{
			key:         "bilibili",
			desktopName: "哔哩哔哩桌面端",
			webName:     "哔哩哔哩官网",
			fallbackURL: "https://www.bilibili.com",
			candidates: []string{
				envPath("LOCALAPPDATA", "Programs", "bilibili", "bilibili.exe"),
				envPath("LOCALAPPDATA", "Programs", "bilibili", "\u54d4\u54e9\u54d4\u54e9.exe"),
				envPath("LOCALAPPDATA", "bilibili", "bilibili.exe"),
				envPath("ProgramFiles", "bilibili", "bilibili.exe"),
				envPath("ProgramFiles(x86)", "bilibili", "bilibili.exe"),
				envPath("APPDATA", "Microsoft", "Windows", "Start Menu", "Programs", "bilibili.lnk"),
				envPath("APPDATA", "Microsoft", "Windows", "Start Menu", "Programs", "\u54d4\u54e9\u54d4\u54e9.lnk"),
				envPath("ProgramData", "Microsoft", "Windows", "Start Menu", "Programs", "bilibili.lnk"),
				envPath("ProgramData", "Microsoft", "Windows", "Start Menu", "Programs", "\u54d4\u54e9\u54d4\u54e9.lnk"),
				envPath("USERPROFILE", "Desktop", "bilibili.lnk"),
				envPath("USERPROFILE", "Desktop", "\u54d4\u54e9\u54d4\u54e9.lnk"),
				envPath("PUBLIC", "Desktop", "bilibili.lnk"),
				envPath("PUBLIC", "Desktop", "\u54d4\u54e9\u54d4\u54e9.lnk"),
			},
		}, true
	default:
		return launchPreset{}, false
	}
}

func openLaunchPreset(preset launchPreset, fallbackURL string) (string, error) {
	if target := findFirstExisting(preset.candidates); target != "" {
		return preset.desktopName, shellOpen(target)
	}
	targetURL := strings.TrimSpace(fallbackURL)
	if targetURL == "" {
		targetURL = preset.fallbackURL
	}
	if targetURL == "" {
		return "", fmt.Errorf("preset target not found: %s", preset.key)
	}
	return preset.webName, shellOpen(targetURL)
}

func findFirstExisting(candidates []string) string {
	for _, candidate := range candidates {
		if exists(candidate) {
			return candidate
		}
	}
	return ""
}

func envPath(env string, parts ...string) string {
	root := strings.TrimSpace(os.Getenv(env))
	if root == "" {
		return ""
	}
	items := append([]string{root}, parts...)
	return filepath.Join(items...)
}

func exists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func shellOpen(target string) error {
	file, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	operation, _ := syscall.UTF16PtrFromString("open")
	result, _, _ := procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(operation)),
		uintptr(unsafe.Pointer(file)),
		0,
		0,
		swShownormal,
	)
	if result <= 32 {
		return fmt.Errorf("ShellExecute failed with code %d", result)
	}
	return nil
}
