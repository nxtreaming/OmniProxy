package securestore

import (
	"encoding/base64"
	"strings"
)

const protectedPrefix = "omniproxy-secret:v1:"

func ProtectString(value string) (string, error) {
	if value == "" || IsProtectedString(value) {
		return value, nil
	}
	protected, err := protectBytes([]byte(value))
	if err != nil {
		return "", err
	}
	return protectedPrefix + base64.RawURLEncoding.EncodeToString(protected), nil
}

func UnprotectString(value string) (string, error) {
	if !IsProtectedString(value) {
		return value, nil
	}
	encoded := strings.TrimPrefix(value, protectedPrefix)
	protected, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	plain, err := unprotectBytes(protected)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func IsProtectedString(value string) bool {
	return strings.HasPrefix(strings.TrimSpace(value), protectedPrefix)
}
