package main

import (
	"embed"
	"net/http"
	"os"
	goruntime "runtime"

	"SailBoard/internal/platform"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed logo.png
var trayIconPNGWindows []byte

//go:embed logo_tb.png
var trayIconPNGDarwin []byte

// trayIconPNG is the tray/menu-bar icon. Windows keeps the solid-background logo — its tray
// icons render fine over an opaque taskbar — while macOS uses the transparent-background variant
// (logo_tb.png), since a solid square reads wrong against the menu bar's own translucent,
// light/dark-mode-aware background (every other menu bar icon on the system is transparent).
var trayIconPNG = func() []byte {
	if goruntime.GOOS == "darwin" {
		return trayIconPNGDarwin
	}
	return trayIconPNGWindows
}()

// settingsWindowTitle is the second window's title, used both by main.go's Wails options and by
// platform.Controller.FocusIfExists (app.go's OpenSettingsWindow) to avoid opening a duplicate.
const settingsWindowTitle = "SailBoard 设置"

// settingsFlag re-launches this same executable as a second, independent process: a normal,
// resizable, native-decorated window (unlike the frameless always-on-top main panel) that the
// user can drag anywhere on screen, showing only the settings UI. Wails v2 has no supported
// multi-window API for a single process, so a second process is the standard workaround — see
// app.go's OpenSettingsWindow and settings_app.go for the rest of this split.
const settingsFlag = "--settings"

// settingsWindowWidth/Height are the settings window's logical/CSS-pixel size (what Settings.css
// is laid out against at 100% display scaling) — passed both to wails.Run's options.App and, on
// Windows, to platform.FixSettingsWindowDirect, which re-applies them scaled by the window's
// actual monitor DPI (see that function's doc comment for why Wails' own scaling isn't trusted
// here).
const (
	settingsWindowWidth  = 420
	settingsWindowHeight = 560
)

func main() {
	fixDarwinLocale()
	if len(os.Args) > 1 && os.Args[1] == settingsFlag {
		runSettingsWindow()
		return
	}
	runMainWindow()
}

// fixDarwinLocale works around a real bug, not a hypothetical one: Wails' own clipboard text
// read/write on macOS (internal/frontend/desktop/darwin/clipboard.go) shells out to pbpaste/
// pbcopy, and pbpaste's output encoding depends on LANG/LC_ALL. A GUI .app launched from Finder/
// LaunchServices (as opposed to a shell) inherits neither — confirmed by hand: `pbpaste` under an
// env with no LANG set emits legacy GBK-ish bytes for CJK clipboard content instead of UTF-8,
// which is exactly the "复制中文显示乱码" (copied Chinese text shows as mojibake) symptom this
// fixes. Setting it here, before wails.Run ever shells out, fixes it for every pbpaste/pbcopy
// call for the lifetime of the process — os/exec inherits the current process's environment by
// default. No-op on Windows, which doesn't use this subprocess-based clipboard path at all.
func fixDarwinLocale() {
	if goruntime.GOOS != "darwin" {
		return
	}
	os.Setenv("LANG", "en_US.UTF-8")
	os.Setenv("LC_ALL", "en_US.UTF-8")
}

func runMainWindow() {
	// Refuse to run a second copy alongside an already-running one — it would double-capture the
	// clipboard, double-register the global hotkey, and fight over the tray icon. Instead hand
	// off to the existing instance (ask it to show its panel) and exit immediately, before ever
	// creating a window of our own.
	if acquired, err := platform.AcquireSingleInstanceLock(); err == nil && !acquired {
		platform.RequestShowMainWindow()
		return
	}

	app := NewApp()

	err := wails.Run(&options.App{
		Title:         appWindowTitle,
		Width:         1200,
		Height:        330, // must match app.go's panelHeight
		MinWidth:      900,
		MinHeight:     280,
		Frameless:     true,
		AlwaysOnTop:   true,
		DisableResize: true,
		StartHidden:   true,
		// Left false (the zero value) rather than omitted, so it's clear this is deliberate: a raw
		// browser right-click menu (Back/Reload/Inspect) has no place in this app, and right-
		// clicking the panel is a real, if rare, user action (see main.tsx's contextmenu listener,
		// a DOM-level backstop for whatever this WebView2-level setting doesn't reliably suppress
		// on its own on every device).
		EnableDefaultContextMenu: false,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		// Fully transparent app background: the visible light glass panel is painted entirely
		// by CSS (frontend/src/App.css), composited over the native Acrylic backdrop below so
		// it reads as real iOS/macOS-style frosted glass (blurring whatever is behind the
		// window) rather than a flat translucent colour.
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 0},
		Windows: &windows.Options{
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			BackdropType:         windows.Acrylic,
			// A frameless window otherwise gets Windows 11's automatic rounded corners + drop
			// shadow, which reads as a floating card. SailBoard is a full-width bottom sheet
			// flush with the screen edges, so those decorations are turned off.
			DisableFramelessWindowDecorations: true,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

// runSettingsWindow is the second process's entry point: a normal window (real title bar, so
// dragging/moving/closing is native OS behaviour, not custom-built) showing only the settings
// form. It shares frontend/dist with the main window — settingsHTMLMiddleware below serves
// settings.html (a second Vite entry point, see frontend/vite.config.ts) in place of index.html
// so no separate embed/build pipeline is needed.
func runSettingsWindow() {
	app := NewSettingsApp()

	err := wails.Run(&options.App{
		Title:            settingsWindowTitle,
		Width:            settingsWindowWidth,
		Height:           settingsWindowHeight,
		MinWidth:         380,
		MinHeight:        460,
		// The main panel is itself AlwaysOnTop (see runMainWindow), which on Windows means
		// WS_EX_TOPMOST — a topmost window stays above every non-topmost window regardless of
		// which one is actually focused/active. Without this, clicking into the settings window
		// (opened from the main panel, or just left open while the panel is later summoned again)
		// doesn't bring it above the panel; it just gets covered. Putting both windows in the same
		// topmost band restores normal focus-order behaviour between the two of them, while
		// leaving how either one relates to unrelated apps unchanged.
		AlwaysOnTop:      true,
		BackgroundColour: &options.RGBA{R: 245, G: 245, B: 247, A: 255},
		AssetServer: &assetserver.Options{
			Assets:     assets,
			Middleware: settingsHTMLMiddleware,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

func settingsHTMLMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			r.URL.Path = "/settings.html"
		}
		next.ServeHTTP(w, r)
	})
}
