// Integration tests for the core client.
// Require a running Uptime Kuma v2 instance.
//
// Configuration via environment variables (see testConfig):
//
//	KUMA_URL   base URL, default http://localhost:3002
//	KUMA_USER  username,  default joel.keb
//	KUMA_PASS  password,  default (empty — set via env)
//
// Run with:
//
//	go test ./tests/integration/... -v -tags integration
package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jinkp/kumaapi/internal/client"
)

// ── Test config ──────────────────────────────────────────────────────────────

type config struct {
	URL   string
	User  string
	Pass  string
	Token string
}

func testConfig() config {
	get := func(key, fallback string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return fallback
	}
	return config{
		URL:   get("KUMA_URL", "http://localhost:3002"),
		User:  get("KUMA_USER", "joel.keb"),
		Pass:  get("KUMA_PASS", ""),
		Token: get("KUMA_TOKEN", ""),
	}
}

var (
	testTokenOnce sync.Once
	testToken     string
	testTokenErr  error
)

// ── Helpers ───────────────────────────────────────────────────────────────────

func newConnectedClient(t *testing.T) *client.Client {
	t.Helper()
	cfg := testConfig()

	c := client.New(cfg.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	if err := c.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	t.Cleanup(c.Disconnect)
	return c
}

func newAuthedClient(t *testing.T) *client.Client {
	t.Helper()
	c := newConnectedClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	if err := c.LoginWithToken(ctx, getTestToken(t)); err != nil {
		t.Fatalf("LoginWithToken() error: %v", err)
	}
	return c
}

func getTestToken(t *testing.T) string {
	t.Helper()
	cfg := testConfig()
	if cfg.Token != "" {
		return cfg.Token
	}
	testTokenOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		c := client.New(cfg.URL)
		if err := c.Connect(ctx); err != nil {
			testTokenErr = err
			return
		}
		defer c.Disconnect()

		if err := c.Login(ctx, cfg.User, cfg.Pass); err != nil {
			testTokenErr = err
			return
		}
		testToken = c.AuthToken()
		if testToken == "" {
			testTokenErr = fmt.Errorf("received empty auth token")
		}
	})
	if testTokenErr != nil {
		t.Fatalf("getTestToken() error: %v", testTokenErr)
	}
	return testToken
}

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestConnect(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	c := newConnectedClient(t)

	if !c.IsConnected() {
		t.Fatal("expected IsConnected() = true after Connect()")
	}
	if c.IsAuthed() {
		t.Fatal("expected IsAuthed() = false before Login()")
	}
}

func TestLogin(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	c := newAuthedClient(t)

	if !c.IsAuthed() {
		t.Fatal("expected IsAuthed() = true after Login()")
	}
	if c.AuthToken() == "" {
		t.Fatal("expected non-empty AuthToken() after Login()")
	}
	t.Logf("JWT token (first 40 chars): %.40s...", c.AuthToken())
}

func TestLoginBadPassword(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	c := newConnectedClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := c.Login(ctx, "joel.keb", "wrong-password-xyz")
	if err == nil {
		t.Fatal("expected error on bad password, got nil")
	}
	t.Logf("correctly rejected: %v", err)
}

func TestLoginWithToken(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// First obtain a fresh token via password login
	c := newAuthedClient(t)
	token := c.AuthToken()

	// Now create a new client and authenticate with the token only
	cfg := testConfig()
	c2 := client.New(cfg.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := c2.Connect(ctx); err != nil {
		t.Fatalf("Connect() c2 error: %v", err)
	}
	defer c2.Disconnect()

	if err := c2.LoginWithToken(ctx, token); err != nil {
		t.Fatalf("LoginWithToken() error: %v", err)
	}
	if !c2.IsAuthed() {
		t.Fatal("expected IsAuthed() = true after LoginWithToken()")
	}
}

func TestServerPushOnLogin(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Verify that the server pushes the expected events immediately after login.
	// We subscribe BEFORE login so we don't miss the burst.
	cfg := testConfig()
	c := newConnectedClient(t)

	expectedEvents := []string{
		"monitorList",
		"monitorTypeList",
		"notificationList",
		"proxyList",
		"dockerHostList",
		"apiKeyList",
	}

	channels := make(map[string]<-chan client.Event, len(expectedEvents))
	for _, name := range expectedEvents {
		ch, unsub := c.Subscribe(name)
		t.Cleanup(unsub)
		channels[name] = ch
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := c.Login(ctx, cfg.User, cfg.Pass); err != nil {
		t.Fatalf("Login() error: %v", err)
	}

	// Each event should arrive within 5 seconds of login
	deadline := time.After(5 * time.Second)
	received := make(map[string]bool, len(expectedEvents))

	for len(received) < len(expectedEvents) {
		allDone := true
		for _, name := range expectedEvents {
			if received[name] {
				continue
			}
			allDone = false
			select {
			case ev, ok := <-channels[name]:
				if ok {
					received[name] = true
					t.Logf("✓ received event %q (args: %d)", ev.Name, len(ev.Args))
				}
			case <-deadline:
				t.Errorf("✗ timed out waiting for event %q", name)
				return
			default:
			}
		}
		if allDone {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	for _, name := range expectedEvents {
		if !received[name] {
			t.Errorf("✗ never received event %q", name)
		}
	}
}

func TestDisconnect(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	c := newAuthedClient(t)

	c.Disconnect()

	if c.IsConnected() {
		t.Fatal("expected IsConnected() = false after Disconnect()")
	}
	if c.IsAuthed() {
		t.Fatal("expected IsAuthed() = false after Disconnect()")
	}
}

func TestEmitWithAckTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	c := newAuthedClient(t)
	c.AckTimeout = 1 * time.Second // very short for the test

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// "nonExistentEvent" has no ACK handler on the server → timeout expected
	_, err := c.EmitWithAck(ctx, "nonExistentEvent", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	t.Logf("correctly timed out: %v", err)
}
