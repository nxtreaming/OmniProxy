//go:build windows

package tray

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	windowClassStyle      = 0
	trayIconID            = 1
	wmNull                = 0x0000
	wmDestroy             = 0x0002
	wmClose               = 0x0010
	wmApp                 = 0x8000
	wmLButtonUp           = 0x0202
	wmLButtonDoubleClick  = 0x0203
	wmRButtonUp           = 0x0205
	trayCallbackMessage   = wmApp + 1
	nimAdd                = 0x00000000
	nimDelete             = 0x00000002
	nifMessage            = 0x00000001
	nifIcon               = 0x00000002
	nifTip                = 0x00000004
	idiApplication        = 32512
	mfString              = 0x00000000
	mfGrayed              = 0x00000001
	mfSeparator           = 0x00000800
	tpmRightButton        = 0x0002
	tpmReturnCommand      = 0x0100
	tpmNonotify           = 0x0080
	cmdToggleProxy        = 1001
	cmdOpenWindow         = 1002
	cmdQuit               = 1003
	classAlreadyExistsErr = 1410
)

var (
	user32                  = windows.NewLazySystemDLL("user32.dll")
	kernel32                = windows.NewLazySystemDLL("kernel32.dll")
	shell32                 = windows.NewLazySystemDLL("shell32.dll")
	procRegisterClassEx     = user32.NewProc("RegisterClassExW")
	procCreateWindowEx      = user32.NewProc("CreateWindowExW")
	procDefWindowProc       = user32.NewProc("DefWindowProcW")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procPostMessage         = user32.NewProc("PostMessageW")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procGetMessage          = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessage     = user32.NewProc("DispatchMessageW")
	procLoadIcon            = user32.NewProc("LoadIconW")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procGetCursorPos        = user32.NewProc("GetCursorPos")
	procCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	procAppendMenu          = user32.NewProc("AppendMenuW")
	procTrackPopupMenu      = user32.NewProc("TrackPopupMenu")
	procDestroyMenu         = user32.NewProc("DestroyMenu")
	procGetModuleHandle     = kernel32.NewProc("GetModuleHandleW")
	procShellNotifyIcon     = shell32.NewProc("Shell_NotifyIconW")

	activeMu      sync.RWMutex
	activeManager *Manager
	wndProc       = syscall.NewCallback(trayWndProc)
)

type Manager struct {
	opts  Options
	hwnd  uintptr
	ready chan error
	done  chan struct{}
	once  sync.Once
}

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

type point struct {
	X int32
	Y int32
}

type msg struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
}

type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

type notifyIconData struct {
	CbSize           uint32
	HWnd             uintptr
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            uintptr
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UVersion         uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         guid
	HBalloonIcon     uintptr
}

func Start(opts Options) (*Manager, error) {
	if opts.Tooltip == "" {
		opts.Tooltip = "OmniProxy"
	}
	manager := &Manager{
		opts:  opts,
		ready: make(chan error, 1),
		done:  make(chan struct{}),
	}
	setActiveManager(manager)
	go manager.run()
	if err := <-manager.ready; err != nil {
		setActiveManager(nil)
		return nil, err
	}
	return manager, nil
}

func (m *Manager) Stop() {
	m.once.Do(func() {
		hwnd := m.hwnd
		if hwnd != 0 {
			procPostMessage.Call(hwnd, wmClose, 0, 0)
		}
		select {
		case <-m.done:
		case <-time.After(2 * time.Second):
		}
		setActiveManager(nil)
	})
}

func (m *Manager) run() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(m.done)

	instance, err := getModuleHandle()
	if err != nil {
		m.ready <- err
		return
	}

	className := windows.StringToUTF16Ptr(fmt.Sprintf("OmniProxyTrayWindow-%d", os.Getpid()))
	wc := wndClassEx{
		Size:      uint32(unsafe.Sizeof(wndClassEx{})),
		Style:     windowClassStyle,
		WndProc:   wndProc,
		Instance:  instance,
		ClassName: className,
	}
	if err := registerWindowClass(&wc); err != nil {
		m.ready <- err
		return
	}

	hwnd, err := createHiddenWindow(instance, className)
	if err != nil {
		m.ready <- err
		return
	}
	m.hwnd = hwnd

	if err := m.addIcon(); err != nil {
		procDestroyWindow.Call(hwnd)
		m.ready <- err
		return
	}
	m.ready <- nil

	var message msg
	for {
		ret, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(ret) == -1 || ret == 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&message)))
	}
	_ = m.deleteIcon()
}

func (m *Manager) addIcon() error {
	nid := notifyIconData{
		CbSize:           uint32(unsafe.Sizeof(notifyIconData{})),
		HWnd:             m.hwnd,
		UID:              trayIconID,
		UFlags:           nifMessage | nifIcon | nifTip,
		UCallbackMessage: trayCallbackMessage,
		HIcon:            loadIcon(),
	}
	copyUTF16(nid.SzTip[:], m.opts.Tooltip)
	return shellNotifyIcon(nimAdd, &nid)
}

func (m *Manager) deleteIcon() error {
	nid := notifyIconData{
		CbSize: uint32(unsafe.Sizeof(notifyIconData{})),
		HWnd:   m.hwnd,
		UID:    trayIconID,
	}
	return shellNotifyIcon(nimDelete, &nid)
}

func (m *Manager) handleMessage(hwnd uintptr, message uint32, wParam uintptr, lParam uintptr) uintptr {
	switch message {
	case trayCallbackMessage:
		switch uint32(lParam) {
		case wmLButtonUp, wmLButtonDoubleClick:
			m.showWindow()
			return 0
		case wmRButtonUp:
			m.showMenu()
			return 0
		}
	case wmClose:
		procDestroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	ret, _, _ := procDefWindowProc.Call(hwnd, uintptr(message), wParam, lParam)
	return ret
}

func (m *Manager) showMenu() {
	hMenu, _, _ := procCreatePopupMenu.Call()
	if hMenu == 0 {
		m.log("tray menu create failed")
		return
	}
	defer procDestroyMenu.Call(hMenu)

	status := "代理状态未知"
	if m.opts.StatusLabel != nil {
		status = m.opts.StatusLabel()
	}
	port := "端口未知"
	if m.opts.PortLabel != nil {
		port = m.opts.PortLabel()
	}
	addMenuItem(hMenu, status, 0, mfString|mfGrayed)
	addMenuItem(hMenu, port, 0, mfString|mfGrayed)
	addSeparator(hMenu)
	if m.isProxyRunning() {
		addMenuItem(hMenu, "停止代理", cmdToggleProxy, mfString)
	} else {
		addMenuItem(hMenu, "启动代理", cmdToggleProxy, mfString)
	}
	addMenuItem(hMenu, "打开主界面", cmdOpenWindow, mfString)
	addSeparator(hMenu)
	addMenuItem(hMenu, "退出", cmdQuit, mfString)

	var cursor point
	if ok, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&cursor))); ok == 0 {
		return
	}
	procSetForegroundWindow.Call(m.hwnd)
	cmd, _, _ := procTrackPopupMenu.Call(
		hMenu,
		tpmRightButton|tpmReturnCommand|tpmNonotify,
		uintptr(cursor.X),
		uintptr(cursor.Y),
		0,
		m.hwnd,
		0,
	)
	procPostMessage.Call(m.hwnd, wmNull, 0, 0)

	switch cmd {
	case cmdToggleProxy:
		m.toggleProxy()
	case cmdOpenWindow:
		m.showWindow()
	case cmdQuit:
		m.quit()
	}
}

func (m *Manager) toggleProxy() {
	go func() {
		var err error
		if m.isProxyRunning() {
			if m.opts.StopProxy != nil {
				err = m.opts.StopProxy()
			}
		} else if m.opts.StartProxy != nil {
			err = m.opts.StartProxy()
		}
		if err != nil {
			m.log("tray proxy toggle failed: %v", err)
		}
	}()
}

func (m *Manager) showWindow() {
	if m.opts.ShowWindow != nil {
		go m.opts.ShowWindow()
	}
}

func (m *Manager) quit() {
	if m.opts.Quit != nil {
		go m.opts.Quit()
	}
}

func (m *Manager) isProxyRunning() bool {
	return m.opts.IsProxyRunning != nil && m.opts.IsProxyRunning()
}

func (m *Manager) log(format string, args ...any) {
	if m.opts.Log != nil {
		m.opts.Log(format, args...)
	}
}

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

func loadIcon() uintptr {
	ret, _, _ := procLoadIcon.Call(0, idiApplication)
	return ret
}

func shellNotifyIcon(message uint32, nid *notifyIconData) error {
	ret, _, err := procShellNotifyIcon.Call(uintptr(message), uintptr(unsafe.Pointer(nid)))
	if ret == 0 {
		return errorFromSyscall("Shell_NotifyIcon", err)
	}
	return nil
}

func addMenuItem(hMenu uintptr, label string, id uintptr, flags uintptr) {
	procAppendMenu.Call(
		hMenu,
		flags,
		id,
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(label))),
	)
}

func addSeparator(hMenu uintptr) {
	procAppendMenu.Call(hMenu, mfSeparator, 0, 0)
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
