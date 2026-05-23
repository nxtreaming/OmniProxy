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
}
