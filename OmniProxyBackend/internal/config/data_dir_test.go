package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDataDirUsesBootstrap(t *testing.T) {
	home := t.TempDir()
	appData := t.TempDir()
	dataDir := filepath.Join(t.TempDir(), "chosen")
	setDataDirTestEnv(t, home, appData)

	if err := SaveBootstrap(dataDir); err != nil {
		t.Fatal(err)
	}

	info, configured, err := ResolveDataDir()
	if err != nil {
		t.Fatal(err)
	}
	if !configured {
		t.Fatal("expected data directory to be configured")
	}
	if info.Source != "bootstrap" {
		t.Fatalf("expected bootstrap source, got %q", info.Source)
	}
	if !samePath(info.DataDir, dataDir) {
		t.Fatalf("expected data dir %q, got %q", dataDir, info.DataDir)
	}
}

func TestResolveDataDirRequiresChoiceWithoutBootstrapOrLegacyData(t *testing.T) {
	home := t.TempDir()
	appData := t.TempDir()
	setDataDirTestEnv(t, home, appData)

	info, configured, err := ResolveDataDir()
	if err != nil {
		t.Fatal(err)
	}
	if configured {
		t.Fatalf("expected unconfigured data dir, got %#v", info)
	}
	if info.Source != "unconfigured" {
		t.Fatalf("expected unconfigured source, got %q", info.Source)
	}
	if !samePath(info.DataDir, filepath.Join(home, ".omniproxy")) {
		t.Fatalf("expected default data dir, got %q", info.DataDir)
	}
}

func TestResolveDataDirKeepsLegacyDefaultData(t *testing.T) {
	home := t.TempDir()
	appData := t.TempDir()
	setDataDirTestEnv(t, home, appData)
	legacyDir := filepath.Join(home, ".omniproxy")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "tokens.json"), []byte("[]"), 0o600); err != nil {
		t.Fatal(err)
	}

	info, configured, err := ResolveDataDir()
	if err != nil {
		t.Fatal(err)
	}
	if !configured {
		t.Fatal("expected legacy data directory to be configured")
	}
	if info.Source != "legacy" {
		t.Fatalf("expected legacy source, got %q", info.Source)
	}
	if !samePath(info.DataDir, legacyDir) {
		t.Fatalf("expected legacy data dir %q, got %q", legacyDir, info.DataDir)
	}
	if _, err := os.Stat(BootstrapPath()); err != nil {
		t.Fatalf("expected bootstrap to be written for legacy data: %v", err)
	}
}

func TestDevRuntimeProfileUsesSeparateDefaults(t *testing.T) {
	t.Cleanup(func() {
		SetRuntimeProfile(RuntimeProfileProduction)
	})
	SetRuntimeProfile(RuntimeProfileDev)

	home := t.TempDir()
	appData := t.TempDir()
	setDataDirTestEnv(t, home, appData)

	if DefaultProxyPort() != 3001 || DefaultControlPort() != 3891 {
		t.Fatalf("unexpected dev default ports: proxy=%d control=%d", DefaultProxyPort(), DefaultControlPort())
	}
	if !samePath(DefaultDataDir(), filepath.Join(home, ".omniproxy-dev")) {
		t.Fatalf("unexpected dev default data dir: %q", DefaultDataDir())
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(configDir, "OmniProxyDev", "bootstrap.json"); !samePath(BootstrapPath(), want) {
		t.Fatalf("expected dev bootstrap path %q, got %q", want, BootstrapPath())
	}

	envDir := filepath.Join(t.TempDir(), "dev-env-data")
	t.Setenv("OMNIPROXY_DEV_DATA_DIR", envDir)
	info, configured, err := ResolveDataDir()
	if err != nil {
		t.Fatal(err)
	}
	if !configured || !info.EnvOverride || info.Source != "env" {
		t.Fatalf("expected dev env data dir, got configured=%v info=%#v", configured, info)
	}
	if !samePath(info.DataDir, envDir) {
		t.Fatalf("expected dev env data dir %q, got %q", envDir, info.DataDir)
	}
}

func TestCopyDataFilesDoesNotOverwriteExistingTargetFiles(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "config.json"), []byte(`{"proxyPort":3000}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "tokens.json"), []byte(`[{"name":"source"}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, "tokens.json"), []byte(`[{"name":"target"}]`), 0o600); err != nil {
		t.Fatal(err)
	}

	copied, skipped, err := CopyDataFiles(src, dst)
	if err != nil {
		t.Fatal(err)
	}
	if len(copied) != 1 || copied[0] != "config.json" {
		t.Fatalf("unexpected copied files: %#v", copied)
	}
	if len(skipped) != 1 || skipped[0] != "tokens.json" {
		t.Fatalf("unexpected skipped files: %#v", skipped)
	}
	content, err := os.ReadFile(filepath.Join(dst, "tokens.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != `[{"name":"target"}]` {
		t.Fatalf("target tokens should not be overwritten, got %s", string(content))
	}
}

func setDataDirTestEnv(t *testing.T, home string, appData string) {
	t.Helper()
	t.Setenv("OMNIPROXY_DATA_DIR", "")
	t.Setenv("OMNIPROXY_DEV_DATA_DIR", "")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("APPDATA", appData)
	t.Setenv("XDG_CONFIG_HOME", appData)
}
