package storage

import (
	"os"
	"path/filepath"
	"testing"
)

type jsonStoreTestItem struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func TestJSONStoreLoadMissingAndSaveRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "items.json")
	store := NewJSONStore[[]jsonStoreTestItem](path)

	missing, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if missing != nil {
		t.Fatalf("expected nil slice for missing file, got %#v", missing)
	}

	want := []jsonStoreTestItem{{Name: "alpha", Count: 2}}
	if err := store.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("unexpected loaded value: %#v", got)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temporary file should not remain after save, stat err=%v", err)
	}
}

func TestJSONStoreLoadEmptyAndInvalidFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "items.json")
	store := NewJSONStore[jsonStoreTestItem](path)

	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	empty, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if empty != (jsonStoreTestItem{}) {
		t.Fatalf("expected zero value for empty file, got %#v", empty)
	}

	if err := os.WriteFile(path, []byte(`{"name":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("expected invalid JSON to return an error")
	}
}
