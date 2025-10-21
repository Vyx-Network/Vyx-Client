package conn

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Message types (1 byte)
const (
	MsgTypeAuth        = 0
	MsgTypeAuthSuccess = 1
	MsgTypeError       = 2
	MsgTypeConnect     = 3
	MsgTypeConnected   = 4
	MsgTypeData        = 5
	MsgTypeClose       = 6
	MsgTypePing        = 7
	MsgTypePong        = 8
	MsgTypeAddress     = 9
	MsgTypeUIDRegister = 10
)

// BinaryMessage represents a message in binary format (no JSON, no base64)
type BinaryMessage struct {
	Type byte
	ID   string
	Addr string
	Data []byte // Raw bytes instead of base64 string
}

// WriteBinaryMessage writes a message in binary format to a writer
// Format: [1 byte: type][2 bytes: ID len][ID bytes][2 bytes: addr len][addr bytes][4 bytes: data len][data bytes]
func WriteBinaryMessage(w io.Writer, msg *BinaryMessage) error {
	// Write message type
	if err := binary.Write(w, binary.BigEndian, msg.Type); err != nil {
		return fmt.Errorf("failed to write message type: %w", err)
	}

	// Write ID length and ID
	idLen := uint16(len(msg.ID))
	if err := binary.Write(w, binary.BigEndian, idLen); err != nil {
		return fmt.Errorf("failed to write ID length: %w", err)
	}
	if idLen > 0 {
		if _, err := w.Write([]byte(msg.ID)); err != nil {
			return fmt.Errorf("failed to write ID: %w", err)
		}
	}

	// Write Addr length and Addr
	addrLen := uint16(len(msg.Addr))
	if err := binary.Write(w, binary.BigEndian, addrLen); err != nil {
		return fmt.Errorf("failed to write addr length: %w", err)
	}
	if addrLen > 0 {
		if _, err := w.Write([]byte(msg.Addr)); err != nil {
			return fmt.Errorf("failed to write addr: %w", err)
		}
	}

	// Write Data length and Data (raw bytes, no base64!)
	dataLen := uint32(len(msg.Data))
	if err := binary.Write(w, binary.BigEndian, dataLen); err != nil {
		return fmt.Errorf("failed to write data length: %w", err)
	}
	if dataLen > 0 {
		if _, err := w.Write(msg.Data); err != nil {
			return fmt.Errorf("failed to write data: %w", err)
		}
	}

	return nil
}

// ReadBinaryMessage reads a message in binary format from a reader
func ReadBinaryMessage(r io.Reader) (*BinaryMessage, error) {
	msg := &BinaryMessage{}

	// Read message type
	if err := binary.Read(r, binary.BigEndian, &msg.Type); err != nil {
		return nil, fmt.Errorf("failed to read message type: %w", err)
	}

	// Read ID length and ID
	var idLen uint16
	if err := binary.Read(r, binary.BigEndian, &idLen); err != nil {
		return nil, fmt.Errorf("failed to read ID length: %w", err)
	}
	if idLen > 0 {
		idBytes := make([]byte, idLen)
		if _, err := io.ReadFull(r, idBytes); err != nil {
			return nil, fmt.Errorf("failed to read ID: %w", err)
		}
		msg.ID = string(idBytes)
	}

	// Read Addr length and Addr
	var addrLen uint16
	if err := binary.Read(r, binary.BigEndian, &addrLen); err != nil {
		return nil, fmt.Errorf("failed to read addr length: %w", err)
	}
	if addrLen > 0 {
		addrBytes := make([]byte, addrLen)
		if _, err := io.ReadFull(r, addrBytes); err != nil {
			return nil, fmt.Errorf("failed to read addr: %w", err)
		}
		msg.Addr = string(addrBytes)
	}

	// Read Data length and Data
	var dataLen uint32
	if err := binary.Read(r, binary.BigEndian, &dataLen); err != nil {
		return nil, fmt.Errorf("failed to read data length: %w", err)
	}
	if dataLen > 0 {
		msg.Data = make([]byte, dataLen)
		if _, err := io.ReadFull(r, msg.Data); err != nil {
			return nil, fmt.Errorf("failed to read data: %w", err)
		}
	}

	return msg, nil
}

// Helper: Convert old Message to BinaryMessage
func MessageToBinary(m *Message) *BinaryMessage {
	bm := &BinaryMessage{
		ID:   m.ID,
		Addr: m.Addr,
	}

	// Map type string to byte
	switch m.Type {
	case "auth":
		bm.Type = MsgTypeAuth
	case "auth_success":
		bm.Type = MsgTypeAuthSuccess
	case "error":
		bm.Type = MsgTypeError
	case "connect":
		bm.Type = MsgTypeConnect
	case "connected":
		bm.Type = MsgTypeConnected
	case "data":
		bm.Type = MsgTypeData
		// Data is already base64 in old format, keep as string for now
		bm.Data = []byte(m.Data)
	case "close":
		bm.Type = MsgTypeClose
	case "ping":
		bm.Type = MsgTypePing
	case "pong":
		bm.Type = MsgTypePong
	case "address":
		bm.Type = MsgTypeAddress
	case "uid-register":
		bm.Type = MsgTypeUIDRegister
	}

	return bm
}

// Helper: Convert BinaryMessage to old Message format
func BinaryToMessage(bm *BinaryMessage) *Message {
	m := &Message{
		ID:   bm.ID,
		Addr: bm.Addr,
		Data: string(bm.Data),
	}

	// Map byte to type string
	switch bm.Type {
	case MsgTypeAuth:
		m.Type = "auth"
	case MsgTypeAuthSuccess:
		m.Type = "auth_success"
	case MsgTypeError:
		m.Type = "error"
	case MsgTypeConnect:
		m.Type = "connect"
	case MsgTypeConnected:
		m.Type = "connected"
	case MsgTypeData:
		m.Type = "data"
	case MsgTypeClose:
		m.Type = "close"
	case MsgTypePing:
		m.Type = "ping"
	case MsgTypePong:
		m.Type = "pong"
	case MsgTypeAddress:
		m.Type = "address"
	case MsgTypeUIDRegister:
		m.Type = "uid-register"
	}

	return m
}
