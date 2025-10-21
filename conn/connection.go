package conn

import (
	"context"
	"encoding/base64"
	"log"
	"net"
	"strings"
	"time"
)

// dialWithDNSFallback tries to connect with DNS fallback for better reliability
func dialWithDNSFallback(address string) (net.Conn, error) {
	// First attempt with default DNS (5 second timeout)
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err == nil {
		return conn, nil
	}

	// If DNS resolution failed, try with Google DNS
	if strings.Contains(err.Error(), "no such host") || strings.Contains(err.Error(), "Temporary failure") {
		log.Printf("DNS resolution failed with system DNS, trying with custom resolver...")

		// Use custom DNS resolver (Google DNS 8.8.8.8)
		resolver := &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{Timeout: 3 * time.Second}
				return d.DialContext(ctx, network, "8.8.8.8:53")
			},
		}

		// Extract host and port from address
		host, port, splitErr := net.SplitHostPort(address)
		if splitErr != nil {
			return nil, splitErr
		}

		// Resolve with custom DNS
		ips, resolveErr := resolver.LookupHost(ctx, host)
		if resolveErr != nil {
			return nil, err // Return original error
		}

		if len(ips) > 0 {
			// Try to connect with resolved IP
			resolvedAddr := net.JoinHostPort(ips[0], port)
			return dialer.DialContext(ctx, "tcp", resolvedAddr)
		}
	}

	return nil, err
}

func handleConnect(msg Message) {
	conn, err := dialWithDNSFallback(msg.Addr)
	if err != nil || conn == nil {
		// Privacy: Don't log destination address to protect proxy user privacy
		log.Printf("Failed to establish connection: %v", err)
		sendCloseMessage(msg.ID)
		return
	}

	// Apply TCP optimizations for better performance
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		// PERFORMANCE: Increase buffers for high-latency connections (200ms RTT to server)
		tcpConn.SetReadBuffer(4 * 1024 * 1024)       // 4 MB read buffer for high BDP
		tcpConn.SetWriteBuffer(4 * 1024 * 1024)      // 4 MB write buffer for high BDP
		tcpConn.SetNoDelay(true)                     // Disable Nagle's algorithm for lower latency
		tcpConn.SetKeepAlive(true)                   // Enable TCP keepalive
		tcpConn.SetKeepAlivePeriod(30 * time.Second) // Keepalive every 30 seconds
	}

	dataChan := make(chan []byte, 10000) // Increased from 100 to 10000 for better throughput
	cc := &Connection{conn: conn, dataChan: dataChan}

	clientMutex.Lock()
	clientConns[msg.ID] = cc
	clientMutex.Unlock()

	// Send confirmation to server that connection is established
	confirmMsg := &Message{
		Type: "connected",
		ID:   msg.ID,
		Data: "",
	}
	if err := sendMessage(confirmMsg); err != nil {
		log.Printf("Failed to send connect confirmation: %v", err)
		conn.Close()
		return
	}

	// Write initial data if any
	if msg.Data != "" {
		data, _ := base64.StdEncoding.DecodeString(msg.Data)
		_, err = conn.Write(data)
		if err != nil {
			log.Printf("Failed to write initial data: %v", err)
			sendCloseMessage(msg.ID)
			return
		}
	}

	go relayFromConnToQuic(cc, msg.ID)
	go relayFromChanToConn(cc, msg.ID)
}
