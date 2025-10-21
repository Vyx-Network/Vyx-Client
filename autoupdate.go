package main

import (
	"client/logger"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

const url = "https://api.github.com/repos/Vyx-Network/Vyx-Client/releases/latest"

func AutoUpdate() error {
	logger.Info("Checking for updates (current version: %s)...", VERSION)

	client := http.Client{
		Timeout: 10 * time.Second,
	}

	release, hasUpdate, err := checkForUpdate(client)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			logger.Info("No releases available yet")
			return nil // No release yet
		}
		return fmt.Errorf("checking for updates: %w", err)
	}

	if !hasUpdate {
		logger.Info("You are running the latest version (%s)", VERSION)
		return nil
	}

	logger.Info("Update available: %s → %s", VERSION, release.TagName)

	assetURL, err := findAssetForPlatform(release)
	if err != nil {
		return fmt.Errorf("finding asset url: %w", err)
	}

	logger.Info("Downloading update from: %s", assetURL)
	assetData, err := downloadUpdate(client, assetURL)
	if err != nil {
		return fmt.Errorf("downloading update: %w", err)
	}

	logger.Info("Download complete (%d bytes). Installing update...", len(assetData))

	if err := replaceExecutable(assetData, release.TagName); err != nil {
		return fmt.Errorf("replacing executable: %w", err)
	}

	logger.Info("Update installed successfully! Please restart the application.")
	return nil
}

func checkForUpdate(client http.Client) (*GitHubRelease, bool, error) {
	req, err := http.NewRequest("GET", url, nil)

	if err != nil {
		return nil, false, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("User-Agent", "Vyx-updater/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("fetching release info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, false, fmt.Errorf("decoding release info: %w", err)
	}
	hasUpdate := semver.Compare(release.TagName, VERSION) == +1

	return &release, hasUpdate, nil
}

func findAssetForPlatform(release *GitHubRelease) (string, error) {
	var assetURL string
	for _, asset := range release.Assets {
		assetName := strings.ToLower(asset.Name)

		if strings.Contains(assetName, runtime.GOOS+"-"+runtime.GOARCH) {
			assetURL = asset.BrowserDownloadURL
			break
		}
	}

	if assetURL == "" {
		return "", fmt.Errorf("no suitable asset found for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	return assetURL, nil
}

func downloadUpdate(client http.Client, url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating download request: %w", err)
	}

	req.Header.Set("User-Agent", "Vyx-updater/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading asset: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func replaceExecutable(newExecutable []byte, newVersion string) error {
	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("getting current executable path: %w", err)
	}

	// On Windows, we can't replace a running executable
	// Create a batch script to replace it after exit
	if runtime.GOOS == "windows" {
		return installUpdateWindows(currentExe, newExecutable, newVersion)
	}

	// On Unix systems, we can replace the executable while running
	return installUpdateUnix(currentExe, newExecutable)
}

func installUpdateWindows(currentExe string, newExecutable []byte, newVersion string) error {
	// Create temp directory for update
	tempDir := filepath.Join(os.TempDir(), "vyx-update")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("creating temp directory: %w", err)
	}

	// Write new executable to temp location
	newExePath := filepath.Join(tempDir, "vyx-client-new.exe")
	if err := os.WriteFile(newExePath, newExecutable, 0755); err != nil {
		return fmt.Errorf("writing new executable: %w", err)
	}

	// Create backup path
	backupPath := currentExe + ".backup"

	// Create batch script to replace exe after current process exits
	batchScript := fmt.Sprintf(`@echo off
echo Updating Vyx Client to %s...
timeout /t 2 /nobreak > nul
move /y "%s" "%s"
move /y "%s" "%s"
start "" "%s"
del "%%~f0"
`, newVersion, currentExe, backupPath, newExePath, currentExe, currentExe)

	batchPath := filepath.Join(tempDir, "update.bat")
	if err := os.WriteFile(batchPath, []byte(batchScript), 0755); err != nil {
		return fmt.Errorf("creating update script: %w", err)
	}

	// Launch batch script in background
	cmd := exec.Command("cmd", "/c", "start", "/min", batchPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launching update script: %w", err)
	}

	logger.Info("Update script created. Application will exit and restart with new version.")

	// Exit the application after a short delay
	go func() {
		time.Sleep(1 * time.Second)
		os.Exit(0)
	}()

	return nil
}

func installUpdateUnix(currentExe string, newExecutable []byte) error {
	// Create backup
	backupPath := currentExe + ".backup"
	if err := os.Rename(currentExe, backupPath); err != nil {
		return fmt.Errorf("creating backup: %w", err)
	}

	// Write new executable
	if err := os.WriteFile(currentExe, newExecutable, 0755); err != nil {
		// Restore backup on failure
		os.Rename(backupPath, currentExe)
		return fmt.Errorf("writing new executable: %w", err)
	}

	// Remove backup
	os.Remove(backupPath)

	logger.Info("Update installed. Restart the application to use the new version.")
	return nil
}
