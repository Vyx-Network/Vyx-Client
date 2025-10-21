package main

import (
	"client/config"
	"client/conn"
	"client/logger"
	"client/platform"
	"client/ui"
	_ "embed"
	"flag"
	"log"
	"os"
	"time"

	"github.com/getlantern/systray"
)

//go:embed assets/tray_icon.ico
var iconData []byte

const (
	VERSION = "v0.1.1" // Semver format (must start with 'v')
	WEBSITE = "https://vyx.network"
)

var (
	guiMode     = flag.Bool("gui", false, "Run in GUI mode (no console window, logs to file)")
	consoleMode = flag.Bool("console", false, "Run in console mode with visible window")
)

func main() {
	flag.Parse()

	// Determine if running in GUI mode
	// Default to GUI mode if built with -H windowsgui, otherwise console mode
	isGUIMode := *guiMode || (!*consoleMode && isBuiltAsGUI())

	// Initialize logger (file for GUI mode, stdout for console mode)
	if err := logger.InitLogger(isGUIMode); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Close()

	logger.Info("Vyx Client v%s starting...", VERSION)
	if isGUIMode {
		logger.Info("Running in GUI mode - logs at: %s", logger.GetLogPath())
	} else {
		logger.Info("Running in console mode")
	}

	// SINGLE INSTANCE LOCK: Prevent multiple instances from running on the same device
	// This ensures the device doesn't appear multiple times in the dashboard
	instanceLock, err := platform.AcquireInstanceLock()
	if err != nil {
		logger.Error("Another instance is already running")
		log.Fatalf("ERROR: %v\n\nPlease close the existing instance before starting a new one.", err)
		return
	}
	defer instanceLock.Release()
	logger.Info("Instance lock acquired - this is the only running instance")

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Error("Could not load config: %v", err)
	} else {
		logger.Info("Config loaded - IsLoggedIn: %v, Email: %s", config.IsLoggedIn(), cfg.Email)
	}

	// Start QUIC connection
	go conn.ConnectQuicServer()

	systray.Run(onReady, onExit)
}

// isBuiltAsGUI checks if the binary was built with -H windowsgui (no console on Windows)
func isBuiltAsGUI() bool {
	// On Windows, if built with -H windowsgui, there's no stdout
	// Try to write to stdout to detect this
	_, err := os.Stdout.Write([]byte{})
	return err != nil
}

func onExit() {
	log.Println("Application exiting gracefully...")
}

func onReady() {
	ui.SetupTray(WEBSITE, iconData)

	// AUTO-START: Enable autostart based on user preference (default: enabled)
	// User can toggle via tray menu
	if config.GetAutoStartEnabled() {
		if err := platform.EnableAutoStart(); err != nil {
			logger.Error("Failed to enable autostart: %v", err)
		} else {
			logger.Info("Autostart enabled")
		}
	} else {
		if err := platform.DisableAutoStart(); err != nil {
			logger.Error("Failed to disable autostart: %v", err)
		} else {
			logger.Info("Autostart disabled")
		}
	}

	// AUTO-UPDATE: Check for updates on startup
	if err := AutoUpdate(); err != nil {
		log.Println(err)
	}

	// AUTO-LOGIN: If not logged in, automatically open browser for first-time setup
	if !config.IsLoggedIn() {
		logger.Info("First time setup - opening browser for login...")
		// Delay slightly to ensure tray is fully initialized
		go func() {
			time.Sleep(500 * time.Millisecond)
			ui.TriggerAutoLogin()
		}()
	}
}
