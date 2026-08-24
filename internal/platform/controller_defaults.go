//go:build !windows

package platform

import "fmt"

// stubController backs every Controller capability that doesn't have a native implementation yet
// on the current OS. It keeps the app compiling and runnable per design doc §0 rule 3:
// unimplemented per-OS capabilities degrade to a no-op rather than blocking the build. Platforms
// with a partial native implementation (see controller_darwin.go) embed this by value and
// override only the methods they've implemented, so adding one more real method later is a
// one-line override here rather than a new struct copy-pasting the other thirty.
type stubController struct{}

func (stubController) Close() {}

func (stubController) RegisterHotkey(spec string, handler func()) (func(), error) {
	if _, err := ParseHotkeySpec(spec); err != nil {
		return func() {}, err
	}
	return func() {}, nil
}

func (stubController) CaptureForeground() ForegroundToken                { return nil }
func (stubController) RestoreForeground(ForegroundToken)                 {}
func (stubController) SendPaste() error                                  { return nil }
func (stubController) FocusSelf(string)                                  {}
func (stubController) PositionSelf(string, int, int, int, int)           {}
func (stubController) WatchFocusLoss([]string, func())                   {}
func (stubController) FocusIfExists(string) bool                         { return false }
func (stubController) OnSettingsChanged(func())                          {}
func (stubController) OnShowRequested(func())                            {}
func (stubController) OnHotkeySuspendRequested(func())                   {}
func (stubController) OnHotkeyResumeRequested(func())                    {}
func (stubController) SlideReveal(string, int, int, int, int, int) error { return nil }
func (stubController) SlideDismiss(string, int) error                    { return nil }
func (stubController) ActiveApp() (AppInfo, error)                       { return AppInfo{}, nil }
func (stubController) WorkAreaNearCursor() (Rect, float64, bool)         { return Rect{}, 1, false }
func (stubController) ReadClipboardImage() ([]byte, int, int, bool) {
	return nil, 0, 0, false
}
func (stubController) WriteClipboardImage([]byte) error {
	return fmt.Errorf("image clipboard support is not implemented on this platform")
}
func (stubController) ReadClipboardRichText() (string, string, string, bool) {
	return "", "", "", false
}
func (stubController) WriteClipboardRichText(string, string, string) error {
	return fmt.Errorf("rich text clipboard support is not implemented on this platform")
}
func (stubController) FileThumbnail(string) ([]byte, bool)  { return nil, false }
func (stubController) PreviewFile([]string) bool            { return false }
func (stubController) ReadClipboardFiles() ([]string, bool) { return nil, false }
func (stubController) WriteClipboardFiles([]string) error {
	return fmt.Errorf("file clipboard support is not implemented on this platform")
}
func (stubController) ClipboardSequence() (uint32, bool) { return 0, false }
func (stubController) SetAutoLaunch(bool) error          { return nil }
func (stubController) AutoLaunchEnabled() (bool, error)  { return false, nil }
func (stubController) ShowTray(TrayOptions)              {}
func (stubController) UpdateTrayPaused(bool)             {}
