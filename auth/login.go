package auth

import (
	"bytes"
	"client/config"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token        string      `json:"token"`
	RefreshToken string      `json:"refreshToken"`
	User         UserProfile `json:"user"`
}

type UserProfile struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// Login authenticates user and saves credentials
func Login(email, password string) error {
	req := LoginRequest{
		Email:    email,
		Password: password,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return err
	}

	resp, err := http.Post("https://api.vyx.network/api/auth/login", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("login failed: %s", string(bodyBytes))
	}

	var authResp AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return err
	}

	// Save to config
	config.GlobalConfig.APIToken = authResp.Token
	config.GlobalConfig.UserID = authResp.User.ID
	config.GlobalConfig.Email = authResp.User.Email

	return config.SaveConfig(config.GlobalConfig)
}

// Register creates a new account
func Register(email, password string) error {
	req := RegisterRequest{
		Email:    email,
		Password: password,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return err
	}

	resp, err := http.Post("https://api.vyx.network/api/auth/register", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("registration failed: %s", string(bodyBytes))
	}

	var authResp AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return err
	}

	// Save to config
	config.GlobalConfig.APIToken = authResp.Token
	config.GlobalConfig.UserID = authResp.User.ID
	config.GlobalConfig.Email = authResp.User.Email

	return config.SaveConfig(config.GlobalConfig)
}

// Logout clears credentials from both memory and secure storage
func Logout() error {
	// SECURITY: Remove token from secure storage (OS keyring)
	if err := config.ClearAuthToken(); err != nil {
		return fmt.Errorf("failed to clear authentication token: %w", err)
	}

	// Clear user data from config
	config.GlobalConfig.APIToken = ""
	config.GlobalConfig.UserID = ""
	config.GlobalConfig.Email = ""

	return config.SaveConfig(config.GlobalConfig)
}
