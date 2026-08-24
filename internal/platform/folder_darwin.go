//go:build darwin

package platform

import "os/exec"

// OpenFolderDirect opens path in Finder. A standalone function (not a Controller method) — see
// folder_windows.go's identical convention (the settings window is a separate process with no
// Controller of its own, and this needs neither — `open` is just launched as an ordinary child
// process).
func OpenFolderDirect(path string) error {
	return exec.Command("open", path).Start()
}
