package integration

import (
	"context"
	"testing"
	"time"

	"github.com/jinkp/kumaapi/internal/models"
)

func TestAddListDeleteAndAttachTag(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	api := newAuthedAPI(t)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	tag, err := api.AddTag(ctx, uniqueName("test-tag"), "#00AAFF")
	if err != nil {
		t.Fatalf("AddTag() error: %v", err)
	}
	if tag == nil || tag.ID == 0 {
		t.Fatal("expected non-nil tag with non-zero ID")
	}
	t.Logf("✓ AddTag → id=%d", tag.ID)

	t.Cleanup(func() {
		ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel2()
		if err := api.DeleteTag(ctx2, tag.ID); err != nil {
			t.Logf("cleanup DeleteTag(%d) error: %v", tag.ID, err)
		}
	})

	tags, err := api.ListTags(ctx)
	if err != nil {
		t.Fatalf("ListTags() error: %v", err)
	}

	found := false
	for _, item := range tags {
		if item.ID == tag.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("tag id=%d not found in ListTags() result", tag.ID)
	}

	monitorID, err := api.AddMonitor(ctx, models.AddMonitorRequest{
		Type:   models.MonitorTypeHTTP,
		Name:   uniqueName("tag-monitor"),
		URL:    "https://example.com",
		Active: true,
	})
	if err != nil {
		t.Fatalf("AddMonitor() for tag relation error: %v", err)
	}
	t.Cleanup(func() {
		ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel2()
		if err := api.DeleteMonitor(ctx2, monitorID, false); err != nil {
			t.Logf("cleanup DeleteMonitor(%d) error: %v", monitorID, err)
		}
	})

	if err := api.AddMonitorTag(ctx, tag.ID, monitorID, "integration"); err != nil {
		t.Fatalf("AddMonitorTag() error: %v", err)
	}
	t.Logf("✓ AddMonitorTag(tag=%d, monitor=%d) OK", tag.ID, monitorID)

	if err := api.DeleteMonitorTag(ctx, tag.ID, monitorID, "integration"); err != nil {
		t.Fatalf("DeleteMonitorTag() error: %v", err)
	}
	t.Logf("✓ DeleteMonitorTag(tag=%d, monitor=%d) OK", tag.ID, monitorID)

	if err := api.DeleteTag(ctx, tag.ID); err != nil {
		t.Fatalf("DeleteTag(%d) error: %v", tag.ID, err)
	}
	t.Logf("✓ DeleteTag(%d) OK", tag.ID)
}
