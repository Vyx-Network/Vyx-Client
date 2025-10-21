//go:build !windows
// +build !windows

package platform

// IsAdmin checks if the current process has administrator/root privileges
func IsAdmin() bool {
	// On Unix, check if running as root (UID 0)
	// This is a placeholder - actual implementation would use syscall.Getuid()
	return true // Assume elevated on Unix for now
}

// RequestElevation is not implemented on Unix platforms
func RequestElevation() error {
	return nil // No-op on Unix
}

// ElevateIfNeeded is not implemented on Unix platforms
func ElevateIfNeeded() error {
	return nil // No-op on Unix
}
