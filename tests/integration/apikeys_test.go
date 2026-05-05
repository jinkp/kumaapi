package integration

import (
	"context"
	"testing"
	"time"

	"github.com/jinkp/kumaapi/internal/models"
)

func TestAddListToggleAndDeleteAPIKey(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	api := newAuthedAPI(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	resp, err := api.AddAPIKey(ctx, models.AddAPIKeyRequest{
		Name: uniqueName("test-apikey"),
	})
	if err != nil {
		t.Fatalf("AddAPIKey() error: %v", err)
	}
	if resp.KeyID == 0 || resp.Key == "" {
		t.Fatalf("expected non-empty key response, got keyID=%d key=%q", resp.KeyID, resp.Key)
	}
	t.Logf("✓ AddAPIKey → keyID=%d", resp.KeyID)

	t.Cleanup(func() {
		ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel2()
		if err := api.DeleteAPIKey(ctx2, resp.KeyID); err != nil {
			t.Logf("cleanup DeleteAPIKey(%d) error: %v", resp.KeyID, err)
		}
	})

	keys, err := api.ListAPIKeys(ctx)
	if err != nil {
		t.Fatalf("ListAPIKeys() error: %v", err)
	}

	found := false
	for _, key := range keys {
		if key.ID == resp.KeyID {
			found = true
			break
		}
	}
	if !found && len(keys) > 0 {
		t.Errorf("api key id=%d not found in ListAPIKeys() result", resp.KeyID)
	}

	if err := api.DisableAPIKey(ctx, resp.KeyID); err != nil {
		t.Fatalf("DisableAPIKey(%d) error: %v", resp.KeyID, err)
	}
	t.Logf("✓ DisableAPIKey(%d) OK", resp.KeyID)

	if err := api.EnableAPIKey(ctx, resp.KeyID); err != nil {
		t.Fatalf("EnableAPIKey(%d) error: %v", resp.KeyID, err)
	}
	t.Logf("✓ EnableAPIKey(%d) OK", resp.KeyID)

	if err := api.DeleteAPIKey(ctx, resp.KeyID); err != nil {
		t.Fatalf("DeleteAPIKey(%d) error: %v", resp.KeyID, err)
	}
	t.Logf("✓ DeleteAPIKey(%d) OK", resp.KeyID)
}
