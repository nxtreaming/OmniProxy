//go:build darwin

package taskautomation

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"OmniProxyBackend/internal/config"
)

const darwinLinuxDOURL = "https://linux.do/"

type darwinPlatform struct{}

type darwinLaunchPreset struct {
	key         string
	desktopName string
	webName     string
	fallbackURL string
	candidates  []string
}

type darwinBrowserSpec struct {
	key        string
	name       string
	chromium   bool
	firefox    bool
	candidates []string
}

func defaultPlatformController() platformController {
	return darwinPlatform{}
}

func (darwinPlatform) ForegroundWindow() windowHandle {
	out, err := exec.Command(
		"osascript",
		"-e",
		`tell application "System Events" to get unix id of first application process whose frontmost is true`,
	).Output()
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil || pid <= 0 {
		return 0
	}
	return windowHandle(pid)
}

func (darwinPlatform) Launch(req launchRequest) (launchResult, error) {
	req = darwinNormalizeLaunchRequest(req)
	if req.Mode == config.TaskAutomationLaunchModeLinuxDO || darwinIsLinuxDOPreset(req.Target) {
		opened, err := darwinOpenLinuxDOInBrowser(req)
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
		preset, _ := darwinLaunchPresetFor("douyin")
		opened, err := darwinOpenLaunchPreset(preset, fallbackURL)
		return launchResult{Opened: opened, PauseBeforeReturn: req.PauseBeforeReturn}, err
	}

	opened, err := darwinOpenConfiguredTarget(target)
	if err == nil {
		return launchResult{Opened: opened, PauseBeforeReturn: req.PauseBeforeReturn}, nil
	}
	if fallbackURL == "" {
		return launchResult{}, err
	}
	if fallbackErr := darwinShellOpen(fallbackURL); fallbackErr != nil {
		return launchResult{}, fmt.Errorf("%w; fallback failed: %v", err, fallbackErr)
	}
	return launchResult{Opened: "备用地址", PauseBeforeReturn: req.PauseBeforeReturn}, nil
}

func (darwinPlatform) PressSpace() error {
	return exec.Command("osascript", "-e", `tell application "System Events" to key code 49`).Run()
}

func (darwinPlatform) Focus(handle windowHandle) error {
	if handle == 0 {
		return fmt.Errorf("CLI window handle is empty")
	}
	script := fmt.Sprintf(
		`tell application "System Events" to set frontmost of first application process whose unix id is %d to true`,
		uintptr(handle),
	)
	if out, err := exec.Command("osascript", "-e", script).CombinedOutput(); err != nil {
		return fmt.Errorf("macOS could not focus window: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func darwinOpenConfiguredTarget(target string) (string, error) {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(target)), "preset:") {
		preset, ok := darwinLaunchPresetFromTarget(target)
		if !ok {
			return "", fmt.Errorf("unknown preset target: %s", target)
		}
		return darwinOpenLaunchPreset(preset, "")
	}
	if darwinIsURLTarget(target) {
		return target, darwinShellOpen(target)
	}
	resolved, ok := darwinResolveLocalTarget(target)
	if !ok {
		return "", fmt.Errorf("target not found: %s", target)
	}
	return resolved, darwinShellOpen(resolved)
}

func darwinLaunchPresetFromTarget(target string) (darwinLaunchPreset, bool) {
	key, ok := strings.CutPrefix(strings.ToLower(strings.TrimSpace(target)), "preset:")
	if !ok {
		return darwinLaunchPreset{}, false
	}
	return darwinLaunchPresetFor(key)
}

func darwinLaunchPresetFor(key string) (darwinLaunchPreset, bool) {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "douyin":
		return darwinLaunchPreset{
			key:         "douyin",
			desktopName: "抖音桌面端",
			webName:     "抖音官网",
			fallbackURL: "https://www.douyin.com",
			candidates: []string{
				"/Applications/Douyin.app",
				filepath.Join(homeDir(), "Applications", "Douyin.app"),
				"/Applications/抖音.app",
				filepath.Join(homeDir(), "Applications", "抖音.app"),
			},
		}, true
	case "bilibili":
		return darwinLaunchPreset{
			key:         "bilibili",
			desktopName: "哔哩哔哩桌面端",
			webName:     "哔哩哔哩官网",
			fallbackURL: "https://www.bilibili.com",
			candidates: []string{
				"/Applications/bilibili.app",
				filepath.Join(homeDir(), "Applications", "bilibili.app"),
				"/Applications/哔哩哔哩.app",
				filepath.Join(homeDir(), "Applications", "哔哩哔哩.app"),
			},
		}, true
	case "linuxdo", "linux-do", "linux.do":
		return darwinLaunchPreset{
			key:         "linuxdo",
			desktopName: "Linux.do",
			webName:     "Linux.do",
			fallbackURL: darwinLinuxDOURL,
		}, true
	default:
		return darwinLaunchPreset{}, false
	}
}

func darwinOpenLaunchPreset(preset darwinLaunchPreset, fallbackURL string) (string, error) {
	if target := darwinFindFirstExisting(preset.candidates); target != "" {
		return preset.desktopName, darwinShellOpen(target)
	}
	targetURL := strings.TrimSpace(fallbackURL)
	if targetURL == "" {
		targetURL = preset.fallbackURL
	}
	if targetURL == "" {
		return "", fmt.Errorf("preset target not found: %s", preset.key)
	}
	return preset.webName, darwinShellOpen(targetURL)
}

func darwinOpenLinuxDOInBrowser(req launchRequest) (string, error) {
	targetURL := darwinLinuxDOTargetURL(req.Target)
	spec, ok := darwinBrowserSpecFor(req.Browser)
	if !ok || spec.key == config.TaskAutomationBrowserDefault {
		if err := darwinShellOpen(targetURL); err != nil {
			return "", err
		}
		return "Linux.do", nil
	}
	args := darwinBrowserLaunchArgs(req, targetURL, spec)
	if err := darwinOpenAppWithArgs(spec, args); err != nil {
		return "", err
	}
	darwinFocusAppSoon(spec.name)
	if strings.TrimSpace(req.BrowserProfile) != "" {
		return fmt.Sprintf("Linux.do（%s / %s）", spec.name, req.BrowserProfile), nil
	}
	return fmt.Sprintf("Linux.do（%s）", spec.name), nil
}

func darwinOpenAppWithArgs(spec darwinBrowserSpec, args []string) error {
	if appPath := darwinFindFirstExisting(spec.candidates); appPath != "" {
		openArgs := append([]string{"-na", appPath, "--args"}, args...)
		return exec.Command("open", openArgs...).Start()
	}
	openArgs := append([]string{"-na", spec.name, "--args"}, args...)
	return exec.Command("open", openArgs...).Start()
}

func darwinBrowserLaunchArgs(req launchRequest, targetURL string, spec darwinBrowserSpec) []string {
	if spec.chromium {
		args := []string{}
		if userData := strings.TrimSpace(req.BrowserUserData); userData != "" {
			args = append(args, "--user-data-dir="+expandUserPath(userData))
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
			expanded := expandUserPath(profile)
			if darwinLooksLikePath(expanded) || darwinExists(expanded) {
				args = append(args, "-profile", expanded)
			} else {
				args = append(args, "-P", profile)
			}
		}
		return append(args, "-new-window", targetURL)
	}
	return []string{targetURL}
}

func darwinBrowserSpecFor(browser string) (darwinBrowserSpec, bool) {
	switch darwinNormalizeBrowserKey(browser) {
	case config.TaskAutomationBrowserDefault:
		return darwinBrowserSpec{key: config.TaskAutomationBrowserDefault, name: "默认浏览器"}, true
	case config.TaskAutomationBrowserEdge:
		return darwinBrowserSpec{
			key:      config.TaskAutomationBrowserEdge,
			name:     "Microsoft Edge",
			chromium: true,
			candidates: []string{
				"/Applications/Microsoft Edge.app",
				filepath.Join(homeDir(), "Applications", "Microsoft Edge.app"),
			},
		}, true
	case config.TaskAutomationBrowserChrome:
		return darwinBrowserSpec{
			key:      config.TaskAutomationBrowserChrome,
			name:     "Google Chrome",
			chromium: true,
			candidates: []string{
				"/Applications/Google Chrome.app",
				filepath.Join(homeDir(), "Applications", "Google Chrome.app"),
			},
		}, true
	case config.TaskAutomationBrowserFirefox:
		return darwinBrowserSpec{
			key:     config.TaskAutomationBrowserFirefox,
			name:    "Firefox",
			firefox: true,
			candidates: []string{
				"/Applications/Firefox.app",
				filepath.Join(homeDir(), "Applications", "Firefox.app"),
			},
		}, true
	default:
		return darwinBrowserSpec{}, false
	}
}

func darwinFocusAppSoon(appName string) {
	appName = strings.TrimSpace(appName)
	if appName == "" || appName == "默认浏览器" {
		return
	}
	for _, delay := range []time.Duration{300 * time.Millisecond, 900 * time.Millisecond, 1600 * time.Millisecond} {
		delay := delay
		time.AfterFunc(delay, func() {
			_ = exec.Command("osascript", "-e", fmt.Sprintf(`tell application %q to activate`, appName)).Run()
		})
	}
}

func darwinNormalizeLaunchRequest(req launchRequest) launchRequest {
	req.Mode = strings.ToLower(strings.TrimSpace(req.Mode))
	req.Target = strings.TrimSpace(req.Target)
	req.FallbackURL = strings.TrimSpace(req.FallbackURL)
	req.Browser = darwinNormalizeBrowserKey(req.Browser)
	req.BrowserUserData = strings.TrimSpace(req.BrowserUserData)
	req.BrowserProfile = strings.TrimSpace(req.BrowserProfile)
	return req
}

func darwinNormalizeBrowserKey(browser string) string {
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

func darwinIsLinuxDOPreset(target string) bool {
	preset, ok := darwinLaunchPresetFromTarget(target)
	return ok && preset.key == "linuxdo"
}

func darwinLinuxDOTargetURL(target string) string {
	target = strings.TrimSpace(target)
	if parsed, err := url.Parse(target); err == nil {
		switch strings.ToLower(parsed.Scheme) {
		case "http", "https":
			return target
		}
	}
	return darwinLinuxDOURL
}

func darwinIsURLTarget(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme != "" && len(parsed.Scheme) > 1
}

func darwinResolveLocalTarget(target string) (string, bool) {
	if target == "" {
		return "", false
	}
	target = expandUserPath(target)
	if darwinExists(target) {
		return target, true
	}
	if resolved, err := exec.LookPath(target); err == nil && resolved != "" {
		return resolved, true
	}
	return "", false
}

func darwinShellOpen(target string) error {
	return exec.Command("open", expandUserPath(target)).Start()
}

func darwinFindFirstExisting(candidates []string) string {
	for _, candidate := range candidates {
		if darwinExists(candidate) {
			return candidate
		}
	}
	return ""
}

func darwinExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	_, err := os.Stat(expandUserPath(path))
	return err == nil
}

func darwinLooksLikePath(value string) bool {
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

func expandUserPath(value string) string {
	value = strings.TrimSpace(os.ExpandEnv(value))
	if strings.HasPrefix(value, "~/") {
		return filepath.Join(homeDir(), strings.TrimPrefix(value, "~/"))
	}
	return value
}

func homeDir() string {
	home, _ := os.UserHomeDir()
	return home
}
