//go:build windows

package taskautomation

import (
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

func forceForegroundWindow(hwnd uintptr) bool {
	if hwnd == 0 {
		return false
	}

	currentThread, _, _ := procGetCurrentThreadID.Call()
	foreground, _, _ := procGetForegroundWindow.Call()
	foregroundThread := uintptr(0)
	if foreground != 0 {
		foregroundThread, _, _ = procGetWindowThreadID.Call(foreground, 0)
	}
	targetThread, _, _ := procGetWindowThreadID.Call(hwnd, 0)

	attachedForeground := attachThreadInput(currentThread, foregroundThread)
	attachedTarget := false
	if targetThread != foregroundThread {
		attachedTarget = attachThreadInput(currentThread, targetThread)
	}
	defer detachThreadInput(currentThread, foregroundThread, attachedForeground)
	defer detachThreadInput(currentThread, targetThread, attachedTarget)

	restoreWindowIfMinimized(hwnd)
	procSetWindowPos.Call(hwnd, ^uintptr(0), 0, 0, 0, 0, swpNoMove|swpNoSize|swpShowWindow)
	procSetWindowPos.Call(hwnd, ^uintptr(1), 0, 0, 0, 0, swpNoMove|swpNoSize|swpShowWindow)
	procBringWindowToTop.Call(hwnd)
	procSetActiveWindow.Call(hwnd)
	procSetFocus.Call(hwnd)
	procSetForegroundWindow.Call(hwnd)
	return isForegroundWindow(hwnd)
}

func restoreWindowIfMinimized(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	minimized, _, _ := procIsIconic.Call(hwnd)
	if minimized != 0 {
		procShowWindow.Call(hwnd, swRestore)
	}
}

func attachThreadInput(currentThread uintptr, targetThread uintptr) bool {
	if currentThread == 0 || targetThread == 0 || currentThread == targetThread {
		return false
	}
	ok, _, _ := procAttachThreadInput.Call(currentThread, targetThread, 1)
	return ok != 0
}

func detachThreadInput(currentThread uintptr, targetThread uintptr, attached bool) {
	if attached {
		procAttachThreadInput.Call(currentThread, targetThread, 0)
	}
}

func unlockForegroundWithAlt() {
	procKeybdEvent.Call(vkMenu, 0, 0, 0)
	procKeybdEvent.Call(vkMenu, 0, keyeventfKeyup, 0)
}

func isForegroundWindow(hwnd uintptr) bool {
	foreground, _, _ := procGetForegroundWindow.Call()
	return foreground == hwnd
}

func focusBrowserWindowSoon(spec browserSpec) {
	for _, delay := range []time.Duration{250 * time.Millisecond, 900 * time.Millisecond, 1600 * time.Millisecond} {
		time.AfterFunc(delay, func() {
			if hwnd := findBrowserWindow(spec); hwnd != 0 {
				if !forceForegroundWindow(hwnd) {
					unlockForegroundWithAlt()
					forceForegroundWindow(hwnd)
				}
			}
		})
	}
}

func findBrowserWindow(spec browserSpec) uintptr {
	names := browserProcessNameSet(spec)
	if len(names) == 0 {
		return 0
	}

	var match uintptr
	callback := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		if match != 0 || !isVisibleWindow(hwnd) {
			return 1
		}
		if names[windowProcessName(hwnd)] {
			match = hwnd
			return 0
		}
		return 1
	})
	procEnumWindows.Call(callback, 0)
	return match
}

func browserProcessNameSet(spec browserSpec) map[string]bool {
	names := map[string]bool{}
	for _, name := range spec.processNames {
		name = strings.ToLower(strings.TrimSpace(name))
		if name != "" {
			names[name] = true
		}
	}
	return names
}

func isVisibleWindow(hwnd uintptr) bool {
	if hwnd == 0 {
		return false
	}
	visible, _, _ := procIsWindowVisible.Call(hwnd)
	return visible != 0
}

func windowProcessName(hwnd uintptr) string {
	var pid uint32
	procGetWindowThreadID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return ""
	}
	return processImageName(pid)
}

func processImageName(pid uint32) string {
	handle, _, _ := procOpenProcess.Call(processQueryLimitedInformation, 0, uintptr(pid))
	if handle == 0 {
		return ""
	}
	defer syscall.CloseHandle(syscall.Handle(handle))

	buffer := make([]uint16, 32768)
	size := uint32(len(buffer))
	ok, _, _ := procQueryProcessImage.Call(
		handle,
		0,
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if ok == 0 || size == 0 {
		return ""
	}
	return strings.ToLower(filepath.Base(syscall.UTF16ToString(buffer[:size])))
}
