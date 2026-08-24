package clipboard

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

type Repository interface {
	Upsert(context.Context, Item) (Item, bool, error)
	GetByID(context.Context, string) (*Item, error)
	Touch(context.Context, string, time.Time) error
}

// ImageStore persists an image's bytes to disk (design doc §11: images are kept as files, not
// DB blobs) and returns the path to save on the Item.
type ImageStore interface {
	Save(hash string, data []byte) (path string, err error)
}

// Service owns capture deduplication and prevents paste writes from re-entering history.
type Service struct {
	repo           Repository
	images         ImageStore
	mu             sync.Mutex
	ignoreNextHash string
}

func NewService(repo Repository, images ImageStore) *Service {
	return &Service{repo: repo, images: images}
}

// Capture classifies and dedups raw clipboard content, tagging it with whatever app currently
// owns focus (best-effort source-app attribution per design doc §12).
func (s *Service) Capture(ctx context.Context, raw RawContent, source AppInfo) (Item, bool, error) {
	typ, text, hash, size, ok := Parse(raw)
	if !ok {
		return Item{}, false, nil
	}
	s.mu.Lock()
	if hash == s.ignoreNextHash {
		s.ignoreNextHash = ""
		s.mu.Unlock()
		return Item{}, false, nil
	}
	s.mu.Unlock()

	var filePath string
	switch {
	case typ == ContentImage && s.images != nil:
		path, err := s.images.Save(hash, raw.ImageBytes)
		if err != nil {
			return Item{}, false, err
		}
		filePath = path
	case typ == ContentFile:
		// Reference-only: the original paths are remembered as-is, never copied onto disk.
		filePath = strings.Join(raw.FilePaths, "\n")
	}

	now := time.Now()
	item := Item{ID: newID(), Type: typ, Text: text, FilePath: filePath, Hash: hash, SourceApp: source, CharCount: CharCount(text), ImageWidth: raw.ImageWidth, ImageHeight: raw.ImageHeight, CreatedAt: now, LastUsedAt: now, ByteSize: size}
	// The rich payload only makes sense riding along with actual text (never file/image content —
	// Parse never sets typ to those when raw.HTML/RTF are populated in the first place, since the
	// watcher only ever reports rich text and images as separate, mutually exclusive ticks, but
	// guard explicitly here rather than relying on that invariant holding forever).
	if (raw.HTML != "" || raw.RTF != "") && typ != ContentFile && typ != ContentImage {
		item.HTML = raw.HTML
		item.RTF = raw.RTF
	}
	return s.repo.Upsert(ctx, item)
}

func (s *Service) PreparePaste(ctx context.Context, id string) (*Item, error) {
	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.ignoreNextHash = item.Hash
	s.mu.Unlock()
	if err := s.repo.Touch(ctx, id, time.Now()); err != nil {
		return nil, err
	}
	return item, nil
}

func newID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return fmtID(time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}
func fmtID(value int64) string {
	return hex.EncodeToString([]byte(time.Unix(0, value).Format("20060102150405.000000000")))
}
