package kumaapi

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jinkp/kumaapi/internal/client"
	"github.com/jinkp/kumaapi/internal/models"
)

// ListMonitors returns all monitors for the authenticated user.
//
// Uptime Kuma sends monitorList as a push event when getMonitorList is called.
// We subscribe BEFORE emitting to avoid a race between the push and subscription.
func (a *API) ListMonitors(ctx context.Context) ([]models.Monitor, error) {
	// Subscribe first, then trigger the push
	ch, unsub := a.c.Subscribe("monitorList")
	defer unsub()

	// getMonitorList has no ACK — it just triggers the monitorList push
	if err := a.c.Emit(ctx, "getMonitorList"); err != nil {
		return nil, fmt.Errorf("kumaapi: getMonitorList emit: %w", err)
	}

	select {
	case ev, ok := <-ch:
		if !ok {
			return nil, client.ErrDisconnected
		}
		return decodeMonitorList(ev)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// GetMonitor returns a single monitor by ID.
func (a *API) GetMonitor(ctx context.Context, monitorID int) (*models.Monitor, error) {
	args, err := a.c.EmitWithAck(ctx, "getMonitor", monitorID)
	if err != nil {
		return nil, fmt.Errorf("kumaapi: getMonitor(%d): %w", monitorID, err)
	}
	return decodeMonitorResponse(args)
}

// AddMonitor creates a new monitor and returns its assigned ID.
func (a *API) AddMonitor(ctx context.Context, req models.AddMonitorRequest) (int, error) {
	// Ensure required defaults
	if req.NotificationIDList == nil {
		req.NotificationIDList = map[string]bool{}
	}
	if req.Interval == 0 {
		req.Interval = 60
	}
	if req.RetryInterval == 0 {
		req.RetryInterval = 60
	}
	if req.MaxRetries == 0 {
		req.MaxRetries = 1
	}
	// accepted_statuscodes is required by the server validator — default to 2xx
	if req.AcceptedStatCodes == nil {
		req.AcceptedStatCodes = []string{"200-299"}
	}
	// conditions is NOT NULL in the DB — must send empty array, never null
	if req.Conditions == nil {
		req.Conditions = []models.MonitorCondition{}
	}

	args, err := a.c.EmitWithAck(ctx, "add", req)
	if err != nil {
		return 0, fmt.Errorf("kumaapi: addMonitor: %w", err)
	}

	// Server ACK: {"ok":true,"msg":"Added Successfully.","monitorID":N}
	var resp models.AddMonitorResponse
	if err := decodeACKArg(args, &resp); err != nil {
		return 0, err
	}
	if !resp.Ok {
		return 0, fmt.Errorf("kumaapi: add rejected: %s", resp.Msg)
	}
	return resp.MonitorID, nil
}

// EditMonitor updates an existing monitor. The req.ID field must be set.
func (a *API) EditMonitor(ctx context.Context, req models.EditMonitorRequest) error {
	if req.ID == 0 {
		return fmt.Errorf("kumaapi: EditMonitor: ID is required")
	}
	if req.NotificationIDList == nil {
		req.NotificationIDList = map[string]bool{}
	}
	if req.AcceptedStatCodes == nil {
		req.AcceptedStatCodes = []string{"200-299"}
	}
	if req.Conditions == nil {
		req.Conditions = []models.MonitorCondition{}
	}

	args, err := a.c.EmitWithAck(ctx, "editMonitor", req)
	// Note: in v2.x the event is "editMonitor" (confirmed in server.js)
	if err != nil {
		return fmt.Errorf("kumaapi: editMonitor(%d): %w", req.ID, err)
	}
	return checkOKResponse(args)
}

// DeleteMonitor deletes a monitor by ID.
// If the monitor is a group, children are unlinked (not deleted).
// Pass deleteChildren=true to also delete all children recursively.
func (a *API) DeleteMonitor(ctx context.Context, monitorID int, deleteChildren bool) error {
	args, err := a.c.EmitWithAck(ctx, "deleteMonitor", monitorID, deleteChildren)
	if err != nil {
		return fmt.Errorf("kumaapi: deleteMonitor(%d): %w", monitorID, err)
	}
	return checkOKResponse(args)
}

// PauseMonitor pauses (deactivates) a monitor.
func (a *API) PauseMonitor(ctx context.Context, monitorID int) error {
	args, err := a.c.EmitWithAck(ctx, "pauseMonitor", monitorID)
	if err != nil {
		return fmt.Errorf("kumaapi: pauseMonitor(%d): %w", monitorID, err)
	}
	return checkOKResponse(args)
}

// ResumeMonitor resumes (activates) a paused monitor.
func (a *API) ResumeMonitor(ctx context.Context, monitorID int) error {
	args, err := a.c.EmitWithAck(ctx, "resumeMonitor", monitorID)
	if err != nil {
		return fmt.Errorf("kumaapi: resumeMonitor(%d): %w", monitorID, err)
	}
	return checkOKResponse(args)
}

// ── Decoders ──────────────────────────────────────────────────────────────────

func decodeMonitorList(ev client.Event) ([]models.Monitor, error) {
	if len(ev.Args) == 0 {
		return nil, fmt.Errorf("kumaapi: monitorList event has no args")
	}
	var raw map[string]models.Monitor
	if err := json.Unmarshal(ev.Args[0], &raw); err != nil {
		return nil, fmt.Errorf("kumaapi: decode monitorList: %w", err)
	}
	monitors := make([]models.Monitor, 0, len(raw))
	for _, m := range raw {
		monitors = append(monitors, m)
	}
	return monitors, nil
}

func decodeMonitorResponse(args []json.RawMessage) (*models.Monitor, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("kumaapi: getMonitor ACK has no args")
	}
	var resp struct {
		Ok      bool           `json:"ok"`
		Msg     string         `json:"msg"`
		Monitor models.Monitor `json:"monitor"`
	}
	if err := json.Unmarshal(args[0], &resp); err != nil {
		return nil, fmt.Errorf("kumaapi: decode getMonitor response: %w", err)
	}
	if !resp.Ok {
		return nil, fmt.Errorf("kumaapi: getMonitor rejected: %s", resp.Msg)
	}
	return &resp.Monitor, nil
}

// ── Shared helpers ────────────────────────────────────────────────────────────

type okResponse struct {
	Ok  bool   `json:"ok"`
	Msg string `json:"msg"`
}

func checkOKResponse(args []json.RawMessage) error {
	if len(args) == 0 {
		return fmt.Errorf("kumaapi: ACK has no args")
	}
	var resp okResponse
	if err := json.Unmarshal(args[0], &resp); err != nil {
		return fmt.Errorf("kumaapi: decode ACK response: %w", err)
	}
	if !resp.Ok {
		return fmt.Errorf("kumaapi: server error: %s", resp.Msg)
	}
	return nil
}

func decodeACKArg(args []json.RawMessage, dst any) error {
	if len(args) == 0 {
		return fmt.Errorf("kumaapi: ACK has no args")
	}
	if err := json.Unmarshal(args[0], dst); err != nil {
		return fmt.Errorf("kumaapi: decode ACK: %w", err)
	}
	return nil
}
