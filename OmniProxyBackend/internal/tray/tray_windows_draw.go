//go:build windows

package tray

import (
	"golang.org/x/sys/windows"
	"unsafe"
)

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
