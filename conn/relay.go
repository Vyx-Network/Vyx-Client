package conn

import (
	"encoding/base64"
	"log"
)

func relayFromConnToQuic(cc *Connection, id string) {
	defer func() {
		// Ensure cleanup on exit
		if r := recover(); r != nil {
			log.Printf("Panic in relayFromConnToQuic for connection %s: %v", id, r)
		}
		sendCloseMessage(id)
	}()

	// PERFORMANCE: Larger buffers for high-latency links (200ms RTT to server)
	buf := make([]byte, 256*1024) // 256 KB for high BDP networks

	for {
		n, err := cc.conn.Read(buf)
		if err != nil {
			// Connection closed or error, exit gracefully
			return
		}

		if n == 0 {
			// No data read, continue
			continue
		}

		data := base64.StdEncoding.EncodeToString(buf[:n])
		msg := Message{Type: "data", ID: id, Data: data}

		err = sendMessage(&msg)
		if err != nil {
			// Failed to send, connection to server likely lost
			log.Printf("Failed to relay data from client connection %s: %v", id, err)
			return
		}
	}
}

func relayFromChanToConn(cc *Connection, id string) {
	defer func() {
		// Ensure cleanup on exit
		if r := recover(); r != nil {
			log.Printf("Panic in relayFromChanToConn for connection %s: %v", id, r)
		}
		sendCloseMessage(id)
	}()

	for data := range cc.dataChan {
		if len(data) == 0 {
			continue
		}

		_, err := cc.conn.Write(data)
		if err != nil {
			// Connection closed or error, exit gracefully
			return
		}
	}
}
