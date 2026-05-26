//go:build !windows && !darwin

package taskautomation

func ListBrowserProfiles(string) ([]BrowserProfile, error) {
	return []BrowserProfile{}, nil
}
