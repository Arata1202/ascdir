package appstore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchAndAppendPriceSchedule(t *testing.T) {
	t.Parallel()
	var mutation map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/apps/app-1/appPriceSchedule":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": resourceJSON("schedule-1", "appPriceSchedules", nil)})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/appPriceSchedules/schedule-1/baseTerritory":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": resourceJSON("USA", "territories", nil)})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/appPriceSchedules/schedule-1/manualPrices":
			price := resourceJSON("price-1", "appPrices", map[string]any{"startDate": "2026-01-01"})
			price["relationships"] = map[string]any{
				"territory":     map[string]any{"data": map[string]string{"type": "territories", "id": "USA"}},
				"appPricePoint": map[string]any{"data": map[string]string{"type": "appPricePoints", "id": "point-1"}},
			}
			writeData(t, w, []any{price})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/appPriceSchedules":
			if err := json.NewDecoder(r.Body).Decode(&mutation); err != nil {
				t.Fatal(err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": resourceJSON("schedule-2", "appPriceSchedules", nil)})
		default:
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := testClient(t, server.URL)
	remote := Metadata{AppID: "app-1", Values: map[string]string{}}
	if err := client.fetchPricing(context.Background(), &remote); err != nil {
		t.Fatal(err)
	}
	if remote.PriceScheduleID != "schedule-1" || !strings.Contains(remote.Values["pricing.schedule"], `"price_point_id":"point-1"`) {
		t.Fatalf("pricing = %#v", remote)
	}
	desired := `{"base_territory":"USA","scheduled_prices":[{"price_point_id":"point-1","start_date":"2026-01-01"},{"price_point_id":"point-2","start_date":"2026-10-01"}]}`
	if err := client.ApplyMetadata(context.Background(), remote, nil, []Change{{Field: "pricing.schedule", Before: remote.Values["pricing.schedule"], After: desired}}); err != nil {
		t.Fatal(err)
	}
	if len(mutation["included"].([]any)) != 1 {
		t.Fatalf("mutation = %#v", mutation)
	}
}

func TestPricingRejectsRemovalOfScheduledPrice(t *testing.T) {
	t.Parallel()
	client := testClient(t, "https://example.invalid")
	remoteJSON := `{"base_territory":"USA","scheduled_prices":[{"price_point_id":"point-1"}]}`
	err := client.applyPricingChanges(context.Background(), Metadata{AppID: "app-1", Values: map[string]string{"pricing.schedule": remoteJSON}}, []Change{{Field: "pricing.schedule", After: `{"base_territory":"USA","scheduled_prices":[{"price_point_id":"point-2"}]}`}})
	if err == nil || !strings.Contains(err.Error(), "cannot be removed") {
		t.Fatalf("error = %v", err)
	}
}

func TestListPricePoints(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/apps/app-1/appPricePoints" || r.URL.Query().Get("filter[territory]") != "USA" {
			http.Error(w, "unexpected request", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		writeData(t, w, []any{resourceJSON("point-1", "appPricePoints", map[string]any{"customerPrice": "0.99", "proceeds": "0.70"})})
	}))
	defer server.Close()
	points, err := testClient(t, server.URL).ListPricePoints(context.Background(), "app-1", "USA")
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 1 || points[0].ID != "point-1" || points[0].CustomerPrice != "0.99" {
		t.Fatalf("points = %#v", points)
	}
}
