package kumaapi

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jinkp/kumaapi/internal/client"
	"github.com/jinkp/kumaapi/internal/models"
)

// ListNotifications returns all notification providers.
//
// Uptime Kuma v2.x does not expose a dedicated getNotificationList event.
// The server pushes notificationList during login and after add/delete, so we
// refresh the current session via loginByToken to trigger the canonical push.
func (a *API) ListNotifications(ctx context.Context) ([]models.Notification, error) {
	ch, unsub := a.c.Subscribe("notificationList")
	defer unsub()

	token := a.c.AuthToken()
	if token == "" {
		return nil, fmt.Errorf("kumaapi: ListNotifications requires a prior login")
	}
	if err := a.c.LoginWithToken(ctx, token); err != nil {
		return nil, fmt.Errorf("kumaapi: refresh notificationList via loginByToken: %w", err)
	}

	select {
	case ev, ok := <-ch:
		if !ok {
			return nil, client.ErrDisconnected
		}
		return decodeNotificationList(ev)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// AddNotification creates or updates a notification and returns its ID.
// Pass notificationID=nil to create a new notification.
func (a *API) AddNotification(ctx context.Context, payload map[string]any, notificationID *int) (int, error) {
	var idArg any
	if notificationID != nil {
		idArg = *notificationID
	}

	args, err := a.c.EmitWithAck(ctx, "addNotification", payload, idArg)
	if err != nil {
		return 0, fmt.Errorf("kumaapi: addNotification: %w", err)
	}

	var resp struct {
		Ok  bool   `json:"ok"`
		Msg string `json:"msg"`
		ID  int    `json:"id"`
	}
	if err := decodeACKArg(args, &resp); err != nil {
		return 0, err
	}
	if !resp.Ok {
		return 0, fmt.Errorf("kumaapi: addNotification rejected: %s", resp.Msg)
	}
	return resp.ID, nil
}

// DeleteNotification deletes a notification by ID.
func (a *API) DeleteNotification(ctx context.Context, id int) error {
	args, err := a.c.EmitWithAck(ctx, "deleteNotification", id)
	if err != nil {
		return fmt.Errorf("kumaapi: deleteNotification(%d): %w", id, err)
	}
	return checkOKResponse(args)
}

// TestNotification sends a test message using the provided notification config.
func (a *API) TestNotification(ctx context.Context, payload map[string]any) error {
	args, err := a.c.EmitWithAck(ctx, "testNotification", payload)
	if err != nil {
		return fmt.Errorf("kumaapi: testNotification: %w", err)
	}
	return checkOKResponse(args)
}

func decodeNotificationList(ev client.Event) ([]models.Notification, error) {
	if len(ev.Args) == 0 {
		return nil, fmt.Errorf("kumaapi: notificationList event has no args")
	}
	var notifications []models.Notification
	if err := json.Unmarshal(ev.Args[0], &notifications); err != nil {
		return nil, fmt.Errorf("kumaapi: decode notificationList: %w", err)
	}
	return notifications, nil
}
