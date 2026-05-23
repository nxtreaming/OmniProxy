//go:build windows

package taskautomation

import "testing"

func TestLaunchPresetFromTarget(t *testing.T) {
	preset, ok := launchPresetFromTarget("preset:BILIBILI")
	if !ok {
		t.Fatal("expected bilibili preset")
	}
	if preset.key != "bilibili" || preset.fallbackURL != "https://www.bilibili.com" {
		t.Fatalf("unexpected bilibili preset: %#v", preset)
	}

	if _, ok := launchPresetFromTarget("https://www.bilibili.com"); ok {
		t.Fatal("expected normal URL not to be treated as a preset")
	}

	preset, ok = launchPresetFromTarget("preset:linuxdo")
	if !ok {
		t.Fatal("expected linux.do preset")
	}
	if preset.key != "linuxdo" || preset.fallbackURL != "https://linux.do/" {
		t.Fatalf("unexpected linux.do preset: %#v", preset)
	}
}

func TestBrowserLaunchArgs(t *testing.T) {
	spec, ok := browserSpecFor("edge")
	if !ok {
		t.Fatal("expected edge browser spec")
	}
	args := browserLaunchArgs(launchRequest{
		BrowserUserData: `%LOCALAPPDATA%\Microsoft\Edge\User Data`,
		BrowserProfile:  "Profile 1",
	}, linuxDOURL, spec)
	if len(args) != 4 || args[1] != "--profile-directory=Profile 1" || args[2] != "--new-window" || args[3] != linuxDOURL {
		t.Fatalf("unexpected chromium args: %#v", args)
	}

	spec, ok = browserSpecFor("firefox")
	if !ok {
		t.Fatal("expected firefox browser spec")
	}
	args = browserLaunchArgs(launchRequest{BrowserProfile: "work"}, linuxDOURL, spec)
	if len(args) != 4 || args[0] != "-P" || args[1] != "work" || args[2] != "-new-window" || args[3] != linuxDOURL {
		t.Fatalf("unexpected firefox args: %#v", args)
	}
}
