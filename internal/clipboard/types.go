package clipboard

import "time"

type ContentType string

const (
	ContentText  ContentType = "text"
	ContentURL   ContentType = "url"
	ContentImage ContentType = "image"
	ContentFile  ContentType = "file"
	ContentColor ContentType = "color"
)

type AppInfo struct {
	Name       string
	Identifier string
	IconPath   string
}

type Item struct {
	ID          string
	Type        ContentType
	Text        string
	FilePath    string
	Hash        string
	SourceApp   AppInfo
	CharCount   int
	ImageWidth  int
	ImageHeight int
	Favorite    bool
	CreatedAt   time.Time
	LastUsedAt  time.Time
	ByteSize    int64
	// HTML/RTF are an optional rich-text payload alongside Text — set only when the source copy
	// (Office, browsers, most rich text editors) carried real formatting, per Service.Capture's
	// doc comment. Empty for a plain text/URL/color item, same as ever. Never set on a file/image
	// item; formatting doesn't apply to those.
	HTML string
	RTF  string
}

type RawContent struct {
	Text        string
	ImageBytes  []byte
	ImageWidth  int
	ImageHeight int
	FilePaths   []string
	// HTML/RTF carry a formatted-text copy's markup verbatim (see
	// platform.Controller.ReadClipboardRichText's doc comment for the byte-for-byte round-trip
	// rationale). Only ever set together with Text, never with ImageBytes/FilePaths — the watcher
	// only reports one of files/rich-text/image/plain-text per tick.
	HTML string
	RTF  string
}
