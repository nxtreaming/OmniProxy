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

func (m *Manager) createMenuWindow(left int32, top int32, width int32, height int32) (uintptr, error) {
	className := windows.StringToUTF16Ptr(m.windowClass)
	title := windows.StringToUTF16Ptr("OmniProxy Tray Menu")
	ret, _, err := procCreateWindowEx.Call(
		wsExTopmost|wsExToolWindow,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(title)),
		wsPopup,
		uintptr(left),
		uintptr(top),
		uintptr(width),
		uintptr(height),
		m.hwnd,
		0,
		m.instance,
		0,
	)
	if ret == 0 {
		return 0, errorFromSyscall("CreateWindowEx", err)
	}
	return ret, nil
}

func (m *Manager) paintMenu(hwnd uintptr) {
	var ps paintStruct
	hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	if hdc == 0 {
		return
	}
	defer procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))

	var client rect
	if ok, _, _ := procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&client))); ok == 0 {
		return
	}
	fillMenuRoundRect(hdc, client, trayMenuRadius, trayColorSurface(), trayColorBorder())

	items, hover := m.menuSnapshot()
	for index, item := range items {
		drawTrayMenuItem(hdc, item, index == hover)
	}
}

func (m *Manager) closeMenu() {
	hwnd := m.currentMenuHwnd()
	if hwnd != 0 {
		procDestroyWindow.Call(hwnd)
	}
}

func (m *Manager) setMenuHwnd(hwnd uintptr) {
	m.menuMu.Lock()
	m.menuHwnd = hwnd
	m.menuMu.Unlock()
}

func (m *Manager) currentMenuHwnd() uintptr {
	m.menuMu.RLock()
	defer m.menuMu.RUnlock()
	return m.menuHwnd
}

func (m *Manager) clearMenuWindow(hwnd uintptr) {
	m.menuMu.Lock()
	if m.menuHwnd == hwnd {
		m.menuHwnd = 0
		m.menuItems = nil
		m.menuHover = -1
	}
	m.menuMu.Unlock()
}

func (m *Manager) setMenuItems(items []trayMenuItem, hover int) {
	m.menuMu.Lock()
	m.menuItems = items
	m.menuHover = hover
	m.menuMu.Unlock()
}

func (m *Manager) menuSnapshot() ([]trayMenuItem, int) {
	m.menuMu.RLock()
	defer m.menuMu.RUnlock()
	items := make([]trayMenuItem, len(m.menuItems))
	copy(items, m.menuItems)
	return items, m.menuHover
}

func (m *Manager) updateMenuHover(index int) bool {
	m.menuMu.Lock()
	defer m.menuMu.Unlock()
	if m.menuHover == index {
		return false
	}
	m.menuHover = index
	return true
}

func (m *Manager) commandAt(point point) uintptr {
	items, _ := m.menuSnapshot()
	index := menuIndexAt(items, point)
	if index < 0 {
		return 0
	}
	return items[index].Command
}

func (m *Manager) runMenuCommand(cmd uintptr) {
	switch cmd {
	case cmdToggleProxy:
		m.toggleProxy()
	case cmdOpenWindow:
		m.showWindow()
	case cmdQuit:
		m.quit()
	}
}

func drawTrayMenuItem(hdc uintptr, item trayMenuItem, selected bool) {
	if item.Kind == trayMenuItemSeparator {
		line := item.Bounds
		line.Left += 16
		line.Right -= 16
		line.Top += 5
		line.Bottom = line.Top + 1
		fillMenuRect(hdc, line, trayColorLine())
		return
	}

	if item.Kind == trayMenuItemHeader {
		dot := rect{
			Left:   item.Bounds.Left + 18,
			Top:    item.Bounds.Top + 22,
			Right:  item.Bounds.Left + 26,
			Bottom: item.Bounds.Top + 30,
		}
		if item.Active {
			fillMenuEllipse(hdc, dot, trayColorSuccess(), trayColorSuccess())
		} else {
			fillMenuEllipse(hdc, dot, trayColorMutedDot(), trayColorMutedDot())
		}

		titleRect := item.Bounds
		titleRect.Left += 38
		titleRect.Right -= 18
		titleRect.Top += 10
		titleRect.Bottom = titleRect.Top + 24
		drawMenuText(hdc, item.Label, titleRect, trayColorTextStrong())

		detailRect := item.Bounds
		detailRect.Left += 38
		detailRect.Right -= 18
		detailRect.Top += 33
		detailRect.Bottom = detailRect.Top + 20
		drawMenuText(hdc, item.Detail, detailRect, trayColorTextMuted())
		return
	}

	if selected {
		hover := item.Bounds
		hover.Left += 8
		hover.Right -= 8
		hover.Top += 3
		hover.Bottom -= 3
		fillMenuRoundRect(hdc, hover, 8, trayColorHover(), trayColorHover())
	}

	textRect := item.Bounds
	textRect.Left += 20
	textRect.Right -= 18
	textColor := trayColorText()
	if selected {
		textColor = trayColorBrand()
	}
	drawMenuText(hdc, item.Label, textRect, textColor)
}

func layoutTrayMenuItems(items []trayMenuItem) int32 {
	y := int32(trayMenuPadding)
	for index := range items {
		height := trayMenuItemHeight(items[index].Kind)
		items[index].Bounds = rect{
			Left:   1,
			Top:    y,
			Right:  trayMenuWidth - 1,
			Bottom: y + height,
		}
		y += height
	}
	return y + trayMenuPadding
}

func trayMenuItemHeight(kind trayMenuItemKind) int32 {
	switch kind {
	case trayMenuItemHeader:
		return 62
	case trayMenuItemSeparator:
		return 12
	default:
		return 38
	}
}

func menuIndexAt(items []trayMenuItem, point point) int {
	for index, item := range items {
		if item.Command == 0 || item.Kind != trayMenuItemAction {
			continue
		}
		if point.X >= item.Bounds.Left && point.X < item.Bounds.Right && point.Y >= item.Bounds.Top && point.Y < item.Bounds.Bottom {
			return index
		}
	}
	return -1
}

func pointFromLParam(lParam uintptr) point {
	return point{
		X: int32(int16(lParam & 0xffff)),
		Y: int32(int16((lParam >> 16) & 0xffff)),
	}
}

func trayMenuPosition(cursor point, width int32, height int32) (int32, int32) {
	left := cursor.X + trayMenuScreenMargin
	top := cursor.Y - height - trayMenuScreenMargin
	screenWidth := systemMetric(smCxScreen)
	screenHeight := systemMetric(smCyScreen)
	if left+width > screenWidth-trayMenuScreenMargin {
		left = cursor.X - width - trayMenuScreenMargin
	}
	if left < trayMenuScreenMargin {
		left = trayMenuScreenMargin
	}
	if top < trayMenuScreenMargin {
		top = cursor.Y + trayMenuScreenMargin
	}
	if top+height > screenHeight-trayMenuScreenMargin {
		top = screenHeight - height - trayMenuScreenMargin
	}
	return left, top
}

func systemMetric(index int32) int32 {
	ret, _, _ := procGetSystemMetrics.Call(uintptr(index))
	return int32(ret)
}

func applyRoundedWindowRegion(hwnd uintptr, width int32, height int32, radius int32) {
	rgn, _, _ := procCreateRoundRectRgn.Call(
		0,
		0,
		uintptr(width+1),
		uintptr(height+1),
		uintptr(radius),
		uintptr(radius),
	)
	if rgn == 0 {
		return
	}
	ok, _, _ := procSetWindowRgn.Call(hwnd, rgn, 1)
	if ok == 0 {
		procDeleteObject.Call(rgn)
	}
}

func drawMenuText(hdc uintptr, text string, bounds rect, color uintptr) {
	if text == "" {
		return
	}
	font, _, _ := procGetStockObject.Call(defaultGUIFont)
	var previous uintptr
	if font != 0 {
		previous, _, _ = procSelectObject.Call(hdc, font)
	}
	procSetBkMode.Call(hdc, transparentBkMode)
	procSetTextColor.Call(hdc, color)
	encoded := windows.StringToUTF16(text)
	procDrawText.Call(
		hdc,
		uintptr(unsafe.Pointer(&encoded[0])),
		uintptr(len(encoded)-1),
		uintptr(unsafe.Pointer(&bounds)),
		dtLeft|dtVCenter|dtSingleLine|dtEndEllipsis,
	)
	if previous != 0 {
		procSelectObject.Call(hdc, previous)
	}
}

func fillMenuRect(hdc uintptr, bounds rect, color uintptr) {
	brush, _, _ := procCreateSolidBrush.Call(color)
	if brush == 0 {
		return
	}
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&bounds)), brush)
	procDeleteObject.Call(brush)
}

func fillMenuRoundRect(hdc uintptr, bounds rect, radius int32, fill uintptr, outline uintptr) {
	brush, _, _ := procCreateSolidBrush.Call(fill)
	if brush == 0 {
		return
	}
	pen, _, _ := procCreatePen.Call(psSolid, 1, outline)
	var previousBrush uintptr
	var previousPen uintptr
	previousBrush, _, _ = procSelectObject.Call(hdc, brush)
	if pen != 0 {
		previousPen, _, _ = procSelectObject.Call(hdc, pen)
	}
	procRoundRect.Call(
		hdc,
		uintptr(bounds.Left),
		uintptr(bounds.Top),
		uintptr(bounds.Right),
		uintptr(bounds.Bottom),
		uintptr(radius),
		uintptr(radius),
	)
	if previousPen != 0 {
		procSelectObject.Call(hdc, previousPen)
	}
	if previousBrush != 0 {
		procSelectObject.Call(hdc, previousBrush)
	}
	if pen != 0 {
		procDeleteObject.Call(pen)
	}
	procDeleteObject.Call(brush)
}

func fillMenuEllipse(hdc uintptr, bounds rect, fill uintptr, outline uintptr) {
	brush, _, _ := procCreateSolidBrush.Call(fill)
	if brush == 0 {
		return
	}
	pen, _, _ := procCreatePen.Call(psSolid, 1, outline)
	var previousBrush uintptr
	var previousPen uintptr
	previousBrush, _, _ = procSelectObject.Call(hdc, brush)
	if pen != 0 {
		previousPen, _, _ = procSelectObject.Call(hdc, pen)
	}
	procEllipse.Call(
		hdc,
		uintptr(bounds.Left),
		uintptr(bounds.Top),
		uintptr(bounds.Right),
		uintptr(bounds.Bottom),
	)
	if previousPen != 0 {
		procSelectObject.Call(hdc, previousPen)
	}
	if previousBrush != 0 {
		procSelectObject.Call(hdc, previousBrush)
	}
	if pen != 0 {
		procDeleteObject.Call(pen)
	}
	procDeleteObject.Call(brush)
}

func trayColorSurface() uintptr    { return rgb(32, 33, 36) }
func trayColorHover() uintptr      { return rgb(43, 46, 54) }
func trayColorBorder() uintptr     { return rgb(82, 88, 99) }
func trayColorLine() uintptr       { return rgb(58, 62, 70) }
func trayColorTextStrong() uintptr { return rgb(232, 235, 240) }
func trayColorText() uintptr       { return rgb(211, 216, 224) }
func trayColorTextMuted() uintptr  { return rgb(142, 150, 163) }
func trayColorBrand() uintptr      { return rgb(154, 190, 255) }
func trayColorSuccess() uintptr    { return rgb(104, 204, 151) }
func trayColorMutedDot() uintptr   { return rgb(94, 101, 114) }

func rgb(red, green, blue byte) uintptr {
	return uintptr(red) | uintptr(green)<<8 | uintptr(blue)<<16
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
