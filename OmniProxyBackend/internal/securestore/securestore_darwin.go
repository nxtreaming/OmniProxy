//go:build darwin

package securestore

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

const (
	darwinProtectedPrefix = "darwin-keychain-v1:"
	darwinSecurityCommand = "/usr/bin/security"
	keychainService       = "OmniProxy credentials"
	keychainAccount       = "OmniProxy"
)

var (
	darwinKeyMu     sync.Mutex
	darwinCachedKey []byte
)

func protectBytes(value []byte) ([]byte, error) {
	key, err := darwinMasterKey()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	out := append([]byte(darwinProtectedPrefix), nonce...)
	out = gcm.Seal(out, nonce, value, nil)
	return out, nil
}

func unprotectBytes(value []byte) ([]byte, error) {
	if !bytes.HasPrefix(value, []byte(darwinProtectedPrefix)) {
		out := make([]byte, len(value))
		copy(out, value)
		return out, nil
	}

	key, err := darwinMasterKey()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	payload := value[len(darwinProtectedPrefix):]
	if len(payload) < gcm.NonceSize() {
		return nil, fmt.Errorf("protected credentials payload is too short")
	}
	nonce := payload[:gcm.NonceSize()]
	ciphertext := payload[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func darwinMasterKey() ([]byte, error) {
	darwinKeyMu.Lock()
	defer darwinKeyMu.Unlock()

	if len(darwinCachedKey) == 32 {
		return append([]byte(nil), darwinCachedKey...), nil
	}

	if key, err := readDarwinMasterKey(); err == nil {
		darwinCachedKey = append([]byte(nil), key...)
		return append([]byte(nil), key...), nil
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := writeDarwinMasterKey(key); err != nil {
		return nil, err
	}
	darwinCachedKey = append([]byte(nil), key...)
	return append([]byte(nil), key...), nil
}

func readDarwinMasterKey() ([]byte, error) {
	out, err := exec.Command(darwinSecurityCommand, "find-generic-password", "-a", keychainAccount, "-s", keychainService, "-w").Output()
	if err != nil {
		return nil, err
	}
	key, err := base64.RawStdEncoding.DecodeString(strings.TrimSpace(string(out)))
	if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("keychain credential key has invalid length")
	}
	return key, nil
}

func writeDarwinMasterKey(key []byte) error {
	encoded := base64.RawStdEncoding.EncodeToString(key)
	out, err := exec.Command(
		darwinSecurityCommand,
		"add-generic-password",
		"-a", keychainAccount,
		"-s", keychainService,
		"-w", encoded,
		"-U",
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("store credentials key in macOS Keychain: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
