//go:build windows

package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var procShellExecuteW = windows.NewLazySystemDLL("shell32.dll").NewProc("ShellExecuteW")

const shellExecuteSuccessThreshold = 32
const swShownormal = 1

func defaultStartUpdateInstaller(filePath string, args []string) error {
	verb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(filePath)
	if err != nil {
		return err
	}
	parameters, err := windows.UTF16PtrFromString(strings.Join(args, " "))
	if err != nil {
		return err
	}
	directory, err := windows.UTF16PtrFromString(filepath.Dir(filePath))
	if err != nil {
		return err
	}

	ret, _, callErr := procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(file)),
		uintptr(unsafe.Pointer(parameters)),
		uintptr(unsafe.Pointer(directory)),
		swShownormal,
	)
	if ret > shellExecuteSuccessThreshold {
		return nil
	}
	if callErr != nil && callErr != windows.ERROR_SUCCESS {
		return fmt.Errorf("shell execute update installer: %w", callErr)
	}
	return fmt.Errorf("shell execute update installer returned %d", ret)
}
