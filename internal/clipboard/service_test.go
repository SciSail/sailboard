package clipboard

import (
	"context"
	"testing"
	"time"
)

// fakeRepository is a minimal in-memory stand-in for storage.Repository, keyed by content hash
// the same way the real SQLite repository dedups via its UNIQUE(content_hash) constraint.
type fakeRepository struct {
	byHash map[string]Item
	byID   map[string]Item
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{byHash: map[string]Item{}, byID: map[string]Item{}}
}

func (r *fakeRepository) Upsert(_ context.Context, item Item) (Item, bool, error) {
	if existing, ok := r.byHash[item.Hash]; ok {
		existing.LastUsedAt = item.LastUsedAt
		r.byHash[item.Hash] = existing
		r.byID[existing.ID] = existing
		return existing, false, nil
	}
	r.byHash[item.Hash] = item
	r.byID[item.ID] = item
	return item, true, nil
}

func (r *fakeRepository) GetByID(_ context.Context, id string) (*Item, error) {
	item, ok := r.byID[id]
	if !ok {
		return nil, errNotFound
	}
	return &item, nil
}

func (r *fakeRepository) Touch(_ context.Context, id string, now time.Time) error {
	item, ok := r.byID[id]
	if !ok {
		return errNotFound
	}
	item.LastUsedAt = now
	r.byID[id] = item
	r.byHash[item.Hash] = item
	return nil
}

type fakeError string

func (e fakeError) Error() string { return string(e) }

const errNotFound = fakeError("not found")

type fakeImageStore struct {
	saved map[string][]byte
}

func (s *fakeImageStore) Save(hash string, data []byte) (string, error) {
	if s.saved == nil {
		s.saved = map[string][]byte{}
	}
	s.saved[hash] = data
	return "/images/" + hash + ".png", nil
}

func TestCaptureSavesImageAndTagsSourceApp(t *testing.T) {
	repo := newFakeRepository()
	images := &fakeImageStore{}
	svc := NewService(repo, images)

	source := AppInfo{Name: "Snipping Tool", Identifier: `C:\snip.exe`, IconPath: "/icons/snip.png"}
	item, changed, err := svc.Capture(context.Background(), RawContent{ImageBytes: []byte{1, 2, 3}, ImageWidth: 10, ImageHeight: 20}, source)
	if err != nil {
		t.Fatalf("Capture() error = %v", err)
	}
	if !changed {
		t.Fatal("Capture() changed = false, want true for a new image")
	}
	if item.Type != ContentImage {
		t.Fatalf("item.Type = %v, want %v", item.Type, ContentImage)
	}
	if item.SourceApp != source {
		t.Fatalf("item.SourceApp = %+v, want %+v", item.SourceApp, source)
	}
	wantPath := "/images/" + item.Hash + ".png"
	if item.FilePath != wantPath {
		t.Fatalf("item.FilePath = %q, want %q", item.FilePath, wantPath)
	}
	if string(images.saved[item.Hash]) != "\x01\x02\x03" {
		t.Fatalf("image store did not receive the raw bytes: %v", images.saved[item.Hash])
	}
}

func TestCaptureStoresFilePathsWithoutTouchingImageStore(t *testing.T) {
	repo := newFakeRepository()
	images := &fakeImageStore{}
	svc := NewService(repo, images)

	paths := []string{`C:\Users\me\report.docx`, `C:\Users\me\photos`}
	item, changed, err := svc.Capture(context.Background(), RawContent{FilePaths: paths}, AppInfo{Name: "explorer.exe"})
	if err != nil {
		t.Fatalf("Capture() error = %v", err)
	}
	if !changed {
		t.Fatal("Capture() changed = false, want true for a new file capture")
	}
	if item.Type != ContentFile {
		t.Fatalf("item.Type = %v, want %v", item.Type, ContentFile)
	}
	wantFilePath := `C:\Users\me\report.docx` + "\n" + `C:\Users\me\photos`
	if item.FilePath != wantFilePath {
		t.Fatalf("item.FilePath = %q, want %q", item.FilePath, wantFilePath)
	}
	if len(images.saved) != 0 {
		t.Fatalf("ImageStore.Save was called for a file capture: %v", images.saved)
	}
}

func TestCaptureIgnoresItsOwnPasteWrite(t *testing.T) {
	repo := newFakeRepository()
	svc := NewService(repo, nil)

	first, _, err := svc.Capture(context.Background(), RawContent{Text: "hello"}, AppInfo{})
	if err != nil {
		t.Fatalf("Capture() error = %v", err)
	}
	if _, err := svc.PreparePaste(context.Background(), first.ID); err != nil {
		t.Fatalf("PreparePaste() error = %v", err)
	}

	// The watcher would observe this as a fresh clipboard change right after SailBoard wrote it
	// back for pasting; Capture must recognise and drop it rather than re-inserting it.
	_, changed, err := svc.Capture(context.Background(), RawContent{Text: "hello"}, AppInfo{Name: "SomeOtherApp"})
	if err != nil {
		t.Fatalf("Capture() error = %v", err)
	}
	if changed {
		t.Fatal("Capture() changed = true, want false for the self-write echo")
	}
}
