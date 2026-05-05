package kumaapi

import (
	"context"
	"fmt"

	"github.com/jinkp/kumaapi/internal/models"
)

// ListTags returns all tags.
func (a *API) ListTags(ctx context.Context) ([]models.Tag, error) {
	args, err := a.c.EmitWithAck(ctx, "getTags")
	if err != nil {
		return nil, fmt.Errorf("kumaapi: getTags: %w", err)
	}
	var resp struct {
		Ok   bool         `json:"ok"`
		Msg  string       `json:"msg"`
		Tags []models.Tag `json:"tags"`
	}
	if err := decodeACKArg(args, &resp); err != nil {
		return nil, err
	}
	if !resp.Ok {
		return nil, fmt.Errorf("kumaapi: getTags rejected: %s", resp.Msg)
	}
	return resp.Tags, nil
}

// AddTag creates a new tag.
func (a *API) AddTag(ctx context.Context, name, color string) (*models.Tag, error) {
	args, err := a.c.EmitWithAck(ctx, "addTag", map[string]any{"name": name, "color": color})
	if err != nil {
		return nil, fmt.Errorf("kumaapi: addTag: %w", err)
	}
	var resp struct {
		Ok  bool       `json:"ok"`
		Msg string     `json:"msg"`
		Tag models.Tag `json:"tag"`
	}
	if err := decodeACKArg(args, &resp); err != nil {
		return nil, err
	}
	if !resp.Ok {
		return nil, fmt.Errorf("kumaapi: addTag rejected: %s", resp.Msg)
	}
	return &resp.Tag, nil
}

// EditTag updates an existing tag.
func (a *API) EditTag(ctx context.Context, id int, name, color string) error {
	args, err := a.c.EmitWithAck(ctx, "editTag", map[string]any{"id": id, "name": name, "color": color})
	if err != nil {
		return fmt.Errorf("kumaapi: editTag(%d): %w", id, err)
	}
	return checkOKResponse(args)
}

// DeleteTag deletes a tag by ID.
func (a *API) DeleteTag(ctx context.Context, id int) error {
	args, err := a.c.EmitWithAck(ctx, "deleteTag", id)
	if err != nil {
		return fmt.Errorf("kumaapi: deleteTag(%d): %w", id, err)
	}
	return checkOKResponse(args)
}

// AddMonitorTag attaches a tag to a monitor.
func (a *API) AddMonitorTag(ctx context.Context, tagID, monitorID int, value string) error {
	args, err := a.c.EmitWithAck(ctx, "addMonitorTag", tagID, monitorID, value)
	if err != nil {
		return fmt.Errorf("kumaapi: addMonitorTag(tag=%d, monitor=%d): %w", tagID, monitorID, err)
	}
	return checkOKResponse(args)
}

// EditMonitorTag updates a monitor-tag relation value.
func (a *API) EditMonitorTag(ctx context.Context, tagID, monitorID int, value string) error {
	args, err := a.c.EmitWithAck(ctx, "editMonitorTag", tagID, monitorID, value)
	if err != nil {
		return fmt.Errorf("kumaapi: editMonitorTag(tag=%d, monitor=%d): %w", tagID, monitorID, err)
	}
	return checkOKResponse(args)
}

// DeleteMonitorTag removes a monitor-tag relation.
func (a *API) DeleteMonitorTag(ctx context.Context, tagID, monitorID int, value string) error {
	args, err := a.c.EmitWithAck(ctx, "deleteMonitorTag", tagID, monitorID, value)
	if err != nil {
		return fmt.Errorf("kumaapi: deleteMonitorTag(tag=%d, monitor=%d): %w", tagID, monitorID, err)
	}
	return checkOKResponse(args)
}
