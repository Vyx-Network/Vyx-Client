package platform

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
)

const serviceTemplate = `[Unit]
Description=Vyx
After=network.target

[Service]
ExecStart=/usr/local/bin/Vyx
Restart=always
User=%s
Environment=PATH=/usr/local/bin:/usr/bin
WorkingDirectory=%s

[Install]
WantedBy=multi-user.target
`

const servicePath = "/etc/systemd/system/vyx.service"
const binPath = "/usr/local/bin/Vyx"

func EnableAutoStart() error {
	usr, err := user.Current()
	if err != nil {
		return err
	}

	executable, err := os.Executable()
	if err != nil {
		return err
	}

	os.Link(executable, binPath)

	serviceContent := fmt.Sprintf(serviceTemplate, usr.Username, usr.HomeDir)

	err = os.WriteFile(servicePath, []byte(serviceContent), 0644)

	err = exec.Command("systemctl", "daemon-reexec").Run()
	if err != nil {
		return err
	}
	err = exec.Command("systemctl", "daemon-reload").Run()
	if err != nil {
		return err
	}
	err = exec.Command("systemctl", "enable", "vyx.service").Run()
	if err != nil {
		return err
	}
	err = exec.Command("systemctl", "start", "vyx.service").Run()
	if err != nil {
		return err
	}

	return nil
}

func DisableAutoStart() error {
	// Stop the service if running
	exec.Command("systemctl", "stop", "vyx.service").Run()

	// Disable the service
	exec.Command("systemctl", "disable", "vyx.service").Run()

	// Remove service file
	os.Remove(servicePath)

	// Reload systemd
	exec.Command("systemctl", "daemon-reload").Run()

	return nil
}

func IsAutoStartEnabled() bool {
	// Check if service file exists
	if _, err := os.Stat(servicePath); err != nil {
		return false
	}

	// Check if service is enabled
	err := exec.Command("systemctl", "is-enabled", "vyx.service").Run()
	return err == nil
}
