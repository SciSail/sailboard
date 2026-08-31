//go:build windows

package platform

import (
	"encoding/binary"
	"reflect"
	"testing"
	"unicode/utf16"
)

func buildClipboardFilesBlock(paths []string) []byte {
	const headerSize = 20
	data := make([]byte, headerSize)
	binary.LittleEndian.PutUint32(data[0:4], headerSize)
	binary.LittleEndian.PutUint32(data[16:20], 1)
	for _, path := range paths {
		for _, unit := range utf16.Encode([]rune(path)) {
			var buf [2]byte
			binary.LittleEndian.PutUint16(buf[:], unit)
			data = append(data, buf[:]...)
		}
		data = append(data, 0, 0)
	}
	return append(data, 0, 0)
}

func TestDecodeClipboardFilesBlock(t *testing.T) {
	want := []string{`C:\work\notes.txt`, `D:\资料\图 片.png`}
	if got := decodeClipboardFilesBlock(buildClipboardFilesBlock(want), 100); !reflect.DeepEqual(got, want) {
		t.Fatalf("paths=%q, want %q", got, want)
	}
}

func TestDecodeClipboardFilesBlockHonorsLimitAndRejectsBadOffset(t *testing.T) {
	data := buildClipboardFilesBlock([]string{`C:\a.txt`, `C:\b.txt`})
	if got := decodeClipboardFilesBlock(data, 1); !reflect.DeepEqual(got, []string{`C:\a.txt`}) {
		t.Fatalf("limited paths=%q", got)
	}
	binary.LittleEndian.PutUint32(data[0:4], uint32(len(data)+1))
	if got := decodeClipboardFilesBlock(data, 10); got != nil {
		t.Fatalf("bad offset paths=%q, want nil", got)
	}
}
