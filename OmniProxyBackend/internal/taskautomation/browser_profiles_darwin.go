//go:build darwin

package taskautomation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"OmniProxyBackend/internal/config"
)

type darwinChromiumProfileMeta struct {
	Name     string `json:"name"`
	UserName string `json:"user_name"`
}

type darwinFirefoxProfileEntry struct {
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
		return darwinChromiumBrowserProfiles(key, "Microsoft Edge", filepath.Join(homeDir(), "Library", "Application Support", "Microsoft Edge"))
	case config.TaskAutomationBrowserChrome:
		return darwinChromiumBrowserProfiles(key, "Google Chrome", filepath.Join(homeDir(), "Library", "Application Support", "Google", "Chrome"))
	case config.TaskAutomationBrowserFirefox:
		return darwinFirefoxBrowserProfiles()
	default:
		return []BrowserProfile{}, nil
	}
}

func darwinChromiumBrowserProfiles(browser string, browserLabel string, userDataDir string) ([]BrowserProfile, error) {
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

	infoCache := darwinReadChromiumInfoCache(filepath.Join(userDataDir, "Local State"))
	profiles := make([]BrowserProfile, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		profilePath := filepath.Join(userDataDir, name)
		if !darwinIsChromiumProfileDir(profilePath, name, infoCache) {
			continue
		}
		meta := infoCache[name]
		profiles = append(profiles, BrowserProfile{
			Browser:      browser,
			BrowserLabel: browserLabel,
			Name:         name,
			Label:        darwinChromiumProfileLabel(name, meta),
			Account:      strings.TrimSpace(meta.UserName),
			UserDataDir:  userDataDir,
			Profile:      name,
			Path:         profilePath,
			IsDefault:    name == "Default",
		})
	}
	darwinSortBrowserProfiles(profiles)
	return profiles, nil
}

func darwinReadChromiumInfoCache(path string) map[string]darwinChromiumProfileMeta {
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]darwinChromiumProfileMeta{}
	}
	var localState struct {
		Profile struct {
			InfoCache map[string]darwinChromiumProfileMeta `json:"info_cache"`
		} `json:"profile"`
	}
	if err := json.Unmarshal(data, &localState); err != nil || localState.Profile.InfoCache == nil {
		return map[string]darwinChromiumProfileMeta{}
	}
	return localState.Profile.InfoCache
}

func darwinIsChromiumProfileDir(path string, name string, infoCache map[string]darwinChromiumProfileMeta) bool {
	if name == "System Profile" || name == "Guest Profile" {
		return false
	}
	if name == "Default" || strings.HasPrefix(name, "Profile ") {
		return true
	}
	if _, ok := infoCache[name]; ok {
		return true
	}
	return darwinExists(filepath.Join(path, "Preferences"))
}

func darwinChromiumProfileLabel(name string, meta darwinChromiumProfileMeta) string {
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

func darwinFirefoxBrowserProfiles() ([]BrowserProfile, error) {
	firefoxDir := filepath.Join(homeDir(), "Library", "Application Support", "Firefox")
	profiles := darwinFirefoxProfilesFromINI(filepath.Join(firefoxDir, "profiles.ini"), firefoxDir)
	if len(profiles) == 0 {
		fallbackProfiles, err := darwinFirefoxProfilesFromDirectory(filepath.Join(firefoxDir, "Profiles"))
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
	darwinSortBrowserProfiles(result)
	return result, nil
}

func darwinFirefoxProfilesFromINI(path string, baseDir string) []darwinFirefoxProfileEntry {
	data, err := os.ReadFile(path)
	if err != nil {
		return []darwinFirefoxProfileEntry{}
	}
	return darwinParseFirefoxProfilesINI(string(data), baseDir)
}

func darwinParseFirefoxProfilesINI(data string, baseDir string) []darwinFirefoxProfileEntry {
	var profiles []darwinFirefoxProfileEntry
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
		if !darwinExists(profilePath) {
			current = map[string]string{}
			inProfile = false
			return
		}
		profiles = append(profiles, darwinFirefoxProfileEntry{
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

func darwinFirefoxProfilesFromDirectory(profilesDir string) ([]darwinFirefoxProfileEntry, error) {
	entries, err := os.ReadDir(profilesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []darwinFirefoxProfileEntry{}, nil
		}
		return nil, err
	}
	profiles := make([]darwinFirefoxProfileEntry, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		profiles = append(profiles, darwinFirefoxProfileEntry{
			Name: entry.Name(),
			Path: filepath.Join(profilesDir, entry.Name()),
		})
	}
	return profiles, nil
}

func darwinSortBrowserProfiles(profiles []BrowserProfile) {
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
