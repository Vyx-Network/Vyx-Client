//go:build !windows
// +build !windows

package platform

import (
	"os"
	"syscall"
)

// acquireLock attempts to acquire an exclusive lock on Unix/Linux/macOS
func acquireLock(file *os.File) error {
	// Try to acquire exclusive lock (non-blocking)
	// LOCK_EX = exclusive lock, LOCK_NB = non-blocking
	err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		return err
	}
	return nil
}

// releaseLock releases the exclusive lock on Unix/Linux/macOS
func releaseLock(file *os.File) {
	// LOCK_UN = unlock
	syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}
