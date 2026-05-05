// Package transport implements the Engine.IO / Socket.IO wire protocol
// on top of a raw WebSocket connection.
//
// # Engine.IO packet types (single-char prefix)
//
//	0 = open    (server handshake JSON)
//	1 = close
//	2 = ping    (server → client)
//	3 = pong    (client → server, response to ping)
//	4 = message (carries a Socket.IO payload)
//
// # Socket.IO packet types (prefix char inside an EIO message "4...")
//
//	0 = CONNECT
//	1 = DISCONNECT
//	2 = EVENT        e.g. 42["monitorList", {...}]
//	3 = ACK          e.g. 430[{"ok":true}]
//	4 = CONNECT_ERROR
//
// # Wire examples captured from Uptime Kuma v2.3.2
//
//	Server → "0{\"sid\":\"...\",\"pingInterval\":25000,...}"   EIO open
//	Server → "40{\"sid\":\"...\"}"                            SIO connect ack
//	Server → "42[\"info\",{...}]"                             SIO event push
//	Server → "430[{\"ok\":true,\"token\":\"...\"}]"           SIO ack (login)
//	Server → "2"                                              EIO ping
//	Client → "3"                                              EIO pong
//	Client → "40"                                             SIO connect
//	Client → "420[\"login\",{...}]"                           SIO event + ack id
package transport

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ── Engine.IO ────────────────────────────────────────────────────────────────

// EIOPacketType is the first character of every Engine.IO frame.
type EIOPacketType byte

const (
	EIOOpen    EIOPacketType = '0'
	EIOClose   EIOPacketType = '1'
	EIOPing    EIOPacketType = '2'
	EIOPong    EIOPacketType = '3'
	EIOMessage EIOPacketType = '4' // carries a Socket.IO payload
)

// EIOPacket is a parsed Engine.IO frame.
type EIOPacket struct {
	Type    EIOPacketType
	Payload string // raw payload after the type byte (may be empty)
}

// ParseEIO parses a raw Engine.IO frame string into an EIOPacket.
// Returns an error if the frame is empty or has an unknown type.
func ParseEIO(raw string) (EIOPacket, error) {
	if len(raw) == 0 {
		return EIOPacket{}, fmt.Errorf("transport: empty EIO frame")
	}
	t := EIOPacketType(raw[0])
	switch t {
	case EIOOpen, EIOClose, EIOPing, EIOPong, EIOMessage:
		return EIOPacket{Type: t, Payload: raw[1:]}, nil
	default:
		return EIOPacket{}, fmt.Errorf("transport: unknown EIO type %q in frame %q", string(t), raw)
	}
}

// EIOHandshake is the JSON payload inside an EIO open packet.
type EIOHandshake struct {
	SID          string   `json:"sid"`
	Upgrades     []string `json:"upgrades"`
	PingInterval int      `json:"pingInterval"` // milliseconds
	PingTimeout  int      `json:"pingTimeout"`  // milliseconds
	MaxPayload   int      `json:"maxPayload"`
}

// ParseHandshake decodes the JSON payload of an EIOOpen packet.
func ParseHandshake(payload string) (EIOHandshake, error) {
	var hs EIOHandshake
	if err := json.Unmarshal([]byte(payload), &hs); err != nil {
		return EIOHandshake{}, fmt.Errorf("transport: invalid handshake JSON: %w", err)
	}
	return hs, nil
}

// ── Socket.IO ────────────────────────────────────────────────────────────────

// SIOPacketType is the first character of the Socket.IO payload
// inside an EIOMessage ("4...") frame.
type SIOPacketType byte

const (
	SIOConnect      SIOPacketType = '0'
	SIODisconnect   SIOPacketType = '1'
	SIOEvent        SIOPacketType = '2'
	SIOAck          SIOPacketType = '3'
	SIOConnectError SIOPacketType = '4'
)

// SIOPacket is a parsed Socket.IO frame.
type SIOPacket struct {
	Type    SIOPacketType
	AckID   int    // -1 if no ACK ID present
	Payload string // raw JSON array string (args), may be empty
}

// ParseSIO parses the EIO message payload (everything after the leading "4")
// into a SIOPacket.
//
// Wire format for an event with ACK:
//
//	"2" + ackID + "[\"eventName\", ...args]"
//	e.g. "20[\"login\",{...}]"
//
// Wire format for an ACK response:
//
//	"3" + ackID + "[...response]"
//	e.g. "30[{\"ok\":true}]"
func ParseSIO(payload string) (SIOPacket, error) {
	if len(payload) == 0 {
		return SIOPacket{}, fmt.Errorf("transport: empty SIO payload")
	}

	t := SIOPacketType(payload[0])
	rest := payload[1:]

	// Extract optional numeric ACK ID before the JSON array
	ackID := -1
	i := 0
	for i < len(rest) && rest[i] >= '0' && rest[i] <= '9' {
		i++
	}
	if i > 0 {
		id, err := strconv.Atoi(rest[:i])
		if err == nil {
			ackID = id
		}
		rest = rest[i:]
	}

	return SIOPacket{
		Type:    t,
		AckID:   ackID,
		Payload: rest,
	}, nil
}

// EventArgs decodes the JSON array payload of a SIOEvent packet into
// the event name and a slice of raw JSON arguments.
//
//	["monitorList", {...}]  →  name="monitorList", args=["{...}"]
func (p SIOPacket) EventArgs() (name string, args []json.RawMessage, err error) {
	if p.Type != SIOEvent && p.Type != SIOAck {
		return "", nil, fmt.Errorf("transport: packet type %q has no event args", string(p.Type))
	}
	if p.Payload == "" {
		return "", nil, nil
	}

	var raw []json.RawMessage
	if err := json.Unmarshal([]byte(p.Payload), &raw); err != nil {
		return "", nil, fmt.Errorf("transport: invalid SIO payload JSON %q: %w", p.Payload, err)
	}
	if len(raw) == 0 {
		return "", nil, nil
	}

	// For ACK packets the payload is already the args array (no name prefix)
	if p.Type == SIOAck {
		return "", raw, nil
	}

	// First element is the event name string
	if err := json.Unmarshal(raw[0], &name); err != nil {
		return "", nil, fmt.Errorf("transport: SIO event name is not a string: %w", err)
	}
	return name, raw[1:], nil
}

// ── Builders ─────────────────────────────────────────────────────────────────

// BuildPong returns the EIO pong frame (response to server ping).
func BuildPong() string { return "3" }

// BuildSIOConnect returns the Socket.IO CONNECT frame.
func BuildSIOConnect() string { return "40" }

// BuildSIOEvent builds a Socket.IO EVENT frame without ACK.
//
//	BuildSIOEvent("addMonitor", payload) → `42["addMonitor",{...}]`
func BuildSIOEvent(name string, args ...any) (string, error) {
	return buildSIOEventFrame("42", name, args...)
}

// BuildSIOEventAck builds a Socket.IO EVENT frame with an ACK ID.
//
//	BuildSIOEventAck(0, "login", payload) → `420["login",{...}]`
func BuildSIOEventAck(ackID int, name string, args ...any) (string, error) {
	prefix := fmt.Sprintf("42%d", ackID)
	return buildSIOEventFrame(prefix, name, args...)
}

func buildSIOEventFrame(prefix, name string, args ...any) (string, error) {
	parts := make([]any, 0, 1+len(args))
	parts = append(parts, name)
	parts = append(parts, args...)

	b, err := json.Marshal(parts)
	if err != nil {
		return "", fmt.Errorf("transport: marshal SIO event %q: %w", name, err)
	}
	return prefix + string(b), nil
}

// SIOConnectPayload extracts the SIO connect namespace data (usually contains sid).
func SIOConnectPayload(payload string) map[string]string {
	var m map[string]string
	_ = json.Unmarshal([]byte(payload), &m)
	return m
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// WSPath returns the WebSocket path for Socket.IO Engine.IO v4.
func WSPath(baseURL string) string {
	base := strings.TrimRight(baseURL, "/")
	base = strings.Replace(base, "http://", "ws://", 1)
	base = strings.Replace(base, "https://", "wss://", 1)
	return base + "/socket.io/?EIO=4&transport=websocket"
}
