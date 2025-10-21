package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

var (
	logFile      *os.File
	IsGUIMode    bool
	statusLogger *StatusLogger
)

// StatusLogger tracks application status for display in system tray
type StatusLogger struct {
	Status           string
	LastUpdate       time.Time
	ActiveConns      int
	TotalDataSent    uint64
	TotalDataRecv    uint64
	Errors           []string
	IsAuthenticated  bool
	ServerAddress    string
	ConnectionUptime time.Time
}

// NewStatusLogger creates a new status logger
func NewStatusLogger() *StatusLogger {
	return &StatusLogger{
		Status:      "Starting...",
		LastUpdate:  time.Now(),
		Errors:      make([]string, 0, 10),
		ActiveConns: 0,
	}
}

// GetStatus returns current status logger
func GetStatus() *StatusLogger {
	if statusLogger == nil {
		statusLogger = NewStatusLogger()
	}
	return statusLogger
}

// UpdateStatus updates the current status
func (s *StatusLogger) UpdateStatus(status string) {
	s.Status = status
	s.LastUpdate = time.Now()
}

// AddError adds an error to the error log (keeps last 10)
func (s *StatusLogger) AddError(err string) {
	s.Errors = append(s.Errors, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), err))
	if len(s.Errors) > 10 {
		s.Errors = s.Errors[1:]
	}
}

// GetStatusText returns formatted status text for tray display
func (s *StatusLogger) GetStatusText() string {
	uptime := "N/A"
	if !s.ConnectionUptime.IsZero() {
		uptime = time.Since(s.ConnectionUptime).Round(time.Second).String()
	}

	dataStr := ""
	if s.TotalDataSent > 0 || s.TotalDataRecv > 0 {
		dataStr = fmt.Sprintf("\nData: ↑%s ↓%s",
			formatBytes(s.TotalDataSent),
			formatBytes(s.TotalDataRecv))
	}

	return fmt.Sprintf("Status: %s\nUptime: %s\nConnections: %d%s",
		s.Status, uptime, s.ActiveConns, dataStr)
}

// formatBytes formats bytes into human-readable format
func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// InitLogger initializes logging to file (for GUI mode) or stdout (for console mode)
func InitLogger(guiMode bool) error {
	IsGUIMode = guiMode
	statusLogger = NewStatusLogger()

	if guiMode {
		// GUI mode: Log to file
		logDir := getLogDirectory()
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return fmt.Errorf("failed to create log directory: %w", err)
		}

		logPath := filepath.Join(logDir, fmt.Sprintf("vyx-%s.log", time.Now().Format("2006-01-02")))
		file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}

		logFile = file

		// Set log output to file
		log.SetOutput(file)
		log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

		log.Printf("=== Vyx Client Started (GUI Mode) ===")
		log.Printf("Log file: %s", logPath)
	} else {
		// Console mode: Keep stdout logging
		log.SetOutput(os.Stdout)
		log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
		log.Println("=== Vyx Client Started (Console Mode) ===")
	}

	return nil
}

// getLogDirectory returns the appropriate log directory for the OS
func getLogDirectory() string {
	var logDir string
	switch runtime.GOOS {
	case "windows":
		logDir = filepath.Join(os.Getenv("APPDATA"), "Vyx", "logs")
	case "darwin":
		homeDir, _ := os.UserHomeDir()
		logDir = filepath.Join(homeDir, "Library", "Logs", "Vyx")
	default: // linux
		homeDir, _ := os.UserHomeDir()
		logDir = filepath.Join(homeDir, ".vyx", "logs")
	}
	return logDir
}

// Info logs an info message and updates status
func Info(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	log.Println(msg)
	if statusLogger != nil {
		statusLogger.UpdateStatus(extractStatus(msg))
	}
}

// Error logs an error message and adds to error list
func Error(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	log.Printf("ERROR: %s", msg)
	if statusLogger != nil {
		statusLogger.AddError(msg)
	}
}

// Debug logs a debug message (only to file, not status)
func Debug(format string, v ...interface{}) {
	log.Printf("DEBUG: "+format, v...)
}

// extractStatus extracts status from log message
func extractStatus(msg string) string {
	// Simple status extraction - can be enhanced
	switch {
	case contains(msg, "Connected to QUIC server"):
		return "Connected"
	case contains(msg, "Successfully authenticated"):
		return "Running"
	case contains(msg, "Failed to connect"):
		return "Connection Failed"
	case contains(msg, "Authentication failed"):
		return "Auth Failed"
	case contains(msg, "Disconnected"):
		return "Disconnected"
	case contains(msg, "reconnecting"):
		return "Reconnecting..."
	default:
		return msg
	}
}

// contains checks if string contains substring (case-insensitive helper)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Close closes the log file
func Close() {
	if logFile != nil {
		log.Println("=== Vyx Client Stopped ===")
		logFile.Close()
	}
}

// GetLogPath returns the current log file path
func GetLogPath() string {
	if logFile != nil {
		return logFile.Name()
	}
	return ""
}

// TailLogs returns the last N lines from the log file
func TailLogs(n int) ([]string, error) {
	if logFile == nil {
		return nil, fmt.Errorf("no log file open")
	}

	// Reopen file for reading
	file, err := os.Open(logFile.Name())
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Read all lines
	content, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	lines := make([]string, 0, n)
	start := 0
	for i, b := range content {
		if b == '\n' {
			lines = append(lines, string(content[start:i]))
			start = i + 1
		}
	}
	if start < len(content) {
		lines = append(lines, string(content[start:]))
	}

	// Return last N lines
	if len(lines) > n {
		return lines[len(lines)-n:], nil
	}
	return lines, nil
}
