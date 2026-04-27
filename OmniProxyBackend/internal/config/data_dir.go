package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const dataDirEnvVar = "OMNIPROXY_DATA_DIR"

type DataDirectoryInfo struct {
	DataDir       string `json:"dataDir"`
	BootstrapPath string `json:"bootstrapPath"`
	EnvOverride   bool   `json:"envOverride"`
	Source        string `json:"source"`
}

type DataDirectoryChangeResult struct {
	DataDir         string   `json:"dataDir"`
	PreviousDataDir string   `json:"previousDataDir"`
	BootstrapPath   string   `json:"bootstrapPath"`
	EnvOverride     bool     `json:"envOverride"`
	MigratedFiles   []string `json:"migratedFiles"`
	SkippedFiles    []string `json:"skippedFiles"`
	RestartRequired bool     `json:"restartRequired"`
	Cancelled       bool     `json:"cancelled"`
}

type dataDirectoryBootstrap struct {
	DataDir string `json:"dataDir"`
}

func DataDir() string {
	info, configured, err := ResolveDataDir()
	if err == nil && configured && info.DataDir != "" {
		return info.DataDir
	}
	return DefaultDataDir()
}

func DefaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "data"
	}
	return filepath.Join(home, ".omniproxy")
}

func BootstrapPath() string {
	if dir, err := os.UserConfigDir(); err == nil && strings.TrimSpace(dir) != "" {
		return filepath.Join(dir, "OmniProxy", "bootstrap.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join("data", "bootstrap.json")
	}
	return filepath.Join(home, ".omniproxy-bootstrap.json")
}

func ResolveDataDir() (DataDirectoryInfo, bool, error) {
	bootstrapPath := BootstrapPath()
	if envDir := strings.TrimSpace(os.Getenv(dataDirEnvVar)); envDir != "" {
		dir, err := cleanAbsPath(envDir)
		if err != nil {
			return DataDirectoryInfo{}, false, err
		}
		return DataDirectoryInfo{DataDir: dir, BootstrapPath: bootstrapPath, EnvOverride: true, Source: "env"}, true, nil
	}

	if dataDir, err := readBootstrapDataDir(bootstrapPath); err != nil {
		return DataDirectoryInfo{}, false, err
	} else if dataDir != "" {
		dir, err := cleanAbsPath(dataDir)
		if err != nil {
			return DataDirectoryInfo{}, false, err
		}
		return DataDirectoryInfo{DataDir: dir, BootstrapPath: bootstrapPath, Source: "bootstrap"}, true, nil
	}

	defaultDir := DefaultDataDir()
	if dataDirHasFiles(defaultDir) {
		dir, err := cleanAbsPath(defaultDir)
		if err != nil {
			return DataDirectoryInfo{}, false, err
		}
		_ = SaveBootstrap(dir)
		return DataDirectoryInfo{DataDir: dir, BootstrapPath: bootstrapPath, Source: "legacy"}, true, nil
	}

	dir, err := cleanAbsPath(defaultDir)
	if err != nil {
		return DataDirectoryInfo{}, false, err
	}
	return DataDirectoryInfo{DataDir: dir, BootstrapPath: bootstrapPath, Source: "unconfigured"}, false, nil
}

func SaveBootstrap(dataDir string) error {
	dir, err := cleanAbsPath(dataDir)
	if err != nil {
		return err
	}
	path := BootstrapPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(dataDirectoryBootstrap{DataDir: dir}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(payload, '\n'), 0o600)
}

func PrepareDataDir(dataDir string) (string, error) {
	dir, err := cleanAbsPath(dataDir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	probe, err := os.CreateTemp(dir, ".omniproxy-write-test-*")
	if err != nil {
		return "", fmt.Errorf("data directory is not writable: %w", err)
	}
	probePath := probe.Name()
	if _, err := probe.Write([]byte("ok")); err != nil {
		_ = probe.Close()
		_ = os.Remove(probePath)
		return "", fmt.Errorf("data directory is not writable: %w", err)
	}
	if err := probe.Close(); err != nil {
		_ = os.Remove(probePath)
		return "", fmt.Errorf("data directory is not writable: %w", err)
	}
	_ = os.Remove(probePath)
	return dir, nil
}

func CopyDataFiles(srcDir string, dstDir string) ([]string, []string, error) {
	src, err := cleanAbsPath(srcDir)
	if err != nil {
		return nil, nil, err
	}
	dst, err := cleanAbsPath(dstDir)
	if err != nil {
		return nil, nil, err
	}
	if samePath(src, dst) {
		return nil, nil, nil
	}

	var copied []string
	var skipped []string
	for _, name := range []string{"config.json", "tokens.json"} {
		srcPath := filepath.Join(src, name)
		dstPath := filepath.Join(dst, name)
		if _, err := os.Stat(srcPath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return copied, skipped, err
		}
		if _, err := os.Stat(dstPath); err == nil {
			skipped = append(skipped, name)
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return copied, skipped, err
		}
		if err := copyFile(srcPath, dstPath); err != nil {
			return copied, skipped, err
		}
		copied = append(copied, name)
	}
	return copied, skipped, nil
}

func readBootstrapDataDir(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	if len(data) == 0 {
		return "", nil
	}
	var bootstrap dataDirectoryBootstrap
	if err := json.Unmarshal(data, &bootstrap); err != nil {
		return "", err
	}
	return strings.TrimSpace(bootstrap.DataDir), nil
}

func dataDirHasFiles(dataDir string) bool {
	for _, name := range []string{"config.json", "tokens.json"} {
		if _, err := os.Stat(filepath.Join(dataDir, name)); err == nil {
			return true
		}
	}
	return false
}

func copyFile(src string, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func cleanAbsPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("data directory is empty")
	}
	return filepath.Abs(path)
}

func samePath(a string, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}
