//go:build windows

package platform

import "os/exec"

// OpenFolderDirect opens path in Explorer. A standalone function (not a Controller method, same
// convention as SetAutoLaunchDirect/NotifySettingsChanged) since the settings window is a
// separate process with no Controller/message-loop of its own, and this needs neither — Explorer
// is just launched as an ordinary child process.
func OpenFolderDirect(path string) error {
	return exec.Command("explorer", path).Start()
}
