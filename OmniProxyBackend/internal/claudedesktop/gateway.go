package claudedesktop

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	ProfileID    = "00000000-0000-4000-8000-000000157210"
	ProfileName  = "OmniProxy"
	GatewayToken = "omniproxy-claude-desktop"
	GatewayPath  = "/claude-desktop"
)

type Paths struct {
	NormalConfigPath  string
	ThreePConfigPath  string
	ConfigLibraryPath string
	ProfilePath       string
	MetaPath          string
	RoutesPath        string
}

type ModelRoute struct {
	RouteID       string `json:"routeId"`
	UpstreamModel string `json:"upstreamModel"`
	LabelOverride string `json:"labelOverride,omitempty"`
	Supports1M    bool   `json:"supports1m,omitempty"`
}

func CurrentPaths() (Paths, error) {
	switch runtime.GOOS {
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if strings.TrimSpace(localAppData) == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return Paths{}, err
			}
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		base := filepath.Clean(localAppData)
		normalDir := pickWindowsClaudeDir(base, false)
		threePDir := pickWindowsClaudeDir(base, true)
		if normalDir == "" {
			normalDir = filepath.Join(base, "Claude")
		}
		if threePDir == "" {
			threePDir = filepath.Join(base, "Claude-3p")
		}
		return pathsFromDirs(normalDir, threePDir), nil
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return Paths{}, err
		}
		appSupport := filepath.Join(home, "Library", "Application Support")
		return pathsFromDirs(filepath.Join(appSupport, "Claude"), filepath.Join(appSupport, "Claude-3p")), nil
	default:
		return Paths{}, fmt.Errorf("Claude Desktop 3P profile is not supported on %s", runtime.GOOS)
	}
}

func pickWindowsClaudeDir(localAppData string, threeP bool) string {
	exactName := "Claude"
	if threeP {
		exactName = "Claude-3p"
	}
	exact := filepath.Join(localAppData, exactName)
	if info, err := os.Stat(exact); err == nil && info.IsDir() {
		return exact
	}

	entries, err := os.ReadDir(localAppData)
	if err != nil {
		return ""
	}
	prefix := "Claude"
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		isThreeP := strings.Contains(name, "-3p")
		if strings.HasPrefix(name, prefix) && isThreeP == threeP {
			return filepath.Join(localAppData, name)
		}
	}
	return ""
}

func pathsFromDirs(normalDir string, threePDir string) Paths {
	configLibraryPath := filepath.Join(threePDir, "configLibrary")
	return Paths{
		NormalConfigPath:  filepath.Join(normalDir, "claude_desktop_config.json"),
		ThreePConfigPath:  filepath.Join(threePDir, "claude_desktop_config.json"),
		ConfigLibraryPath: configLibraryPath,
		ProfilePath:       filepath.Join(configLibraryPath, ProfileID+".json"),
		MetaPath:          filepath.Join(configLibraryPath, "_meta.json"),
		RoutesPath:        filepath.Join(configLibraryPath, ProfileID+".omniproxy-routes.json"),
	}
}

func GatewayBaseURL(port int) string {
	return fmt.Sprintf("http://127.0.0.1:%d%s", port, GatewayPath)
}

func IsGatewayPath(path string) bool {
	return path == GatewayPath || strings.HasPrefix(path, GatewayPath+"/")
}

func IsModelsPath(path string) bool {
	return path == GatewayPath+"/v1/models"
}

func IsMessagesPath(path string) bool {
	return path == GatewayPath+"/v1/messages"
}

func StripGatewayPath(path string) string {
	if path == GatewayPath {
		return "/"
	}
	return strings.TrimPrefix(path, GatewayPath)
}

func ValidateGatewayAuth(header http.Header) error {
	value := strings.TrimSpace(header.Get("Authorization"))
	token := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(value, "Bearer "), "bearer "))
	if token == "" {
		return errors.New("Claude Desktop gateway missing Authorization header")
	}
	if token != GatewayToken {
		return errors.New("Claude Desktop gateway token is invalid")
	}
	return nil
}

func BuildGatewayProfile(baseURL string, routes []ModelRoute) map[string]any {
	models := make([]any, 0, len(routes))
	for _, route := range routes {
		model := map[string]any{"name": route.RouteID}
		if strings.TrimSpace(route.LabelOverride) != "" {
			model["labelOverride"] = route.LabelOverride
		}
		if route.Supports1M {
			model["supports1m"] = true
		}
		models = append(models, model)
	}
	return map[string]any{
		"disableDeploymentModeChooser": true,
		"inferenceGatewayApiKey":       GatewayToken,
		"inferenceGatewayAuthScheme":   "bearer",
		"inferenceGatewayBaseUrl":      baseURL,
		"inferenceProvider":            "gateway",
		"inferenceModels":              models,
	}
}

func ModelListResponse(routes []ModelRoute) map[string]any {
	data := make([]any, 0, len(routes))
	for _, route := range routes {
		item := map[string]any{
			"type":       "model",
			"id":         route.RouteID,
			"created_at": "2024-01-01T00:00:00Z",
		}
		if route.Supports1M {
			item["supports1m"] = true
		}
		data = append(data, item)
	}
	firstID := ""
	lastID := ""
	if len(routes) > 0 {
		firstID = routes[0].RouteID
		lastID = routes[len(routes)-1].RouteID
	}
	return map[string]any{
		"data":     data,
		"has_more": false,
		"first_id": firstID,
		"last_id":  lastID,
	}
}

func LoadRoutes() ([]ModelRoute, error) {
	paths, err := CurrentPaths()
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(paths.RoutesPath)
	if err != nil {
		return nil, err
	}
	var routes []ModelRoute
	if err := json.Unmarshal(raw, &routes); err != nil {
		return nil, err
	}
	if len(routes) == 0 {
		return nil, errors.New("Claude Desktop gateway route mapping is empty")
	}
	return routes, nil
}

func WriteRoutes(path string, routes []ModelRoute) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(routes, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o600)
}

func RewriteRequestBody(body []byte, routes []ModelRoute) ([]byte, string, error) {
	var payload map[string]any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return nil, "", err
	}
	requested, _ := payload["model"].(string)
	requested = strings.TrimSpace(requested)
	if requested == "" {
		return nil, "", errors.New("Claude Desktop request is missing model")
	}
	for _, route := range routes {
		if route.RouteID == requested {
			payload["model"] = route.UpstreamModel
			raw, err := json.Marshal(payload)
			if err != nil {
				return nil, "", err
			}
			return raw, route.UpstreamModel, nil
		}
	}
	return nil, "", fmt.Errorf("Claude Desktop model route is not configured: %s", requested)
}
