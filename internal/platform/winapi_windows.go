//go:build windows

package platform

import "syscall"

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	shcore   = syscall.NewLazyDLL("shcore.dll")

	procGetWindowRect            = user32.NewProc("GetWindowRect")
	procShowWindow               = user32.NewProc("ShowWindow")
	procRegisterHotKey           = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey         = user32.NewProc("UnregisterHotKey")
	procGetMessage               = user32.NewProc("GetMessageW")
	procTranslateMessage         = user32.NewProc("TranslateMessage")
	procDispatchMessage          = user32.NewProc("DispatchMessageW")
	procPostQuitMessage          = user32.NewProc("PostQuitMessage")
	procPostThreadMessage        = user32.NewProc("PostThreadMessageW")
	procPostMessage              = user32.NewProc("PostMessageW")
	procRegisterClassEx          = user32.NewProc("RegisterClassExW")
	procCreateWindowEx           = user32.NewProc("CreateWindowExW")
	procDestroyWindow            = user32.NewProc("DestroyWindow")
	procDefWindowProc            = user32.NewProc("DefWindowProcW")
	procGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
	procSetForegroundWindow      = user32.NewProc("SetForegroundWindow")
	procGetWindowThreadPID       = user32.NewProc("GetWindowThreadProcessId")
	procFindWindow               = user32.NewProc("FindWindowW")
	procFindWindowEx             = user32.NewProc("FindWindowExW")
	procMonitorFromWindow        = user32.NewProc("MonitorFromWindow")
	procSetWindowLongPtr         = user32.NewProc("SetWindowLongPtrW")
	procCallWindowProc           = user32.NewProc("CallWindowProcW")
	procGetAsyncKeyState         = user32.NewProc("GetAsyncKeyState")
	procAttachThreadInput        = user32.NewProc("AttachThreadInput")
	procBringWindowToTop         = user32.NewProc("BringWindowToTop")
	procSetWindowPos             = user32.NewProc("SetWindowPos")
	procSetWinEventHook          = user32.NewProc("SetWinEventHook")
	procUnhookWinEvent           = user32.NewProc("UnhookWinEvent")
	procSetFocus                 = user32.NewProc("SetFocus")
	procSendInput                = user32.NewProc("SendInput")
	procGetCursorPos             = user32.NewProc("GetCursorPos")
	procMonitorFromPoint         = user32.NewProc("MonitorFromPoint")
	procGetMonitorInfo           = user32.NewProc("GetMonitorInfoW")
	procOpenClipboard            = user32.NewProc("OpenClipboard")
	procCloseClipboard           = user32.NewProc("CloseClipboard")
	procGetClipboardData         = user32.NewProc("GetClipboardData")
	procIsClipboardFormatAvl     = user32.NewProc("IsClipboardFormatAvailable")
	procGetClipboardSeqNum       = user32.NewProc("GetClipboardSequenceNumber")
	procAddClipboardListener     = user32.NewProc("AddClipboardFormatListener")
	procRemoveClipboardListener  = user32.NewProc("RemoveClipboardFormatListener")
	procEmptyClipboard           = user32.NewProc("EmptyClipboard")
	procSetClipboardData         = user32.NewProc("SetClipboardData")
	procRegisterClipboardFormat  = user32.NewProc("RegisterClipboardFormatW")
	procLoadCursor               = user32.NewProc("LoadCursorW")
	procLoadIcon                 = user32.NewProc("LoadIconW")
	procCreateIconFromResourceEx = user32.NewProc("CreateIconFromResourceEx")
	procGetSystemMetrics         = user32.NewProc("GetSystemMetrics")
	procGetIconInfo              = user32.NewProc("GetIconInfo")
	procDestroyIcon              = user32.NewProc("DestroyIcon")
	procCreatePopupMenu          = user32.NewProc("CreatePopupMenu")
	procAppendMenu               = user32.NewProc("AppendMenuW")
	procDestroyMenu              = user32.NewProc("DestroyMenu")
	procTrackPopupMenu           = user32.NewProc("TrackPopupMenu")
	procSetForegroundWindow2     = procSetForegroundWindow
	procGetDC                    = user32.NewProc("GetDC")
	procReleaseDC                = user32.NewProc("ReleaseDC")

	procCreateMutex        = kernel32.NewProc("CreateMutexW")
	procGetModuleHandle    = kernel32.NewProc("GetModuleHandleW")
	procGlobalLock         = kernel32.NewProc("GlobalLock")
	procGlobalUnlock       = kernel32.NewProc("GlobalUnlock")
	procGlobalSize         = kernel32.NewProc("GlobalSize")
	procGlobalAlloc        = kernel32.NewProc("GlobalAlloc")
	procGlobalFree         = kernel32.NewProc("GlobalFree")
	procOpenProcess        = kernel32.NewProc("OpenProcess")
	procGetCurrentThreadId = kernel32.NewProc("GetCurrentThreadId")
	procQueryFullProcImg   = kernel32.NewProc("QueryFullProcessImageNameW")
	procCloseHandle        = kernel32.NewProc("CloseHandle")

	procShellNotifyIcon = shell32.NewProc("Shell_NotifyIconW")
	procSHGetFileInfo   = shell32.NewProc("SHGetFileInfoW")
	procDragQueryFile   = shell32.NewProc("DragQueryFileW")

	procGetDIBits          = gdi32.NewProc("GetDIBits")
	procCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	procSelectObject       = gdi32.NewProc("SelectObject")
	procDeleteDC           = gdi32.NewProc("DeleteDC")
	procDeleteObject       = gdi32.NewProc("DeleteObject")

	procGetDpiForMonitor = shcore.NewProc("GetDpiForMonitor")
)

type point struct{ X, Y int32 }

type rect struct{ Left, Top, Right, Bottom int32 }

type monitorInfo struct {
	CbSize    uint32
	RcMonitor rect
	RcWork    rect
	DwFlags   uint32
}

type wndClassEx struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

type msg struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
}

const (
	wmDestroy         = 0x0002
	wmHotkey          = 0x0312
	wmClipboardUpdate = 0x031D
	wmCommand         = 0x0111
	wmApp             = 0x8000
	wmLButtonUp       = 0x0202
	wmLButtonDblClk   = 0x0203
	wmRButtonUp       = 0x0205
	wmClose           = 0x0010
	wmQuit            = 0x0012

	trayCallbackMsg   = wmApp + 1
	wmExecute         = wmApp + 2
	wmSettingsChanged = wmApp + 3
	wmShowMain        = wmApp + 4
	wmSuspendHotkey   = wmApp + 5
	wmResumeHotkey    = wmApp + 6

	modAlt      = 0x0001
	modControl  = 0x0002
	modShift    = 0x0004
	modWin      = 0x0008
	modNoRepeat = 0x4000

	monitorDefaultToNearest = 2

	mdtEffectiveDpi = 0    // GetDpiForMonitor's MDT_EFFECTIVE_DPI
	stdDpi          = 96.0 // the DPI Windows treats as 100% scaling

	cfBitmap      = 2
	cfDIB         = 8
	cfUnicodeText = 13
	cfHDrop       = 15
	cfDIBV5       = 17

	dropEffectCopy = 1

	biRGB       = 0
	biBitfields = 3

	lcsSRGB     = 0x73524742 // 'sRGB' as a little-endian DWORD, BITMAPV5HEADER's bV5CSType
	lcsGMImages = 4          // BITMAPV5HEADER's bV5Intent

	inputKeyboard  = 1
	keyEventFKeyUp = 0x0002

	vkControl = 0x11
	vkShift   = 0x10
	vkMenu    = 0x12 // Alt
	vkSpace   = 0x20
	vkV       = 0x56

	shgfiIcon              = 0x000000100
	shgfiUseFileAttributes = 0x000000010
	fileAttributeNormal    = 0x80

	smCxIcon = 11
	smCyIcon = 12

	processQueryLimitedInformation = 0x1000

	idiApplication = 32512
	iconResVersion = 0x00030000
	lrDefaultColor = 0x0000

	nifMessage = 0x00000001
	nifIcon    = 0x00000002
	nifTip     = 0x00000004
	nimAdd     = 0x00000000
	nimModify  = 0x00000001
	nimDelete  = 0x00000002

	mfString    = 0x00000000
	mfSeparator = 0x00000800
	mfDisabled  = 0x00000002
	mfGrayed    = 0x00000001

	tpmReturnCmd  = 0x0100
	tpmRightAlign = 0x0008

	gmemMoveable = 0x0002

	swpNoZOrder   = 0x0004
	swpNoMove     = 0x0002
	swpNoActivate = 0x0010

	gwlpWndProc = -4 // SetWindowLongPtrW/GetWindowLongPtrW index for the window procedure

	wmSysCommand = 0x0112
	scKeyMenu    = 0xF100 // WM_SYSCOMMAND's low-order 4 bits carry extra info, so mask before comparing

	eventSystemForeground = 0x0003
	winEventOutOfContext  = 0x0000

	swHide = 0
	swShow = 5
)

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

// bitmapV5Header mirrors Win32's BITMAPV5HEADER (124 bytes) — the CF_DIBV5 clipboard format's
// header, used instead of plain CF_DIB (bitmapInfoHeader above) specifically because it has an
// explicit bV5AlphaMask field: that's what tells an alpha-aware receiving app "byte 4 of every
// pixel is real alpha," which plain 32bpp CF_DIB has no way to signal (see writeClipboardImage's
// doc comment). CIEXYZTRIPLE (Endpoints) is 9 consecutive LONGs (3 CIEXYZ colour points, each 3
// FXPT2DOT16 fixed-point LONGs) — left zeroed here along with the gamma/profile fields, which are
// only meaningful when Compression selects LCS_CALIBRATED_RGB, not LCS_sRGB.
type bitmapV5Header struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
	RedMask       uint32
	GreenMask     uint32
	BlueMask      uint32
	AlphaMask     uint32
	CSType        uint32
	Endpoints     [9]int32
	GammaRed      uint32
	GammaGreen    uint32
	GammaBlue     uint32
	Intent        uint32
	ProfileData   uint32
	ProfileSize   uint32
	Reserved      uint32
}

type iconInfo struct {
	FIcon    int32
	XHotspot uint32
	YHotspot uint32
	HbmMask  uintptr
	HbmColor uintptr
}

// dropFiles mirrors the Win32 DROPFILES header that precedes a CF_HDROP payload: PFiles is the
// byte offset (from the start of this struct) to the file list, a sequence of null-terminated
// strings ending in an extra null terminator. FWide=1 marks the list as UTF-16 (matching
// DragQueryFileW), which is what we always write.
type dropFiles struct {
	PFiles uint32
	Pt     point
	FNC    int32
	FWide  int32
}

type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

// notifyIconData mirrors NOTIFYICONDATAW in full (including the trailing fields we never set)
// so cbSize matches what Shell_NotifyIcon expects on this Windows version.
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

type keybdInput struct {
	WVk         uint16
	WScan       uint16
	DwFlags     uint32
	Time        uint32
	DwExtraInfo uintptr
}

// input mirrors the Win32 INPUT struct for keyboard events (type=INPUT_KEYBOARD). Go already
// aligns Ki to an 8-byte boundary because it embeds a uintptr, matching the union's natural
// alignment; the trailing padding brings the total to 40 bytes to match sizeof(INPUT) on amd64.
type input struct {
	Type uint32
	Ki   keybdInput
	_    [8]byte
}
