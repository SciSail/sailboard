//go:build windows

package platform

import (
	"bytes"
	"image/png"
	"syscall"
	"unsafe"
)

const (
	trayMenuShow  = 1
	trayMenuPause = 2
	trayMenuQuit  = 3
)

// trayState owns the Shell_NotifyIcon lifecycle for the tray icon hosted on w.hwnd. Left-click
// shows the window; right-click pops a small Show / Pause-Resume / Quit menu, matching design
// doc §31.3's "系统托盘图标" first-version scope.
type trayState struct {
	w     *msgWindow
	opts  TrayOptions
	hIcon uintptr
	// ownsIcon marks whether hIcon was created by us (via CreateIconFromResourceEx, from
	// opts.IconPNG) and must be destroyed on cleanup, versus a shared system icon from LoadIcon
	// which must NOT be destroyed (per MSDN).
	ownsIcon bool
	paused   bool
}

func (w *msgWindow) showTray(opts TrayOptions) {
	w.mu.Lock()
	if w.tray != nil {
		w.tray.opts = opts
		w.mu.Unlock()
		return
	}
	w.mu.Unlock()

	hIcon, owns := iconFromPNGBytes(opts.IconPNG)
	if hIcon == 0 {
		hIcon, _, _ = procLoadIcon.Call(0, idiApplication)
		owns = false
	}
	t := &trayState{w: w, opts: opts, hIcon: hIcon, ownsIcon: owns}
	w.mu.Lock()
	w.tray = t
	w.mu.Unlock()

	t.add()
}

// iconFromPNGBytes builds an HICON directly from PNG-encoded bytes via CreateIconFromResourceEx,
// which has supported PNG-format icon resources natively since Windows Vista — no need to
// convert to a classic DIB+mask pair first. ok is false (hIcon left 0) for an empty/malformed
// image, letting the caller fall back to a generic system icon.
func iconFromPNGBytes(data []byte) (hIcon uintptr, ok bool) {
	if len(data) == 0 {
		return 0, false
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, false
	}
	h, _, _ := procCreateIconFromResourceEx.Call(
		uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)),
		1, iconResVersion,
		uintptr(cfg.Width), uintptr(cfg.Height),
		lrDefaultColor,
	)
	if h == 0 {
		return 0, false
	}
	return h, true
}

func (w *msgWindow) updateTrayPaused(paused bool) {
	w.mu.Lock()
	t := w.tray
	w.mu.Unlock()
	if t == nil {
		return
	}
	t.paused = paused
	t.modify()
}

func (t *trayState) nid() notifyIconData {
	var nid notifyIconData
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.HWnd = t.w.hwnd
	nid.UID = 1
	nid.UFlags = nifMessage | nifIcon | nifTip
	nid.UCallbackMessage = trayCallbackMsg
	nid.HIcon = t.hIcon
	tip := "SailBoard"
	if t.paused {
		tip = "SailBoard (已暂停)"
	}
	copyUTF16(nid.SzTip[:], tip)
	return nid
}

func (t *trayState) add() {
	nid := t.nid()
	procShellNotifyIcon.Call(nimAdd, uintptr(unsafe.Pointer(&nid)))
}

func (t *trayState) modify() {
	nid := t.nid()
	procShellNotifyIcon.Call(nimModify, uintptr(unsafe.Pointer(&nid)))
}

func (t *trayState) remove() {
	nid := t.nid()
	procShellNotifyIcon.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))
	if t.ownsIcon && t.hIcon != 0 {
		procDestroyIcon.Call(t.hIcon)
	}
}

// onCallback handles the WM_APP+1 message Shell_NotifyIcon delivers on every mouse event over
// the tray icon; lParam's low word carries the original mouse message id.
func (t *trayState) onCallback(lParam uint32) {
	switch lParam & 0xffff {
	case wmLButtonUp, wmLButtonDblClk:
		if t.opts.OnShow != nil {
			t.opts.OnShow()
		}
	case wmRButtonUp:
		t.showMenu()
	}
}

func (t *trayState) showMenu() {
	hMenu, _, _ := procCreatePopupMenu.Call()
	if hMenu == 0 {
		return
	}
	defer procDestroyMenu.Call(hMenu)

	pauseLabel := "暂停记录"
	if t.paused {
		pauseLabel = "恢复记录"
	}
	appendMenuString(hMenu, trayMenuShow, "显示 SailBoard")
	appendMenuString(hMenu, trayMenuPause, pauseLabel)
	procAppendMenu.Call(hMenu, mfSeparator, 0, 0)
	appendMenuString(hMenu, trayMenuQuit, "退出")

	var pt point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))

	// Required by Microsoft docs so the popup menu dismisses correctly when the user clicks
	// elsewhere: the owner window must be the foreground window around TrackPopupMenu.
	procSetForegroundWindow.Call(t.w.hwnd)
	cmd, _, _ := procTrackPopupMenu.Call(hMenu, tpmReturnCmd|tpmRightAlign,
		uintptr(pt.X), uintptr(pt.Y), 0, t.w.hwnd, 0)

	switch cmd {
	case trayMenuShow:
		if t.opts.OnShow != nil {
			t.opts.OnShow()
		}
	case trayMenuPause:
		if t.opts.OnTogglePause != nil {
			t.opts.OnTogglePause()
		}
	case trayMenuQuit:
		if t.opts.OnQuit != nil {
			t.opts.OnQuit()
		}
	}
}

func appendMenuString(hMenu uintptr, id uintptr, text string) {
	ptr, err := syscall.UTF16PtrFromString(text)
	if err != nil {
		return
	}
	procAppendMenu.Call(hMenu, mfString, id, uintptr(unsafe.Pointer(ptr)))
}

func copyUTF16(dst []uint16, s string) {
	src, err := syscall.UTF16FromString(s)
	if err != nil {
		return
	}
	n := len(src)
	if n > len(dst) {
		n = len(dst)
	}
	copy(dst[:n], src[:n])
}
