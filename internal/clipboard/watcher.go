package clipboard

import (
	"context"
	"fmt"
	"time"
)

// Watcher observes clipboard changes and reports them as RawContent. On Windows
// Changes is fed by WM_CLIPBOARDUPDATE; platforms without a native notification
// use the interval ticker. Reads are always injected so this package contains no
// OS-specific clipboard code.
type Watcher struct {
	// ReadSnapshot is the preferred reader. It must return only after all native
	// clipboard locks have been released. A read error is retried with backoff.
	ReadSnapshot func() (RawContent, error)
	// Changes is a non-blocking notification channel. Notifications contain no
	// data; the reader must fetch the latest snapshot.
	Changes <-chan struct{}
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
	// SettleDelay is the quiet period after a native change notification before
	// the clipboard is opened. Windows file copies publish Shell/OLE formats
	// asynchronously, so reading immediately from WM_CLIPBOARDUPDATE can compete
	// with Explorer even though the notification itself is delivered after Ctrl+C.
	SettleDelay time.Duration
	// RetryDelay is the initial wait after a transient read failure. Keeping this
	// separate from Interval prevents an OpenClipboard failure from turning into
	// an aggressive retry loop while the source application is still publishing.
	RetryDelay time.Duration
	// MaxRetryDelay caps the exponential retry backoff.
	MaxRetryDelay time.Duration
}

func (w Watcher) Start(ctx context.Context, onChange func(RawContent)) {
	interval := w.Interval
	if interval == 0 {
		interval = 250 * time.Millisecond
	}
	// A Windows listener is authoritative, but keep a slow safety poll in case a
	// listener registration is lost. On polling-only platforms this is the normal
	// cadence.
	pollInterval := interval
	if w.Changes != nil && pollInterval < 2*time.Second {
		pollInterval = 2 * time.Second
	}
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	lastText := ""
	var lastSeq uint32
	haveSeq := false
	var timer *time.Timer
	var timerC <-chan time.Time
	pending := true // capture the current clipboard once at startup
	settleDelay := w.SettleDelay
	if settleDelay <= 0 {
		settleDelay = 40 * time.Millisecond
	}
	initialRetryDelay := w.RetryDelay
	if initialRetryDelay <= 0 {
		initialRetryDelay = 25 * time.Millisecond
	}
	retryDelay := initialRetryDelay
	maxRetryDelay := w.MaxRetryDelay
	if maxRetryDelay < initialRetryDelay {
		maxRetryDelay = time.Second
		if maxRetryDelay < initialRetryDelay {
			maxRetryDelay = initialRetryDelay
		}
	}

	read := w.ReadSnapshot
	if read == nil {
		read = func() (RawContent, error) {
			if w.ReadFiles != nil {
				if paths, ok := w.ReadFiles(); ok {
					return RawContent{FilePaths: paths}, nil
				}
			}
			if w.ReadRichText != nil {
				if html, rtf, text, ok := w.ReadRichText(); ok {
					return RawContent{HTML: html, RTF: rtf, Text: text}, nil
				}
			}
			if w.ReadImage != nil {
				if data, width, height, ok := w.ReadImage(); ok {
					return RawContent{ImageBytes: data, ImageWidth: width, ImageHeight: height}, nil
				}
			}
			if w.ReadText == nil {
				return RawContent{}, fmt.Errorf("clipboard reader is not configured")
			}
			text, err := w.ReadText()
			if err != nil {
				return RawContent{}, err
			}
			if text == "" || text == lastText {
				return RawContent{}, nil
			}
			lastText = text
			return RawContent{Text: text}, nil
		}
	}
	changes := w.Changes
	schedule := func(delay time.Duration) {
		if timer == nil {
			timer = time.NewTimer(delay)
		} else {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(delay)
		}
		timerC = timer.C
	}
	if changes != nil {
		schedule(settleDelay)
	} else {
		schedule(0)
	}
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-changes:
			if !ok {
				// A host may tear down its notification source before the
				// watcher context is cancelled. Disable that select arm and
				// keep the safety poll alive instead of spinning on a closed
				// channel forever.
				changes = nil
				continue
			}
			pending = true
			// Every new notification restarts the quiet period. Explorer can
			// publish several delayed Shell/OLE formats for one file copy; opening
			// the clipboard between those updates is the contention we avoid.
			schedule(settleDelay)
		case <-ticker.C:
			pending = true
			if changes != nil {
				// The safety poll must obey the same quiet period as event-driven
				// reads, otherwise it can reintroduce the race every two seconds.
				schedule(settleDelay)
			} else {
				schedule(0)
			}
		case <-timerC:
			timerC = nil
			if !pending {
				continue
			}
			if w.IsPaused != nil && w.IsPaused() {
				schedule(interval)
				continue
			}
			var seq uint32
			var seqOK bool
			if w.Sequence != nil {
				seq, seqOK = w.Sequence()
			}
			if seqOK && haveSeq && seq == lastSeq {
				pending = false
				retryDelay = initialRetryDelay
				continue
			}
			raw, err := read()
			if err != nil {
				// Do not consume the sequence on failure. OpenClipboard
				// contention is transient and the next attempt must retry it.
				schedule(retryDelay)
				if retryDelay < maxRetryDelay {
					retryDelay *= 2
					if retryDelay > maxRetryDelay {
						retryDelay = maxRetryDelay
					}
				}
				continue
			}
			if seqOK {
				haveSeq, lastSeq = true, seq
			}
			pending = false
			retryDelay = initialRetryDelay
			if hasContent(raw) {
				onChange(raw)
			}
		}
	}
}

func hasContent(raw RawContent) bool {
	return raw.Text != "" || raw.HTML != "" || raw.RTF != "" || len(raw.FilePaths) > 0 || len(raw.ImageBytes) > 0
}
