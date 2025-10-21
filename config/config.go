package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

type Config struct {
	ServerURL string `json:"server_url"`
	// SECURITY: APIToken is stored in OS keyring, not in JSON file
	APIToken string `json:"-"` // json:"-" excludes from JSON serialization
	UserID   string `json:"user_id,omitempty"`
	Email    string `json:"email,omitempty"`
	// PRIVACY: VerboseLogging enables detailed connection logs (default: false)
	// When false, destination addresses are not logged to protect proxy user privacy
	VerboseLogging bool `json:"verbose_logging,omitempty"`
	// AutoStart controls whether the app starts on system boot (default: true)
	AutoStart *bool `json:"auto_start,omitempty"` // Use pointer to distinguish between false and unset
}

var GlobalConfig *Config

// LoadConfig reads configuration from config.json and retrieves token from secure storage
func LoadConfig() (*Config, error) {
	configPath := getConfigPath()

	// Create default config if doesn't exist
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		defaultConfig := &Config{
			ServerURL: "proxy.vyx.network",
		}
		SaveConfig(defaultConfig)
		GlobalConfig = defaultConfig
		return defaultConfig, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// SECURITY MIGRATION: Check for legacy plaintext token in JSON
	// This handles migration from old insecure storage to secure keyring
	var legacyConfig struct {
		APIToken string `json:"api_token"`
	}
	if err := json.Unmarshal(data, &legacyConfig); err == nil && legacyConfig.APIToken != "" {
		log.Println("SECURITY: Migrating plaintext token to secure storage...")
		if config.UserID != "" {
			if err := MigrateFromPlaintextConfig(legacyConfig.APIToken, config.UserID); err != nil {
				log.Printf("Warning: Failed to migrate token to secure storage: %v", err)
			} else {
				// Migration successful - re-save config without plaintext token
				config.APIToken = legacyConfig.APIToken // Temporarily set for SaveConfig
				if err := SaveConfig(&config); err != nil {
					log.Printf("Warning: Failed to save config after migration: %v", err)
				}
			}
		}
	}

	// Retrieve token from secure storage (if user is logged in)
	if config.UserID != "" {
		storage := NewSecureStorage(config.UserID)
		token, err := storage.GetToken()
		if err == nil {
			config.APIToken = token
		} else {
			// Token not found in keyring - user needs to login again
			log.Printf("No token found in secure storage for user %s", config.UserID)
		}
	}

	GlobalConfig = &config
	return &config, nil
}

// SaveConfig writes configuration to config.json and stores token in secure storage
func SaveConfig(config *Config) error {
	configPath := getConfigPath()

	// Create config directory if it doesn't exist
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	// SECURITY: Save token to secure storage (OS keyring) if present
	if config.APIToken != "" && config.UserID != "" {
		storage := NewSecureStorage(config.UserID)
		if err := storage.SaveToken(config.APIToken); err != nil {
			log.Printf("Warning: Failed to save token to secure storage: %v", err)
			// Continue anyway to save other config data
		}
	}

	// Marshal config to JSON (APIToken excluded due to json:"-" tag)
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	// SECURITY: Use 0600 permissions (read/write for owner only, not world-readable)
	// Changed from 0644 to prevent other users from reading config file
	return os.WriteFile(configPath, data, 0600)
}

// getConfigPath returns the path to config.json
func getConfigPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".vyx", "config.json")
}

// IsLoggedIn checks if user is authenticated by verifying token in secure storage
func IsLoggedIn() bool {
	if GlobalConfig == nil || GlobalConfig.UserID == "" {
		return false
	}

	// Check in-memory token first (already loaded)
	if GlobalConfig.APIToken != "" {
		return true
	}

	// Check secure storage as fallback
	storage := NewSecureStorage(GlobalConfig.UserID)
	return storage.HasToken()
}

// ClearAuthToken removes the authentication token from secure storage
// This should be called during logout
func ClearAuthToken() error {
	if GlobalConfig == nil || GlobalConfig.UserID == "" {
		return nil // Nothing to clear
	}

	storage := NewSecureStorage(GlobalConfig.UserID)
	if err := storage.DeleteToken(); err != nil {
		return err
	}

	// Clear in-memory token as well
	GlobalConfig.APIToken = ""
	return nil
}

// GetAutoStartEnabled returns the autostart preference (default: true)
func GetAutoStartEnabled() bool {
	if GlobalConfig == nil || GlobalConfig.AutoStart == nil {
		return true // Default to enabled
	}
	return *GlobalConfig.AutoStart
}

// SetAutoStartEnabled sets the autostart preference
func SetAutoStartEnabled(enabled bool) error {
	if GlobalConfig == nil {
		return fmt.Errorf("config not initialized")
	}

	GlobalConfig.AutoStart = &enabled
	return SaveConfig(GlobalConfig)
}
