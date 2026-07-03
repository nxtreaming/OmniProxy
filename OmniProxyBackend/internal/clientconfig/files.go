package clientconfig

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func ReadJSONObject(path string) (map[string]any, error) {
	data := map[string]any{}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return data, nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return data, nil
	}
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func WriteJSONObject(path string, data map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o600)
}

func BackupFile(path string, backupPath string, missingContent []byte) error {
	if _, err := os.Stat(backupPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		raw = missingContent
	}
	if err := os.MkdirAll(filepath.Dir(backupPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(backupPath, raw, 0o600)
}

func RestoreBackup(path string, backupPath string) error {
	raw, err := os.ReadFile(backupPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}
