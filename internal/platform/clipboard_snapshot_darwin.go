//go:build darwin

package platform

// macOS pasteboard reads do not take the same process-wide OpenClipboard lock
// as Win32. Keep the existing native readers behind the same snapshot contract
// so the watcher and persistence pipeline are shared across platforms.
func readClipboardSnapshotDarwin(c *darwinController) (ClipboardSnapshot, error) {
	var snap ClipboardSnapshot
	if paths, ok := c.ReadClipboardFiles(); ok {
		snap.FilePaths = paths
		return snap, nil
	}
	if html, rtf, text, ok := c.ReadClipboardRichText(); ok {
		snap.HTML, snap.RTF, snap.Text = html, rtf, text
		return snap, nil
	}
	if data, width, height, ok := c.ReadClipboardImage(); ok {
		snap.ImagePNG, snap.ImageWidth, snap.ImageHeight, snap.ImageFrom = data, width, height, "TIFF"
	}
	return snap, nil
}
