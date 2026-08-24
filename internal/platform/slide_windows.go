//go:build windows

package platform

import (
	"fmt"
	"time"
	"unsafe"
)

// slideStepMs is the interval between position updates during a reveal/dismiss slide — small
// enough to read as smooth motion, large enough not to flood the message queue with redundant
// SetWindowPos calls.
const slideStepMs = 10

// slideReveal shows SailBoard's window (found by title) already positioned at its final
// (x, y, width, height) rectangle, then physically slides the whole window up from just below the
// bottom of the screen to that position over durationMs, via repeated SetWindowPos calls.
//
// This is the third design tried for this reveal/dismiss animation (see git log for the other
// two): first a CSS clip-path + native click-through toggle, then an AnimateWindow(AW_BLEND)
// cross-fade. Both ran into the same underlying problem from different angles — AnimateWindow's
// blend, like the click-through experiment's WS_EX_LAYERED toggling before it, turned out to fade
// a *snapshot* of the window's content rather than live-composite it, so anything animating
// inside the page (like .sheet's own CSS slide) froze into that snapshot and was never visible;
// confirmed by live testing, where the whole panel read as a flat cross-fade instead of a slide.
// Physically moving the window sidesteps that whole family of bugs, because it never touches
// alpha or layered-window state at all — the window stays fully opaque and normally composited
// (Wails' own Acrylic backdrop, via SetWindowCompositionAttribute, and WebView2's live content)
// throughout, exactly as when the user drags an ordinary window around, which is why that always
// looks smooth. .sheet's own CSS transform is no longer used for this motion — the physical
// window position is, so the background moves in lockstep with the content by construction,
// rather than needing separately-timed animations kept in sync.
func slideReveal(title string, x, y, width, height, durationMs int) error {
	hwnd := findWindowByTitle(title)
	if hwnd == 0 {
		return fmt.Errorf("window titled %q not found", title)
	}
	startY := y + height // one panel-height below the final position: fully off-screen at the bottom
	procSetWindowPos.Call(hwnd, 0, uintptr(x), uintptr(startY), uintptr(width), uintptr(height), swpNoZOrder|swpNoActivate)
	procShowWindow.Call(hwnd, swShow)
	slideY(hwnd, x, width, height, startY, y, durationMs)
	return nil
}

// slideDismiss is slideReveal's counterpart: reads the window's current rectangle (found by
// title), slides it back down by its own height (off-screen at the bottom), then hides it.
func slideDismiss(title string, durationMs int) error {
	hwnd := findWindowByTitle(title)
	if hwnd == 0 {
		return fmt.Errorf("window titled %q not found", title)
	}
	var r rect
	procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	x, y := int(r.Left), int(r.Top)
	width, height := int(r.Right-r.Left), int(r.Bottom-r.Top)
	slideY(hwnd, x, width, height, y, y+height, durationMs)
	procShowWindow.Call(hwnd, swHide)
	return nil
}

// slideY animates hwnd's vertical position from fromY to toY over durationMs, holding x/width/
// height fixed — an ease-out curve (fast start, gentle finish) reads closer to the CSS
// cubic-bezier this replaced than a linear step would.
func slideY(hwnd uintptr, x, width, height, fromY, toY, durationMs int) {
	move := func(y int) {
		procSetWindowPos.Call(hwnd, 0, uintptr(x), uintptr(y), uintptr(width), uintptr(height), swpNoZOrder|swpNoActivate)
	}
	if durationMs <= 0 || fromY == toY {
		move(toY)
		return
	}
	total := time.Duration(durationMs) * time.Millisecond
	start := time.Now()
	for {
		t := float64(time.Since(start)) / float64(total)
		if t >= 1 {
			break
		}
		eased := 1 - (1-t)*(1-t)*(1-t) // ease-out cubic
		move(fromY + int(float64(toY-fromY)*eased))
		time.Sleep(slideStepMs * time.Millisecond)
	}
	move(toY)
}
