package kumaapi

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jinkp/kumaapi/internal/client"
	"github.com/jinkp/kumaapi/internal/models"
)

// GetHeartbeats returns the last 100 heartbeats for the given monitor.
// The server sends these as a "heartbeatList" push event immediately after
// the request.
func (a *API) GetHeartbeats(ctx context.Context, monitorID int) ([]models.Heartbeat, error) {
	ch, unsub := a.c.Subscribe("heartbeatList")
	defer unsub()

	if err := a.c.Emit(ctx, "getMonitorList"); err != nil {
		return nil, fmt.Errorf("kumaapi: trigger heartbeatList via getMonitorList: %w", err)
	}

	// heartbeatList is sent automatically after login for each monitor,
	// but can also be triggered by requesting the monitor list.
	// We filter by monitorID on arrival.
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return nil, client.ErrDisconnected
			}
			beats, id, _, err := decodeHeartbeatList(ev)
			if err != nil {
				continue
			}
			if id == monitorID {
				return beats, nil
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// WatchHeartbeats returns a channel that receives heartbeats for the
// given monitor as they arrive in real time.
//
// The returned channel is closed when the connection drops or the
// caller calls the returned cancel function.
//
//	ch, cancel := api.WatchHeartbeats(ctx, monitorID)
//	defer cancel()
//	for hb := range ch {
//	    fmt.Printf("%s status=%d\n", hb.Time, hb.Status)
//	}
func (a *API) WatchHeartbeats(ctx context.Context, monitorID int) (<-chan models.Heartbeat, func()) {
	out := make(chan models.Heartbeat, 32)
	src, unsub := a.c.Subscribe("heartbeat")

	go func() {
		defer close(out)
		defer unsub()
		for {
			select {
			case ev, ok := <-src:
				if !ok {
					return
				}
				hb, err := decodeHeartbeat(ev)
				if err != nil || hb.MonitorID != monitorID {
					continue
				}
				select {
				case out <- hb:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	cancel := func() { unsub() }
	return out, cancel
}

// ── Decoders ──────────────────────────────────────────────────────────────────

// decodeHeartbeat parses a single "heartbeat" push event.
// Wire: 42["heartbeat", {monitorID, status, time, ...}]
func decodeHeartbeat(ev client.Event) (models.Heartbeat, error) {
	if len(ev.Args) == 0 {
		return models.Heartbeat{}, fmt.Errorf("kumaapi: heartbeat event has no args")
	}
	var hb models.Heartbeat
	if err := json.Unmarshal(ev.Args[0], &hb); err != nil {
		return models.Heartbeat{}, fmt.Errorf("kumaapi: decode heartbeat: %w", err)
	}
	return hb, nil
}

// decodeHeartbeatList parses a "heartbeatList" push event.
// Wire: 42["heartbeatList", monitorID, [heartbeats...], overwrite]
func decodeHeartbeatList(ev client.Event) ([]models.Heartbeat, int, bool, error) {
	// Args: [monitorID, [beats...], overwrite]
	if len(ev.Args) < 2 {
		return nil, 0, false, fmt.Errorf("kumaapi: heartbeatList has < 2 args")
	}

	var monitorID int
	if err := json.Unmarshal(ev.Args[0], &monitorID); err != nil {
		return nil, 0, false, fmt.Errorf("kumaapi: decode heartbeatList monitorID: %w", err)
	}

	var beats []models.Heartbeat
	if err := json.Unmarshal(ev.Args[1], &beats); err != nil {
		return nil, monitorID, false, fmt.Errorf("kumaapi: decode heartbeatList beats: %w", err)
	}

	var overwrite bool
	if len(ev.Args) >= 3 {
		_ = json.Unmarshal(ev.Args[2], &overwrite)
	}

	return beats, monitorID, overwrite, nil
}
