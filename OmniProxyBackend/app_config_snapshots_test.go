package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"omniproxy/internal/config"
	"omniproxy/internal/logs"
	"omniproxy/internal/storage"
	"omniproxy/internal/token"
)

func TestConfigSnapshotsCreateListAndRestore(t *testing.T) {
	app := newConfigSnapshotTestServer(t)
	next := app.cfg
	next.ProxyPort = 3123
	if _, err := app.saveConfig(next); err != nil {
		t.Fatal(err)
	}
	snapshot, err := app.createConfigSnapshot("before-change")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.ID == "" || snapshot.Name != "before-change" {
		t.Fatalf("unexpected snapshot summary: %#v", snapshot)
	}

	later := next
	later.ProxyPort = 3124
	if _, err := app.saveConfig(later); err != nil {
		t.Fatal(err)
	}
	restored, err := app.restoreConfigSnapshot(snapshot.ID)
	if err != nil {
		t.Fatal(err)
	}
	if restored.ProxyPort != 3123 {
		t.Fatalf("expected restored proxy port 3123, got %d", restored.ProxyPort)
	}

	items, err := app.listConfigSnapshots()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != snapshot.ID {
		t.Fatalf("expected one listed snapshot, got %#v", items)
	}
	if err := app.deleteConfigSnapshot(snapshot.ID); err != nil {
		t.Fatal(err)
	}
	items, err = app.listConfigSnapshots()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("expected snapshot to be deleted, got %#v", items)
	}
}

func TestConfigImportAcceptsBundleAndRawConfig(t *testing.T) {
	app := newConfigSnapshotTestServer(t)
	cfg := config.Default()
	cfg.ProxyPort = 3456
	bundle := configExportBundle{Version: 1, Config: cfg}
	data, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	result, err := app.importConfigBundleBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if result.Config.ProxyPort != 3456 {
		t.Fatalf("expected imported bundle config, got %#v", result.Config)
	}

	raw := config.Default()
	raw.ProxyPort = 4567
	data, err = json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	result, err = app.importConfigBundleBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if result.Config.ProxyPort != 4567 {
		t.Fatalf("expected imported raw config, got %#v", result.Config)
	}
}

func TestWriteConfigExportBundleAddsJSONExtension(t *testing.T) {
	app := newConfigSnapshotTestServer(t)
	target := filepath.Join(t.TempDir(), "backup")
	result, err := app.writeConfigExportBundle(target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(result.Path, ".json") {
		t.Fatalf("expected json extension, got %q", result.Path)
	}
	if _, err := os.Stat(result.Path); err != nil {
		t.Fatal(err)
	}
}

func newConfigSnapshotTestServer(t *testing.T) *appServer {
	t.Helper()
	dataDir := t.TempDir()
	manager, err := token.NewManager(storage.NewJSONStore[[]token.Token](filepath.Join(dataDir, "tokens.json")), 15)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	return &appServer{
		dataDir:     dataDir,
		cfg:         cfg,
		configStore: config.NewStore(filepath.Join(dataDir, "config.json")),
		tokens:      manager,
		logs:        logs.NewRecorder(10),
	}
}
