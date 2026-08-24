//go:build darwin

package platform

import (
	"os"
	"path/filepath"
	"syscall"
)

// singleInstanceLockPath is a fixed, app-specific location (independent of the current appName
// parameter — there's only ever one SailBoard) so every launch, including one started before a
// settings change, contends for the exact same file.
func singleInstanceLockPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "SailBoard")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "singleinstance.lock"), nil
}

// singleInstanceLockFile keeps the winning *os.File reachable for the process's entire lifetime.
// This isn't optional bookkeeping: os.File registers a runtime finalizer that closes its fd (and
// so releases the flock) once the object is garbage collected, and AcquireSingleInstanceLock's
// own local variable doesn't survive past its return — verified the hard way, by launching a
// second real .app instance and watching it become a full second process anyway. A short-lived
// standalone test program never ran long enough to trigger a GC cycle and see the bug; the real
// app, running for minutes with wails.Run() churning through allocations, reliably did.
var singleInstanceLockFile *os.File

// AcquireSingleInstanceLock claims an exclusive, non-blocking flock() on a fixed lock file so
// only one SailBoard main-window process ever runs at a time — the macOS counterpart to
// singleinstance_windows.go's named mutex (CreateMutex). ok is false when another process already
// holds it — main.go should then call RequestShowMainWindow and exit immediately rather than
// starting a redundant second copy.
func AcquireSingleInstanceLock() (ok bool, err error) {
	path, err := singleInstanceLockPath()
	if err != nil {
		return false, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return false, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if err == syscall.EWOULDBLOCK {
			return false, nil
		}
		return false, err
	}
	singleInstanceLockFile = f
	return true, nil
}
