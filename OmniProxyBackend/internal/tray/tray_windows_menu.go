//go:build windows

package tray

import (
	"golang.org/x/sys/windows"
	"unsafe"
)

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
