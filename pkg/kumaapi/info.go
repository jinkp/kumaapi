package kumaapi

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jinkp/kumaapi/internal/client"
	"github.com/jinkp/kumaapi/internal/models"
)

// GetInfo returns the most recent server metadata from the Socket.IO info event.
func (a *API) GetInfo(ctx context.Context) (*models.Info, error) {
	if ev, ok := a.c.LastEvent("info"); ok {
		return decodeInfoEvent(ev)
	}

	token := a.c.AuthToken()
	if token == "" {
		return nil, fmt.Errorf("kumaapi: info event unavailable before authentication")
	}

	ch, unsub := a.c.Subscribe("info")
	defer unsub()

	if err := a.c.LoginWithToken(ctx, token); err != nil {
		return nil, fmt.Errorf("kumaapi: refresh info via loginByToken: %w", err)
	}

	select {
	case ev, ok := <-ch:
		if !ok {
			return nil, client.ErrDisconnected
		}
		return decodeInfoEvent(ev)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func decodeInfoEvent(ev client.Event) (*models.Info, error) {
	if len(ev.Args) == 0 {
		return nil, fmt.Errorf("kumaapi: info event has no args")
	}
	var info models.Info
	if err := json.Unmarshal(ev.Args[0], &info); err != nil {
		return nil, fmt.Errorf("kumaapi: decode info event: %w", err)
	}
	return &info, nil
}
