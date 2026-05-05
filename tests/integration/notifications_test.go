package integration

import (
	"context"
	"testing"
	"time"
)

func TestAddListAndDeleteNotification(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	api := newAuthedAPI(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	payload := map[string]any{
		"name":               uniqueName("test-webhook"),
		"type":               "webhook",
		"active":             true,
		"isDefault":          false,
		"webhookURL":         "https://example.com/webhook",
		"webhookContentType": "json",
		"applyExisting":      false,
	}

	id, err := api.AddNotification(ctx, payload, nil)
	if err != nil {
		t.Fatalf("AddNotification() error: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero notification id")
	}
	t.Logf("✓ AddNotification → id=%d", id)

	t.Cleanup(func() {
		ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel2()
		if err := api.DeleteNotification(ctx2, id); err != nil {
			t.Logf("cleanup DeleteNotification(%d) error: %v", id, err)
		}
	})

	notifications, err := api.ListNotifications(ctx)
	if err != nil {
		t.Fatalf("ListNotifications() error: %v", err)
	}

	found := false
	for _, n := range notifications {
		if n.ID == id {
			found = true
			if n.Name != payload["name"] {
				t.Errorf("expected name %q, got %q", payload["name"], n.Name)
			}
			break
		}
	}
	if !found {
		t.Errorf("notification id=%d not found in ListNotifications() result", id)
	}

	if err := api.DeleteNotification(ctx, id); err != nil {
		t.Fatalf("DeleteNotification(%d) error: %v", id, err)
	}
	t.Logf("✓ DeleteNotification(%d) OK", id)
}
