//go:build !windows

package taskautomation

func ListBrowserProfiles(string) ([]BrowserProfile, error) {
	return []BrowserProfile{}, nil
}
