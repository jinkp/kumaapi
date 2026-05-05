package kumaapi

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/jinkp/kumaapi/internal/client"
	"github.com/jinkp/kumaapi/internal/models"
)

// ListMaintenances returns all maintenances for the authenticated user.
func (a *API) ListMaintenances(ctx context.Context) ([]models.Maintenance, error) {
	ch, unsub := a.c.Subscribe("maintenanceList")
	defer unsub()

	args, err := a.c.EmitWithAck(ctx, "getMaintenanceList")
	if err != nil {
		return nil, fmt.Errorf("kumaapi: getMaintenanceList: %w", err)
	}
	if err := checkOKResponse(args); err != nil {
		return nil, err
	}

	select {
	case ev, ok := <-ch:
		if !ok {
			return nil, client.ErrDisconnected
		}
		return decodeMaintenanceList(ev)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// GetMaintenance returns a maintenance by ID.
func (a *API) GetMaintenance(ctx context.Context, id int) (*models.Maintenance, error) {
	args, err := a.c.EmitWithAck(ctx, "getMaintenance", id)
	if err != nil {
		return nil, fmt.Errorf("kumaapi: getMaintenance(%d): %w", id, err)
	}
	var resp struct {
		Ok          bool               `json:"ok"`
		Msg         string             `json:"msg"`
		Maintenance models.Maintenance `json:"maintenance"`
	}
	if err := decodeACKArg(args, &resp); err != nil {
		return nil, err
	}
	if !resp.Ok {
		return nil, fmt.Errorf("kumaapi: getMaintenance rejected: %s", resp.Msg)
	}
	return &resp.Maintenance, nil
}

// AddMaintenance creates a maintenance and returns its ID.
func (a *API) AddMaintenance(ctx context.Context, m models.Maintenance) (int, error) {
	args, err := a.c.EmitWithAck(ctx, "addMaintenance", encodeMaintenancePayload(m, false))
	if err != nil {
		return 0, fmt.Errorf("kumaapi: addMaintenance: %w", err)
	}
	var resp struct {
		Ok            bool   `json:"ok"`
		Msg           string `json:"msg"`
		MaintenanceID int    `json:"maintenanceID"`
	}
	if err := decodeACKArg(args, &resp); err != nil {
		return 0, err
	}
	if !resp.Ok {
		return 0, fmt.Errorf("kumaapi: addMaintenance rejected: %s", resp.Msg)
	}
	return resp.MaintenanceID, nil
}

// EditMaintenance updates an existing maintenance.
func (a *API) EditMaintenance(ctx context.Context, m models.Maintenance) error {
	if m.ID == 0 {
		return fmt.Errorf("kumaapi: EditMaintenance: ID is required")
	}
	args, err := a.c.EmitWithAck(ctx, "editMaintenance", encodeMaintenancePayload(m, true))
	if err != nil {
		return fmt.Errorf("kumaapi: editMaintenance(%d): %w", m.ID, err)
	}
	return checkOKResponse(args)
}

// DeleteMaintenance deletes a maintenance by ID.
func (a *API) DeleteMaintenance(ctx context.Context, id int) error {
	args, err := a.c.EmitWithAck(ctx, "deleteMaintenance", id)
	if err != nil {
		return fmt.Errorf("kumaapi: deleteMaintenance(%d): %w", id, err)
	}
	return checkOKResponse(args)
}

// PauseMaintenance pauses a maintenance.
func (a *API) PauseMaintenance(ctx context.Context, id int) error {
	args, err := a.c.EmitWithAck(ctx, "pauseMaintenance", id)
	if err != nil {
		return fmt.Errorf("kumaapi: pauseMaintenance(%d): %w", id, err)
	}
	return checkOKResponse(args)
}

// ResumeMaintenance resumes a maintenance.
func (a *API) ResumeMaintenance(ctx context.Context, id int) error {
	args, err := a.c.EmitWithAck(ctx, "resumeMaintenance", id)
	if err != nil {
		return fmt.Errorf("kumaapi: resumeMaintenance(%d): %w", id, err)
	}
	return checkOKResponse(args)
}

// AddMonitorToMaintenance replaces the monitor set attached to a maintenance.
func (a *API) AddMonitorToMaintenance(ctx context.Context, maintenanceID int, monitorIDs []int) error {
	monitors := make([]map[string]int, 0, len(monitorIDs))
	for _, id := range monitorIDs {
		monitors = append(monitors, map[string]int{"id": id})
	}
	args, err := a.c.EmitWithAck(ctx, "addMonitorMaintenance", maintenanceID, monitors)
	if err != nil {
		return fmt.Errorf("kumaapi: addMonitorMaintenance(%d): %w", maintenanceID, err)
	}
	return checkOKResponse(args)
}

func decodeMaintenanceList(ev client.Event) ([]models.Maintenance, error) {
	if len(ev.Args) == 0 {
		return nil, fmt.Errorf("kumaapi: maintenanceList event has no args")
	}
	var raw map[string]models.Maintenance
	if err := json.Unmarshal(ev.Args[0], &raw); err != nil {
		return nil, fmt.Errorf("kumaapi: decode maintenanceList: %w", err)
	}
	out := make([]models.Maintenance, 0, len(raw))
	for key, item := range raw {
		if item.ID == 0 {
			if id, err := strconv.Atoi(key); err == nil {
				item.ID = id
			}
		}
		out = append(out, item)
	}
	return out, nil
}

func encodeMaintenancePayload(m models.Maintenance, includeID bool) map[string]any {
	dateRange := []any{nil, nil}
	if m.StartDate != "" {
		dateRange[0] = m.StartDate
	}
	if m.EndDate != "" {
		dateRange[1] = m.EndDate
	}

	timeRange := []any{nil, nil}
	if tm, ok := maintenanceTimeObject(m.StartTime); ok {
		timeRange[0] = tm
	}
	if tm, ok := maintenanceTimeObject(m.EndTime); ok {
		timeRange[1] = tm
	}

	payload := map[string]any{
		"title":          m.Title,
		"description":    m.Description,
		"strategy":       m.Strategy,
		"active":         m.Active.IsActive(),
		"cron":           m.Cron,
		"dateRange":      dateRange,
		"timeRange":      timeRange,
		"weekdays":       defaultIntSlice(m.Weekdays),
		"daysOfMonth":    defaultIntSlice(m.DaysOfMonth),
		"intervalDay":    m.IntervalDay,
		"timezoneOption": m.TimezoneOption,
	}
	if includeID {
		payload["id"] = m.ID
	}
	return payload
}

func maintenanceTimeObject(value string) (map[string]int, bool) {
	if value == "" {
		return nil, false
	}
	parts := strings.Split(value, ":")
	if len(parts) < 2 {
		return nil, false
	}
	hours, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, false
	}
	minutes, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, false
	}
	seconds := 0
	if len(parts) >= 3 {
		seconds, err = strconv.Atoi(parts[2])
		if err != nil {
			return nil, false
		}
	}
	return map[string]int{
		"hours":   hours,
		"minutes": minutes,
		"seconds": seconds,
	}, true
}

func defaultIntSlice(items []int) []int {
	if items == nil {
		return []int{}
	}
	return items
}
