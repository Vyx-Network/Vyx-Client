//go:build windows
// +build windows

package platform

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32     = syscall.NewLazyDLL("kernel32.dll")
	lockFileEx   = kernel32.NewProc("LockFileEx")
	unlockFileEx = kernel32.NewProc("UnlockFileEx")
)

const (
	LOCKFILE_EXCLUSIVE_LOCK   = 0x00000002
	LOCKFILE_FAIL_IMMEDIATELY = 0x00000001
)

// acquireLock attempts to acquire an exclusive lock on Windows
func acquireLock(file *os.File) error {
	// Get file handle
	handle := syscall.Handle(file.Fd())

	// Create overlapped structure (required for LockFileEx)
	var overlapped syscall.Overlapped

	// Try to acquire exclusive lock (non-blocking)
	// LOCKFILE_EXCLUSIVE_LOCK | LOCKFILE_FAIL_IMMEDIATELY
	flags := uint32(LOCKFILE_EXCLUSIVE_LOCK | LOCKFILE_FAIL_IMMEDIATELY)

	ret, _, err := lockFileEx.Call(
		uintptr(handle),
		uintptr(flags),
		uintptr(0), // reserved
		uintptr(1), // lock 1 byte
		uintptr(0), // high order 32 bits of length
		uintptr(unsafe.Pointer(&overlapped)),
	)

	if ret == 0 {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}

	return nil
}

// releaseLock releases the exclusive lock on Windows
func releaseLock(file *os.File) {
	handle := syscall.Handle(file.Fd())
	var overlapped syscall.Overlapped

	unlockFileEx.Call(
		uintptr(handle),
		uintptr(0), // reserved
		uintptr(1), // unlock 1 byte
		uintptr(0), // high order 32 bits of length
		uintptr(unsafe.Pointer(&overlapped)),
	)
}
