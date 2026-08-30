#!/bin/bash
# Packages build/bin/SailBoard.app into a "drag SailBoard.app onto Applications" DMG with an
# instructional background image (build/darwin/dmg-background.png) and pre-arranged icon
# positions, via a temporary read-write DMG that Finder is scripted (AppleScript) to lay out
# before it's converted to the final compressed image. See README.md's macOS build section for
# when to run this — after `wails build`, not instead of it.
#
# Background image size (600x400px) intentionally matches the Finder window's content area
# 1:1 in points: Finder scales a DMG background picture to fit the window rather than treating
# it as an @2x retina asset, so any other ratio throws off the icon-position math below.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BG_SRC="$REPO_ROOT/build/darwin/dmg-background.png"
APP="$REPO_ROOT/build/bin/SailBoard.app"
VOLNAME="SailBoard"
STAGING="$REPO_ROOT/build/dmg-staging"
RW_DMG="$REPO_ROOT/build/bin/SailBoard-rw.dmg"
FINAL_DMG="${1:-$REPO_ROOT/build/bin/SailBoard-1.1.0-arm64.dmg}"
MOUNT_POINT="/Volumes/$VOLNAME"

if [ ! -d "$APP" ]; then
  echo "error: $APP not found — run wails build first" >&2
  exit 1
fi

rm -rf "$STAGING"
mkdir -p "$STAGING/.background"
cp -R "$APP" "$STAGING/"
ln -s /Applications "$STAGING/Applications"
cp "$BG_SRC" "$STAGING/.background/background.png"

rm -f "$RW_DMG" "$FINAL_DMG"
if [ -d "$MOUNT_POINT" ]; then
  hdiutil detach "$MOUNT_POINT" -force 2>&1 || true
fi

hdiutil create -volname "$VOLNAME" -srcfolder "$STAGING" -ov -fs HFS+ -format UDRW "$RW_DMG"
hdiutil attach "$RW_DMG" -mountpoint "$MOUNT_POINT"

osascript <<APPLESCRIPT
tell application "Finder"
    tell disk "$VOLNAME"
        open
        tell container window
            set current view to icon view
            set toolbar visible to false
            set statusbar visible to false
            set the bounds to {200, 120, 800, 520}
        end tell
        set theViewOptions to the icon view options of container window
        set arrangement of theViewOptions to not arranged
        set icon size of theViewOptions to 96
        set text size of theViewOptions to 12
        set background picture of theViewOptions to file ".background:background.png"
        set position of item "SailBoard.app" of container window to {150, 250}
        set position of item "Applications" of container window to {450, 250}
        close
        open
        update without registering applications
        delay 1
        close
    end tell
end tell
APPLESCRIPT

sync
hdiutil detach "$MOUNT_POINT" 2>&1 || (sleep 2 && hdiutil detach "$MOUNT_POINT" -force)

hdiutil convert "$RW_DMG" -format UDZO -ov -o "$FINAL_DMG"
rm -f "$RW_DMG"
rm -rf "$STAGING"

echo "DMG built: $FINAL_DMG"
