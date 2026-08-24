package clipboard

import (
	"context"
	"time"
)

// Watcher polls the system clipboard for changes and reports them as RawContent. It keeps every
// OS-specific clipboard API out of this package: all reads are injected as plain functions, so
// the same Watcher drives both the native Windows controller and any future platform.
type Watcher struct {
	// ReadText returns the current clipboard text (Wails' runtime.ClipboardGetText works here
	// on every platform Wails supports).
	ReadText func() (string, error)
	// ReadImage returns the current clipboard image re-encoded as PNG, if present. Optional —
	// leave nil on platforms without a native image reader yet (design doc §0 rule 3: stub
	// behind the interface rather than blocking the build).
	ReadImage func() (data []byte, width, height int, ok bool)
	// ReadFiles returns the file/folder paths currently on the clipboard (CF_HDROP), if present.
	// Checked before ReadRichText/ReadImage/ReadText each tick, since a file copy is the most
	// specific/intentional. Optional, same stub convention as ReadImage.
	ReadFiles func() (paths []string, ok bool)
	// ReadRichText returns "HTML Format"/"Rich Text Format" markup alongside plain text, if the
	// clipboard holds a genuine formatted-text copy (see platform.Controller.ReadClipboardRichText
	// for what "genuine" means). Checked before ReadImage: Office always attaches a preview bitmap
	// to a text copy, and that copy should be captured as (rich) text, not as that bitmap.
	// Optional, same stub convention as ReadImage.
	ReadRichText func() (html, rtf, text string, ok bool)
	// Sequence returns a counter that changes whenever the clipboard content changes, letting
	// the watcher skip a read entirely most ticks (design doc §8.2). Optional; when the second
	// return is false the watcher falls back to comparing clipboard text between ticks.
	Sequence func() (uint32, bool)
	// IsPaused, if set, lets the watcher honour SailBoard's "pause recording" toggle without
	// tearing down and rebuilding the polling goroutine.
	IsPaused func() bool
	Interval time.Duration
}

func (w Watcher) Start(ctx context.Context, onChange func(RawContent)) {
	interval := w.Interval
	if interval == 0 {
		interval = 250 * time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	lastText := ""
	var lastSeq uint32
	haveSeq := false

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if w.IsPaused != nil && w.IsPaused() {
				continue
			}
			if w.Sequence != nil {
				if seq, ok := w.Sequence(); ok {
					if haveSeq && seq == lastSeq {
						continue
					}
					haveSeq, lastSeq = true, seq
				}
			}
			if w.ReadFiles != nil {
				if paths, ok := w.ReadFiles(); ok {
					onChange(RawContent{FilePaths: paths})
					continue
				}
			}
			if w.ReadRichText != nil {
				if html, rtf, text, ok := w.ReadRichText(); ok {
					onChange(RawContent{HTML: html, RTF: rtf, Text: text})
					continue
				}
			}
			if w.ReadImage != nil {
				if data, width, height, ok := w.ReadImage(); ok {
					onChange(RawContent{ImageBytes: data, ImageWidth: width, ImageHeight: height})
					continue
				}
			}
			text, err := w.ReadText()
			if err != nil || text == "" || text == lastText {
				continue
			}
			lastText = text
			onChange(RawContent{Text: text})
		}
	}
}
