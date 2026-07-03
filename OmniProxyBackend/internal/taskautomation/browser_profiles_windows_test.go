//go:build windows

package taskautomation

import (
	"os"
	"path/filepath"
	"testing"

	"omniproxy/internal/config"
)

func TestChromiumBrowserProfiles(t *testing.T) {
	userDataDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(userDataDir, "Default"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(userDataDir, "Profile 1"), 0o755); err != nil {
		t.Fatal(err)
	}
	localState := `{"profile":{"info_cache":{"Default":{"name":"Default","user_name":"main@example.com"},"Profile 1":{"name":"工作","user_name":"work@example.com"}}}}`
	if err := os.WriteFile(filepath.Join(userDataDir, "Local State"), []byte(localState), 0o600); err != nil {
		t.Fatal(err)
	}

	profiles, err := chromiumBrowserProfiles(config.TaskAutomationBrowserEdge, "Microsoft Edge", userDataDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(profiles))
	}
	if profiles[0].Profile != "Default" || profiles[0].UserDataDir != userDataDir || !profiles[0].IsDefault {
		t.Fatalf("unexpected default profile: %#v", profiles[0])
	}
	if profiles[1].Label != "工作 (Profile 1) - work@example.com" {
		t.Fatalf("unexpected profile label: %q", profiles[1].Label)
	}
}

func TestParseFirefoxProfilesINI(t *testing.T) {
	baseDir := t.TempDir()
	profilePath := filepath.Join(baseDir, "Profiles", "abc.default-release")
	if err := os.MkdirAll(profilePath, 0o755); err != nil {
		t.Fatal(err)
	}
	data := `[Install308046B0AF4A39CB]
Default=Profiles/abc.default-release

[Profile0]
Name=default-release
IsRelative=1
Path=Profiles/abc.default-release
Default=1
`

	profiles := parseFirefoxProfilesINI(data, baseDir)
	if len(profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(profiles))
	}
	if profiles[0].Name != "default-release" || profiles[0].Path != profilePath || !profiles[0].IsDefault {
		t.Fatalf("unexpected firefox profile: %#v", profiles[0])
	}
}
