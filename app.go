package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"
	"time"

	"SailBoard/internal/clipboard"
	"SailBoard/internal/platform"
	"SailBoard/internal/storage"
	"SailBoard/internal/webpreview"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// panelHeight must match main.go's window Height option: the always-on-top bottom card rail is
// a fixed height, only its width and vertical position adapt to the monitor under the cursor.
// 330, not 360: App.css's .sheet grid dropped its bottom 30px hint-bar row when that bar moved
// into the toolbar, so the window itself shrinks by the same amount rather than leaving the
// card rail to stretch into the freed space.
//
// This is a logical/CSS-pixel size (what App.css's layout is designed against at 100% display
// scaling) — ShowWindow scales it by platform.Controller.WorkAreaNearCursor's reported DPI
// factor before handing it to PositionSelf/SlideReveal, which place the window in physical
// pixels. Passing panelHeight to those calls unscaled is what previously made the panel render
// at half its intended height (content clipped vertically) under 200% Windows display scaling:
// a 330-physical-pixel-tall window only has 165 CSS px of vertical space for WebView2 to lay
// App.css out in at that scale.
const panelHeight = 330

// panelAnimationMs is the reveal/dismiss slide duration for ShowWindow/HideWindowAnimated (see
// platform.Controller.SlideReveal/SlideDismiss).
const panelAnimationMs = 220

// appWindowTitle must match main.go's options.App.Title — platform.Controller.FocusSelf looks
// the window up by title since Wails doesn't expose the native HWND through its public API.
const appWindowTitle = "SailBoard"

// App struct
type App struct {
	ctx        context.Context
	cancel     context.CancelFunc
	repository *storage.Repository
	clipboard  *clipboard.Service
	platform   platform.Controller
	dataDir    string

	mu               sync.Mutex
	unregisterHotkey func()
	foregroundToken  platform.ForegroundToken
	paused           bool
	iconCache        map[string]string

	// animMu serializes ShowWindow's and HideWindowAnimated's native AnimateWindow calls against
	// each other, so a rapid re-summon or dismiss can never run two of them concurrently on the
	// same HWND — see ShowWindow's doc comment.
	animMu  sync.Mutex
	assetMu sync.Mutex
	assets  *diskAssetStore
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{iconCache: map[string]string{}}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	dataDir, err := appDataDir()
	if err != nil {
		runtime.LogErrorf(ctx, "create application data directory: %v", err)
		return
	}
	a.dataDir = dataDir
	repository, err := storage.Open(filepath.Join(dataDir, "sailboard.db"))
	if err != nil {
		runtime.LogErrorf(ctx, "open history database: %v", err)
		return
	}
	a.repository = repository
	images := &diskImageStore{dir: filepath.Join(dataDir, "images")}
	a.clipboard = clipboard.NewService(repository, images)
	a.assets = &diskAssetStore{imagesDir: images.dir, richDir: filepath.Join(dataDir, "assets")}
	a.clipboard.SetAssetStore(a.assets)
	if err := repository.RegisterLegacyImageAssets(ctx); err != nil {
		runtime.LogErrorf(ctx, "register legacy image assets: %v", err)
	}
	a.reconcileAssets(ctx)

	ctrl, err := platform.New(appWindowTitle)
	if err != nil {
		runtime.LogErrorf(ctx, "init native platform layer (hotkey/tray/paste will be unavailable): %v", err)
	}
	a.platform = ctrl

	watchCtx, cancel := context.WithCancel(ctx)
	a.cancel = cancel
	watcher := clipboard.Watcher{
		ReadText: func() (string, error) { return runtime.ClipboardGetText(ctx) },
		IsPaused: a.isPaused,
		Interval: 250 * time.Millisecond,
	}
	if a.platform != nil {
		if a.platform.ClipboardSnapshotSupported() {
			watcher.ReadSnapshot = func() (clipboard.RawContent, error) {
				snap, err := a.platform.ReadClipboardSnapshot()
				if err != nil {
					return clipboard.RawContent{}, err
				}
				// The native macOS snapshot intentionally only covers native
				// representations; use Wails' text reader when no richer payload
				// was present. Windows already returns CF_UNICODETEXT here.
				if goruntime.GOOS == "darwin" && snap.Text == "" && snap.HTML == "" && snap.RTF == "" && len(snap.FilePaths) == 0 && len(snap.ImagePNG) == 0 {
					if text, textErr := runtime.ClipboardGetText(ctx); textErr == nil {
						snap.Text = text
					}
				}
				return clipboard.RawContent{Text: snap.Text, HTML: snap.HTML, RTF: snap.RTF, FilePaths: snap.FilePaths,
					ImageBytes: snap.ImagePNG, ImageWidth: snap.ImageWidth, ImageHeight: snap.ImageHeight}, nil
			}
			if changes, ok := a.platform.ClipboardChanges(); ok {
				watcher.Changes = changes
				if goruntime.GOOS == "windows" {
					// Explorer publishes file-copy data through delayed Shell/OLE
					// formats. WM_CLIPBOARDUPDATE tells us content changed, but it
					// does not guarantee those formats have finished rendering. Give
					// the system a quiet window and retry gently: capture latency is
					// preferable to competing with the user's own copy/paste.
					watcher.SettleDelay = 600 * time.Millisecond
					watcher.RetryDelay = 600 * time.Millisecond
					watcher.MaxRetryDelay = 2 * time.Second
				}
			}
		} else {
			watcher.ReadImage = a.platform.ReadClipboardImage
			watcher.ReadFiles = a.platform.ReadClipboardFiles
			watcher.ReadRichText = a.platform.ReadClipboardRichText
		}
		watcher.Sequence = a.platform.ClipboardSequence
	}
	go watcher.Start(watchCtx, func(raw clipboard.RawContent) {
		source := a.resolveSourceApp()
		// Serialize capture with asset reconciliation. A rich capture first writes an
		// external image, then commits its DB reference; without this short critical
		// section a concurrent settings/cleanup callback could observe the gap and
		// remove the just-written file before the reference is committed. The native
		// clipboard has already been closed by ReadClipboardSnapshot at this point,
		// so this lock cannot affect the user's copy/paste operation.
		a.assetMu.Lock()
		item, _, err := a.clipboard.Capture(watchCtx, raw, source)
		a.assetMu.Unlock()
		if err != nil {
			runtime.LogErrorf(ctx, "capture clipboard: %v", err)
			return
		}
		// item.ID is only empty for Capture's two no-op cases (nothing parseable on the
		// clipboard, or this is SailBoard's own paste-write echoing back). Every other capture —
		// new item or a re-pin of an existing one via Upsert's dedup — must refresh the UI so the
		// card jumps to the top with its updated time, not just brand-new items.
		if item.ID != "" {
			runtime.EventsEmit(ctx, "history:changed", item.ID)
			a.reconcileAssets(ctx)
		}
	})

	// Checked before GetSettings below (rather than where it's used, further down) so a fresh
	// install's platform-appropriate shortcut override lands before applyShortcut registers it,
	// not after.
	hasSettings, hasSettingsErr := repository.HasSettings(ctx)
	isFreshInstall := hasSettingsErr == nil && !hasSettings

	settings, err := repository.GetSettings(ctx)
	if err == nil {
		if isFreshInstall && goruntime.GOOS == "darwin" {
			// DefaultSettings() (internal/storage, deliberately OS-agnostic) always returns the
			// Windows convention "Ctrl+Shift+V" — Mac users expect Cmd+Shift+V instead. Only a
			// fresh install's default gets overridden; an existing saved shortcut (including one
			// a user explicitly set to a literal Ctrl combo) is left alone.
			settings.Shortcut = "Cmd+Shift+V"
		}
		_ = repository.Cleanup(ctx, settings)
		a.reconcileAssets(ctx)
		a.applyShortcut(settings.Shortcut)
		if a.platform != nil {
			if err := a.platform.SetAutoLaunch(settings.LaunchAtLogin); err != nil {
				runtime.LogErrorf(ctx, "sync launch-at-login setting: %v", err)
			}
		}
	}

	// A fresh install has never saved a settings row. Persist those defaults now so this only
	// fires once, then walk the user straight into Settings immediately rather than waiting for a
	// hotkey press they have no way of knowing yet.
	if isFreshInstall {
		_ = repository.SaveSettings(ctx, settings)
		// An otherwise-empty history reads as "is this broken?" rather than "nothing copied
		// yet" — seed one welcoming tip through the normal capture pipeline (so it behaves like
		// any other history item: searchable, deletable, favoritable) instead of leaving the
		// panel blank or touching the user's real OS clipboard to plant it.
		if a.clipboard != nil {
			tip := fmt.Sprintf("%s 随时唤出剪贴板历史", settings.Shortcut)
			_, _, _ = a.clipboard.Capture(ctx, clipboard.RawContent{Text: tip}, clipboard.AppInfo{Name: "SailBoard"})
		}
		if err := a.OpenSettingsWindow(); err != nil {
			runtime.LogErrorf(ctx, "open settings window on first run: %v", err)
		}
	}

	if a.platform != nil {
		a.platform.ShowTray(platform.TrayOptions{
			IconPNG:       trayIconPNG,
			OnShow:        func() { a.ShowWindow() },
			OnTogglePause: func() { _, _ = a.TogglePaused() },
			OnQuit:        func() { runtime.Quit(ctx) },
		})
		// Clicking another app (or its window otherwise taking the foreground) should hide the
		// panel immediately, the same way Spotlight/Alfred-style launchers behave — there should
		// never be a state where SailBoard is still visible but the user is typing into some
		// other app. The frontend runs this through the same animated requestHide() as Esc.
		a.platform.WatchFocusLoss([]string{appWindowTitle, settingsWindowTitle}, func() { runtime.EventsEmit(ctx, "focus:lost") })
		// The settings window is a separate process (see settings_app.go): it saves straight to
		// the same SQLite file and then pokes us here via NotifySettingsChanged/OnSettingsChanged
		// so the hotkey — which can't just wait for a later sync point, the old one already
		// stopped working the moment it changed — and retention/cleanup take effect immediately.
		a.platform.OnSettingsChanged(func() {
			if settings, err := a.repository.GetSettings(a.ctx); err == nil {
				a.applyShortcut(settings.Shortcut)
				_ = a.repository.Cleanup(a.ctx, settings)
				a.reconcileAssets(a.ctx)
			}
			runtime.EventsEmit(a.ctx, "history:changed")
		})
		// A redundant second launch (see main.go's runMainWindow/AcquireSingleInstanceLock) hands
		// off to this process instead of starting its own copy — "open SailBoard again" should
		// behave like "bring the existing one to the front".
		a.platform.OnShowRequested(func() { a.ShowWindow() })
		// See platform.Controller.OnHotkeySuspendRequested's doc comment: the settings window
		// triggers these around its shortcut-capture UI so the *old* hotkey doesn't pop the main
		// panel over it while the user is trying out a new combination.
		a.platform.OnHotkeySuspendRequested(func() { a.suspendHotkey() })
		a.platform.OnHotkeyResumeRequested(func() { a.resumeHotkey() })
	}

	// Show the main panel once on every launch — even with an empty history — purely as a
	// "SailBoard has started" confirmation, since it otherwise starts completely silently in the
	// tray with no visible signal at all. The short delay lets Wails finish its own StartHidden
	// setup first, so this reveal isn't immediately raced with it.
	go func() {
		time.Sleep(300 * time.Millisecond)
		a.ShowWindow()
	}()
}

func (a *App) shutdown(ctx context.Context) {
	if a.cancel != nil {
		a.cancel()
	}
	a.mu.Lock()
	unregister := a.unregisterHotkey
	a.mu.Unlock()
	if unregister != nil {
		unregister()
	}
	if a.platform != nil {
		a.platform.Close()
	}
	if a.repository != nil {
		_ = a.repository.Close()
	}
}

// applyShortcut (re)binds the global hotkey to spec, replacing whatever was previously
// registered. Errors (e.g. an invalid or already-taken combo per design doc §54) are logged and
// left for the settings UI's notice banner rather than crashing capture/show.
func (a *App) applyShortcut(spec string) {
	if a.platform == nil {
		return
	}
	a.mu.Lock()
	old := a.unregisterHotkey
	a.mu.Unlock()
	if old != nil {
		old()
	}
	unregister, err := a.platform.RegisterHotkey(spec, func() { a.ShowWindow() })
	if err != nil {
		runtime.LogErrorf(a.ctx, "register hotkey %q: %v", spec, err)
		a.mu.Lock()
		a.unregisterHotkey = nil
		a.mu.Unlock()
		return
	}
	a.mu.Lock()
	a.unregisterHotkey = unregister
	a.mu.Unlock()
}

// suspendHotkey unregisters the current global hotkey without registering a replacement, leaving
// a.unregisterHotkey nil — see platform.Controller.OnHotkeySuspendRequested's doc comment. Safe
// to call with nothing registered (e.g. a second suspend request, or applyShortcut itself already
// failed earlier): old is nil and there's nothing to do.
func (a *App) suspendHotkey() {
	a.mu.Lock()
	old := a.unregisterHotkey
	a.unregisterHotkey = nil
	a.mu.Unlock()
	if old != nil {
		old()
	}
}

// resumeHotkey re-reads the saved shortcut and re-registers it via applyShortcut — the settings
// window's capture UI never actually saves anything on its own (that's still a separate, explicit
// "保存" click), so "resume" always means "go back to whatever was last saved", the same value
// suspendHotkey's unregister just tore down.
func (a *App) resumeHotkey() {
	if a.repository == nil {
		return
	}
	if settings, err := a.repository.GetSettings(a.ctx); err == nil {
		a.applyShortcut(settings.Shortcut)
	}
}

// resolveSourceApp asks the platform layer who currently owns focus and, best-effort, caches
// their icon to disk once per executable so repeat captures from the same app are free.
func (a *App) resolveSourceApp() clipboard.AppInfo {
	if a.platform == nil {
		return clipboard.AppInfo{}
	}
	info, err := a.platform.ActiveApp()
	if err != nil || info.Name == "" {
		return clipboard.AppInfo{}
	}
	return clipboard.AppInfo{Name: info.Name, Identifier: info.ExecutablePath, IconPath: a.cacheIcon(info)}
}

func (a *App) cacheIcon(info platform.AppInfo) string {
	if len(info.IconPNG) == 0 || info.ExecutablePath == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(info.ExecutablePath))
	path := filepath.Join(a.dataDir, "icons", hex.EncodeToString(sum[:])+".png")
	if _, err := os.Stat(path); err == nil {
		return path
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return ""
	}
	if err := os.WriteFile(path, info.IconPNG, 0644); err != nil {
		return ""
	}
	return path
}

func (a *App) isPaused() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.paused
}

// TogglePaused flips clipboard recording on/off (design doc §31.1) without tearing down the
// watcher goroutine; it just makes IsPaused() true.
func (a *App) TogglePaused() (bool, error) {
	if err := a.ready(); err != nil {
		return false, err
	}
	a.mu.Lock()
	a.paused = !a.paused
	paused := a.paused
	a.mu.Unlock()
	if a.platform != nil {
		a.platform.UpdateTrayPaused(paused)
	}
	runtime.EventsEmit(a.ctx, "paused:changed", paused)
	return paused, nil
}

func (a *App) GetPaused() bool { return a.isPaused() }

type ClipboardItemDTO struct {
	ID         string   `json:"id"`
	Type       string   `json:"type"`
	Text       string   `json:"text,omitempty"`
	FilePath   string   `json:"filePath,omitempty"`
	SourceName string   `json:"sourceName,omitempty"`
	SourceIcon string   `json:"sourceIcon,omitempty"`
	CharCount  int      `json:"charCount"`
	Width      int      `json:"width"`
	Height     int      `json:"height"`
	Favorite   bool     `json:"favorite"`
	CreatedAt  int64    `json:"createdAt"`
	LastUsedAt int64    `json:"lastUsedAt"`
	FileNames  []string `json:"fileNames,omitempty"`
}

func (a *App) toDTO(item clipboard.Item) ClipboardItemDTO {
	var fileNames []string
	if item.Type == clipboard.ContentFile && item.FilePath != "" {
		for _, p := range strings.Split(item.FilePath, "\n") {
			fileNames = append(fileNames, filepath.Base(p))
		}
	}
	return ClipboardItemDTO{
		ID: item.ID, Type: string(item.Type), Text: item.Text, FilePath: item.FilePath,
		SourceName: item.SourceApp.Name, SourceIcon: a.iconDataURL(item.SourceApp.IconPath),
		CharCount: item.CharCount, Width: item.ImageWidth, Height: item.ImageHeight,
		Favorite: item.Favorite, CreatedAt: item.CreatedAt.UnixMilli(), LastUsedAt: item.LastUsedAt.UnixMilli(),
		FileNames: fileNames,
	}
}
func (a *App) toDTOs(items []clipboard.Item) []ClipboardItemDTO {
	result := make([]ClipboardItemDTO, len(items))
	for i, item := range items {
		result[i] = a.toDTO(item)
	}
	return result
}

// iconDataURL reads a cached source-app icon PNG and inlines it as a data: URI, so the frontend
// never needs filesystem access. Results are cached in memory since the same handful of source
// apps repeat across most history entries.
func (a *App) iconDataURL(path string) string {
	if path == "" {
		return ""
	}
	a.mu.Lock()
	if cached, ok := a.iconCache[path]; ok {
		a.mu.Unlock()
		return cached
	}
	a.mu.Unlock()

	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	url := "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
	a.mu.Lock()
	a.iconCache[path] = url
	a.mu.Unlock()
	return url
}

func (a *App) ready() error {
	if a.repository == nil {
		return errors.New("SailBoard is still starting")
	}
	return nil
}

func (a *App) GetHistory(limit, offset int) ([]ClipboardItemDTO, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	a.purgeMissingFiles()
	// The frontend still supplies limit for API compatibility, but the user-facing setting is
	// authoritative. A value of zero means no SQL LIMIT (see Repository.List).
	if settings, err := a.repository.GetSettings(a.ctx); err == nil {
		limit = settings.HistoryDisplayLimit
	}
	items, err := a.repository.List(a.ctx, limit, offset, false)
	return a.toDTOs(items), err
}
func (a *App) SearchHistory(query string, favoriteOnly bool) ([]ClipboardItemDTO, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	a.purgeMissingFiles()
	items, err := a.repository.Search(a.ctx, query, favoriteOnly)
	return a.toDTOs(items), err
}

// purgeMissingFiles removes file clipboard entries whose original paths no longer exist. File
// clipboard entries store one or more newline-separated paths; the entry is no longer usable as
// soon as any one of those paths disappears. Stat errors other than a definite not-exist result
// are left alone, since a temporary permission/network failure must not silently delete history.
func (a *App) purgeMissingFiles() {
	items, err := a.repository.List(a.ctx, 0, 0, false)
	if err != nil {
		return
	}
	removed := false
	for _, item := range items {
		if item.Type != clipboard.ContentFile || !filePathsMissing(item.FilePath) {
			continue
		}
		if err := a.repository.Delete(a.ctx, item.ID); err == nil {
			removed = true
		}
	}
	if removed {
		a.reconcileAssets(a.ctx)
	}
}

func filePathsMissing(filePath string) bool {
	paths := strings.Split(filePath, "\n")
	if len(paths) == 0 {
		return true
	}
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			return true
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return true
		}
	}
	return false
}
func (a *App) ToggleFavorite(id string) error {
	if err := a.ready(); err != nil {
		return err
	}
	item, err := a.repository.GetByID(a.ctx, id)
	if err != nil {
		return err
	}
	err = a.repository.SetFavorite(a.ctx, id, !item.Favorite)
	if err == nil {
		runtime.EventsEmit(a.ctx, "history:changed", id)
	}
	return err
}
func (a *App) DeleteItem(id string) error {
	if err := a.ready(); err != nil {
		return err
	}
	err := a.repository.Delete(a.ctx, id)
	if err == nil {
		a.reconcileAssets(a.ctx)
		runtime.EventsEmit(a.ctx, "history:changed", id)
	}
	return err
}

// Settings are owned entirely by the standalone settings window process now (see
// settings_app.go's SettingsApp) — it reads/writes the same SQLite file directly and pokes this
// process via platform.NotifySettingsChanged/OnSettingsChanged (wired in startup) to reload and
// reapply them. The main window has no settings UI of its own to bind these to any more.

// GetImageDataURL lazily loads an image item's PNG bytes from disk and inlines them as a data:
// URI, fetched on demand per visible card rather than eagerly for the whole history list.
func (a *App) GetImageDataURL(id string) (string, error) {
	if err := a.ready(); err != nil {
		return "", err
	}
	item, err := a.repository.GetByID(a.ctx, id)
	if err != nil {
		return "", err
	}
	if item.Type != clipboard.ContentImage || item.FilePath == "" {
		return "", errors.New("item has no image")
	}
	data, err := os.ReadFile(item.FilePath)
	if err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(data), nil
}

// GetFileThumbnail lazily produces a thumbnail (real preview for a decodable image, otherwise a
// generic per-type/per-folder icon — see platform.Controller.FileThumbnail) for a file/folder
// history item's first path, fetched on demand per visible card the same way GetImageDataURL is.
// A multi-file capture only gets a thumbnail for its first entry; the card lists every name.
func (a *App) GetFileThumbnail(id string) (string, error) {
	if err := a.ready(); err != nil {
		return "", err
	}
	item, err := a.repository.GetByID(a.ctx, id)
	if err != nil {
		return "", err
	}
	if item.Type != clipboard.ContentFile || item.FilePath == "" {
		return "", errors.New("item has no files")
	}
	if a.platform == nil {
		return "", errors.New("file thumbnails are not yet available")
	}
	first := strings.SplitN(item.FilePath, "\n", 2)[0]
	data, ok := a.platform.FileThumbnail(first)
	if !ok {
		return "", errors.New("no thumbnail available")
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(data), nil
}

// PreviewSelection toggles the OS-native Quick Look preview panel (macOS only — see
// platform.Controller.PreviewFile's doc comment) for a history item, triggered by the frontend's
// Space key handler. Every content type is previewable: image/file items point Quick Look at
// their own file(s) already on disk, while text/URL/color items — which have no file of their
// own — get one synthesized on demand (see quicklookTextFile/quicklookWeblocFile) so the same
// native panel works uniformly across every card type. Returns whether the panel is now showing.
// a.platform is never nil at this point on a real build (only nil if platform.New itself failed),
// but PreviewFile itself already no-ops (returns false) on Windows and the non-darwin/non-windows
// stub, so the frontend's Space handler needs no GOOS check of its own — it just does nothing
// there.
func (a *App) PreviewSelection(id string) (bool, error) {
	if err := a.ready(); err != nil {
		return false, err
	}
	item, err := a.repository.GetByID(a.ctx, id)
	if err != nil {
		return false, err
	}
	var paths []string
	switch item.Type {
	case clipboard.ContentImage:
		if item.FilePath == "" {
			return false, errors.New("item has no image")
		}
		paths = []string{item.FilePath}
	case clipboard.ContentFile:
		if item.FilePath == "" {
			return false, errors.New("item has no files")
		}
		paths = strings.Split(item.FilePath, "\n")
	case clipboard.ContentURL:
		if item.Text == "" {
			return false, errors.New("item has no url")
		}
		path, err := a.quicklookWeblocFile(item.Hash, item.Text)
		if err != nil {
			return false, err
		}
		paths = []string{path}
	case clipboard.ContentText, clipboard.ContentColor:
		if item.Text == "" {
			return false, errors.New("item has no text")
		}
		path, err := a.quicklookTextFile(item.Hash, item.Text)
		if err != nil {
			return false, err
		}
		paths = []string{path}
	default:
		return false, errors.New("item is not previewable")
	}
	if a.platform == nil {
		return false, nil
	}
	return a.platform.PreviewFile(paths), nil
}

// quicklookTextFile writes text to a content-addressed .txt file under the app data dir — reusing
// hash (the same dedup hash already computed for the item at capture time) as the filename so
// repeated previews of the same item, or of two items that happen to share content, reuse one
// file instead of rewriting it on every Space press — for Quick Look to preview a text/color item,
// neither of which has a file of its own on disk the way an image/file item does.
func (a *App) quicklookTextFile(hash, text string) (string, error) {
	dir := filepath.Join(a.dataDir, "quicklook")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, hash+".txt")
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		return "", err
	}
	return path, nil
}

// quicklookWeblocFile is quicklookTextFile's counterpart for a URL item: a .webloc file is a
// plist wrapping a single URL, the standard macOS bookmark-file format Quick Look already ships a
// generator for (title + favicon), so a link previews as a link rather than as its raw text.
func (a *App) quicklookWeblocFile(hash, rawURL string) (string, error) {
	dir := filepath.Join(a.dataDir, "quicklook")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, hash+".webloc")
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	type dict struct {
		Key string `xml:"key"`
		URL string `xml:"string"`
	}
	type plist struct {
		XMLName xml.Name `xml:"plist"`
		Version string   `xml:"version,attr"`
		Dict    dict     `xml:"dict"`
	}
	body, err := xml.MarshalIndent(plist{Version: "1.0", Dict: dict{Key: "URL", URL: rawURL}}, "", "  ")
	if err != nil {
		return "", err
	}
	content := xml.Header + `<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n" + string(body) + "\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}
	return path, nil
}

// PreviewURL fetches title/favicon metadata for a URL history item on demand (design doc §17):
// never called from the watcher, so a slow site can never block clipboard capture.
func (a *App) PreviewURL(id string) (webpreview.Preview, error) {
	if err := a.ready(); err != nil {
		return webpreview.Preview{}, err
	}
	item, err := a.repository.GetByID(a.ctx, id)
	if err != nil {
		return webpreview.Preview{}, err
	}
	if item.Type != clipboard.ContentURL {
		return webpreview.Preview{}, errors.New("item is not a URL")
	}
	return webpreview.Fetch(a.ctx, item.Text)
}

func (a *App) CopyItem(id string) error {
	if err := a.ready(); err != nil {
		return err
	}
	item, err := a.clipboard.PreparePaste(a.ctx, id)
	if err != nil {
		return err
	}
	// PreparePaste already bumped last_used_at (re-pinning this item to the top of history, same
	// as a fresh capture would); the UI needs to hear about it even though the item itself isn't
	// "new", so it re-sorts and shows the refreshed time immediately rather than on the next
	// unrelated history change.
	runtime.EventsEmit(a.ctx, "history:changed", item.ID)
	if item.Type == clipboard.ContentFile {
		if a.platform == nil {
			return errors.New("file clipboard support is not yet available")
		}
		if filePathsMissing(item.FilePath) {
			// The card may have been loaded before the user moved/deleted one of its files.
			// Remove the stale option immediately instead of silently copying only a subset.
			if deleteErr := a.repository.Delete(a.ctx, item.ID); deleteErr == nil {
				a.reconcileAssets(a.ctx)
				runtime.EventsEmit(a.ctx, "history:changed", item.ID)
			}
			return errors.New("file(s) no longer exist at their original location")
		}
		var existing []string
		for _, p := range strings.Split(item.FilePath, "\n") {
			if _, statErr := os.Stat(p); statErr == nil {
				existing = append(existing, p)
			}
		}
		if len(existing) == 0 {
			return errors.New("file(s) no longer exist at their original location")
		}
		return a.platform.WriteClipboardFiles(existing)
	}
	if item.Type == clipboard.ContentImage {
		if a.platform == nil {
			return errors.New("image clipboard support is not yet available")
		}
		data, err := os.ReadFile(item.FilePath)
		if err != nil {
			return err
		}
		return a.platform.WriteClipboardImage(data)
	}
	if (item.HTML != "" || item.RTF != "") && a.platform != nil {
		// Write both the rich payload and plain text together — see
		// platform.Controller.WriteClipboardRichText's doc comment: the receiving app picks
		// whichever format it understands (formatted in Word/Excel/PowerPoint/browsers, plain
		// text everywhere else), so nothing here needs to know what's currently focused.
		htmlPayload := item.HTML
		if refs, refsErr := a.repository.AssetsForItem(a.ctx, item.ID); refsErr == nil && len(refs) > 0 {
			byHash := make(map[string]clipboard.AssetRef, len(refs))
			for _, ref := range refs {
				byHash[ref.Hash] = ref
			}
			htmlPayload = clipboard.HydrateLocalImages(htmlPayload, func(hash string) ([]byte, string, error) {
				ref, ok := byHash[hash]
				if !ok {
					return nil, "", os.ErrNotExist
				}
				data, err := os.ReadFile(ref.Path)
				return data, ref.MIME, err
			})
		}
		if err := a.platform.WriteClipboardRichText(htmlPayload, item.RTF, item.Text); err == nil {
			return nil
		}
		// Fall through to the plain-text-only path below on error, so a rich-text write failure
		// still leaves the user with a working plain-text paste rather than nothing at all.
	}
	return runtime.ClipboardSetText(a.ctx, item.Text)
}

// PasteItem implements the full "select -> hide -> restore focus -> Ctrl+V" loop from design
// doc §21. When the native paste injection fails (e.g. an elevated foreground app, per §24) the
// content is still on the clipboard, so the caller only needs to surface a soft notice.
func (a *App) PasteItem(id string) error {
	if err := a.CopyItem(id); err != nil {
		return err
	}
	a.HideWindow()
	if a.platform == nil {
		return nil
	}
	if err := a.platform.SendPaste(); err != nil {
		return err
	}
	return nil
}

// ShowWindow records whatever window currently has focus, then slides SailBoard's window up from
// off-screen to the bottom of the monitor under the cursor (design doc §55/§56) — see
// platform.Controller.SlideReveal's doc comment for why the whole window physically moves,
// background included, rather than just .sheet's content sliding within an already-shown window.
func (a *App) ShowWindow() {
	if a.platform != nil {
		a.mu.Lock()
		a.foregroundToken = a.platform.CaptureForeground()
		a.mu.Unlock()
	}
	runtime.WindowUnminimise(a.ctx)
	if a.platform != nil {
		if area, scale, ok := a.platform.WorkAreaNearCursor(); ok {
			h := int(math.Round(float64(panelHeight) * scale))
			x, y, w := area.X, area.Y+area.Height-h, area.Width
			// SlideReveal blocks in whatever goroutine calls it for the full panelAnimationMs, so
			// it runs on its own goroutine (serialized against HideWindow/HideWindowAnimated via
			// animMu — see that field's doc comment) rather than stalling ShowWindow itself:
			// FocusSelf and the window:shown event below need to fire right away.
			go func() {
				a.animMu.Lock()
				defer a.animMu.Unlock()
				if err := a.platform.SlideReveal(appWindowTitle, x, y, w, h, panelAnimationMs); err != nil {
					a.platform.PositionSelf(appWindowTitle, x, y, w, h)
					runtime.WindowShow(a.ctx)
				}
				// Re-focus once the slide has actually settled into its final position, in addition
				// to the immediate FocusSelf call below (kept as-is for keyboard responsiveness —
				// this isn't a replacement for it, just an extra settle pass once the window is no
				// longer being repositioned).
				a.platform.FocusSelf(appWindowTitle)
			}()
		} else {
			runtime.WindowShow(a.ctx)
		}
		// Wails' own WindowShow() already tries to focus the window and silently fails here:
		// our hotkey handler runs on a goroutine spawned off the native message-loop thread, so
		// by the time it runs, Windows no longer grants the "currently handling this input
		// event" exemption that SetForegroundWindow needs from a background process. See
		// platform.Controller.FocusSelf's doc comment for the AttachThreadInput workaround.
		a.platform.FocusSelf(appWindowTitle)
	} else {
		runtime.WindowShow(a.ctx)
	}
	runtime.EventsEmit(a.ctx, "window:shown")
}

// HideWindow hides the panel instantly (no animation) and restores focus to whatever app had it
// before SailBoard was summoned (design doc §25); the token is single-use and cleared on every
// hide. Used by PasteItem, where dismissal should feel immediate rather than something the user
// watches happen — see HideWindowAnimated for the Esc/focus-loss counterpart.
func (a *App) HideWindow() {
	a.animMu.Lock()
	defer a.animMu.Unlock()
	runtime.WindowHide(a.ctx)
	a.mu.Lock()
	token := a.foregroundToken
	a.foregroundToken = nil
	a.mu.Unlock()
	if a.platform != nil && token != nil {
		a.platform.RestoreForeground(token)
	}
}

// HideWindowAnimated hides the panel with the same physical slide ShowWindow uses to reveal it
// (platform.Controller.SlideDismiss), then restores focus the same way HideWindow does. Used for
// the Esc/focus-loss dismissal, where the panel closing is something the user is actively
// watching — unlike PasteItem's dismissal, which intentionally stays instant via plain HideWindow.
func (a *App) HideWindowAnimated() {
	a.animMu.Lock()
	defer a.animMu.Unlock()
	a.mu.Lock()
	token := a.foregroundToken
	a.foregroundToken = nil
	a.mu.Unlock()
	if a.platform == nil {
		runtime.WindowHide(a.ctx)
	} else if err := a.platform.SlideDismiss(appWindowTitle, panelAnimationMs); err != nil {
		runtime.WindowHide(a.ctx)
	}
	if a.platform != nil && token != nil {
		a.platform.RestoreForeground(token)
	}
}

// OpenSettingsWindow launches the standalone settings window (see main.go's runSettingsWindow /
// settings_app.go) as a second process, or simply focuses it if one is already open.
func (a *App) OpenSettingsWindow() error {
	if a.platform != nil && a.platform.FocusIfExists(settingsWindowTitle) {
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return exec.Command(exe, settingsFlag).Start()
}

func appDataDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(base, "SailBoard")
	return path, os.MkdirAll(path, 0755)
}

// diskImageStore implements clipboard.ImageStore, saving each distinct image once under its
// content hash (design doc §11: files on disk, not DB blobs) so repeated copies of the same
// image don't multiply storage use.
type diskImageStore struct{ dir string }

func (s *diskImageStore) Save(hash string, data []byte) (string, error) {
	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(s.dir, hash+".png")
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}
	return path, nil
}

type diskAssetStore struct {
	imagesDir string
	richDir   string
	mu        sync.Mutex
}

func (s *diskAssetStore) SaveAsset(data []byte, mimeType string) (clipboard.AssetRef, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	ext := ".bin"
	switch strings.ToLower(mimeType) {
	case "image/jpeg":
		ext = ".jpg"
	case "image/gif":
		ext = ".gif"
	case "image/webp":
		ext = ".webp"
	case "image/bmp":
		ext = ".bmp"
	case "image/tiff":
		ext = ".tiff"
	default:
		ext = ".png"
	}
	if err := os.MkdirAll(s.richDir, 0755); err != nil {
		return clipboard.AssetRef{}, err
	}
	path := filepath.Join(s.richDir, hash+ext)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.WriteFile(path, data, 0644); err != nil {
			return clipboard.AssetRef{}, err
		}
	} else if err != nil {
		return clipboard.AssetRef{}, err
	}
	return clipboard.AssetRef{Hash: hash, Path: path, MIME: mimeType, ByteSize: int64(len(data))}, nil
}

func (a *App) reconcileAssets(ctx context.Context) {
	if a.assets == nil || a.repository == nil {
		return
	}
	a.assetMu.Lock()
	defer a.assetMu.Unlock()
	_ = a.repository.PruneUnreferencedAssets(ctx)
	paths, err := a.repository.ReferencedAssetPaths(ctx)
	if err != nil {
		return
	}
	_ = a.assets.Reconcile(paths)
}

func (s *diskAssetStore) Reconcile(active []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	keep := make(map[string]bool, len(active))
	for _, p := range active {
		if abs, err := filepath.Abs(p); err == nil {
			keep[strings.ToLower(filepath.Clean(abs))] = true
		}
	}
	for _, dir := range []string{s.imagesDir, s.richDir} {
		entries, err := os.ReadDir(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			p := filepath.Join(dir, entry.Name())
			abs, err := filepath.Abs(p)
			if err != nil {
				continue
			}
			if !keep[strings.ToLower(filepath.Clean(abs))] {
				_ = os.Remove(p)
			}
		}
	}
	return nil
}
