package main

import (
	"os"
	goruntime "runtime"
	"strconv"
	"strings"
	"time"
	"unicode"
)

var appVersion = "dev"
var appStartedAt = time.Now()

func currentAppInfo() appInfo {
	current := strings.TrimSpace(appVersion)
	if current == "" {
		current = "dev"
	}
	executablePath, _ := os.Executable()
	return appInfo{
		Name:           "OmniProxy",
		Version:        current,
		IsDevelopment:  isDevelopmentVersion(current),
		UpdateEndpoint: latestReleaseURL,
		Platform:       goruntime.GOOS + "/" + goruntime.GOARCH,
		GoVersion:      goruntime.Version(),
		ExecutablePath: executablePath,
		StartedAt:      appStartedAt.Format(time.RFC3339),
	}
}

func compareVersions(left string, right string) int {
	leftVersion, leftOK := parseVersion(left)
	rightVersion, rightOK := parseVersion(right)
	if !leftOK || !rightOK {
		return 0
	}
	maxLen := len(leftVersion.parts)
	if len(rightVersion.parts) > maxLen {
		maxLen = len(rightVersion.parts)
	}
	for i := 0; i < maxLen; i++ {
		leftValue := 0
		if i < len(leftVersion.parts) {
			leftValue = leftVersion.parts[i]
		}
		rightValue := 0
		if i < len(rightVersion.parts) {
			rightValue = rightVersion.parts[i]
		}
		if leftValue > rightValue {
			return 1
		}
		if leftValue < rightValue {
			return -1
		}
	}
	if leftVersion.prerelease == "" && rightVersion.prerelease != "" {
		return 1
	}
	if leftVersion.prerelease != "" && rightVersion.prerelease == "" {
		return -1
	}
	return comparePrereleaseVersions(leftVersion.prerelease, rightVersion.prerelease)
}

type parsedVersion struct {
	parts      []int
	prerelease string
}

func parseVersion(version string) (parsedVersion, bool) {
	version = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(version), "v"))
	if version == "" {
		return parsedVersion{}, false
	}
	prerelease := ""
	if core, suffix, ok := strings.Cut(version, "-"); ok {
		version = core
		prerelease = strings.TrimSpace(suffix)
	}
	rawParts := strings.Split(version, ".")
	parts := make([]int, 0, len(rawParts))
	for _, raw := range rawParts {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return parsedVersion{}, false
		}
		digits := strings.Builder{}
		for _, char := range raw {
			if !unicode.IsDigit(char) {
				break
			}
			digits.WriteRune(char)
		}
		if digits.Len() == 0 {
			return parsedVersion{}, false
		}
		value, err := strconv.Atoi(digits.String())
		if err != nil {
			return parsedVersion{}, false
		}
		parts = append(parts, value)
	}
	return parsedVersion{parts: parts, prerelease: prerelease}, len(parts) > 0
}

func comparePrereleaseVersions(left string, right string) int {
	leftParts := strings.Split(left, ".")
	rightParts := strings.Split(right, ".")
	maxLen := len(leftParts)
	if len(rightParts) > maxLen {
		maxLen = len(rightParts)
	}
	for i := 0; i < maxLen; i++ {
		if i >= len(leftParts) {
			return -1
		}
		if i >= len(rightParts) {
			return 1
		}
		if result := comparePrereleaseIdentifier(leftParts[i], rightParts[i]); result != 0 {
			return result
		}
	}
	return 0
}

func comparePrereleaseIdentifier(left string, right string) int {
	leftNumeric := isNumericPrereleaseIdentifier(left)
	rightNumeric := isNumericPrereleaseIdentifier(right)
	if leftNumeric && rightNumeric {
		return compareNumericStrings(left, right)
	}
	if leftNumeric && !rightNumeric {
		return -1
	}
	if !leftNumeric && rightNumeric {
		return 1
	}
	if left > right {
		return 1
	}
	if left < right {
		return -1
	}
	return 0
}

func isNumericPrereleaseIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if !unicode.IsDigit(char) {
			return false
		}
	}
	return true
}

func compareNumericStrings(left string, right string) int {
	left = strings.TrimLeft(left, "0")
	right = strings.TrimLeft(right, "0")
	if left == "" {
		left = "0"
	}
	if right == "" {
		right = "0"
	}
	if len(left) > len(right) {
		return 1
	}
	if len(left) < len(right) {
		return -1
	}
	if left > right {
		return 1
	}
	if left < right {
		return -1
	}
	return 0
}

func versionParts(version string) ([]int, bool) {
	parsed, ok := parseVersion(version)
	return parsed.parts, ok
}
