//go:build windows

package platform

// windowsController wires the hidden message window (hotkeys + tray) together with the
// stateless Win32 helper functions into the cross-platform Controller interface app.go depends
// on.
type windowsController struct {
	appName string
	win     *msgWindow
}

// New creates the Windows controller: one hidden message-only window backing both global
// hotkeys and the tray icon, per the shared design in winmsg_windows.go.
func New(appName string) (Controller, error) {
	win, err := newMsgWindow()
	if err != nil {
		return nil, err
	}
	return &windowsController{appName: appName, win: win}, nil
}

func (c *windowsController) Close() {
	c.win.close()
}

func (c *windowsController) RegisterHotkey(spec string, handler func()) (func(), error) {
	hk, err := ParseHotkeySpec(spec)
	if err != nil {
		return nil, err
	}
	vk, err := keyToVK(hk.Key)
	if err != nil {
		return nil, err
	}
	return c.win.registerHotkey(hotkeyModifiers(hk), vk, handler)
}

func (c *windowsController) CaptureForeground() ForegroundToken      { return captureForeground() }
func (c *windowsController) RestoreForeground(token ForegroundToken) { restoreForeground(token) }
func (c *windowsController) SendPaste() error                        { return sendPaste() }
func (c *windowsController) FocusSelf(title string)                  { focusSelf(title) }
func (c *windowsController) PositionSelf(title string, x, y, width, height int) {
	positionSelf(title, x, y, width, height)
}
func (c *windowsController) WatchFocusLoss(titles []string, onLostFocus func()) {
	watchFocusLoss(c.win, titles, onLostFocus)
}
func (c *windowsController) FocusIfExists(title string) bool { return focusIfExists(title) }
func (c *windowsController) OnSettingsChanged(callback func()) {
	c.win.setOnSettingsChanged(callback)
}

func (c *windowsController) OnShowRequested(callback func()) {
	c.win.setOnShowRequested(callback)
}
func (c *windowsController) OnHotkeySuspendRequested(callback func()) {
	c.win.setOnHotkeySuspend(callback)
}
func (c *windowsController) OnHotkeyResumeRequested(callback func()) {
	c.win.setOnHotkeyResume(callback)
}
func (c *windowsController) SlideReveal(title string, x, y, width, height, durationMs int) error {
	return slideReveal(title, x, y, width, height, durationMs)
}
func (c *windowsController) SlideDismiss(title string, durationMs int) error {
	return slideDismiss(title, durationMs)
}

func (c *windowsController) ActiveApp() (AppInfo, error) { return activeApp() }
func (c *windowsController) WorkAreaNearCursor() (Rect, float64, bool) {
	return workAreaNearCursor()
}

func (c *windowsController) ReadClipboardImage() (data []byte, width, height int, ok bool) {
	return readClipboardImage()
}

func (c *windowsController) WriteClipboardImage(data []byte) error {
	return writeClipboardImage(data)
}

func (c *windowsController) ReadClipboardRichText() (html, rtf, text string, ok bool) {
	return readClipboardRichText()
}

func (c *windowsController) WriteClipboardRichText(html, rtf, text string) error {
	return writeClipboardRichText(html, rtf, text)
}

func (c *windowsController) FileThumbnail(path string) (data []byte, ok bool) {
	return fileThumbnail(path)
}

// PreviewFile has no Windows equivalent (no Quick Look-style system preview panel) — always
// reports false so the frontend's Space handler harmlessly no-ops on this platform.
func (c *windowsController) PreviewFile(paths []string) bool { return false }

func (c *windowsController) ReadClipboardFiles() (paths []string, ok bool) {
	return readClipboardFiles()
}

func (c *windowsController) WriteClipboardFiles(paths []string) error {
	return writeClipboardFiles(paths)
}

func (c *windowsController) ClipboardSequence() (uint32, bool) { return clipboardSequence() }

func (c *windowsController) SetAutoLaunch(enabled bool) error {
	return setAutoLaunch(c.appName, enabled)
}
func (c *windowsController) AutoLaunchEnabled() (bool, error) {
	return autoLaunchEnabled(c.appName)
}

func (c *windowsController) ShowTray(opts TrayOptions)    { c.win.showTray(opts) }
func (c *windowsController) UpdateTrayPaused(paused bool) { c.win.updateTrayPaused(paused) }
