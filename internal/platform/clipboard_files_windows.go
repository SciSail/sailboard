//go:build windows

package platform

import (
	"fmt"
	"syscall"
	"unsafe"
)

// readClipboardFiles reads CF_HDROP (the format Explorer writes for a Ctrl+C on files/folders)
// from the clipboard, if present, as a list of absolute paths. Only the path strings are copied
// out — nothing about the files themselves is read, matching SailBoard's reference-only design
// for file/folder history.
func readClipboardFiles() (paths []string, ok bool) {
	if avail, _, _ := procIsClipboardFormatAvl.Call(cfHDrop); avail == 0 {
		return nil, false
	}
	if opened, _, _ := procOpenClipboard.Call(0); opened == 0 {
		return nil, false
	}

	hDrop, _, _ := procGetClipboardData.Call(cfHDrop)
	if hDrop == 0 {
		procCloseClipboard.Call()
		return nil, false
	}
	count, _, _ := procDragQueryFile.Call(hDrop, 0xFFFFFFFF, 0, 0)
	if count == 0 {
		procCloseClipboard.Call()
		return nil, false
	}

	result := make([]string, 0, count)
	buf := make([]uint16, 1024)
	for i := uintptr(0); i < count; i++ {
		n, _, _ := procDragQueryFile.Call(hDrop, i, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		if n == 0 {
			continue
		}
		result = append(result, syscall.UTF16ToString(buf[:n]))
	}
	// Everything needed has already been copied into Go strings above; release the clipboard
	// before returning rather than holding it through the caller's own processing.
	procCloseClipboard.Call()

	if len(result) == 0 {
		return nil, false
	}
	return result, true
}

// writeClipboardFiles writes paths back onto the system clipboard as a native CF_HDROP file drop
// plus a "Preferred DropEffect" of DROPEFFECT_COPY, so pasting a history item into Explorer (or
// any other CF_HDROP-aware app) performs a real copy from the original location — never a move,
// which would be surprising and destructive for content SailBoard never actually took ownership
// of.
//
// Both GlobalAlloc'd buffers are fully prepared, and the "Preferred DropEffect" format ID is
// resolved, *before* OpenClipboard — none of that needs the clipboard open, so the actual
// critical section is just Open -> Empty -> the two SetClipboardData calls -> Close. Every other
// process's own clipboard access (Explorer writing a fresh CF_HDROP for a file copy included) is
// blocked system-wide for as long as we hold it, so it stays open only for genuinely
// clipboard-only work, same principle as readClipboardImage's fix elsewhere in this package.
func writeClipboardFiles(paths []string) error {
	if len(paths) == 0 {
		return fmt.Errorf("no files to write")
	}

	hDrop, err := allocDropFilesBlock(paths)
	if err != nil {
		return err
	}
	// dropEffectMem/dropEffectFormat are best-effort: a zero format ID (registration failed) or a
	// zero handle (alloc failed) just means the preferred-drop-effect SetClipboardData is skipped
	// below — CF_HDROP alone still pastes correctly without it.
	dropEffectFormat, dropEffectMem := prepareDropEffectCopyBlock()

	if opened, _, _ := procOpenClipboard.Call(0); opened == 0 {
		procGlobalFree.Call(hDrop)
		if dropEffectMem != 0 {
			procGlobalFree.Call(dropEffectMem)
		}
		return fmt.Errorf("OpenClipboard failed")
	}
	procEmptyClipboard.Call()
	if ret, _, _ := procSetClipboardData.Call(cfHDrop, hDrop); ret == 0 {
		procCloseClipboard.Call()
		procGlobalFree.Call(hDrop)
		if dropEffectMem != 0 {
			procGlobalFree.Call(dropEffectMem)
		}
		return fmt.Errorf("SetClipboardData failed")
	}
	// Ownership of hDrop now belongs to the OS; must not GlobalFree it after a successful
	// SetClipboardData.
	if dropEffectFormat != 0 && dropEffectMem != 0 {
		if ret, _, _ := procSetClipboardData.Call(dropEffectFormat, dropEffectMem); ret == 0 {
			procGlobalFree.Call(dropEffectMem)
		}
		// Ownership passes to the OS on success here too; nothing left to free either way.
	}
	procCloseClipboard.Call()
	return nil
}

// allocDropFilesBlock builds the DROPFILES header + double-NUL-terminated UTF-16 path list into
// one GlobalAlloc'd block, ready to hand straight to SetClipboardData(CF_HDROP, ...).
func allocDropFilesBlock(paths []string) (uintptr, error) {
	var wideList []uint16
	for _, p := range paths {
		u, err := syscall.UTF16FromString(p)
		if err != nil {
			return 0, err
		}
		wideList = append(wideList, u...) // each entry already carries its own trailing NUL
	}
	wideList = append(wideList, 0) // extra NUL terminates the whole list, per DROPFILES

	var hdr dropFiles
	hdr.PFiles = uint32(unsafe.Sizeof(hdr))
	hdr.FWide = 1
	headerBytes := unsafe.Slice((*byte)(unsafe.Pointer(&hdr)), unsafe.Sizeof(hdr))
	listBytes := unsafe.Slice((*byte)(unsafe.Pointer(&wideList[0])), len(wideList)*2)
	total := len(headerBytes) + len(listBytes)

	hMem, _, _ := procGlobalAlloc.Call(gmemMoveable, uintptr(total))
	if hMem == 0 {
		return 0, fmt.Errorf("GlobalAlloc failed")
	}
	ptr, _, _ := procGlobalLock.Call(hMem)
	if ptr == 0 {
		procGlobalFree.Call(hMem)
		return 0, fmt.Errorf("GlobalLock failed")
	}
	dst := unsafe.Slice((*byte)(unsafe.Pointer(ptr)), total)
	copy(dst, headerBytes)
	copy(dst[len(headerBytes):], listBytes)
	procGlobalUnlock.Call(hMem)
	return hMem, nil
}

// prepareDropEffectCopyBlock resolves the "Preferred DropEffect" clipboard format (the same
// marker Explorer itself writes on a real Ctrl+C, so a paste reads as a copy rather than
// defaulting to a move in apps that check it) and allocates its DROPEFFECT_COPY payload, without
// touching the clipboard. Returns a zero format and/or zero handle on any failure — purely
// best-effort, the caller skips setting it rather than failing the whole write.
func prepareDropEffectCopyBlock() (format uintptr, hMem uintptr) {
	formatNamePtr, err := syscall.UTF16PtrFromString("Preferred DropEffect")
	if err != nil {
		return 0, 0
	}
	format, _, _ = procRegisterClipboardFormat.Call(uintptr(unsafe.Pointer(formatNamePtr)))
	if format == 0 {
		return 0, 0
	}
	hMem, _, _ = procGlobalAlloc.Call(gmemMoveable, 4)
	if hMem == 0 {
		return format, 0
	}
	ptr, _, _ := procGlobalLock.Call(hMem)
	if ptr == 0 {
		procGlobalFree.Call(hMem)
		return format, 0
	}
	*(*uint32)(unsafe.Pointer(ptr)) = dropEffectCopy
	procGlobalUnlock.Call(hMem)
	return format, hMem
}
