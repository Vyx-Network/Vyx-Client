//go:build windows
// +build windows

package platform

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	shell32           = windows.NewLazySystemDLL("shell32.dll")
	procShellExecuteW = shell32.NewProc("ShellExecuteW")
)

// IsAdmin checks if the current process has administrator privileges
func IsAdmin() bool {
	var sid *windows.SID

	// Get the SID for the Administrators group
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid,
	)
	if err != nil {
		return false
	}
	defer windows.FreeSid(sid)

	// Check if the current token is a member of the Administrators group
	token := windows.Token(0)
	member, err := token.IsMember(sid)
	if err != nil {
		return false
	}

	return member
}

// RequestElevation requests UAC elevation by restarting the process as administrator
func RequestElevation() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Get command line arguments
	args := ""
	if len(os.Args) > 1 {
		for _, arg := range os.Args[1:] {
			args += arg + " "
		}
	}

	// Prepare parameters for ShellExecute
	verb, _ := syscall.UTF16PtrFromString("runas")
	exe, _ := syscall.UTF16PtrFromString(exePath)
	params, _ := syscall.UTF16PtrFromString(args)
	cwd, _ := syscall.UTF16PtrFromString("")

	// Call ShellExecuteW with "runas" to trigger UAC
	ret, _, _ := procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(exe)),
		uintptr(unsafe.Pointer(params)),
		uintptr(unsafe.Pointer(cwd)),
		uintptr(windows.SW_NORMAL),
	)

	// If ShellExecute succeeds (returns > 32), exit this process
	if ret > 32 {
		os.Exit(0)
		return nil
	}

	return fmt.Errorf("UAC elevation failed or was cancelled")
}

// ElevateIfNeeded checks if running as admin, and if not, requests elevation
func ElevateIfNeeded() error {
	if !IsAdmin() {
		return RequestElevation()
	}
	return nil
}
