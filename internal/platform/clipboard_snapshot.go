package platform

// ClipboardSnapshot is a copied, platform-neutral clipboard snapshot. All byte
// buffers are owned by the caller and are safe to use after the native clipboard
// has been closed.
type ClipboardSnapshot struct {
	Text        string
	HTML        string
	RTF         string
	FilePaths   []string
	ImagePNG    []byte
	ImageWidth  int
	ImageHeight int
	ImageFrom   string
}
