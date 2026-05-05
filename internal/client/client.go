// Package client provides the core Socket.IO client for Uptime Kuma v2.x.
//
// Architecture:
//
//	Connect() dials the WebSocket and performs the Engine.IO + Socket.IO
//	handshake. A background read loop goroutine (readLoop) parses every
//	incoming frame and routes it to the eventBus:
//	  - Push events  → eventBus.publish()
//	  - ACK replies  → eventBus.resolveAck()
//	  - EIO pings    → immediate pong write
//
//	Callers use Emit() to send events without waiting for a reply, or
//	EmitWithAck() to send and block until the server's callback arrives.
//
//	The client is safe for concurrent use after Connect() returns.
package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/jinkp/kumaapi/internal/transport"
	"nhooyr.io/websocket"
)

// Sentinel errors.
var (
	ErrDisconnected = errors.New("client: not connected")
	ErrNotAuthed    = errors.New("client: not authenticated")
	ErrLoginFailed  = errors.New("client: login failed")
	ErrAckTimeout   = errors.New("client: ack timeout")
)

const (
	defaultAckTimeout = 15 * time.Second
	readBufSize       = 1024 * 1024 // 1 MB — matches Uptime Kuma's maxPayload
)

// Client is a Socket.IO client connected to one Uptime Kuma instance.
// Obtain one via New(); call Connect() before any other method.
type Client struct {
	baseURL string

	mu        sync.Mutex
	conn      *websocket.Conn
	connected bool
	authed    bool
	authToken string // JWT returned by login, reused for reconnect

	bus        *eventBus
	cancelRead context.CancelFunc // cancels the read loop

	AckTimeout time.Duration // default: 15s
}

// New creates a new Client targeting the given Uptime Kuma base URL.
// Call Connect() to establish the connection.
//
//	c := client.New("http://localhost:3002")
func New(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		bus:        newEventBus(),
		AckTimeout: defaultAckTimeout,
	}
}

// Connect dials the WebSocket, performs the Engine.IO + Socket.IO handshake,
// and starts the background read loop. It blocks until the SIO CONNECT ack
// is received or the context is cancelled.
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return nil
	}

	wsURL := transport.WSPath(c.baseURL)

	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Origin": []string{c.baseURL},
		},
	})
	if err != nil {
		return fmt.Errorf("client: dial %s: %w", wsURL, err)
	}
	conn.SetReadLimit(readBufSize)
	c.conn = conn

	// Engine.IO open frame must arrive first
	if err := c.readEIOOpen(ctx); err != nil {
		_ = conn.Close(websocket.StatusProtocolError, "bad handshake")
		return err
	}

	// Send Socket.IO CONNECT
	if err := c.writeRaw(ctx, transport.BuildSIOConnect()); err != nil {
		_ = conn.Close(websocket.StatusProtocolError, "sio connect failed")
		return err
	}

	// Wait for SIO CONNECT ack (40{sid})
	if err := c.readSIOConnectAck(ctx); err != nil {
		_ = conn.Close(websocket.StatusProtocolError, "sio connect ack failed")
		return err
	}

	// Start background read loop
	readCtx, cancel := context.WithCancel(context.Background())
	c.cancelRead = cancel
	c.connected = true

	go c.readLoop(readCtx)

	return nil
}

// Disconnect gracefully closes the WebSocket connection and shuts down
// the read loop. Safe to call multiple times.
func (c *Client) Disconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return
	}
	c.connected = false
	c.authed = false

	if c.cancelRead != nil {
		c.cancelRead()
	}
	_ = c.conn.Close(websocket.StatusNormalClosure, "disconnect")
	c.bus.closeAll()
}

// IsConnected reports whether the WebSocket connection is active.
func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

// IsAuthed reports whether the client has successfully authenticated.
func (c *Client) IsAuthed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.authed
}

// Emit sends a Socket.IO event without expecting an ACK callback.
// Use EmitWithAck when you need the server's response.
func (c *Client) Emit(ctx context.Context, eventName string, args ...any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return ErrDisconnected
	}
	frame, err := transport.BuildSIOEvent(eventName, args...)
	if err != nil {
		return err
	}
	return c.writeRaw(ctx, frame)
}

// EmitWithAck sends a Socket.IO event with an ACK ID and blocks until the
// server responds with the callback payload, or until AckTimeout elapses.
//
// Returns the raw JSON arguments from the server's ACK response.
func (c *Client) EmitWithAck(ctx context.Context, eventName string, args ...any) ([]json.RawMessage, error) {
	c.mu.Lock()
	if !c.connected {
		c.mu.Unlock()
		return nil, ErrDisconnected
	}

	ackID, ackCh := c.bus.reserveAck()
	frame, err := transport.BuildSIOEventAck(ackID, eventName, args...)
	if err != nil {
		c.bus.cancelAck(ackID)
		c.mu.Unlock()
		return nil, err
	}

	if err := c.writeRaw(ctx, frame); err != nil {
		c.bus.cancelAck(ackID)
		c.mu.Unlock()
		return nil, err
	}
	c.mu.Unlock()

	timeout := c.AckTimeout
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case result := <-ackCh:
		if result.Err != nil {
			return nil, result.Err
		}
		return result.Args, nil
	case <-timer.C:
		c.bus.cancelAck(ackID)
		return nil, fmt.Errorf("%w: event=%q ackID=%d", ErrAckTimeout, eventName, ackID)
	case <-ctx.Done():
		c.bus.cancelAck(ackID)
		return nil, ctx.Err()
	}
}

// Subscribe returns a channel that receives all push events with the given name.
// The caller must call the returned unsubscribe function when done.
//
//	ch, unsub := c.Subscribe("heartbeat")
//	defer unsub()
//	for ev := range ch { ... }
func (c *Client) Subscribe(eventName string) (<-chan Event, func()) {
	return c.bus.subscribe(eventName)
}

// LastEvent returns the last push event seen for the given name.
func (c *Client) LastEvent(eventName string) (Event, bool) {
	return c.bus.lastEvent(eventName)
}

// ── Handshake helpers (called before read loop starts) ───────────────────────

func (c *Client) readEIOOpen(ctx context.Context) error {
	_, data, err := c.conn.Read(ctx)
	if err != nil {
		return fmt.Errorf("client: read EIO open: %w", err)
	}
	pkt, err := transport.ParseEIO(string(data))
	if err != nil {
		return err
	}
	if pkt.Type != transport.EIOOpen {
		return fmt.Errorf("client: expected EIO open (0), got %q", string(pkt.Type))
	}
	// We could store the SID / ping interval here for keepalive tuning.
	// For now the server drives pings and we respond in the read loop.
	return nil
}

func (c *Client) readSIOConnectAck(ctx context.Context) error {
	_, data, err := c.conn.Read(ctx)
	if err != nil {
		return fmt.Errorf("client: read SIO connect ack: %w", err)
	}
	pkt, err := transport.ParseEIO(string(data))
	if err != nil {
		return err
	}
	if pkt.Type != transport.EIOMessage {
		return fmt.Errorf("client: expected EIO message for SIO connect ack, got %q", string(pkt.Type))
	}
	sio, err := transport.ParseSIO(pkt.Payload)
	if err != nil {
		return err
	}
	if sio.Type != transport.SIOConnect {
		return fmt.Errorf("client: expected SIO CONNECT (0), got %q", string(sio.Type))
	}
	return nil
}

// ── Background read loop ─────────────────────────────────────────────────────

func (c *Client) readLoop(ctx context.Context) {
	defer func() {
		c.mu.Lock()
		wasConnected := c.connected
		c.connected = false
		c.authed = false
		c.mu.Unlock()

		if wasConnected {
			c.bus.closeAll()
		}
	}()

	for {
		_, data, err := c.conn.Read(ctx)
		if err != nil {
			// Context cancelled = intentional disconnect
			if ctx.Err() != nil {
				return
			}
			// Unexpected read error — connection dropped
			return
		}

		if err := c.handleFrame(ctx, string(data)); err != nil {
			// Non-fatal parse errors: log and continue
			// Fatal errors (write failures) break the loop
			continue
		}
	}
}

func (c *Client) handleFrame(ctx context.Context, raw string) error {
	pkt, err := transport.ParseEIO(raw)
	if err != nil {
		return nil // unknown frame type, skip
	}

	switch pkt.Type {
	case transport.EIOPing:
		// Respond with pong immediately (do not hold the client lock)
		return c.writeRaw(ctx, transport.BuildPong())

	case transport.EIOMessage:
		return c.handleSIOFrame(pkt.Payload)

	case transport.EIOClose:
		c.Disconnect()
		return nil
	}
	return nil
}

func (c *Client) handleSIOFrame(payload string) error {
	sio, err := transport.ParseSIO(payload)
	if err != nil {
		return nil
	}

	switch sio.Type {
	case transport.SIOEvent:
		name, args, err := sio.EventArgs()
		if err != nil || name == "" {
			return nil
		}
		c.bus.publish(Event{Name: name, Args: args})

	case transport.SIOAck:
		if sio.AckID < 0 {
			return nil
		}
		_, args, err := sio.EventArgs()
		c.bus.resolveAck(sio.AckID, ackResult{Args: args, Err: err})

	case transport.SIODisconnect:
		c.Disconnect()

	case transport.SIOConnectError:
		// Deliver to any subscriber of the internal error event
		c.bus.publish(Event{Name: "_connectError", Args: nil})
	}
	return nil
}

// ── Internal write helper ────────────────────────────────────────────────────

// writeRaw sends a raw text frame. Caller must NOT hold c.mu when called
// from the read loop; caller MUST hold c.mu when called from Connect.
func (c *Client) writeRaw(ctx context.Context, frame string) error {
	return c.conn.Write(ctx, websocket.MessageText, []byte(frame))
}
