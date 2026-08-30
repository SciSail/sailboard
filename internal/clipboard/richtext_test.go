package clipboard

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeAssetStore struct {
	refs []AssetRef
	data map[string][]byte
}

func (s *fakeAssetStore) SaveAsset(data []byte, mime string) (AssetRef, error) {
	h := sha256.Sum256(data)
	hash := hex.EncodeToString(h[:])
	if s.data == nil {
		s.data = map[string][]byte{}
	}
	s.data[hash] = append([]byte(nil), data...)
	ref := AssetRef{Hash: hash, Path: "assets/" + hash + ".png", MIME: mime, ByteSize: int64(len(data))}
	s.refs = append(s.refs, ref)
	return ref, nil
}

func TestMaterializeAndHydrateLocalCFHTML(t *testing.T) {
	store := &fakeAssetStore{}
	data := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 1, 2, 3}
	path := filepath.Join(t.TempDir(), "a.png")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	fileURL := "file:///" + strings.ReplaceAll(strings.TrimPrefix(filepath.ToSlash(path), "/"), " ", "%20")
	input := BuildCFHTML(`<p>hi<img src="` + fileURL + `"></p>`)
	got, refs, err := materializeLocalImages(input, store, os.ReadFile)
	if err != nil || len(refs) != 1 {
		t.Fatalf("materialize refs=%d err=%v", len(refs), err)
	}
	if !strings.Contains(got, "sailboard-asset://") {
		t.Fatalf("payload did not contain asset placeholder: %s", got)
	}
	hydrated := HydrateLocalImages(got, func(hash string) ([]byte, string, error) { return store.data[hash], "image/png", nil })
	if !strings.Contains(hydrated, "data:image/png;base64,") {
		t.Fatalf("payload was not hydrated: %s", hydrated)
	}
	if _, ok := extractCFHTMLFragment(hydrated); !ok {
		t.Fatal("hydrated payload lost CF_HTML header")
	}
}

func TestMaterializeSkipsRemoteAndUNC(t *testing.T) {
	store := &fakeAssetStore{}
	input := `<p><img src="https://example.com/a.png"><img src="file://server/share/a.png"></p>`
	got, refs, err := materializeLocalImages(input, store, func(string) ([]byte, error) { return nil, errors.New("must not read") })
	if err != nil || len(refs) != 0 || got != input {
		t.Fatalf("got=%q refs=%d err=%v", got, len(refs), err)
	}
}

func TestBuildCFHTMLOffsetsUseBytes(t *testing.T) {
	payload := BuildCFHTML("<p>中文</p>")
	frag, ok := extractCFHTMLFragment(payload)
	if !ok || frag != "<p>中文</p>" {
		t.Fatalf("fragment=%q ok=%v", frag, ok)
	}
}

func TestCaptureRichPayloadUpdatesOnDedup(t *testing.T) {
	repo := newFakeRepository()
	svc := NewService(repo, nil)
	first, _, err := svc.Capture(context.Background(), RawContent{Text: "same", HTML: "<b>old</b>"}, AppInfo{})
	if err != nil || first.HTML != "<b>old</b>" {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	second, changed, err := svc.Capture(context.Background(), RawContent{Text: "same", HTML: "<i>new</i>"}, AppInfo{})
	if err != nil || changed || second.HTML != "<i>new</i>" {
		t.Fatalf("second=%+v changed=%v err=%v", second, changed, err)
	}
}
