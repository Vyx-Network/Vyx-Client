package conn

import (
	"client/config"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"
)

// ServerInfo represents a QUIC server from the API
type ServerInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Region      string `json:"region"`
	Address     string `json:"address"`
	Status      string `json:"status"`
	Connections struct {
		Current            int64   `json:"current"`
		Maximum            int64   `json:"maximum"`
		Available          int64   `json:"available"`
		UtilizationPercent float64 `json:"utilization_percent"`
	} `json:"connections"`
}

// ServerListResponse is the API response format
type ServerListResponse struct {
	Servers     []ServerInfo `json:"servers"`
	Recommended *struct {
		ServerID string `json:"server_id"`
		Reason   string `json:"reason"`
	} `json:"recommended,omitempty"`
}

// DiscoverServers fetches the list of available servers from the API
func DiscoverServers(apiURL string) ([]ServerInfo, error) {
	// Fetch server list with timeout
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(apiURL + "/api/servers")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch server list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var response ServerListResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(response.Servers) == 0 {
		return nil, fmt.Errorf("no servers available")
	}

	log.Printf("Discovered %d servers from API", len(response.Servers))
	return response.Servers, nil
}

// TestLatency measures latency to a server address (TCP connection probe)
func TestLatency(address string) time.Duration {
	start := time.Now()

	// Extract host from address (e.g., "us.vyx.network:8443" → "us.vyx.network")
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		// If no port, use address as-is
		host = address
	}

	// Test latency to HTTPS port (443) instead of QUIC port (8443)
	// QUIC port is UDP-only, but we need TCP for latency measurement
	testAddr := net.JoinHostPort(host, "443")

	conn, err := net.DialTimeout("tcp", testAddr, 3*time.Second)
	if err != nil {
		// If connection fails, return high latency
		return 5 * time.Second
	}
	defer conn.Close()

	latency := time.Since(start)
	return latency
}

// SelectBestServer chooses the optimal server based on load and latency
func SelectBestServer(servers []ServerInfo) (string, error) {
	if len(servers) == 0 {
		return "", fmt.Errorf("no servers available")
	}

	// Filter out unhealthy servers
	healthy := make([]ServerInfo, 0)
	for _, s := range servers {
		if s.Status == "healthy" {
			healthy = append(healthy, s)
		}
	}

	if len(healthy) == 0 {
		log.Println("Warning: No healthy servers, using all servers")
		healthy = servers
	}

	// If only one server, use it
	if len(healthy) == 1 {
		log.Printf("Selected server: %s (%s) - only available server", healthy[0].Name, healthy[0].Address)
		return healthy[0].Address, nil
	}

	// Test latency to each server and select best combination of low load + low latency
	type serverScore struct {
		server  ServerInfo
		latency time.Duration
		score   float64 // Lower is better
	}

	scores := make([]serverScore, 0, len(healthy))

	for _, server := range healthy {
		// Skip overloaded servers (>90% utilization)
		if server.Connections.UtilizationPercent > 90 {
			log.Printf("Skipping overloaded server: %s (%.1f%% utilization)", server.Name, server.Connections.UtilizationPercent)
			continue
		}

		latency := TestLatency(server.Address)

		// Calculate score: weighted combination of load and latency
		// Load weight: 60%, Latency weight: 40%
		loadScore := server.Connections.UtilizationPercent
		latencyScore := float64(latency.Milliseconds()) / 10.0 // Normalize to 0-100 range

		totalScore := (loadScore * 0.6) + (latencyScore * 0.4)

		scores = append(scores, serverScore{
			server:  server,
			latency: latency,
			score:   totalScore,
		})

		log.Printf("Server %s: load=%.1f%%, latency=%dms, score=%.1f",
			server.Name, loadScore, latency.Milliseconds(), totalScore)
	}

	if len(scores) == 0 {
		// All servers overloaded, use least loaded
		best := healthy[0]
		for _, s := range healthy[1:] {
			if s.Connections.UtilizationPercent < best.Connections.UtilizationPercent {
				best = s
			}
		}
		log.Printf("All servers overloaded, selected least loaded: %s (%.1f%%)", best.Name, best.Connections.UtilizationPercent)
		return best.Address, nil
	}

	// Select server with lowest score
	best := scores[0]
	for _, s := range scores[1:] {
		if s.score < best.score {
			best = s
		}
	}

	log.Printf("Selected best server: %s (%s) - load=%.1f%%, latency=%dms",
		best.server.Name, best.server.Address, best.server.Connections.UtilizationPercent, best.latency.Milliseconds())

	return best.server.Address, nil
}

// GetOptimalServer discovers and selects the best server, with DNS fallback
func GetOptimalServer(apiURL string, fallbackAddr string) string {
	// DEBUG MODE: Skip server discovery and use localhost
	if config.GlobalConfig != nil && config.GlobalConfig.DebugMode {
		debugAddr := "127.0.0.1:8443"
		log.Printf("DEBUG MODE: Skipping server discovery, using localhost: %s", debugAddr)
		return debugAddr
	}

	// Try API-based discovery first
	servers, err := DiscoverServers(apiURL)
	if err != nil {
		log.Printf("Server discovery failed: %v, using fallback: %s", err, fallbackAddr)
		return fallbackAddr
	}

	// Select best server
	bestAddr, err := SelectBestServer(servers)
	if err != nil {
		log.Printf("Failed to select server: %v, using fallback: %s", err, fallbackAddr)
		return fallbackAddr
	}

	return bestAddr
}
