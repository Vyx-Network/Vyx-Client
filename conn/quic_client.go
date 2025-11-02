package conn

import (
	"client/config"
	"client/logger"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
)

type Message struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Addr string `json:"addr,omitempty"`
	Data string `json:"data,omitempty"`
}

type Connection struct {
	conn     net.Conn
	dataChan chan []byte
}

var (
	quicConn            *quic.Conn
	quicStream          *quic.Stream
	quicMutex           sync.Mutex
	clientConns         = make(map[string]*Connection)
	clientMutex         sync.RWMutex        // Changed to RWMutex for better read performance
	shouldAutoReconnect bool         = true // Controls whether client should auto-reconnect
	autoReconnectMutex  sync.RWMutex
)

/* Retry Strategy:
- Attempt 1: Immediate (no delay)
- Attempts 2-4: 5 seconds (quick recovery)
- Attempts 5-7: 15 seconds (network stabilization)
- Attempts 8-10: 30 seconds (exponential backoff)
- Attempts 11+: 60 seconds (long-term retry)
- Max retry interval: 5 minutes (for persistent failures)

Special cases:
- Not logged in: 30 second intervals (avoid spam)
- Auth failures: 60 second intervals (credential issues)
*/

// buildTLSConfig creates TLS configuration based on server address
func buildTLSConfig(serverAddr string) *tls.Config {
	config := &tls.Config{
		NextProtos: []string{"vyx-proxy"},
		MinVersion: tls.VersionTLS12, // Minimum TLS 1.2 for security
	}

	// Extract hostname from address
	host := serverAddr
	if strings.Contains(serverAddr, ":") {
		host, _, _ = net.SplitHostPort(serverAddr)
	}

	// Development mode: localhost or 127.0.0.1
	if host == "localhost" || host == "127.0.0.1" {
		log.Println("Development mode: Using InsecureSkipVerify for localhost")
		config.InsecureSkipVerify = true
	} else {
		// Production mode: Enable proper certificate verification
		log.Printf("Production mode: Verifying TLS certificate for %s", host)
		config.ServerName = host
		config.InsecureSkipVerify = false
	}

	return config
}

// getRetryDelay calculates retry delay based on attempt count with exponential backoff
func getRetryDelay(attempt int, authFailed bool, notLoggedIn bool) time.Duration {
	// Special case: Not logged in - use longer delay to avoid spam
	if notLoggedIn {
		return 30 * time.Second
	}

	// Special case: Auth failed - likely credential issue, use longer delay
	if authFailed {
		return 60 * time.Second
	}

	// Progressive backoff strategy
	switch {
	case attempt == 1:
		return 0 // Immediate first retry
	case attempt <= 4:
		return 5 * time.Second // Quick recovery attempts
	case attempt <= 7:
		return 15 * time.Second // Network stabilization
	case attempt <= 10:
		return 30 * time.Second // Exponential backoff
	case attempt <= 15:
		return 60 * time.Second // Long-term retry
	default:
		return 5 * time.Minute // Max retry interval for persistent failures
	}
}

func ConnectQuicServer() {
	connectionAttempts := 0
	consecutiveAuthFailures := 0
	lastConnectionSuccessful := false

	for {
		// Check if auto-reconnect is disabled (user clicked "Stop Sharing")
		autoReconnectMutex.RLock()
		autoReconnect := shouldAutoReconnect
		autoReconnectMutex.RUnlock()

		if !autoReconnect {
			// User has disabled auto-reconnect, wait before checking again
			logger.GetStatus().UpdateStatus("Stopped")
			time.Sleep(5 * time.Second)
			continue
		}

		ctx := context.Background()

		// Determine server address using smart discovery
		var serverAddr string
		var apiURL string

		// DEBUG MODE: Use localhost servers for local development
		if config.GlobalConfig.DebugMode {
			serverAddr = "127.0.0.1:8443"
			apiURL = "http://127.0.0.1:8080"
			log.Printf("DEBUG MODE: Using localhost server (QUIC: %s, API: %s)", serverAddr, apiURL)
		} else {
			// PRODUCTION MODE: Use configured servers
			apiURL = config.GlobalConfig.ServerURL
			if apiURL == "" {
				apiURL = "https://vyx.network"
			} else if !strings.HasPrefix(apiURL, "http://") && !strings.HasPrefix(apiURL, "https://") {
				// Add https:// if no protocol specified
				apiURL = "https://" + apiURL
			}

			// Get optimal server address
			// Try API discovery first, fallback to US server (closer to Asia)
			serverAddr = GetOptimalServer(apiURL, "us.vyx.network:8443")
		}

		// Log connection attempt with attempt number
		if connectionAttempts > 0 {
			log.Printf("Connection attempt #%d to server: %s", connectionAttempts+1, serverAddr)
		} else {
			log.Printf("Using server: %s", serverAddr)
		}

		// Build TLS config based on environment (dev vs production)
		tlsConf := buildTLSConfig(serverAddr)

		// Configure QUIC with longer timeouts for stable connections
		// PERFORMANCE: Tuned for high-latency (200ms RTT) connections to server
		quicConfig := &quic.Config{
			MaxIdleTimeout:                 15 * time.Minute, // Keep connections alive for 15 minutes idle
			KeepAlivePeriod:                30 * time.Second, // Send keepalive every 30 seconds
			InitialStreamReceiveWindow:     4 * 1024 * 1024,  // 4 MB initial stream window (high BDP)
			MaxStreamReceiveWindow:         16 * 1024 * 1024, // 16 MB max stream window
			InitialConnectionReceiveWindow: 8 * 1024 * 1024,  // 8 MB initial connection window
			MaxConnectionReceiveWindow:     32 * 1024 * 1024, // 32 MB max connection window
		}

		conn, err := quic.DialAddr(ctx, serverAddr, tlsConf, quicConfig)
		if err != nil {
			log.Printf("Failed to connect to QUIC server: %v", err)
			logger.GetStatus().UpdateStatus(fmt.Sprintf("Connection failed (attempt %d)", connectionAttempts+1))

			// Calculate retry delay
			retryDelay := getRetryDelay(connectionAttempts+1, false, false)
			log.Printf("Retrying in %v...", retryDelay)

			time.Sleep(retryDelay)
			connectionAttempts++
			continue
		}

		log.Println("Connected to QUIC server")
		logger.GetStatus().UpdateStatus("Connected")
		logger.GetStatus().ServerAddress = serverAddr

		// let the server accept our bidirectional stream and register us
		time.Sleep(100 * time.Millisecond)

		stream, err := conn.OpenStreamSync(ctx)
		if err != nil {
			log.Printf("Failed to open QUIC stream: %v", err)
			logger.GetStatus().UpdateStatus("Stream failed")
			conn.CloseWithError(1, "failed to open stream")

			retryDelay := getRetryDelay(connectionAttempts+1, false, false)
			log.Printf("Retrying in %v...", retryDelay)
			time.Sleep(retryDelay)
			connectionAttempts++
			continue
		}

		quicMutex.Lock()
		quicConn = conn
		quicStream = stream
		quicMutex.Unlock()

		// Authenticate with server
		authResult := authenticateWithServer(stream)

		if !authResult {
			consecutiveAuthFailures++
			log.Printf("Authentication failed (failure #%d)", consecutiveAuthFailures)

			// Check if not logged in
			notLoggedIn := !config.IsLoggedIn()
			if notLoggedIn {
				logger.GetStatus().UpdateStatus("Not logged in - Click 'Connect' to authenticate")
				log.Println("Not logged in. Waiting for user authentication...")
			} else {
				logger.GetStatus().UpdateStatus("Authentication failed")
				log.Println("Authentication failed. Check credentials or API token.")
			}

			conn.CloseWithError(1, "authentication failed")

			// Use appropriate retry delay
			retryDelay := getRetryDelay(connectionAttempts+1, true, notLoggedIn)
			log.Printf("Retrying in %v...", retryDelay)
			time.Sleep(retryDelay)
			connectionAttempts++
			continue
		}

		// Reset counters on successful auth
		connectionAttempts = 0
		consecutiveAuthFailures = 0
		lastConnectionSuccessful = true

		log.Println("Successfully authenticated with server")
		logger.GetStatus().UpdateStatus("Running")
		logger.GetStatus().IsAuthenticated = true
		logger.GetStatus().ConnectionUptime = time.Now()

		// Run the reader (blocks until connection closes)
		quicReader(stream)

		// Connection closed - prepare to reconnect
		log.Println("QUIC connection closed, reconnecting...")
		logger.GetStatus().UpdateStatus("Reconnecting...")
		logger.GetStatus().IsAuthenticated = false
		logger.GetStatus().ConnectionUptime = time.Time{}

		// If we had a successful connection before, use quick retry
		// Otherwise use progressive backoff
		if lastConnectionSuccessful {
			log.Println("Previous connection was successful, attempting quick reconnect...")
			time.Sleep(2 * time.Second)
			lastConnectionSuccessful = false
		} else {
			retryDelay := getRetryDelay(1, false, false)
			log.Printf("Reconnecting in %v...", retryDelay)
			time.Sleep(retryDelay)
		}
	}
}

func quicReader(stream *quic.Stream) {
	decoder := json.NewDecoder(stream)
	messageCount := 0
	lastMessageTime := time.Now()

	// Start connection health monitor
	healthTicker := time.NewTicker(30 * time.Second)
	defer healthTicker.Stop()

	// Monitor channel for health checks
	healthChan := make(chan bool, 1)

	// Health monitor goroutine
	go func() {
		for range healthTicker.C {
			timeSinceLastMessage := time.Since(lastMessageTime)

			// If no messages for 3 minutes, log warning
			if timeSinceLastMessage > 3*time.Minute {
				log.Printf("Warning: No messages received for %v (connection may be stale)", timeSinceLastMessage)
			}

			// If no messages for 10 minutes, consider connection dead
			if timeSinceLastMessage > 10*time.Minute {
				log.Printf("Connection appears dead (no messages for %v), triggering reconnect", timeSinceLastMessage)
				healthChan <- false
				return
			}
		}
	}()

	for {
		select {
		case <-healthChan:
			// Health check failed, close connection
			log.Println("Health check failed, closing connection")
			clientMutex.Lock()
			for id, cc := range clientConns {
				cc.conn.Close()
				close(cc.dataChan)
				delete(clientConns, id)
			}
			clientMutex.Unlock()
			return

		default:
			// Set read deadline to avoid blocking forever
			stream.SetReadDeadline(time.Now().Add(60 * time.Second))

			var msg Message
			err := decoder.Decode(&msg)

			if err != nil {
				// Check if it's a timeout (expected during idle periods)
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					// Timeout is normal during idle, just continue
					continue
				}

				// Real error occurred
				log.Printf("QUIC read error: %v", err)
				logger.GetStatus().UpdateStatus("Connection lost")

				// Clean up all client connections
				clientMutex.Lock()
				for id, cc := range clientConns {
					cc.conn.Close()
					close(cc.dataChan)
					delete(clientConns, id)
				}
				clientMutex.Unlock()

				return
			}

			// Update health tracking
			messageCount++
			lastMessageTime = time.Now()

			// Privacy: Don't log message types or destination addresses
			// log.Printf("received %+v", msg.Type)

			switch msg.Type {
			case "connect":
				// Privacy: Don't log destination addresses to protect proxy user privacy
				// log.Println("to-to ", msg.Addr)
				go handleConnect(msg)
			case "data":
				clientMutex.RLock()
				if cc, ok := clientConns[msg.ID]; ok {
					if data, err := base64.StdEncoding.DecodeString(msg.Data); err == nil {
						select {
						case cc.dataChan <- data:
							// Successfully sent data
						default:
							// Channel full, log warning
							log.Printf("Warning: Data channel full for connection %s", msg.ID)
						}
					}
				}
				clientMutex.RUnlock()
			case "close":
				clientMutex.Lock() // Write lock needed for delete
				if cc, ok := clientConns[msg.ID]; ok {
					cc.conn.Close()
					close(cc.dataChan)
					delete(clientConns, msg.ID)
				}
				clientMutex.Unlock()
			case "ping":
				err := sendMessage(&Message{
					Type: "pong",
					ID:   msg.ID,
				})
				if err != nil {
					log.Printf("Error sending pong: %v", err)
					return // Exit reader, will trigger reconnect
				}
			default:
				log.Printf("Warning: Unknown message type: %s", msg.Type)
			}
		}
	}
}

func sendMessage(msg *Message) error {
	quicMutex.Lock()
	defer quicMutex.Unlock()

	if quicStream == nil {
		log.Println("Cannot send message: no active QUIC stream")
		return fmt.Errorf("no active QUIC stream")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal message of type %s: %v", msg.Type, err)
		return err
	}
	data = append(data, '\n')

	_, err = quicStream.Write(data)
	if err != nil {
		log.Printf("Error writing to QUIC stream: %v", err)
		return err
	}

	return nil
}

func sendCloseMessage(id string) {
	msg := Message{Type: "close", ID: id}
	sendMessage(&msg)
	clientMutex.Lock()
	if cc, ok := clientConns[id]; ok {
		cc.conn.Close()
		close(cc.dataChan)
		delete(clientConns, id)
	}
	clientMutex.Unlock()
}

// DisconnectQuic closes the QUIC connection and disables auto-reconnect
// Used when user clicks "Stop Sharing" or logs out
func DisconnectQuic() {
	// Disable auto-reconnect first
	autoReconnectMutex.Lock()
	shouldAutoReconnect = false
	autoReconnectMutex.Unlock()

	quicMutex.Lock()
	defer quicMutex.Unlock()

	if quicConn != nil {
		quicConn.CloseWithError(0, "user stopped sharing")
		quicConn = nil
	}

	if quicStream != nil {
		quicStream.Close()
		quicStream = nil
	}

	// Close all client connections
	clientMutex.Lock()
	for id, cc := range clientConns {
		cc.conn.Close()
		close(cc.dataChan)
		delete(clientConns, id)
	}
	clientMutex.Unlock()
}

// authenticateWithServer sends authentication credentials to server
func authenticateWithServer(stream *quic.Stream) bool {
	// Reload config if it's nil
	if config.GlobalConfig == nil {
		log.Println("Config is nil, reloading...")
		cfg, err := config.LoadConfig()
		if err != nil {
			log.Printf("Failed to reload config: %v", err)
			return false
		}
		log.Printf("Config reloaded - IsLoggedIn: %v, Email: %s", config.IsLoggedIn(), cfg.Email)
	}

	// Check if user is logged in
	if !config.IsLoggedIn() {
		log.Println("ERROR: Not logged in. Please login via the system tray menu.")
		log.Println("Click 'Connect' in the system tray to authenticate.")
		return false
	}

	// Create client metadata
	metadata := map[string]string{
		"client_type":    "desktop",
		"os":             getOSName(),
		"os_version":     getOSVersion(),
		"client_version": "1.0.0",
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		log.Printf("Failed to marshal metadata: %v", err)
		metadataJSON = []byte("{}")
	}

	// Send authentication message
	authMsg := Message{
		Type: "auth",
		ID:   config.GlobalConfig.APIToken,
		Data: string(metadataJSON),
	}

	log.Printf("Sending auth message with token: %s...", config.GlobalConfig.APIToken[:min(10, len(config.GlobalConfig.APIToken))])
	encoder := json.NewEncoder(stream)
	if err := encoder.Encode(authMsg); err != nil {
		log.Printf("Failed to send authentication: %v", err)
		return false
	}
	log.Println("Auth message sent, waiting for response...")

	// Wait for response (timeout after 10 seconds)
	responseChan := make(chan Message, 1)
	errorChan := make(chan error, 1)

	go func() {
		decoder := json.NewDecoder(stream)
		var response Message
		if err := decoder.Decode(&response); err != nil {
			errorChan <- err
			return
		}
		responseChan <- response
	}()

	select {
	case response := <-responseChan:
		log.Printf("Received response type: %s", response.Type)
		if response.Type == "auth_success" {
			log.Printf("Authenticated as: %s", response.Data)
			return true
		}
		if response.Type == "error" {
			log.Printf("Authentication error: %s", response.Data)
			return false
		}
		log.Printf("Unexpected response type: %s, Data: %s", response.Type, response.Data)
		return false
	case err := <-errorChan:
		log.Printf("Failed to read auth response: %v", err)
		return false
	case <-time.After(10 * time.Second):
		log.Println("Authentication timeout")
		return false
	}
}

// getOSName returns a human-readable OS name
func getOSName() string {
	switch runtime.GOOS {
	case "windows":
		return "windows"
	case "darwin":
		return "mac"
	case "linux":
		return "linux"
	default:
		return runtime.GOOS
	}
}

// getOSVersion returns the OS version string
func getOSVersion() string {
	switch runtime.GOOS {
	case "windows":
		// Could query Windows version via WMI, but simplified for now
		return "Windows"
	case "darwin":
		return "macOS"
	case "linux":
		return "Linux"
	default:
		return runtime.GOOS
	}
}
