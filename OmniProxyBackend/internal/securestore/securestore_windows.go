//go:build windows

package securestore

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var entropy = []byte("OmniProxy token store v1")

func protectBytes(value []byte) ([]byte, error) {
	in := dataBlob(value)
	entropyBlob := dataBlob(entropy)
	name, err := windows.UTF16PtrFromString("OmniProxy credentials")
	if err != nil {
		return nil, err
	}
	var out windows.DataBlob
	if err := windows.CryptProtectData(&in, name, &entropyBlob, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return nil, fmt.Errorf("protect credentials with DPAPI: %w", err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return copyBlob(out), nil
}

func unprotectBytes(value []byte) ([]byte, error) {
	in := dataBlob(value)
	entropyBlob := dataBlob(entropy)
	var out windows.DataBlob
	if err := windows.CryptUnprotectData(&in, nil, &entropyBlob, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return nil, fmt.Errorf("unprotect credentials with DPAPI: %w", err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return copyBlob(out), nil
}

func dataBlob(value []byte) windows.DataBlob {
	if len(value) == 0 {
		return windows.DataBlob{}
	}
	return windows.DataBlob{
		Size: uint32(len(value)),
		Data: &value[0],
	}
}

func copyBlob(blob windows.DataBlob) []byte {
	if blob.Size == 0 || blob.Data == nil {
		return nil
	}
	out := make([]byte, blob.Size)
	copy(out, unsafe.Slice(blob.Data, blob.Size))
	return out
}
