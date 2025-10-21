package platform

import (
	"fmt"
	"os"
	"path/filepath"
)

// InstanceLock represents a lock to prevent multiple instances
type InstanceLock struct {
	lockFile *os.File
	lockPath string
}

// AcquireInstanceLock attempts to acquire a single-instance lock
// Returns an InstanceLock that should be released on exit, or an error if another instance is running
func AcquireInstanceLock() (*InstanceLock, error) {
	// Get lock file path in config directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	lockPath := filepath.Join(homeDir, ".vyx", "instance.lock")

	// Create .vyx directory if it doesn't exist
	lockDir := filepath.Dir(lockPath)
	if err := os.MkdirAll(lockDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create lock directory: %w", err)
	}

	// Try to open/create lock file
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open lock file: %w", err)
	}

	// Try to acquire exclusive lock (platform-specific implementation in _unix.go or _windows.go)
	if err := acquireLock(lockFile); err != nil {
		lockFile.Close()
		return nil, fmt.Errorf("another instance of Vyx is already running")
	}

	// Write current PID to lock file for debugging
	pid := fmt.Sprintf("%d\n", os.Getpid())
	lockFile.Truncate(0)
	lockFile.Seek(0, 0)
	lockFile.WriteString(pid)
	lockFile.Sync()

	return &InstanceLock{
		lockFile: lockFile,
		lockPath: lockPath,
	}, nil
}

// Release releases the instance lock
func (l *InstanceLock) Release() error {
	if l.lockFile == nil {
		return nil
	}

	// Release the lock (platform-specific)
	releaseLock(l.lockFile)

	// Close and remove lock file
	l.lockFile.Close()
	os.Remove(l.lockPath)

	return nil
}

// Platform-specific functions (implemented in _unix.go and _windows.go):
// - acquireLock(file *os.File) error
// - releaseLock(file *os.File)
