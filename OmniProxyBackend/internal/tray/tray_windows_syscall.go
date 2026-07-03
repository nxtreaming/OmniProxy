//go:build windows

package tray

import (
	"fmt"
	"golang.org/x/sys/windows"
	"syscall"
	"unsafe"
)

func setActiveManager(manager *Manager) {
	activeMu.Lock()
	activeManager = manager
	activeMu.Unlock()
}

func getActiveManager() *Manager {
	activeMu.RLock()
	defer activeMu.RUnlock()
	return activeManager
}

func trayWndProc(hwnd uintptr, message uint32, wParam uintptr, lParam uintptr) uintptr {
	if manager := getActiveManager(); manager != nil {
		return manager.handleMessage(hwnd, message, wParam, lParam)
	}
	ret, _, _ := procDefWindowProc.Call(hwnd, uintptr(message), wParam, lParam)
	return ret
}

func getModuleHandle() (uintptr, error) {
	ret, _, err := procGetModuleHandle.Call(0)
	if ret == 0 {
		return 0, errorFromSyscall("GetModuleHandle", err)
	}
	return ret, nil
}

func registerWindowClass(wc *wndClassEx) error {
	ret, _, err := procRegisterClassEx.Call(uintptr(unsafe.Pointer(wc)))
	if ret == 0 {
		if errno, ok := err.(syscall.Errno); ok && errno == classAlreadyExistsErr {
			return nil
		}
		return errorFromSyscall("RegisterClassEx", err)
	}
	return nil
}

func createHiddenWindow(instance uintptr, className *uint16) (uintptr, error) {
	ret, _, err := procCreateWindowEx.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("OmniProxy Tray"))),
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		instance,
		0,
	)
	if ret == 0 {
		return 0, errorFromSyscall("CreateWindowEx", err)
	}
	return ret, nil
}

func shellNotifyIcon(message uint32, nid *notifyIconData) error {
	ret, _, err := procShellNotifyIcon.Call(uintptr(message), uintptr(unsafe.Pointer(nid)))
	if ret == 0 {
		return errorFromSyscall("Shell_NotifyIcon", err)
	}
	return nil
}

func copyUTF16(dst []uint16, value string) {
	encoded := windows.StringToUTF16(value)
	if len(encoded) > len(dst) {
		encoded = encoded[:len(dst)]
		encoded[len(encoded)-1] = 0
	}
	copy(dst, encoded)
}

func errorFromSyscall(name string, err error) error {
	if err == nil || err == syscall.Errno(0) {
		return fmt.Errorf("%s failed", name)
	}
	return fmt.Errorf("%s failed: %w", name, err)
}
