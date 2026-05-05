package integration

import (
	"context"
	"testing"
	"time"

	"github.com/jinkp/kumaapi/internal/models"
)

func TestAddListAndDeleteMaintenance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	api := newAuthedAPI(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	id, err := api.AddMaintenance(ctx, models.Maintenance{
		Title:       uniqueName("test-maintenance"),
		Description: "integration test",
		Strategy:    "manual",
		Active:      models.FlexInt(1),
	})
	if err != nil {
		t.Fatalf("AddMaintenance() error: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero maintenance id")
	}
	t.Logf("✓ AddMaintenance → id=%d", id)

	t.Cleanup(func() {
		ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel2()
		if err := api.DeleteMaintenance(ctx2, id); err != nil {
			t.Logf("cleanup DeleteMaintenance(%d) error: %v", id, err)
		}
	})

	list, err := api.ListMaintenances(ctx)
	if err != nil {
		t.Fatalf("ListMaintenances() error: %v", err)
	}

	found := false
	for _, m := range list {
		if m.ID == id {
			found = true
			break
		}
	}
	if !found && len(list) > 0 {
		t.Errorf("maintenance id=%d not found in ListMaintenances() result", id)
	}

	if err := api.DeleteMaintenance(ctx, id); err != nil {
		t.Fatalf("DeleteMaintenance(%d) error: %v", id, err)
	}
	t.Logf("✓ DeleteMaintenance(%d) OK", id)
}
