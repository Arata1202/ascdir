package appstore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchAndApplyTerritoryAvailability(t *testing.T) {
	t.Parallel()
	var mutation map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/apps/app-1/appAvailabilityV2":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": resourceJSON("availability-1", "appAvailabilities", map[string]any{"availableInNewTerritories": true})})
		case r.Method == http.MethodGet && r.URL.Path == "/v2/appAvailabilities/availability-1/territoryAvailabilities":
			item := resourceJSON("territory-1", "territoryAvailabilities", map[string]any{"available": true, "releaseDate": "2026-09-01", "preOrderEnabled": false})
			item["relationships"] = map[string]any{"territory": map[string]any{"data": map[string]string{"type": "territories", "id": "JPN"}}}
			writeData(t, w, []any{item})
		case r.Method == http.MethodPatch && r.URL.Path == "/v1/territoryAvailabilities/territory-1":
			if err := json.NewDecoder(r.Body).Decode(&mutation); err != nil {
				t.Fatal(err)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := testClient(t, server.URL)
	metadata := Metadata{AppID: "app-1", Values: map[string]string{}, TerritoryAvailabilityIDs: map[string]string{}}
	if err := client.fetchAvailability(context.Background(), &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata.AvailabilityID != "availability-1" || metadata.Values["availability.territories.JPN.available"] != "true" || metadata.TerritoryAvailabilityIDs["JPN"] != "territory-1" {
		t.Fatalf("availability = %#v", metadata)
	}
	changes := []Change{{Field: "availability.territories.JPN.available", After: "false"}, {Field: "availability.territories.JPN.release_date", After: ""}}
	if err := client.ApplyMetadata(context.Background(), metadata, nil, changes); err != nil {
		t.Fatal(err)
	}
	attributes := mutation["data"].(map[string]any)["attributes"].(map[string]any)
	if attributes["available"] != false || attributes["releaseDate"] != nil {
		t.Fatalf("attributes = %#v", attributes)
	}
}

func TestCreateAvailabilityUsesInlineTerritories(t *testing.T) {
	t.Parallel()
	var mutation map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v2/appAvailabilities" {
			http.Error(w, "unexpected request", http.StatusNotFound)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&mutation); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": resourceJSON("availability-1", "appAvailabilities", nil)})
	}))
	defer server.Close()
	client := testClient(t, server.URL)
	err := client.createAvailability(context.Background(), "app-1", map[string]string{
		"availability.available_in_new_territories":      "false",
		"availability.territories.USA.available":         "true",
		"availability.territories.USA.pre_order_enabled": "true",
		"availability.territories.USA.release_date":      "2026-09-01",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(mutation["included"].([]any)) != 1 {
		t.Fatalf("mutation = %#v", mutation)
	}
}

func TestFetchAvailabilityTreatsNotFoundAsUncreated(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"errors": []any{map[string]any{"status": "404", "code": "NOT_FOUND", "title": "Not Found", "detail": "missing"}}})
	}))
	defer server.Close()
	metadata := Metadata{AppID: "app-1", Values: map[string]string{}, TerritoryAvailabilityIDs: map[string]string{}}
	if err := testClient(t, server.URL).fetchAvailability(context.Background(), &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata.AvailabilityID != "" {
		t.Fatalf("availability ID = %q", metadata.AvailabilityID)
	}
}
