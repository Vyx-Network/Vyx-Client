package ui

import (
	"client/config"
	"client/conn"
	"client/logger"
	"client/platform"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/getlantern/systray"
)

// Channel to signal successful authentication
var authSuccessChan = make(chan bool, 1)

// Channel to trigger login from external sources (e.g., auto-login on startup)
var triggerLoginChan = make(chan bool, 1)

// Channel to cancel pending authentication timeouts
var cancelAuthTimeoutChan = make(chan bool, 10)

func SetupTray(websiteUrl string, icon []byte) {
	// DEBUG MODE: Use localhost website for authentication
	if config.GlobalConfig != nil && config.GlobalConfig.DebugMode {
		websiteUrl = "http://127.0.0.1:8080"
		log.Printf("DEBUG MODE: Using localhost website: %s", websiteUrl)
	}

	systray.SetTemplateIcon(icon, icon)
	systray.SetTooltip("Vyx - Proxy Node Client")

	// Status display (non-clickable)
	statusItem := systray.AddMenuItem("Status: Starting...", "Current connection status")
	statusItem.Disable()

	uptimeItem := systray.AddMenuItem("Uptime: --", "Connection uptime")
	uptimeItem.Disable()

	connsItem := systray.AddMenuItem("Active Connections: 0", "Number of active proxy connections")
	connsItem.Disable()

	systray.AddSeparator()

	// Action items
	loginItem := systray.AddMenuItem("Login", "Login with your account")
	startItem := systray.AddMenuItem("Start Sharing", "Start sharing bandwidth and earning credits")
	stopItem := systray.AddMenuItem("Stop Sharing", "Stop sharing bandwidth")
	dashboard := systray.AddMenuItem("Dashboard", "Open dashboard")
	logout := systray.AddMenuItem("Logout", "Logout and clear credentials")
	systray.AddSeparator()

	// Settings menu
	autoStartItem := systray.AddMenuItemCheckbox("Run at Startup", "Start Vyx automatically when computer starts", config.GetAutoStartEnabled())
	systray.AddSeparator()

	quitItem := systray.AddMenuItem("Quit", "Quit the whole app")

	// Start status updater
	go updateStatusDisplay(statusItem, uptimeItem, connsItem)

	// Show/hide menu items based on login status and connection status
	updateMenuVisibility := func() {
		isLoggedIn := config.IsLoggedIn()
		isSharing := conn.IsConnected()

		if isLoggedIn {
			loginItem.Hide()
			dashboard.Show()
			logout.Show()

			if isSharing {
				startItem.Hide()
				stopItem.Show()
			} else {
				startItem.Show()
				stopItem.Hide()
			}
		} else {
			loginItem.Show()
			startItem.Hide()
			stopItem.Hide()
			dashboard.Hide()
			logout.Hide()
		}
	}

	// Initial visibility setup
	updateMenuVisibility()

	// Periodic visibility updater (every 2 seconds)
	// This ensures Start/Stop buttons update when connection state changes
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			updateMenuVisibility()
		}
	}()

	go func() {
		for {
			select {
			case <-triggerLoginChan:
				// External trigger for login (e.g., auto-login on first start)
				if !config.IsLoggedIn() {
					log.Println("Auto-triggering login on first start...")
					triggerLogin(websiteUrl, loginItem, startItem, stopItem, dashboard, logout, updateMenuVisibility)
				}
			case <-loginItem.ClickedCh:
				// Login button - trigger authentication flow
				if config.IsLoggedIn() {
					log.Println("Already logged in")
					updateMenuVisibility()
				} else {
					triggerLogin(websiteUrl, loginItem, startItem, stopItem, dashboard, logout, updateMenuVisibility)
				}
			case <-startItem.ClickedCh:
				// Start sharing bandwidth
				if config.IsLoggedIn() {
					log.Println("Starting bandwidth sharing...")
					conn.ReconnectQuic()
					// Give it a moment to connect, then update UI
					go func() {
						time.Sleep(500 * time.Millisecond)
						updateMenuVisibility()
					}()
				} else {
					log.Println("Cannot start sharing - not logged in")
				}
			case <-stopItem.ClickedCh:
				// Stop sharing bandwidth
				log.Println("Stopping bandwidth sharing...")
				conn.DisconnectQuic()
				updateMenuVisibility()
			case <-authSuccessChan:
				// BUG FIX: Only update UI after successful authentication
				log.Println("Authentication successful - updating UI and reconnecting...")
				updateMenuVisibility()

				// Cancel any pending authentication timeouts
				select {
				case cancelAuthTimeoutChan <- true:
					log.Println("Cancelled pending authentication timeout")
				default:
					// No timeout waiting, that's fine
				}

				// AUTO-RECONNECT: Trigger connection after successful login
				go func() {
					conn.ReconnectQuic()
					// Give it a moment to connect, then update UI
					time.Sleep(500 * time.Millisecond)
					updateMenuVisibility()
				}()
			case <-dashboard.ClickedCh:
				err := open(websiteUrl + "/dashboard")
				if err != nil {
					log.Println("Failed to open browser:", err)
				}
			case <-logout.ClickedCh:
				// Disconnect QUIC connection first
				conn.DisconnectQuic()
				log.Println("Disconnected from server")

				// Clear credentials
				if config.GlobalConfig != nil {
					config.GlobalConfig.APIToken = ""
					config.GlobalConfig.UserID = ""
					config.GlobalConfig.Email = ""
					if err := config.SaveConfig(config.GlobalConfig); err != nil {
						log.Println("Failed to save config:", err)
					}
				}
				log.Println("Logged out successfully")

				// Update menu visibility
				updateMenuVisibility()
			case <-autoStartItem.ClickedCh:
				// Toggle autostart preference
				currentState := config.GetAutoStartEnabled()
				newState := !currentState

				// Update config
				if err := config.SetAutoStartEnabled(newState); err != nil {
					logger.Error("Failed to save autostart preference: %v", err)
				}

				// Apply the change to the system
				if newState {
					if err := platform.EnableAutoStart(); err != nil {
						logger.Error("Failed to enable autostart: %v", err)
						log.Printf("ERROR: Could not enable autostart: %v", err)
						// Revert config change
						config.SetAutoStartEnabled(false)
						autoStartItem.Uncheck()
					} else {
						logger.Info("Autostart enabled")
						log.Println("Autostart enabled - Vyx will start when computer boots")
						autoStartItem.Check()
					}
				} else {
					if err := platform.DisableAutoStart(); err != nil {
						logger.Error("Failed to disable autostart: %v", err)
						log.Printf("ERROR: Could not disable autostart: %v", err)
						// Revert config change
						config.SetAutoStartEnabled(true)
						autoStartItem.Check()
					} else {
						logger.Info("Autostart disabled")
						log.Println("Autostart disabled - Vyx will not start automatically")
						autoStartItem.Uncheck()
					}
				}
			case <-quitItem.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

func open(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler"}
	case "darwin":
		cmd = "open"
	default:
		cmd = "xdg-open"
	}
	args = append(args, url)
	return exec.Command(cmd, args...).Start()
}

func startAuthServer() string {
	// Try up to 5 times to find an available port
	var server *http.Server
	var port string
	maxRetries := 5

	for i := 0; i < maxRetries; i++ {
		port = fmt.Sprintf("%d", 50000+rand.Intn(10000))

		mux := http.NewServeMux()
		mux.HandleFunc("/auth-result", func(w http.ResponseWriter, r *http.Request) {
			log.Printf("Received auth callback: Method=%s, Origin=%s, RemoteAddr=%s",
				r.Method, r.Header.Get("Origin"), r.RemoteAddr)

			// Add CORS headers - restrict origins based on debug mode
			origin := r.Header.Get("Origin")
			var allowedOrigins []string

			// In debug mode, allow localhost origins for development
			if config.GlobalConfig != nil && config.GlobalConfig.DebugMode {
				allowedOrigins = []string{
					"http://localhost:3000",
					"http://127.0.0.1:8080",
					"http://localhost:8080",
					"https://vyx.network",
					"https://www.vyx.network",
				}
			} else {
				// In production, only allow production origins
				allowedOrigins = []string{
					"https://vyx.network",
					"https://www.vyx.network",
				}
			}

			originAllowed := false
			for _, allowedOrigin := range allowedOrigins {
				if origin == allowedOrigin {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					originAllowed = true
					break
				}
			}

			if !originAllowed && origin != "" {
				log.Printf("WARNING: Rejected CORS origin: %s (not in allowed list)", origin)
			}

			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Max-Age", "3600") // Cache preflight requests for 1 hour

			// Security headers to protect against common web vulnerabilities
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none';")

			// Handle preflight OPTIONS request
			if r.Method == "OPTIONS" {
				log.Println("Handled CORS preflight request")
				w.WriteHeader(http.StatusOK)
				return
			}

			if r.Method != "POST" {
				log.Printf("ERROR: Invalid method %s (expected POST)", r.Method)
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				log.Println("Failed to read auth response:", err)
				http.Error(w, "Failed to read body", http.StatusBadRequest)
				return
			}

			var authData struct {
				Token  string `json:"token"`
				UserID string `json:"user_id"`
				Email  string `json:"email"`
			}

			if err := json.Unmarshal(body, &authData); err != nil {
				log.Println("Failed to parse auth response:", err)
				log.Printf("Received body: %s", string(body))
				http.Error(w, "Invalid JSON", http.StatusBadRequest)
				return
			}

			log.Printf("Received auth data - Token: %s..., UserID: %s, Email: %s",
				authData.Token[:min(10, len(authData.Token))],
				authData.UserID,
				authData.Email)

			// Save credentials to config
			if config.GlobalConfig == nil {
				config.GlobalConfig = &config.Config{
					ServerURL: "api.vyx.network:8443",
				}
			}
			config.GlobalConfig.APIToken = authData.Token
			config.GlobalConfig.UserID = authData.UserID
			config.GlobalConfig.Email = authData.Email

			if err := config.SaveConfig(config.GlobalConfig); err != nil {
				log.Println("Failed to save config:", err)
				http.Error(w, "Failed to save config", http.StatusInternalServerError)
				return
			}

			log.Printf("Successfully authenticated as: %s", authData.Email)
			log.Printf("Config saved. IsLoggedIn: %v", config.IsLoggedIn())

			// BUG FIX: Signal successful authentication to update UI
			select {
			case authSuccessChan <- true:
				log.Println("Sent auth success signal to tray")
			default:
				log.Println("Auth success channel full, tray already notified")
			}

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})

		// MAC FIX: Explicitly bind to 127.0.0.1 to avoid firewall issues on macOS
		server = &http.Server{
			Addr:         "127.0.0.1:" + port,
			Handler:      mux,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		}

		// Test if we can bind to this port
		log.Printf("Attempting to start auth server on 127.0.0.1:%s (attempt %d/%d)", port, i+1, maxRetries)

		// Start server in goroutine with error channel
		errChan := make(chan error, 1)
		go func() {
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				errChan <- err
			}
		}()

		// Wait a moment to see if server starts successfully
		select {
		case err := <-errChan:
			log.Printf("Failed to start server on port %s: %v", port, err)
			if i < maxRetries-1 {
				log.Println("Retrying with different port...")
				continue
			}
			log.Printf("CRITICAL: Could not start auth server after %d attempts", maxRetries)
			return ""
		case <-time.After(100 * time.Millisecond):
			// Server started successfully
			log.Printf("✓ Auth server started successfully on 127.0.0.1:%s", port)
			log.Printf("Ready to receive authentication callback from browser")
			return port
		}
	}

	return port
}

// updateStatusDisplay updates the tray menu status every 2 seconds
func updateStatusDisplay(statusItem, uptimeItem, connsItem *systray.MenuItem) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		status := logger.GetStatus()

		// Update status text
		statusItem.SetTitle(fmt.Sprintf("Status: %s", status.Status))

		// Update uptime
		uptime := "Not connected"
		if !status.ConnectionUptime.IsZero() {
			duration := time.Since(status.ConnectionUptime)
			uptime = formatDuration(duration)
		}
		uptimeItem.SetTitle(fmt.Sprintf("Uptime: %s", uptime))

		// Update connections
		connsItem.SetTitle(fmt.Sprintf("Active Connections: %d", status.ActiveConns))

		// Update tooltip with simple status (avoid duplicating menu items)
		tooltipText := fmt.Sprintf("Vyx - %s", status.Status)
		if status.ServerAddress != "" {
			tooltipText = fmt.Sprintf("Vyx - %s (%s)", status.Status, status.ServerAddress)
		}
		systray.SetTooltip(tooltipText)
	}
}

// formatDuration formats a duration into human-readable format
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	} else if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

// ShowNotification shows a system tray notification (if supported)
func ShowNotification(title, message string) {
	// Note: systray library doesn't support notifications directly
	// For Windows, you could use https://github.com/gen2brain/beeep
	// For now, we'll just log it
	log.Printf("NOTIFICATION: %s - %s", title, message)
}

// TriggerAutoLogin triggers automatic browser login on first start
// Should be called after tray is initialized
func TriggerAutoLogin() {
	select {
	case triggerLoginChan <- true:
		logger.Info("Triggered auto-login on first start")
	default:
		logger.Info("Auto-login already in progress")
	}
}

// triggerLogin handles the login flow (shared between manual click and auto-trigger)
func triggerLogin(websiteUrl string, loginItem, startItem, stopItem, dashboard, logout *systray.MenuItem, updateMenuVisibility func()) {
	// Start HTTP server to receive credentials
	port := startAuthServer()

	// Check if server started successfully
	if port == "" {
		log.Println("CRITICAL ERROR: Failed to start authentication server")
		log.Println("Possible causes:")
		log.Println("  1. Firewall is blocking local connections")
		log.Println("  2. All attempted ports are already in use")
		log.Println("  3. macOS security settings blocking the app")
		log.Println("")
		log.Println("Solutions to try:")
		log.Println("  1. Restart the app completely (Quit and reopen)")
		log.Println("  2. Check System Preferences → Security & Privacy → Firewall")
		log.Println("  3. Allow incoming connections for this app")
		return
	}

	authURL := websiteUrl + "/desktop-auth/check?port=" + port
	log.Printf("Opening browser for authentication on port %s...", port)
	log.Printf("Auth URL: %s", authURL)

	err := open(authURL)
	if err != nil {
		log.Printf("ERROR: Failed to open browser: %v", err)
		log.Printf("Please manually open this URL in your browser:")
		log.Printf("  %s", authURL)
	} else {
		log.Println("Browser opened successfully - waiting for authentication...")
	}

	// Start timeout watcher (30 seconds for security)
	go func() {
		timer := time.NewTimer(30 * time.Second)
		defer timer.Stop()

		select {
		case <-cancelAuthTimeoutChan:
			// Auth succeeded, timeout cancelled
			log.Println("Authentication timeout cancelled - login successful")
			return
		case <-timer.C:
			log.Println("WARNING: Authentication timeout (30 seconds) - no response from browser")
			log.Println("Please try again or check the logs for errors")
			// UI stays in "Connect" state, user can try again
		}
	}()
}
