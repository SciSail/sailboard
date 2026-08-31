//go:build windows

package platform

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image/png"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
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
		// Explorer commonly exposes CF_HDROP through delayed Shell/OLE
		// rendering. Copy its one HGLOBAL block and close the clipboard before
		// parsing any paths; enumerating thousands of entries while the global
		// clipboard is open would block every other application's copy/paste.
		block := readClipboardBlockLocked(cfHDrop, maxClipboardFilesBytes)
		procCloseClipboard.Call()
		if paths := decodeClipboardFilesBlock(block, maxClipboardFileCount); len(paths) > 0 {
			snap.FilePaths = paths
		}
		// A clipboard advertising CF_HDROP is a file copy even if its delayed
		// payload was unavailable or malformed. Do not reopen it to probe rich
		// text/image fallbacks; missing one history capture is safer than
		// competing with the user's file operation.
		return snap, nil
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

// decodeClipboardFilesBlock parses a copied CF_HDROP HGLOBAL after the system
// clipboard has already been closed. DROPFILES contains a byte offset followed
// by a double-NUL-terminated path list, normally UTF-16 on modern Windows.
func decodeClipboardFilesBlock(data []byte, maxCount int) []string {
	const dropFilesHeaderBytes = 20
	if len(data) < dropFilesHeaderBytes || maxCount <= 0 {
		return nil
	}
	offset := int(binary.LittleEndian.Uint32(data[0:4]))
	if offset < dropFilesHeaderBytes || offset >= len(data) {
		return nil
	}
	payload := data[offset:]
	wide := binary.LittleEndian.Uint32(data[16:20]) != 0
	paths := make([]string, 0, 1)
	if wide {
		if len(payload) < 4 {
			return nil
		}
		if len(payload)%2 != 0 {
			payload = payload[:len(payload)-1]
		}
		units := unsafe.Slice((*uint16)(unsafe.Pointer(&payload[0])), len(payload)/2)
		start := 0
		for i, unit := range units {
			if unit != 0 {
				continue
			}
			if i == start {
				break
			}
			paths = append(paths, syscall.UTF16ToString(units[start:i]))
			if len(paths) >= maxCount {
				break
			}
			start = i + 1
		}
		return paths
	}

	// Legacy producers may set fWide=FALSE and use the active Windows ANSI
	// code page. Convert each entry explicitly instead of assuming UTF-8.
	start := 0
	for i, b := range payload {
		if b != 0 {
			continue
		}
		if i == start {
			break
		}
		if path := ansiClipboardPath(payload[start:i]); path != "" {
			paths = append(paths, path)
		}
		if len(paths) >= maxCount {
			break
		}
		start = i + 1
	}
	return paths
}

func ansiClipboardPath(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	needed, err := windows.MultiByteToWideChar(0, 0, &data[0], int32(len(data)), nil, 0)
	if err != nil || needed <= 0 {
		return ""
	}
	wide := make([]uint16, needed)
	written, err := windows.MultiByteToWideChar(0, 0, &data[0], int32(len(data)), &wide[0], needed)
	if err != nil || written <= 0 {
		return ""
	}
	return syscall.UTF16ToString(wide[:written])
}
