// Package kumaapi is the public SDK for interacting with Uptime Kuma v2.x
// via its Socket.IO API.
//
// # Quick start
//
//	api, err := kumaapi.New("http://localhost:3002")
//	if err != nil { log.Fatal(err) }
//	defer api.Disconnect()
//
//	if err := api.Login(context.Background(), "admin", "password"); err != nil {
//	    log.Fatal(err)
//	}
//
//	monitors, err := api.ListMonitors(context.Background())
//
// # Context manager pattern
//
//	api, _ := kumaapi.New("http://localhost:3002")
//	ctx := context.Background()
//	api.Login(ctx, user, pass)
//	defer api.Disconnect()
package kumaapi

import (
	"context"
	"fmt"

	"github.com/jinkp/kumaapi/internal/client"
)

// API is the public facade over the internal Socket.IO client.
// Obtain one via New() and call Login() or LoginWithToken() before
// making any data requests.
type API struct {
	c *client.Client
}

// New creates an API client targeting the given Uptime Kuma base URL
// and immediately establishes the WebSocket connection.
//
// Returns an error if the connection cannot be established.
func New(ctx context.Context, baseURL string) (*API, error) {
	c := client.New(baseURL)
	if err := c.Connect(ctx); err != nil {
		return nil, fmt.Errorf("kumaapi: connect to %s: %w", baseURL, err)
	}
	return &API{c: c}, nil
}

// Login authenticates with username and password.
// Must be called before any data operations.
func (a *API) Login(ctx context.Context, username, password string) error {
	return a.c.Login(ctx, username, password)
}

// LoginWithToken authenticates using a JWT token previously obtained
// from Login(). Useful for CLI sessions that persist the token in config.
func (a *API) LoginWithToken(ctx context.Context, token string) error {
	return a.c.LoginWithToken(ctx, token)
}

// AuthToken returns the JWT token from the last successful login.
// Empty string if not yet authenticated.
func (a *API) AuthToken() string {
	return a.c.AuthToken()
}

// Disconnect closes the WebSocket connection. Safe to call multiple times.
// Should be deferred after New().
func (a *API) Disconnect() {
	a.c.Disconnect()
}

// IsConnected reports whether the underlying connection is active.
func (a *API) IsConnected() bool {
	return a.c.IsConnected()
}
