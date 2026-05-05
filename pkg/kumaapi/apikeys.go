package kumaapi

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jinkp/kumaapi/internal/client"
	"github.com/jinkp/kumaapi/internal/models"
)

// ListAPIKeys returns all API keys for the authenticated user.
func (a *API) ListAPIKeys(ctx context.Context) ([]models.APIKey, error) {
	ch, unsub := a.c.Subscribe("apiKeyList")
	defer unsub()

	args, err := a.c.EmitWithAck(ctx, "getAPIKeyList")
	if err != nil {
		return nil, fmt.Errorf("kumaapi: getAPIKeyList: %w", err)
	}
	if err := checkOKResponse(args); err != nil {
		return nil, err
	}

	select {
	case ev, ok := <-ch:
		if !ok {
			return nil, client.ErrDisconnected
		}
		return decodeAPIKeyList(ev)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// AddAPIKey creates a new API key.
func (a *API) AddAPIKey(ctx context.Context, req models.AddAPIKeyRequest) (models.AddAPIKeyResponse, error) {
	payload := map[string]any{
		"name":    req.Name,
		"expires": req.Expires,
		"active":  true,
	}
	args, err := a.c.EmitWithAck(ctx, "addAPIKey", payload)
	if err != nil {
		return models.AddAPIKeyResponse{}, fmt.Errorf("kumaapi: addAPIKey: %w", err)
	}
	var resp models.AddAPIKeyResponse
	if err := decodeACKArg(args, &resp); err != nil {
		return models.AddAPIKeyResponse{}, err
	}
	if !resp.Ok {
		return models.AddAPIKeyResponse{}, fmt.Errorf("kumaapi: addAPIKey rejected: %s", resp.Msg)
	}
	return resp, nil
}

// DeleteAPIKey deletes an API key by ID.
func (a *API) DeleteAPIKey(ctx context.Context, id int) error {
	args, err := a.c.EmitWithAck(ctx, "deleteAPIKey", id)
	if err != nil {
		return fmt.Errorf("kumaapi: deleteAPIKey(%d): %w", id, err)
	}
	return checkOKResponse(args)
}

// EnableAPIKey enables an API key.
func (a *API) EnableAPIKey(ctx context.Context, id int) error {
	args, err := a.c.EmitWithAck(ctx, "enableAPIKey", id)
	if err != nil {
		return fmt.Errorf("kumaapi: enableAPIKey(%d): %w", id, err)
	}
	return checkOKResponse(args)
}

// DisableAPIKey disables an API key.
func (a *API) DisableAPIKey(ctx context.Context, id int) error {
	args, err := a.c.EmitWithAck(ctx, "disableAPIKey", id)
	if err != nil {
		return fmt.Errorf("kumaapi: disableAPIKey(%d): %w", id, err)
	}
	return checkOKResponse(args)
}

func decodeAPIKeyList(ev client.Event) ([]models.APIKey, error) {
	if len(ev.Args) == 0 {
		return nil, fmt.Errorf("kumaapi: apiKeyList event has no args")
	}
	var keys []models.APIKey
	if err := json.Unmarshal(ev.Args[0], &keys); err != nil {
		return nil, fmt.Errorf("kumaapi: decode apiKeyList: %w", err)
	}
	return keys, nil
}
