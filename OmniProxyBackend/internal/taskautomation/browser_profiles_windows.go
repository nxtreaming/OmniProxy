//go:build windows

package taskautomation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"OmniProxyBackend/internal/config"
)

type chromiumProfileMeta struct {
	Name     string `json:"name"`
	UserName string `json:"user_name"`
}

type firefoxProfileEntry struct {
	Name      string
	Path      string
	IsDefault bool
}

func ListBrowserProfiles(browser string) ([]BrowserProfile, error) {
	key := normalizeBrowserProfileKey(browser)
	if key == config.TaskAutomationBrowserDefault {
		var all []BrowserProfile
		for _, browserKey := range []string{
			config.TaskAutomationBrowserEdge,
			config.TaskAutomationBrowserChrome,
			config.TaskAutomationBrowserFirefox,
		} {
			profiles, err := ListBrowserProfiles(browserKey)
			if err != nil {
				return nil, err
			}
			all = append(all, profiles...)
		}
		return all, nil
	}

	switch key {
	case config.TaskAutomationBrowserEdge:
		return chromiumBrowserProfiles(key, "Microsoft Edge", envPath("LOCALAPPDATA", "Microsoft", "Edge", "User Data"))
	case config.TaskAutomationBrowserChrome:
		return chromiumBrowserProfiles(key, "Google Chrome", envPath("LOCALAPPDATA", "Google", "Chrome", "User Data"))
	case config.TaskAutomationBrowserFirefox:
		return firefoxBrowserProfiles()
	default:
		return []BrowserProfile{}, nil
	}
}

func chromiumBrowserProfiles(browser string, browserLabel string, userDataDir string) ([]BrowserProfile, error) {
	if strings.TrimSpace(userDataDir) == "" {
		return []BrowserProfile{}, nil
	}
	entries, err := os.ReadDir(userDataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []BrowserProfile{}, nil
		}
		return nil, err
	}

	infoCache := readChromiumInfoCache(filepath.Join(userDataDir, "Local State"))
	profiles := make([]BrowserProfile, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		profilePath := filepath.Join(userDataDir, name)
		if !isChromiumProfileDir(profilePath, name, infoCache) {
			continue
		}
		meta := infoCache[name]
		profiles = append(profiles, BrowserProfile{
			Browser:      browser,
			BrowserLabel: browserLabel,
			Name:         name,
			Label:        chromiumProfileLabel(name, meta),
			Account:      strings.TrimSpace(meta.UserName),
			UserDataDir:  userDataDir,
			Profile:      name,
			Path:         profilePath,
			IsDefault:    name == "Default",
		})
	}
	sortBrowserProfiles(profiles)
	return profiles, nil
}

func readChromiumInfoCache(path string) map[string]chromiumProfileMeta {
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]chromiumProfileMeta{}
	}
	var localState struct {
		Profile struct {
			InfoCache map[string]chromiumProfileMeta `json:"info_cache"`
		} `json:"profile"`
	}
	if err := json.Unmarshal(data, &localState); err != nil || localState.Profile.InfoCache == nil {
		return map[string]chromiumProfileMeta{}
	}
	return localState.Profile.InfoCache
}

func isChromiumProfileDir(path string, name string, infoCache map[string]chromiumProfileMeta) bool {
	if name == "System Profile" || name == "Guest Profile" {
		return false
	}
	if name == "Default" || strings.HasPrefix(name, "Profile ") {
		return true
	}
	if _, ok := infoCache[name]; ok {
		return true
	}
	return exists(filepath.Join(path, "Preferences"))
}

func chromiumProfileLabel(name string, meta chromiumProfileMeta) string {
	label := strings.TrimSpace(meta.Name)
	if label == "" {
		label = name
	} else if label != name {
		label += " (" + name + ")"
	}
	if account := strings.TrimSpace(meta.UserName); account != "" {
		label += " - " + account
	}
	return label
}

func firefoxBrowserProfiles() ([]BrowserProfile, error) {
	firefoxDir := envPath("APPDATA", "Mozilla", "Firefox")
	if strings.TrimSpace(firefoxDir) == "" {
		return []BrowserProfile{}, nil
	}
	profiles := firefoxProfilesFromINI(filepath.Join(firefoxDir, "profiles.ini"), firefoxDir)
	if len(profiles) == 0 {
		fallbackProfiles, err := firefoxProfilesFromDirectory(filepath.Join(firefoxDir, "Profiles"))
		if err != nil {
			return nil, err
		}
		profiles = fallbackProfiles
	}

	result := make([]BrowserProfile, 0, len(profiles))
	for _, profile := range profiles {
		label := strings.TrimSpace(profile.Name)
		if label == "" {
			label = filepath.Base(profile.Path)
		}
		if profile.IsDefault {
			label += " (默认)"
		}
		result = append(result, BrowserProfile{
			Browser:      config.TaskAutomationBrowserFirefox,
			BrowserLabel: "Firefox",
			Name:         strings.TrimSpace(profile.Name),
			Label:        label,
			UserDataDir:  "",
			Profile:      profile.Path,
			Path:         profile.Path,
			IsDefault:    profile.IsDefault,
		})
	}
	sortBrowserProfiles(result)
	return result, nil
}

func firefoxProfilesFromINI(path string, baseDir string) []firefoxProfileEntry {
	data, err := os.ReadFile(path)
	if err != nil {
		return []firefoxProfileEntry{}
	}
	return parseFirefoxProfilesINI(string(data), baseDir)
}

func parseFirefoxProfilesINI(data string, baseDir string) []firefoxProfileEntry {
	var profiles []firefoxProfileEntry
	current := map[string]string{}
	inProfile := false

	flush := func() {
		if !inProfile {
			return
		}
		profilePath := strings.TrimSpace(current["path"])
		if profilePath == "" {
			current = map[string]string{}
			inProfile = false
			return
		}
		if current["isrelative"] == "1" {
			profilePath = filepath.Join(baseDir, profilePath)
		}
		if !exists(profilePath) {
			current = map[string]string{}
			inProfile = false
			return
		}
		profiles = append(profiles, firefoxProfileEntry{
			Name:      strings.TrimSpace(current["name"]),
			Path:      profilePath,
			IsDefault: current["default"] == "1",
		})
		current = map[string]string{}
		inProfile = false
	}

	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			flush()
			section := strings.TrimSpace(line[1 : len(line)-1])
			inProfile = strings.HasPrefix(strings.ToLower(section), "profile")
			continue
		}
		if !inProfile {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		current[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(value)
	}
	flush()
	return profiles
}

func firefoxProfilesFromDirectory(profilesDir string) ([]firefoxProfileEntry, error) {
	entries, err := os.ReadDir(profilesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []firefoxProfileEntry{}, nil
		}
		return nil, err
	}
	profiles := make([]firefoxProfileEntry, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		profiles = append(profiles, firefoxProfileEntry{
			Name: entry.Name(),
			Path: filepath.Join(profilesDir, entry.Name()),
		})
	}
	return profiles, nil
}

func sortBrowserProfiles(profiles []BrowserProfile) {
	sort.SliceStable(profiles, func(i, j int) bool {
		if profiles[i].IsDefault != profiles[j].IsDefault {
			return profiles[i].IsDefault
		}
		left := strings.ToLower(profiles[i].Label)
		right := strings.ToLower(profiles[j].Label)
		if left == right {
			return strings.ToLower(profiles[i].Path) < strings.ToLower(profiles[j].Path)
		}
		return left < right
	})
}
