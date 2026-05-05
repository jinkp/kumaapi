package client

import (
	"encoding/json"
	"sync"
)

// Event is a parsed Socket.IO push event received from the server.
type Event struct {
	Name string            // e.g. "monitorList", "heartbeat"
	Args []json.RawMessage // raw JSON arguments (may be empty)
}

// ackResult holds the server's callback response for a specific ACK ID.
type ackResult struct {
	Args []json.RawMessage
	Err  error
}

// eventBus dispatches incoming Socket.IO events to registered subscribers
// and resolves pending ACK waiters.
//
// Design:
//   - Subscribers register a channel per event name.
//   - Each push event is fanned out to all matching channels (non-blocking).
//   - ACK results are routed by numeric ID to a waiting caller's channel.
type eventBus struct {
	mu          sync.RWMutex
	subscribers map[string][]chan Event // eventName → list of listener channels
	ackWaiters  map[int]chan ackResult  // ackID    → response channel
	lastEvents  map[string]Event
	nextAckID   int
}

func newEventBus() *eventBus {
	return &eventBus{
		subscribers: make(map[string][]chan Event),
		ackWaiters:  make(map[int]chan ackResult),
		lastEvents:  make(map[string]Event),
	}
}

// subscribe registers a buffered channel that receives events with the given name.
// The caller must drain or close the channel; the bus never blocks on send.
// Returns an unsubscribe function — call it when done.
func (b *eventBus) subscribe(eventName string) (<-chan Event, func()) {
	ch := make(chan Event, 16)
	b.mu.Lock()
	b.subscribers[eventName] = append(b.subscribers[eventName], ch)
	b.mu.Unlock()

	unsub := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		list := b.subscribers[eventName]
		for i, c := range list {
			if c == ch {
				b.subscribers[eventName] = append(list[:i], list[i+1:]...)
				close(ch)
				return
			}
		}
	}
	return ch, unsub
}

// publish fans out an event to all registered subscribers for its name.
// Drops the event on any channel that is full (non-blocking).
func (b *eventBus) publish(ev Event) {
	b.mu.Lock()
	b.lastEvents[ev.Name] = ev
	listeners := b.subscribers[ev.Name]
	b.mu.Unlock()

	for _, ch := range listeners {
		select {
		case ch <- ev:
		default: // subscriber is slow; drop rather than block the read loop
		}
	}
}

func (b *eventBus) lastEvent(eventName string) (Event, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	ev, ok := b.lastEvents[eventName]
	return ev, ok
}

// reserveAck allocates the next ACK ID and registers a result channel for it.
// The caller must call waitAck(id) to receive the result, or cancelAck(id) to clean up.
func (b *eventBus) reserveAck() (id int, ch <-chan ackResult) {
	result := make(chan ackResult, 1)
	b.mu.Lock()
	id = b.nextAckID
	b.nextAckID++
	b.ackWaiters[id] = result
	b.mu.Unlock()
	return id, result
}

// resolveAck delivers the result to the waiter for the given ACK ID and removes it.
func (b *eventBus) resolveAck(id int, result ackResult) {
	b.mu.Lock()
	ch, ok := b.ackWaiters[id]
	if ok {
		delete(b.ackWaiters, id)
	}
	b.mu.Unlock()

	if ok {
		ch <- result
	}
}

// cancelAck removes a pending ACK waiter without delivering a result.
func (b *eventBus) cancelAck(id int) {
	b.mu.Lock()
	delete(b.ackWaiters, id)
	b.mu.Unlock()
}

// closeAll closes all subscriber channels (called on disconnect).
func (b *eventBus) closeAll() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for name, list := range b.subscribers {
		for _, ch := range list {
			close(ch)
		}
		delete(b.subscribers, name)
	}
	for name := range b.lastEvents {
		delete(b.lastEvents, name)
	}
	// Resolve pending ACKs with a disconnected error
	for id, ch := range b.ackWaiters {
		ch <- ackResult{Err: ErrDisconnected}
		delete(b.ackWaiters, id)
	}
}
