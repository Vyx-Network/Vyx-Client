package conn

import "log"

// ReconnectQuic forces a reconnection to the QUIC server and enables auto-reconnect
// Used when user clicks "Start Sharing" or logs in
func ReconnectQuic() {
	log.Println("Enabling bandwidth sharing...")

	// Enable auto-reconnect first
	autoReconnectMutex.Lock()
	shouldAutoReconnect = true
	autoReconnectMutex.Unlock()

	// Close existing connection if any
	quicMutex.Lock()
	if quicConn != nil {
		quicConn.CloseWithError(0, "reconnecting")
		quicConn = nil
	}
	if quicStream != nil {
		quicStream.Close()
		quicStream = nil
	}
	quicMutex.Unlock()

	// The ConnectQuicServer goroutine will automatically retry now that auto-reconnect is enabled
	log.Println("Auto-reconnect enabled, will connect shortly...")
}

// IsConnected returns true if currently connected to QUIC server
func IsConnected() bool {
	quicMutex.Lock()
	defer quicMutex.Unlock()
	return quicConn != nil && quicStream != nil
}
