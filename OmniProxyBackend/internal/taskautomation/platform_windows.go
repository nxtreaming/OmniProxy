//go:build windows

package taskautomation

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

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

	linuxDOURL = "https://linux.do/"
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

type launchPreset struct {
	key         string
	desktopName string
	webName     string
	fallbackURL string
	candidates  []string
}

type browserSpec struct {
	key            string
	name           string
	command        string
	defaultDataDir string
	chromium       bool
	firefox        bool
	processNames   []string
	candidates     []string
}

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

	restoreWindowIfMinimized(hwnd)
	procSetWindowPos.Call(hwnd, ^uintptr(0), 0, 0, 0, 0, swpNoMove|swpNoSize|swpShowWindow)
	procSetWindowPos.Call(hwnd, ^uintptr(1), 0, 0, 0, 0, swpNoMove|swpNoSize|swpShowWindow)
	procBringWindowToTop.Call(hwnd)
	procSetActiveWindow.Call(hwnd)
	procSetFocus.Call(hwnd)
	procSetForegroundWindow.Call(hwnd)
	return isForegroundWindow(hwnd)
}

func restoreWindowIfMinimized(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	minimized, _, _ := procIsIconic.Call(hwnd)
	if minimized != 0 {
		procShowWindow.Call(hwnd, swRestore)
	}
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
	case "linuxdo", "linux-do", "linux.do":
		return launchPreset{
			key:         "linuxdo",
			desktopName: "Linux.do",
			webName:     "Linux.do",
			fallbackURL: linuxDOURL,
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

func normalizeLaunchRequest(req launchRequest) launchRequest {
	req.Mode = strings.ToLower(strings.TrimSpace(req.Mode))
	req.Target = strings.TrimSpace(req.Target)
	req.FallbackURL = strings.TrimSpace(req.FallbackURL)
	req.Browser = normalizeBrowserKey(req.Browser)
	req.BrowserUserData = strings.TrimSpace(req.BrowserUserData)
	req.BrowserProfile = strings.TrimSpace(req.BrowserProfile)
	return req
}

func isLinuxDOPreset(target string) bool {
	preset, ok := launchPresetFromTarget(target)
	return ok && preset.key == "linuxdo"
}

func openLinuxDOInBrowser(req launchRequest) (string, error) {
	targetURL := linuxDOTargetURL(req.Target)
	spec, ok := browserSpecFor(req.Browser)
	if !ok || spec.key == config.TaskAutomationBrowserDefault {
		if err := shellOpen(targetURL); err != nil {
			return "", err
		}
		focusBrowserWindowSoon(spec)
		return "Linux.do", nil
	}
	executable := resolveBrowserExecutable(spec)
	if executable == "" {
		return "", fmt.Errorf("%s not found", spec.name)
	}
	args := browserLaunchArgs(req, targetURL, spec)
	if err := exec.Command(executable, args...).Start(); err != nil {
		return "", err
	}
	focusBrowserWindowSoon(spec)
	if strings.TrimSpace(req.BrowserProfile) != "" {
		return fmt.Sprintf("Linux.do（%s / %s）", spec.name, req.BrowserProfile), nil
	}
	return fmt.Sprintf("Linux.do（%s）", spec.name), nil
}

func linuxDOTargetURL(target string) string {
	target = strings.TrimSpace(target)
	if parsed, err := url.Parse(target); err == nil {
		switch strings.ToLower(parsed.Scheme) {
		case "http", "https":
			return target
		}
	}
	return linuxDOURL
}

func browserLaunchArgs(req launchRequest, targetURL string, spec browserSpec) []string {
	if spec.chromium {
		args := []string{}
		if userData := customChromiumUserDataDir(req, spec); userData != "" {
			args = append(args, "--user-data-dir="+userData)
		}
		if profile := strings.TrimSpace(req.BrowserProfile); profile != "" {
			args = append(args, "--profile-directory="+profile)
		}
		return append(args, "--new-window", targetURL)
	}
	if spec.firefox {
		args := []string{}
		profile := strings.TrimSpace(req.BrowserProfile)
		if profile == "" {
			profile = strings.TrimSpace(req.BrowserUserData)
		}
		if profile != "" {
			expanded := expandEnvPath(profile)
			if looksLikePath(expanded) || exists(expanded) {
				args = append(args, "-profile", expanded)
			} else {
				args = append(args, "-P", profile)
			}
		}
		return append(args, "-new-window", targetURL)
	}
	return []string{targetURL}
}

func focusBrowserWindowSoon(spec browserSpec) {
	for _, delay := range []time.Duration{250 * time.Millisecond, 900 * time.Millisecond, 1600 * time.Millisecond} {
		time.AfterFunc(delay, func() {
			if hwnd := findBrowserWindow(spec); hwnd != 0 {
				if !forceForegroundWindow(hwnd) {
					unlockForegroundWithAlt()
					forceForegroundWindow(hwnd)
				}
			}
		})
	}
}

func findBrowserWindow(spec browserSpec) uintptr {
	names := browserProcessNameSet(spec)
	if len(names) == 0 {
		return 0
	}

	var match uintptr
	callback := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		if match != 0 || !isVisibleWindow(hwnd) {
			return 1
		}
		if names[windowProcessName(hwnd)] {
			match = hwnd
			return 0
		}
		return 1
	})
	procEnumWindows.Call(callback, 0)
	return match
}

func browserProcessNameSet(spec browserSpec) map[string]bool {
	names := map[string]bool{}
	for _, name := range spec.processNames {
		name = strings.ToLower(strings.TrimSpace(name))
		if name != "" {
			names[name] = true
		}
	}
	return names
}

func isVisibleWindow(hwnd uintptr) bool {
	if hwnd == 0 {
		return false
	}
	visible, _, _ := procIsWindowVisible.Call(hwnd)
	return visible != 0
}

func windowProcessName(hwnd uintptr) string {
	var pid uint32
	procGetWindowThreadID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return ""
	}
	return processImageName(pid)
}

func processImageName(pid uint32) string {
	handle, _, _ := procOpenProcess.Call(processQueryLimitedInformation, 0, uintptr(pid))
	if handle == 0 {
		return ""
	}
	defer syscall.CloseHandle(syscall.Handle(handle))

	buffer := make([]uint16, 32768)
	size := uint32(len(buffer))
	ok, _, _ := procQueryProcessImage.Call(
		handle,
		0,
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if ok == 0 || size == 0 {
		return ""
	}
	return strings.ToLower(filepath.Base(syscall.UTF16ToString(buffer[:size])))
}

func resolveBrowserExecutable(spec browserSpec) string {
	if target := findFirstExisting(spec.candidates); target != "" {
		return target
	}
	if spec.command != "" {
		if resolved, err := exec.LookPath(spec.command); err == nil && resolved != "" {
			return resolved
		}
	}
	return ""
}

func browserSpecFor(browser string) (browserSpec, bool) {
	switch normalizeBrowserKey(browser) {
	case config.TaskAutomationBrowserDefault:
		return browserSpec{
			key:  config.TaskAutomationBrowserDefault,
			name: "默认浏览器",
			processNames: []string{
				"msedge.exe",
				"chrome.exe",
				"firefox.exe",
				"brave.exe",
				"opera.exe",
				"vivaldi.exe",
				"qqbrowser.exe",
				"360chrome.exe",
				"360se.exe",
			},
		}, true
	case config.TaskAutomationBrowserEdge:
		return browserSpec{
			key:            config.TaskAutomationBrowserEdge,
			name:           "Microsoft Edge",
			command:        "msedge.exe",
			defaultDataDir: envPath("LOCALAPPDATA", "Microsoft", "Edge", "User Data"),
			chromium:       true,
			processNames:   []string{"msedge.exe"},
			candidates: []string{
				envPath("ProgramFiles(x86)", "Microsoft", "Edge", "Application", "msedge.exe"),
				envPath("ProgramFiles", "Microsoft", "Edge", "Application", "msedge.exe"),
				envPath("LOCALAPPDATA", "Microsoft", "Edge", "Application", "msedge.exe"),
			},
		}, true
	case config.TaskAutomationBrowserChrome:
		return browserSpec{
			key:            config.TaskAutomationBrowserChrome,
			name:           "Google Chrome",
			command:        "chrome.exe",
			defaultDataDir: envPath("LOCALAPPDATA", "Google", "Chrome", "User Data"),
			chromium:       true,
			processNames:   []string{"chrome.exe"},
			candidates: []string{
				envPath("ProgramFiles", "Google", "Chrome", "Application", "chrome.exe"),
				envPath("ProgramFiles(x86)", "Google", "Chrome", "Application", "chrome.exe"),
				envPath("LOCALAPPDATA", "Google", "Chrome", "Application", "chrome.exe"),
			},
		}, true
	case config.TaskAutomationBrowserFirefox:
		return browserSpec{
			key:     config.TaskAutomationBrowserFirefox,
			name:    "Firefox",
			command: "firefox.exe",
			firefox: true,
			processNames: []string{
				"firefox.exe",
			},
			candidates: []string{
				envPath("ProgramFiles", "Mozilla Firefox", "firefox.exe"),
				envPath("ProgramFiles(x86)", "Mozilla Firefox", "firefox.exe"),
				envPath("LOCALAPPDATA", "Mozilla Firefox", "firefox.exe"),
			},
		}, true
	default:
		return browserSpec{}, false
	}
}

func customChromiumUserDataDir(req launchRequest, spec browserSpec) string {
	userData := expandEnvPath(req.BrowserUserData)
	if userData == "" || samePath(userData, spec.defaultDataDir) {
		return ""
	}
	return userData
}

func samePath(left string, right string) bool {
	left = expandEnvPath(left)
	right = expandEnvPath(right)
	if strings.TrimSpace(left) == "" || strings.TrimSpace(right) == "" {
		return false
	}
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}

func normalizeBrowserKey(browser string) string {
	switch strings.ToLower(strings.TrimSpace(strings.ReplaceAll(browser, "_", "-"))) {
	case config.TaskAutomationBrowserEdge, "msedge", "microsoft-edge":
		return config.TaskAutomationBrowserEdge
	case config.TaskAutomationBrowserChrome, "google-chrome":
		return config.TaskAutomationBrowserChrome
	case config.TaskAutomationBrowserFirefox, "mozilla-firefox":
		return config.TaskAutomationBrowserFirefox
	case config.TaskAutomationBrowserDefault, "":
		return config.TaskAutomationBrowserDefault
	default:
		return config.TaskAutomationBrowserDefault
	}
}

func expandEnvPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return os.ExpandEnv(expandPercentEnv(value))
}

func expandPercentEnv(value string) string {
	var out strings.Builder
	for {
		start := strings.IndexByte(value, '%')
		if start < 0 {
			out.WriteString(value)
			break
		}
		end := strings.IndexByte(value[start+1:], '%')
		if end < 0 {
			out.WriteString(value)
			break
		}
		end += start + 1
		out.WriteString(value[:start])
		key := value[start+1 : end]
		if replacement := os.Getenv(key); replacement != "" {
			out.WriteString(replacement)
		} else {
			out.WriteString(value[start : end+1])
		}
		value = value[end+1:]
	}
	return out.String()
}

func looksLikePath(value string) bool {
	if value == "" {
		return false
	}
	if filepath.IsAbs(value) {
		return true
	}
	if strings.ContainsAny(value, `\/`) {
		return true
	}
	_, err := os.Stat(value)
	return err == nil || !errors.Is(err, os.ErrNotExist)
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
		swShow,
	)
	if result <= 32 {
		return fmt.Errorf("ShellExecute failed with code %d", result)
	}
	return nil
}
