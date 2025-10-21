package config

import (
	"errors"
	"fmt"
	"log"

	"github.com/zalando/go-keyring"
)

const (
	// KeyringService is the application identifier for the OS keyring
	KeyringService = "vyx-proxy-client"
	// KeyringTokenKey is the key used to store the API token
	KeyringTokenKey = "api-token"
)

// SecureStorage provides cross-platform secure credential storage
// Uses OS-specific keyrings:
// - Windows: Windows Credential Manager
// - macOS: Keychain
// - Linux: Secret Service API (gnome-keyring, kwallet)
type SecureStorage struct {
	service string
	userID  string
}

// NewSecureStorage creates a new secure storage instance
// userID is used as the account identifier in the keyring
func NewSecureStorage(userID string) *SecureStorage {
	if userID == "" {
		userID = "default-user"
	}
	return &SecureStorage{
		service: KeyringService,
		userID:  userID,
	}
}

// SaveToken securely stores the API token in the OS keyring
func (s *SecureStorage) SaveToken(token string) error {
	if token == "" {
		return errors.New("token cannot be empty")
	}

	err := keyring.Set(s.service, s.userID, token)
	if err != nil {
		return fmt.Errorf("failed to save token to secure storage: %w", err)
	}

	log.Printf("Token securely saved for user: %s", s.userID)
	return nil
}

// GetToken retrieves the API token from the OS keyring
func (s *SecureStorage) GetToken() (string, error) {
	token, err := keyring.Get(s.service, s.userID)
	if err != nil {
		// Check if token doesn't exist (common case for new installations)
		if errors.Is(err, keyring.ErrNotFound) {
			return "", fmt.Errorf("no token found in secure storage (user: %s)", s.userID)
		}
		return "", fmt.Errorf("failed to retrieve token from secure storage: %w", err)
	}

	if token == "" {
		return "", errors.New("retrieved token is empty")
	}

	return token, nil
}

// DeleteToken removes the API token from the OS keyring
func (s *SecureStorage) DeleteToken() error {
	err := keyring.Delete(s.service, s.userID)
	if err != nil {
		// Ignore error if token doesn't exist
		if errors.Is(err, keyring.ErrNotFound) {
			log.Printf("Token not found in secure storage (user: %s), nothing to delete", s.userID)
			return nil
		}
		return fmt.Errorf("failed to delete token from secure storage: %w", err)
	}

	log.Printf("Token securely deleted for user: %s", s.userID)
	return nil
}

// MigrateFromPlaintextConfig migrates an existing plaintext token to secure storage
// This is used for backward compatibility when upgrading from insecure storage
func MigrateFromPlaintextConfig(plaintextToken, userID string) error {
	if plaintextToken == "" {
		return errors.New("no plaintext token to migrate")
	}

	storage := NewSecureStorage(userID)

	// Save token to secure storage
	if err := storage.SaveToken(plaintextToken); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	log.Printf("Successfully migrated token to secure storage for user: %s", userID)
	return nil
}

// HasToken checks if a token exists in secure storage without retrieving it
func (s *SecureStorage) HasToken() bool {
	_, err := s.GetToken()
	return err == nil
}
