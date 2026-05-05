package client

import (
	"context"
	"encoding/json"
	"fmt"
)

// loginRequest mirrors the payload Uptime Kuma expects for the "login" event.
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Token    string `json:"token"` // empty on first login
}

// loginResponse is the server's ACK payload for "login" and "loginByToken".
type loginResponse struct {
	Ok    bool   `json:"ok"`
	Msg   string `json:"msg"`
	Token string `json:"token"` // JWT — store for reconnect
}

// Login authenticates with username and password.
// On success the JWT token is stored internally for LoginWithToken calls.
//
// The server pushes a batch of events (monitorList, notificationList, etc.)
// BEFORE delivering the ACK, so the read loop must already be running —
// which is guaranteed because Login is called after Connect().
func (c *Client) Login(ctx context.Context, username, password string) error {
	req := loginRequest{
		Username: username,
		Password: password,
		Token:    "",
	}

	args, err := c.EmitWithAck(ctx, "login", req)
	if err != nil {
		return fmt.Errorf("client: login emit: %w", err)
	}

	resp, err := decodeLoginResponse(args)
	if err != nil {
		return err
	}
	if !resp.Ok {
		msg := resp.Msg
		if msg == "" {
			msg = "server rejected credentials"
		}
		return fmt.Errorf("%w: %s", ErrLoginFailed, msg)
	}

	c.mu.Lock()
	c.authed = true
	c.authToken = resp.Token
	c.mu.Unlock()

	return nil
}

// LoginWithToken authenticates using a previously obtained JWT token.
// Uptime Kuma issues this token on a successful Login(); it can be persisted
// in the CLI config to avoid re-entering credentials every time.
func (c *Client) LoginWithToken(ctx context.Context, token string) error {
	args, err := c.EmitWithAck(ctx, "loginByToken", token)
	if err != nil {
		return fmt.Errorf("client: loginByToken emit: %w", err)
	}

	resp, err := decodeLoginResponse(args)
	if err != nil {
		return err
	}
	if !resp.Ok {
		msg := resp.Msg
		if msg == "" {
			msg = "token rejected"
		}
		return fmt.Errorf("%w: %s", ErrLoginFailed, msg)
	}
	if resp.Token == "" {
		resp.Token = token
	}

	c.mu.Lock()
	c.authed = true
	c.authToken = resp.Token
	c.mu.Unlock()

	return nil
}

// AuthToken returns the JWT token obtained after a successful login.
// Empty string if not yet authenticated.
func (c *Client) AuthToken() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.authToken
}

// decodeLoginResponse parses the ACK args from login / loginByToken events.
// The server sends: 430[{"ok":true,"token":"..."}]
// so args[0] is the response object.
func decodeLoginResponse(args []json.RawMessage) (loginResponse, error) {
	if len(args) == 0 {
		return loginResponse{}, fmt.Errorf("client: login ACK has no arguments")
	}
	var resp loginResponse
	if err := json.Unmarshal(args[0], &resp); err != nil {
		return loginResponse{}, fmt.Errorf("client: decode login response: %w", err)
	}
	return resp, nil
}
