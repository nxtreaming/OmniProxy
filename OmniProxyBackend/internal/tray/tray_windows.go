//go:build windows

package tray

import (
	"fmt"
	"golang.org/x/sys/windows"
	"os"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	windowClassStyle      = 0
	trayIconID            = 1
	wmNull                = 0x0000
	wmActivate            = 0x0006
	wmKillFocus           = 0x0008
	wmPaint               = 0x000f
	wmDestroy             = 0x0002
	wmClose               = 0x0010
	wmEraseBkgnd          = 0x0014
	wmKeyDown             = 0x0100
	wmApp                 = 0x8000
	wmMouseMove           = 0x0200
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
	idcArrow              = 32512
	waInactive            = 0
	swShow                = 5
	vkEscape              = 0x1b
	wsPopup               = 0x80000000
	wsExTopmost           = 0x00000008
	wsExToolWindow        = 0x00000080
	smCxScreen            = 0
	smCyScreen            = 1
	dtLeft                = 0x00000000
	dtVCenter             = 0x00000004
	dtSingleLine          = 0x00000020
	dtEndEllipsis         = 0x00008000
	transparentBkMode     = 1
	defaultGUIFont        = 17
	psSolid               = 0
	cmdToggleProxy        = 1001
	cmdOpenWindow         = 1002
	cmdQuit               = 1003
	classAlreadyExistsErr = 1410
	trayMenuWidth         = 300
	trayMenuRadius        = 14
	trayMenuPadding       = 6
	trayMenuScreenMargin  = 8
)

var (
	user32                  = windows.NewLazySystemDLL("user32.dll")
	gdi32                   = windows.NewLazySystemDLL("gdi32.dll")
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
	procLoadCursor          = user32.NewProc("LoadCursorW")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procGetCursorPos        = user32.NewProc("GetCursorPos")
	procDestroyIcon         = user32.NewProc("DestroyIcon")
	procShowWindow          = user32.NewProc("ShowWindow")
	procInvalidateRect      = user32.NewProc("InvalidateRect")
	procGetSystemMetrics    = user32.NewProc("GetSystemMetrics")
	procGetClientRect       = user32.NewProc("GetClientRect")
	procBeginPaint          = user32.NewProc("BeginPaint")
	procEndPaint            = user32.NewProc("EndPaint")
	procSetWindowRgn        = user32.NewProc("SetWindowRgn")
	procFillRect            = user32.NewProc("FillRect")
	procDrawText            = user32.NewProc("DrawTextW")
	procRoundRect           = gdi32.NewProc("RoundRect")
	procEllipse             = gdi32.NewProc("Ellipse")
	procCreateRoundRectRgn  = gdi32.NewProc("CreateRoundRectRgn")
	procCreateSolidBrush    = gdi32.NewProc("CreateSolidBrush")
	procCreatePen           = gdi32.NewProc("CreatePen")
	procDeleteObject        = gdi32.NewProc("DeleteObject")
	procGetStockObject      = gdi32.NewProc("GetStockObject")
	procSelectObject        = gdi32.NewProc("SelectObject")
	procSetBkMode           = gdi32.NewProc("SetBkMode")
	procSetTextColor        = gdi32.NewProc("SetTextColor")
	procGetModuleHandle     = kernel32.NewProc("GetModuleHandleW")
	procShellNotifyIcon     = shell32.NewProc("Shell_NotifyIconW")
	procExtractIconEx       = shell32.NewProc("ExtractIconExW")

	activeMu      sync.RWMutex
	activeManager *Manager
	wndProc       = syscall.NewCallback(trayWndProc)
)

type Manager struct {
	opts        Options
	hwnd        uintptr
	hicon       uintptr
	instance    uintptr
	windowClass string
	ready       chan error
	done        chan struct{}
	once        sync.Once
	menuMu      sync.RWMutex
	menuHwnd    uintptr
	menuHover   int
	menuItems   []trayMenuItem
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

type rect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

type paintStruct struct {
	HDC         uintptr
	Erase       int32
	Paint       rect
	Restore     int32
	IncUpdate   int32
	RGBReserved [32]byte
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

type trayMenuItemKind int

const (
	trayMenuItemHeader trayMenuItemKind = iota
	trayMenuItemAction
	trayMenuItemSeparator
)

type trayMenuItem struct {
	Label   string
	Detail  string
	Kind    trayMenuItemKind
	Active  bool
	Command uintptr
	Bounds  rect
}

func Start(opts Options) (*Manager, error) {
	if opts.Tooltip == "" {
		opts.Tooltip = "OmniProxy"
	}
	manager := &Manager{
		opts:      opts,
		ready:     make(chan error, 1),
		done:      make(chan struct{}),
		menuHover: -1,
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

	className := fmt.Sprintf("OmniProxyTrayWindow-%d", os.Getpid())
	classNamePtr := windows.StringToUTF16Ptr(className)
	wc := wndClassEx{
		Size:      uint32(unsafe.Sizeof(wndClassEx{})),
		Style:     windowClassStyle,
		WndProc:   wndProc,
		Instance:  instance,
		Cursor:    loadArrowCursor(),
		ClassName: classNamePtr,
	}
	if err := registerWindowClass(&wc); err != nil {
		m.ready <- err
		return
	}
	m.instance = instance
	m.windowClass = className

	hwnd, err := createHiddenWindow(instance, classNamePtr)
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
	m.destroyIcon()
}

func (m *Manager) addIcon() error {
	nid := notifyIconData{
		CbSize:           uint32(unsafe.Sizeof(notifyIconData{})),
		HWnd:             m.hwnd,
		UID:              trayIconID,
		UFlags:           nifMessage | nifIcon | nifTip,
		UCallbackMessage: trayCallbackMessage,
		HIcon:            m.loadIcon(),
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

func (m *Manager) loadIcon() uintptr {
	hicon := extractExecutableIcon()
	if hicon != 0 {
		m.hicon = hicon
		return hicon
	}
	ret, _, _ := procLoadIcon.Call(0, idiApplication)
	return ret
}

func loadArrowCursor() uintptr {
	ret, _, _ := procLoadCursor.Call(0, idcArrow)
	return ret
}

func (m *Manager) destroyIcon() {
	if m.hicon == 0 {
		return
	}
	procDestroyIcon.Call(m.hicon)
	m.hicon = 0
}

func extractExecutableIcon() uintptr {
	executable, err := os.Executable()
	if err != nil {
		return 0
	}
	var largeIcon uintptr
	var smallIcon uintptr
	count, _, _ := procExtractIconEx.Call(
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(executable))),
		0,
		uintptr(unsafe.Pointer(&largeIcon)),
		uintptr(unsafe.Pointer(&smallIcon)),
		1,
	)
	if count == 0 {
		return 0
	}
	if smallIcon != 0 {
		if largeIcon != 0 {
			procDestroyIcon.Call(largeIcon)
		}
		return smallIcon
	}
	return largeIcon
}

func (m *Manager) handleMessage(hwnd uintptr, message uint32, wParam uintptr, lParam uintptr) uintptr {
	if hwnd == m.currentMenuHwnd() {
		return m.handleMenuMessage(hwnd, message, wParam, lParam)
	}

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
		m.closeMenu()
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
	m.closeMenu()
	status := "代理状态未知"
	if m.opts.StatusLabel != nil {
		status = m.opts.StatusLabel()
	}
	port := "端口未知"
	if m.opts.PortLabel != nil {
		port = m.opts.PortLabel()
	}
	running := m.isProxyRunning()
	items := []trayMenuItem{
		{Label: status, Detail: port, Kind: trayMenuItemHeader, Active: running},
		{Kind: trayMenuItemSeparator},
	}
	if running {
		items = append(items, trayMenuItem{Label: "停止代理", Kind: trayMenuItemAction, Command: cmdToggleProxy})
	} else {
		items = append(items, trayMenuItem{Label: "启动代理", Kind: trayMenuItemAction, Command: cmdToggleProxy})
	}
	items = append(items,
		trayMenuItem{Label: "打开主界面", Kind: trayMenuItemAction, Command: cmdOpenWindow},
		trayMenuItem{Kind: trayMenuItemSeparator},
		trayMenuItem{Label: "退出", Kind: trayMenuItemAction, Command: cmdQuit},
	)
	menuHeight := layoutTrayMenuItems(items)
	m.setMenuItems(items, -1)

	var cursor point
	if ok, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&cursor))); ok == 0 {
		m.setMenuItems(nil, -1)
		return
	}
	left, top := trayMenuPosition(cursor, trayMenuWidth, menuHeight)
	hwnd, err := m.createMenuWindow(left, top, trayMenuWidth, menuHeight)
	if err != nil {
		m.setMenuItems(nil, -1)
		m.log("tray menu create failed: %v", err)
		return
	}
	m.setMenuHwnd(hwnd)
	applyRoundedWindowRegion(hwnd, trayMenuWidth, menuHeight, trayMenuRadius)
	procShowWindow.Call(hwnd, swShow)
	procSetForegroundWindow.Call(hwnd)
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

func (m *Manager) handleMenuMessage(hwnd uintptr, message uint32, wParam uintptr, lParam uintptr) uintptr {
	switch message {
	case wmPaint:
		m.paintMenu(hwnd)
		return 0
	case wmEraseBkgnd:
		return 1
	case wmMouseMove:
		items, _ := m.menuSnapshot()
		if m.updateMenuHover(menuIndexAt(items, pointFromLParam(lParam))) {
			procInvalidateRect.Call(hwnd, 0, 0)
		}
		return 0
	case wmLButtonUp:
		cmd := m.commandAt(pointFromLParam(lParam))
		m.closeMenu()
		m.runMenuCommand(cmd)
		return 0
	case wmKeyDown:
		if wParam == vkEscape {
			m.closeMenu()
			return 0
		}
	case wmActivate:
		if wParam&0xffff == waInactive {
			m.closeMenu()
			return 0
		}
	case wmKillFocus, wmClose:
		m.closeMenu()
		return 0
	case wmDestroy:
		m.clearMenuWindow(hwnd)
		return 0
	}
	ret, _, _ := procDefWindowProc.Call(hwnd, uintptr(message), wParam, lParam)
	return ret
}
