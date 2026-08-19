package appstore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWaitForAssetDeliveryPollsUntilComplete(t *testing.T) {
	t.Parallel()
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		state := "PROCESSING"
		if requests == 2 {
			state = "COMPLETE"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": resourceJSON("shot-1", "appScreenshots", map[string]any{"assetDeliveryState": map[string]any{"state": state}})})
	}))
	defer server.Close()
	client := testClient(t, server.URL)
	client.sleep = func(context.Context, time.Duration) error { return nil }
	if err := client.waitForAssetDelivery(context.Background(), "appScreenshots", "shot-1", "assetDeliveryState"); err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d", requests)
	}
}

func TestWaitForAssetDeliveryReturnsAppleFailure(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": resourceJSON("preview-1", "appPreviews", map[string]any{
			"videoDeliveryState": map[string]any{"state": "FAILED", "errors": []any{map[string]any{"description": "invalid codec"}}},
		})})
	}))
	defer server.Close()
	client := testClient(t, server.URL)
	err := client.waitForAssetDelivery(context.Background(), "appPreviews", "preview-1", "videoDeliveryState")
	if err == nil || !strings.Contains(err.Error(), "invalid codec") {
		t.Fatalf("error = %v", err)
	}
}
