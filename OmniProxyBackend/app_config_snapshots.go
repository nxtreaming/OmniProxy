package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"omniproxy/internal/config"
	"omniproxy/internal/logs"
)

const configExportVersion = 1

type configSnapshotSummary struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
}

type configSnapshot struct {
	Version   int           `json:"version"`
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	CreatedAt string        `json:"createdAt"`
	Config    config.Config `json:"config"`
}

type configExportBundle struct {
	Version    int           `json:"version"`
	ExportedAt string        `json:"exportedAt"`
	Config     config.Config `json:"config"`
}

type configExportResult struct {
	Path     string `json:"path,omitempty"`
	FileName string `json:"fileName,omitempty"`
	Size     int64  `json:"size,omitempty"`
}

type configImportResult struct {
	Config  config.Config `json:"config"`
	Message string        `json:"message"`
}

func (a *appServer) listConfigSnapshots() ([]configSnapshotSummary, error) {
	dir, err := a.configSnapshotDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []configSnapshotSummary{}, nil
		}
		return nil, err
	}
	out := make([]configSnapshotSummary, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		snapshot, err := readConfigSnapshot(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		out = append(out, configSnapshotSummary{
			ID:        snapshot.ID,
			Name:      snapshot.Name,
			CreatedAt: snapshot.CreatedAt,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].CreatedAt > out[j].CreatedAt
	})
	return out, nil
}

func (a *appServer) createConfigSnapshot(name string) (configSnapshotSummary, error) {
	dir, err := a.configSnapshotDir()
	if err != nil {
		return configSnapshotSummary{}, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return configSnapshotSummary{}, err
	}
	now := time.Now()
	id, err := newConfigSnapshotID(now)
	if err != nil {
		return configSnapshotSummary{}, err
	}
	a.mu.Lock()
	cfg := a.cfg
	a.mu.Unlock()
	snapshot := configSnapshot{
		Version:   configExportVersion,
		ID:        id,
		Name:      normalizeConfigSnapshotName(name, now),
		CreatedAt: timeString(now),
		Config:    config.Normalize(cfg),
	}
	path := filepath.Join(dir, id+".json")
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return configSnapshotSummary{}, err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return configSnapshotSummary{}, err
	}
	a.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: "configuration snapshot created"})
	return configSnapshotSummary{ID: snapshot.ID, Name: snapshot.Name, CreatedAt: snapshot.CreatedAt}, nil
}

func (a *appServer) restoreConfigSnapshot(id string) (config.Config, error) {
	snapshot, err := a.loadConfigSnapshot(id)
	if err != nil {
		return config.Config{}, err
	}
	cfg, err := a.saveConfig(snapshot.Config)
	if err != nil {
		return config.Config{}, err
	}
	a.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: "configuration snapshot restored"})
	return cfg, nil
}

func (a *appServer) deleteConfigSnapshot(id string) error {
	path, err := a.configSnapshotPath(id)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	a.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: "configuration snapshot deleted"})
	return nil
}

func (a *appServer) configExportBundle() configExportBundle {
	a.mu.Lock()
	cfg := a.cfg
	a.mu.Unlock()
	return configExportBundle{
		Version:    configExportVersion,
		ExportedAt: timeString(time.Now()),
		Config:     config.Normalize(cfg),
	}
}

func (a *appServer) writeConfigExportBundle(path string) (configExportResult, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return configExportResult{}, errors.New("export path is empty")
	}
	if strings.ToLower(filepath.Ext(path)) != ".json" {
		path += ".json"
	}
	data, err := json.MarshalIndent(a.configExportBundle(), "", "  ")
	if err != nil {
		return configExportResult{}, err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return configExportResult{}, err
	}
	return configExportResult{Path: path, FileName: filepath.Base(path), Size: int64(len(data))}, nil
}

func configExportFilename() string {
	return "omniproxy-config-" + time.Now().Format("20060102-150405") + ".json"
}

func (a *appServer) importConfigBundleBytes(data []byte) (configImportResult, error) {
	cfg, err := parseConfigImport(data)
	if err != nil {
		return configImportResult{}, err
	}
	saved, err := a.saveConfig(cfg)
	if err != nil {
		return configImportResult{}, err
	}
	a.logs.Add(logs.Entry{Level: logs.LevelInfo, Message: "configuration bundle imported"})
	return configImportResult{Config: saved, Message: "配置已导入并保存"}, nil
}

func parseConfigImport(data []byte) (config.Config, error) {
	var bundle configExportBundle
	if err := json.Unmarshal(data, &bundle); err == nil && bundle.Config.ProxyPort > 0 {
		return config.Normalize(bundle.Config), nil
	}
	var cfg config.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return config.Config{}, fmt.Errorf("invalid config bundle: %w", err)
	}
	if cfg.ProxyPort <= 0 {
		return config.Config{}, errors.New("invalid config bundle")
	}
	return config.Normalize(cfg), nil
}

func (a *appServer) loadConfigSnapshot(id string) (configSnapshot, error) {
	path, err := a.configSnapshotPath(id)
	if err != nil {
		return configSnapshot{}, err
	}
	return readConfigSnapshot(path)
}

func readConfigSnapshot(path string) (configSnapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return configSnapshot{}, err
	}
	var snapshot configSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return configSnapshot{}, err
	}
	if snapshot.ID == "" || snapshot.Config.ProxyPort <= 0 {
		return configSnapshot{}, errors.New("invalid config snapshot")
	}
	snapshot.Config = config.Normalize(snapshot.Config)
	return snapshot, nil
}

func (a *appServer) configSnapshotPath(id string) (string, error) {
	cleanID := cleanConfigSnapshotID(id)
	if cleanID == "" {
		return "", errors.New("snapshot id is required")
	}
	dir, err := a.configSnapshotDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, cleanID+".json"), nil
}

func (a *appServer) configSnapshotDir() (string, error) {
	a.mu.Lock()
	dataDir := a.dataDir
	a.mu.Unlock()
	if strings.TrimSpace(dataDir) == "" {
		return "", errors.New("data directory is not ready")
	}
	return filepath.Join(dataDir, "config_snapshots"), nil
}

func newConfigSnapshotID(now time.Time) (string, error) {
	var suffix [3]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", err
	}
	return now.Format("20060102-150405") + "-" + hex.EncodeToString(suffix[:]), nil
}

func normalizeConfigSnapshotName(name string, now time.Time) string {
	name = strings.TrimSpace(name)
	if name != "" {
		return name
	}
	return "配置快照 " + now.Format("2006-01-02 15:04")
}

func cleanConfigSnapshotID(id string) string {
	id = strings.TrimSpace(id)
	id = strings.TrimSuffix(id, ".json")
	if id == "" || strings.ContainsAny(id, `/\`) {
		return ""
	}
	return id
}
