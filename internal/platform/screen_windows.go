//go:build windows

package platform

import "unsafe"

// workAreaNearCursor returns the usable (taskbar-excluded) area of the monitor under the mouse
// cursor, matching design doc §55's fallback order (cursor's monitor, else primary display —
// the "previous foreground window's monitor" tier is approximated by capturing the cursor
// position immediately after the hotkey fires, before SailBoard steals focus), along with that
// monitor's DPI scale factor. GetMonitorInfo/SetWindowPos/CreateWindowEx all operate in physical
// pixels under this app's Per-Monitor-V2 DPI awareness (build/windows/wails.exe.manifest), so a
// fixed-size length like the panel's target height must be scaled by this factor before being
// handed to PositionSelf/SlideReveal — otherwise it renders at half its intended on-screen size
// at 200% scaling (and proportionally off at any other non-100% scale).
func workAreaNearCursor() (Rect, float64, bool) {
	var pt point
	if ok, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt))); ok == 0 {
		return Rect{}, 1, false
	}
	// MonitorFromPoint's C signature takes a POINT *by value*; the x64 calling convention packs
	// that 8-byte struct into a single register (low 32 bits = x, high 32 bits = y), not two
	// separate arguments — passing pt.X and pt.Y as separate syscall args here previously shoved
	// dwFlags into the wrong register and silently broke monitor detection.
	packedPoint := uintptr(uint32(pt.X)) | uintptr(uint32(pt.Y))<<32
	hMonitor, _, _ := procMonitorFromPoint.Call(packedPoint, monitorDefaultToNearest)
	if hMonitor == 0 {
		return Rect{}, 1, false
	}
	var mi monitorInfo
	mi.CbSize = uint32(unsafe.Sizeof(mi))
	if ok, _, _ := procGetMonitorInfo.Call(hMonitor, uintptr(unsafe.Pointer(&mi))); ok == 0 {
		return Rect{}, 1, false
	}
	work := mi.RcWork
	area := Rect{
		X:      int(work.Left),
		Y:      int(work.Top),
		Width:  int(work.Right - work.Left),
		Height: int(work.Bottom - work.Top),
	}

	// GetDpiForMonitor failing (e.g. on some virtualized/RDP setups) is treated as "100% scaling"
	// rather than a hard failure — the area lookup above already succeeded and is more important
	// to return than to fail the whole call over a DPI query that's secondary to it.
	scale := 1.0
	var dpiX, dpiY uint32
	if hr, _, _ := procGetDpiForMonitor.Call(hMonitor, mdtEffectiveDpi, uintptr(unsafe.Pointer(&dpiX)), uintptr(unsafe.Pointer(&dpiY))); hr == 0 && dpiX != 0 {
		scale = float64(dpiX) / stdDpi
	}
	return area, scale, true
}
