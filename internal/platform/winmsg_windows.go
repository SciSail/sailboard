//go:build windows

package platform

import (
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

const msgWindowClassName = "SailBoardMessageWindow"

// hwndMessage is the special parent handle that creates a message-only window: it never
// appears on screen but can still receive posted/sent messages, which is all RegisterHotKey
// and Shell_NotifyIcon callbacks need.
const hwndMessage = ^uintptr(0) - 2 // (HWND)-3

// msgWindow owns one hidden window + Win32 message loop that both global hotkeys and the tray
// icon deliver their events through. Win32 message queues are thread-affine, so the loop runs
// on a single goroutine locked to its OS thread for the lifetime of the app.
type msgWindow struct {
	hwnd uintptr

	mu         sync.Mutex
	hotkeys    map[int32]func()
	nextHotkey int32

	tray *trayState

	onSettingsChanged  func()
	onShowRequested    func()
	onHotkeySuspend    func()
	onHotkeyResume     func()
	clipboardChanges   chan struct{}
	clipboardListening bool

	jobsMu sync.Mutex
	jobs   []func()

	readyErr chan error
}

var wndProcCallback = syscall.NewCallback(wndProcDispatch)

var activeWindows sync.Map // hwnd(uintptr) -> *msgWindow

func newMsgWindow() (*msgWindow, error) {
	w := &msgWindow{hotkeys: make(map[int32]func()), readyErr: make(chan error, 1), clipboardChanges: make(chan struct{}, 1)}
	go w.run()
	if err := <-w.readyErr; err != nil {
		return nil, err
	}
	return w, nil
}

// run creates the hidden window and pumps its message queue until WM_QUIT. It must stay on one
// OS thread for the window's entire lifetime.
func (w *msgWindow) run() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hInstance, _, _ := procGetModuleHandle.Call(0)
	classNamePtr, _ := syscall.UTF16PtrFromString(msgWindowClassName)

	wc := wndClassEx{
		LpfnWndProc:   wndProcCallback,
		HInstance:     hInstance,
		LpszClassName: classNamePtr,
	}
	wc.CbSize = uint32(unsafe.Sizeof(wc))
	if atom, _, err := procRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 {
		w.readyErr <- fmt.Errorf("RegisterClassEx: %w", err)
		return
	}

	titlePtr, _ := syscall.UTF16PtrFromString(msgWindowClassName)
	hwnd, _, err := procCreateWindowEx.Call(0,
		uintptr(unsafe.Pointer(classNamePtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		0, 0, 0, 0, 0,
		hwndMessage,
		0, hInstance, 0)
	if hwnd == 0 {
		w.readyErr <- fmt.Errorf("CreateWindowEx: %w", err)
		return
	}
	w.hwnd = hwnd
	activeWindows.Store(hwnd, w)
	if result, _, _ := procAddClipboardListener.Call(hwnd); result != 0 {
		w.clipboardListening = true
	}
	w.readyErr <- nil

	var m msg
	for {
		ret, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(ret) <= 0 { // 0 = WM_QUIT, -1 = error
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}
	activeWindows.Delete(hwnd)
}

func wndProcDispatch(hwnd, message, wParam, lParam uintptr) uintptr {
	if v, ok := activeWindows.Load(hwnd); ok {
		w := v.(*msgWindow)
		if handled, result := w.handle(uint32(message), wParam, lParam); handled {
			return result
		}
	}
	ret, _, _ := procDefWindowProc.Call(hwnd, message, wParam, lParam)
	return ret
}

func (w *msgWindow) handle(message uint32, wParam, lParam uintptr) (bool, uintptr) {
	switch message {
	case wmClipboardUpdate:
		select {
		case w.clipboardChanges <- struct{}{}:
		default:
		}
		return true, 0
	case wmHotkey:
		w.mu.Lock()
		handler := w.hotkeys[int32(wParam)]
		w.mu.Unlock()
		if handler != nil {
			go handler()
		}
		return true, 0
	case trayCallbackMsg:
		if w.tray != nil {
			w.tray.onCallback(uint32(lParam))
		}
		return true, 0
	case wmExecute:
		w.jobsMu.Lock()
		var job func()
		if len(w.jobs) > 0 {
			job = w.jobs[0]
			w.jobs = w.jobs[1:]
		}
		w.jobsMu.Unlock()
		if job != nil {
			job()
		}
		return true, 0
	case wmSettingsChanged:
		w.mu.Lock()
		cb := w.onSettingsChanged
		w.mu.Unlock()
		if cb != nil {
			go cb()
		}
		return true, 0
	case wmShowMain:
		w.mu.Lock()
		cb := w.onShowRequested
		w.mu.Unlock()
		if cb != nil {
			go cb()
		}
		return true, 0
	case wmSuspendHotkey:
		w.mu.Lock()
		cb := w.onHotkeySuspend
		w.mu.Unlock()
		if cb != nil {
			go cb()
		}
		return true, 0
	case wmResumeHotkey:
		w.mu.Lock()
		cb := w.onHotkeyResume
		w.mu.Unlock()
		if cb != nil {
			go cb()
		}
		return true, 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return true, 0
	}
	return false, 0
}

func (w *msgWindow) setOnShowRequested(cb func()) {
	w.mu.Lock()
	w.onShowRequested = cb
	w.mu.Unlock()
}

func (w *msgWindow) setOnSettingsChanged(cb func()) {
	w.mu.Lock()
	w.onSettingsChanged = cb
	w.mu.Unlock()
}

func (w *msgWindow) setOnHotkeySuspend(cb func()) {
	w.mu.Lock()
	w.onHotkeySuspend = cb
	w.mu.Unlock()
}

func (w *msgWindow) setOnHotkeyResume(cb func()) {
	w.mu.Lock()
	w.onHotkeyResume = cb
	w.mu.Unlock()
}

func (w *msgWindow) close() {
	w.mu.Lock()
	tray := w.tray
	w.mu.Unlock()
	if tray != nil {
		tray.remove()
	}
	// DestroyWindow, like RegisterHotKey, must be called from the thread that created the
	// window — it is silently ignored (or fails) otherwise, so it goes through the same
	// message-loop job queue as registerHotkey rather than calling it directly here.
	w.runSync(func() {
		if w.clipboardListening {
			procRemoveClipboardListener.Call(w.hwnd)
			w.clipboardListening = false
		}
		procDestroyWindow.Call(w.hwnd)
	})
}

// runSync marshals fn onto the message loop's own OS thread and blocks until it has run.
// RegisterHotKey/UnregisterHotKey (unlike Shell_NotifyIcon) fail with "Invalid window; it
// belongs to other thread" unless called from the exact thread that owns the window, so those
// calls can't just take w.hwnd from whatever goroutine happens to call in.
func (w *msgWindow) runSync(fn func()) {
	done := make(chan struct{})
	w.jobsMu.Lock()
	w.jobs = append(w.jobs, func() { fn(); close(done) })
	w.jobsMu.Unlock()
	procPostMessage.Call(w.hwnd, wmExecute, 0, 0)
	<-done
}

// registerHotkey allocates a fresh hotkey id and calls RegisterHotKey on the message loop
// thread, tying delivery to our message window; WM_HOTKEY then lands in its queue, pumped by
// msgWindow.run.
func (w *msgWindow) registerHotkey(mods uint32, vk uint32, handler func()) (func(), error) {
	w.mu.Lock()
	id := w.nextHotkey + 1
	w.nextHotkey = id
	w.mu.Unlock()

	var ok uintptr
	var callErr error
	w.runSync(func() {
		ok, _, callErr = procRegisterHotKey.Call(w.hwnd, uintptr(id), uintptr(mods|modNoRepeat), uintptr(vk))
	})
	if ok == 0 {
		return nil, fmt.Errorf("RegisterHotKey: %w", callErr)
	}

	w.mu.Lock()
	w.hotkeys[id] = handler
	w.mu.Unlock()

	return func() {
		w.runSync(func() { procUnregisterHotKey.Call(w.hwnd, uintptr(id)) })
		w.mu.Lock()
		delete(w.hotkeys, id)
		w.mu.Unlock()
	}, nil
}
