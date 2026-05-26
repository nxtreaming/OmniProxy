//go:build darwin

package securestore

import (
	"crypto/sha256"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	sum := sha256.Sum256([]byte("securestore-test-master-key"))
	darwinMasterKeyForTest = func() []byte {
		return sum[:]
	}
	os.Exit(m.Run())
}
