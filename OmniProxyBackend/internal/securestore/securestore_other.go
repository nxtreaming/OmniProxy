//go:build !windows

package securestore

func protectBytes(value []byte) ([]byte, error) {
	out := make([]byte, len(value))
	copy(out, value)
	return out, nil
}

func unprotectBytes(value []byte) ([]byte, error) {
	out := make([]byte, len(value))
	copy(out, value)
	return out, nil
}
