//go:build windows

package platform

import (
	"bytes"
	"fmt"
	"image/png"
	"syscall"
	"unsafe"
)

const (
	maxClipboardTextBytes  = 32 << 20
	maxClipboardImageBytes = 256 << 20
	maxClipboardFilesBytes = 16 << 20
	maxClipboardFileCount  = 4096
)

// readClipboardSnapshot performs one short OpenClipboard session. It copies
// only the representation that will be used, closes the native clipboard, and
// does all decoding/encoding afterwards.
func readClipboardSnapshot() (ClipboardSnapshot, error) {
	var snap ClipboardSnapshot
	if opened, _, _ := procOpenClipboard.Call(0); opened == 0 {
		return snap, fmt.Errorf("OpenClipboard failed")
	}

	if isClipboardFormatAvailable(cfHDrop) {
		if paths := readClipboardFilesLockedSnapshot(); len(paths) > 0 {
			procCloseClipboard.Call()
			snap.FilePaths = paths
			return snap, nil
		}
	}

	htmlFormat := registeredClipboardFormat("HTML Format")
	rtfFormat := registeredClipboardFormat("Rich Text Format")
	html, rtf := "", ""
	if htmlFormat != 0 && isClipboardFormatAvailable(htmlFormat) {
		if b := readClipboardBlockLocked(htmlFormat, maxClipboardTextBytes); len(b) > 0 {
			html = string(bytes.TrimRight(b, "\x00"))
		}
	}
	if rtfFormat != 0 && isClipboardFormatAvailable(rtfFormat) {
		if b := readClipboardBlockLocked(rtfFormat, maxClipboardTextBytes); len(b) > 0 {
			rtf = string(bytes.TrimRight(b, "\x00"))
		}
	}
	text := readUnicodeTextLimitedLocked(maxClipboardTextBytes)
	if stringsTrimSpaceNonEmpty(text) && (html != "" || rtf != "") {
		procCloseClipboard.Call()
		snap.HTML, snap.RTF, snap.Text = html, rtf, text
		return snap, nil
	}

	var imageData []byte
	imageFrom := ""
	var fallbackFormat uintptr
	fallbackName := ""
	if pngFormat := registeredClipboardFormat("PNG"); pngFormat != 0 && isClipboardFormatAvailable(pngFormat) {
		imageData = readClipboardBlockLocked(pngFormat, maxClipboardImageBytes)
		if !bytes.HasPrefix(imageData, []byte("\x89PNG\r\n\x1a\n")) {
			imageData = nil
		} else {
			imageFrom = "PNG"
		}
	}
	if len(imageData) == 0 {
		for _, candidate := range []struct {
			id   uint32
			name string
		}{{cfDIBV5, "CF_DIBV5"}, {cfDIB, "CF_DIB"}} {
			if !isClipboardFormatAvailable(uintptr(candidate.id)) {
				continue
			}
			if b := readClipboardBlockLocked(uintptr(candidate.id), maxClipboardImageBytes); len(b) > 0 {
				imageData, imageFrom = b, candidate.name
				if candidate.id == cfDIBV5 {
					// Most clipboards publish both formats. If the richer V5
					// block is malformed, retry the legacy block after releasing
					// the clipboard rather than discarding an otherwise valid image.
					fallbackFormat, fallbackName = cfDIB, "CF_DIB"
				}
				break
			}
		}
	}
	if len(imageData) > 0 {
		procCloseClipboard.Call()
		if data, width, height, ok := decodeClipboardImageBlock(imageData, imageFrom); ok {
			snap.ImagePNG, snap.ImageWidth, snap.ImageHeight, snap.ImageFrom = data, width, height, imageFrom
			return snap, nil
		}
		if fallbackFormat != 0 {
			if fallbackData, err := readClipboardBlockAfterClose(fallbackFormat, maxClipboardImageBytes); err == nil {
				if data, width, height, ok := decodeClipboardImageBlock(fallbackData, fallbackName); ok {
					snap.ImagePNG, snap.ImageWidth, snap.ImageHeight, snap.ImageFrom = data, width, height, fallbackName
					return snap, nil
				}
			}
		}
		snap.Text = text
		return snap, nil
	}
	procCloseClipboard.Call()
	snap.Text = text
	return snap, nil
}

func decodeClipboardImageBlock(data []byte, from string) ([]byte, int, int, bool) {
	if from == "PNG" {
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, 0, 0, false
		}
		b := img.Bounds()
		return data, b.Dx(), b.Dy(), true
	}
	img, err := decodeDIB(data)
	if err != nil {
		return nil, 0, 0, false
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, 0, 0, false
	}
	b := img.Bounds()
	return out.Bytes(), b.Dx(), b.Dy(), true
}

func readClipboardBlockAfterClose(format uintptr, max int) ([]byte, error) {
	if opened, _, _ := procOpenClipboard.Call(0); opened == 0 {
		return nil, fmt.Errorf("OpenClipboard failed")
	}
	defer procCloseClipboard.Call()
	return readClipboardBlockLocked(format, max), nil
}

func stringsTrimSpaceNonEmpty(s string) bool {
	for _, r := range s {
		if r > ' ' {
			return true
		}
	}
	return false
}

func readClipboardBlockLocked(format uintptr, max int) []byte {
	hMem, _, _ := procGetClipboardData.Call(format)
	if hMem == 0 {
		return nil
	}
	size, _, _ := procGlobalSize.Call(hMem)
	if size == 0 || size > uintptr(max) {
		return nil
	}
	ptr, _, _ := procGlobalLock.Call(hMem)
	if ptr == 0 {
		return nil
	}
	defer procGlobalUnlock.Call(hMem)
	data := make([]byte, int(size))
	copy(data, unsafe.Slice((*byte)(unsafe.Pointer(ptr)), int(size)))
	return data
}

func readUnicodeTextLimitedLocked(max int) string {
	hMem, _, _ := procGetClipboardData.Call(cfUnicodeText)
	if hMem == 0 {
		return ""
	}
	size, _, _ := procGlobalSize.Call(hMem)
	if size < 2 || size > uintptr(max) {
		return ""
	}
	ptr, _, _ := procGlobalLock.Call(hMem)
	if ptr == 0 {
		return ""
	}
	defer procGlobalUnlock.Call(hMem)
	return syscall.UTF16ToString(unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), int(size)/2))
}

func readClipboardFilesLockedSnapshot() []string {
	hDrop, _, _ := procGetClipboardData.Call(cfHDrop)
	if hDrop == 0 {
		return nil
	}
	count, _, _ := procDragQueryFile.Call(hDrop, 0xFFFFFFFF, 0, 0)
	if count == 0 {
		return nil
	}
	if count > maxClipboardFileCount {
		count = maxClipboardFileCount
	}
	paths := make([]string, 0, count)
	var total int
	for i := uintptr(0); i < count; i++ {
		n, _, _ := procDragQueryFile.Call(hDrop, i, 0, 0)
		if n == 0 || total > maxClipboardFilesBytes-int(n+1)*2 {
			continue
		}
		buf := make([]uint16, int(n)+1)
		got, _, _ := procDragQueryFile.Call(hDrop, i, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		if got == 0 {
			continue
		}
		paths = append(paths, syscall.UTF16ToString(buf[:got]))
		total += int((got + 1) * 2)
	}
	return paths
}
