//go:build darwin

package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// launchAgentLabel/launchAgentPath mirror runKeyPath's role on Windows — the stable identifier
// used to find/create/remove this app's autostart entry. A LaunchAgent plist under
// ~/Library/LaunchAgents is the standard per-user "run at login" mechanism on macOS: no cgo, no
// launchctl call needed either — launchd itself scans this directory at every login, so the file
// just needs to exist (or not) by the time the user next logs in.
func launchAgentLabel(appName string) string {
	return "com.wails." + appName
}

func launchAgentPath(appName string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", launchAgentLabel(appName)+".plist"), nil
}

// SetAutoLaunchDirect and AutoLaunchEnabledDirect are standalone entry points to the same
// LaunchAgent logic the Controller's SetAutoLaunch/AutoLaunchEnabled use, for callers (the
// settings window process) that only need this one capability — mirrors autolaunch_windows.go's
// identical split.
func SetAutoLaunchDirect(appName string, enabled bool) error {
	return setAutoLaunchDarwin(appName, enabled)
}

func AutoLaunchEnabledDirect(appName string) (bool, error) {
	return autoLaunchEnabledDarwin(appName)
}

func (c *darwinController) SetAutoLaunch(enabled bool) error {
	return setAutoLaunchDarwin(c.appName, enabled)
}

func (c *darwinController) AutoLaunchEnabled() (bool, error) {
	return autoLaunchEnabledDarwin(c.appName)
}

func setAutoLaunchDarwin(appName string, enabled bool) error {
	path, err := launchAgentPath(appName)
	if err != nil {
		return err
	}
	if !enabled {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>
`, launchAgentLabel(appName), exePath)
	return os.WriteFile(path, []byte(plist), 0644)
}

// darwinProgramArgumentRe pulls the first (only) <string> out of the <array> this package itself
// writes under ProgramArguments in setAutoLaunchDarwin — not a general plist parser, just enough
// to read back exactly the shape written above.
var darwinProgramArgumentRe = regexp.MustCompile(`(?s)<key>ProgramArguments</key>\s*<array>\s*<string>(.*?)</string>`)

// autoLaunchEnabledDarwin mirrors autoLaunchEnabled's "will this really run" check on Windows: a
// LaunchAgent plist surviving after the app it points at was moved/deleted would never actually
// launch anything, so that's reported as disabled rather than a stale false positive.
func autoLaunchEnabledDarwin(appName string) (bool, error) {
	path, err := launchAgentPath(appName)
	if err != nil {
		return false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	m := darwinProgramArgumentRe.FindSubmatch(data)
	if m == nil {
		return false, nil
	}
	if _, statErr := os.Stat(string(m[1])); statErr != nil {
		return false, nil
	}
	return true, nil
}
