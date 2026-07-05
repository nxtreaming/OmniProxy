package clientconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadWriteJSONObjectHandlesMissingBOMAndNestedDirectories(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "settings.json")

	empty, err := ReadJSONObject(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(empty) != 0 {
		t.Fatalf("expected empty object for missing file, got %#v", empty)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append([]byte{0xEF, 0xBB, 0xBF}, []byte(`{"security":{"auth":{"old":"keep"}}}`)...), 0o600); err != nil {
		t.Fatal(err)
	}
	data, err := ReadJSONObject(path)
	if err != nil {
		t.Fatal(err)
	}
	security, ok := data["security"].(map[string]any)
	if !ok || security["auth"] == nil {
		t.Fatalf("expected BOM-prefixed JSON to be decoded, got %#v", data)
	}

	data["added"] = "value"
	if err := WriteJSONObject(path, data); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if raw[len(raw)-1] != '\n' {
		t.Fatalf("expected written JSON to end with newline, got %q", raw)
	}
}

func TestBackupFileCreatesOnceAndRestoreBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "client.json")
	backupPath := path + ".omniproxy.bak"

	if err := BackupFile(path, backupPath, []byte("{}\n")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"current":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := BackupFile(path, backupPath, []byte("should-not-overwrite")); err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(backup) != "{}\n" {
		t.Fatalf("expected first backup content to be preserved, got %q", backup)
	}

	if err := RestoreBackup(path, backupPath); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(restored) != "{}\n" {
		t.Fatalf("expected restored backup content, got %q", restored)
	}
}
