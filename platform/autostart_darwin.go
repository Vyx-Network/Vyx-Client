package platform

import (
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
)

const plistName = "com.vyx.client.plist"

func getPlistPath() string {
	usr, _ := user.Current()
	launchAgentsDir := filepath.Join(usr.HomeDir, "Library", "LaunchAgents")
	return filepath.Join(launchAgentsDir, plistName)
}

func EnableAutoStart() error {
	usr, err := user.Current()
	if err != nil {
		return err
	}

	executable, err := os.Executable()
	if err != nil {
		return err
	}

	launchAgentsDir := filepath.Join(usr.HomeDir, "Library", "LaunchAgents")
	os.MkdirAll(launchAgentsDir, 0755)

	currentPlistPath := filepath.Join("./assets", plistName)

	plistPath := getPlistPath()

	if data, err := os.ReadFile(currentPlistPath); err != nil {
		return err
	} else if err := os.WriteFile(plistPath, []byte(strings.Replace(string(data), "{executable_path}", executable, 1)), 0644); err != nil {
		return err
	}

	return exec.Command("launchctl", "load", plistPath).Start()
}

func DisableAutoStart() error {
	plistPath := getPlistPath()

	// Unload the service
	exec.Command("launchctl", "unload", plistPath).Run()

	// Remove plist file
	os.Remove(plistPath)

	return nil
}

func IsAutoStartEnabled() bool {
	plistPath := getPlistPath()

	// Check if plist file exists
	_, err := os.Stat(plistPath)
	return err == nil
}
