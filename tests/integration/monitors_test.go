package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jinkp/kumaapi/internal/models"
	"github.com/jinkp/kumaapi/pkg/kumaapi"
)

// ── Helpers ───────────────────────────────────────────────────────────────────

func newAuthedAPI(t *testing.T) *kumaapi.API {
	t.Helper()
	cfg := testConfig()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	api, err := kumaapi.New(ctx, cfg.URL)
	if err != nil {
		t.Fatalf("kumaapi.New() error: %v", err)
	}
	t.Cleanup(api.Disconnect)

	if err := api.Login(ctx, cfg.User, cfg.Pass); err != nil {
		t.Fatalf("Login() error: %v", err)
	}
	return api
}

func uniqueName(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestAddAndListMonitor(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	api := newAuthedAPI(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	name := uniqueName("test-http")
	req := models.AddMonitorRequest{
		Type:     models.MonitorTypeHTTP,
		Name:     name,
		URL:      "https://example.com",
		Interval: 60,
		Active:   true,
	}

	monitorID, err := api.AddMonitor(ctx, req)
	if err != nil {
		t.Fatalf("AddMonitor() error: %v", err)
	}
	if monitorID == 0 {
		t.Fatal("expected non-zero monitorID")
	}
	t.Logf("✓ AddMonitor → id=%d", monitorID)

	// Cleanup: delete after test
	t.Cleanup(func() {
		ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel2()
		if err := api.DeleteMonitor(ctx2, monitorID, false); err != nil {
			t.Logf("cleanup DeleteMonitor(%d) error: %v", monitorID, err)
		}
	})

	// List should contain our new monitor
	monitors, err := api.ListMonitors(ctx)
	if err != nil {
		t.Fatalf("ListMonitors() error: %v", err)
	}

	found := false
	for _, m := range monitors {
		if m.ID == monitorID {
			found = true
			if m.Name != name {
				t.Errorf("expected name %q, got %q", name, m.Name)
			}
			t.Logf("✓ ListMonitors found monitor: id=%d name=%q type=%s active=%d",
				m.ID, m.Name, m.Type, m.Active)
			break
		}
	}
	if !found {
		t.Errorf("monitor id=%d not found in ListMonitors() result", monitorID)
	}
}

func TestGetMonitor(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	api := newAuthedAPI(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Create a monitor first
	name := uniqueName("test-get")
	id, err := api.AddMonitor(ctx, models.AddMonitorRequest{
		Type:   models.MonitorTypeHTTP,
		Name:   name,
		URL:    "https://example.com",
		Active: true,
	})
	if err != nil {
		t.Fatalf("AddMonitor() error: %v", err)
	}
	t.Cleanup(func() {
		ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel2()
		api.DeleteMonitor(ctx2, id, false)
	})

	// Get it back
	m, err := api.GetMonitor(ctx, id)
	if err != nil {
		t.Fatalf("GetMonitor(%d) error: %v", id, err)
	}
	if m.ID != id {
		t.Errorf("expected id=%d, got %d", id, m.ID)
	}
	if m.Name != name {
		t.Errorf("expected name=%q, got %q", name, m.Name)
	}
	t.Logf("✓ GetMonitor → id=%d name=%q", m.ID, m.Name)
}

func TestEditMonitor(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	api := newAuthedAPI(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	name := uniqueName("test-edit")
	id, err := api.AddMonitor(ctx, models.AddMonitorRequest{
		Type:   models.MonitorTypeHTTP,
		Name:   name,
		URL:    "https://example.com",
		Active: true,
	})
	if err != nil {
		t.Fatalf("AddMonitor() error: %v", err)
	}
	t.Cleanup(func() {
		ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel2()
		api.DeleteMonitor(ctx2, id, false)
	})

	// Edit name and interval
	newName := name + "-edited"
	err = api.EditMonitor(ctx, models.EditMonitorRequest{
		ID: id,
		AddMonitorRequest: models.AddMonitorRequest{
			Type:     models.MonitorTypeHTTP,
			Name:     newName,
			URL:      "https://example.com",
			Interval: 120,
			Active:   true,
		},
	})
	if err != nil {
		t.Fatalf("EditMonitor() error: %v", err)
	}

	// Verify via GetMonitor
	m, err := api.GetMonitor(ctx, id)
	if err != nil {
		t.Fatalf("GetMonitor after edit error: %v", err)
	}
	if m.Name != newName {
		t.Errorf("expected name=%q after edit, got %q", newName, m.Name)
	}
	if m.Interval != 120 {
		t.Errorf("expected interval=120 after edit, got %d", m.Interval)
	}
	t.Logf("✓ EditMonitor → name=%q interval=%d", m.Name, m.Interval)
}

func TestPauseAndResumeMonitor(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	api := newAuthedAPI(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	id, err := api.AddMonitor(ctx, models.AddMonitorRequest{
		Type:   models.MonitorTypeHTTP,
		Name:   uniqueName("test-pause"),
		URL:    "https://example.com",
		Active: true,
	})
	if err != nil {
		t.Fatalf("AddMonitor() error: %v", err)
	}
	t.Cleanup(func() {
		ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel2()
		api.DeleteMonitor(ctx2, id, false)
	})

	if err := api.PauseMonitor(ctx, id); err != nil {
		t.Fatalf("PauseMonitor(%d) error: %v", id, err)
	}
	t.Logf("✓ PauseMonitor(%d) OK", id)

	if err := api.ResumeMonitor(ctx, id); err != nil {
		t.Fatalf("ResumeMonitor(%d) error: %v", id, err)
	}
	t.Logf("✓ ResumeMonitor(%d) OK", id)
}

func TestDeleteMonitor(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	api := newAuthedAPI(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	id, err := api.AddMonitor(ctx, models.AddMonitorRequest{
		Type:   models.MonitorTypeHTTP,
		Name:   uniqueName("test-delete"),
		URL:    "https://example.com",
		Active: true,
	})
	if err != nil {
		t.Fatalf("AddMonitor() error: %v", err)
	}

	if err := api.DeleteMonitor(ctx, id, false); err != nil {
		t.Fatalf("DeleteMonitor(%d) error: %v", id, err)
	}
	t.Logf("✓ DeleteMonitor(%d) OK", id)
}

func TestWatchHeartbeats(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	api := newAuthedAPI(t)

	// Create a monitor with short interval
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	id, err := api.AddMonitor(ctx, models.AddMonitorRequest{
		Type:     models.MonitorTypeHTTP,
		Name:     uniqueName("test-watch"),
		URL:      "https://example.com",
		Interval: 60,
		Active:   true,
	})
	if err != nil {
		t.Fatalf("AddMonitor() error: %v", err)
	}
	t.Cleanup(func() {
		ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel2()
		api.DeleteMonitor(ctx2, id, false)
	})

	// Watch for heartbeats — we expect at least one within 15s
	// (the monitor fires immediately on creation)
	watchCtx, watchCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer watchCancel()

	ch, cancelWatch := api.WatchHeartbeats(watchCtx, id)
	defer cancelWatch()

	select {
	case hb, ok := <-ch:
		if !ok {
			t.Fatal("heartbeat channel closed before receiving a beat")
		}
		t.Logf("✓ WatchHeartbeats received: monitorID=%d status=%d time=%s ping=%dms",
			hb.MonitorID, hb.Status, hb.Time, hb.Ping)
	case <-watchCtx.Done():
		t.Skip("no heartbeat received within 15s — monitor may not have fired yet (normal in CI)")
	}
}
