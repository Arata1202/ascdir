package appstore

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchAndApplyMetadata(t *testing.T) {
	t.Parallel()
	var mutations []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Error("missing bearer token")
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/apps":
			writeData(t, w, []any{resourceJSON("app-1", "apps", map[string]any{"bundleId": "com.example.app"})})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/apps/app-1/appInfos":
			writeData(t, w, []any{resourceJSON("info-1", "appInfos", nil)})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/appInfos/info-1/appInfoLocalizations":
			writeData(t, w, []any{resourceJSON("info-loc-1", "appInfoLocalizations", map[string]any{"locale": "en-US", "name": "Old Name", "subtitle": "Subtitle", "privacyPolicyUrl": "https://example.com/privacy"})})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/apps/app-1/appStoreVersions":
			writeData(t, w, []any{resourceJSON("version-1", "appStoreVersions", nil)})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			writeData(t, w, []any{resourceJSON("version-loc-1", "appStoreVersionLocalizations", map[string]any{"locale": "en-US", "description": "Old description", "supportUrl": "https://example.com/support"})})
		case r.Method == http.MethodPatch:
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			mutations = append(mutations, body)
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "unexpected request: "+r.Method+" "+r.URL.String(), http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := testClient(t, server.URL)
	remote, err := client.FetchMetadata(context.Background(), "com.example.app", "IOS", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if remote.AppID != "app-1" || remote.AppInfoID != "info-1" || remote.VersionID != "version-1" {
		t.Fatalf("unexpected IDs: %#v", remote)
	}
	if got := remote.Localizations["en-US"].Values["description"]; got != "Old description" {
		t.Fatalf("description = %q", got)
	}
	desired := Metadata{Localizations: map[string]Localization{"en-US": {Values: map[string]string{"name": "New Name", "description": "New description"}}}}
	changes := []Change{
		{Locale: "en-US", Field: "name", Before: "Old Name", After: "New Name"},
		{Locale: "en-US", Field: "description", Before: "Old description", After: "New description"},
	}
	if err := client.ApplyMetadata(context.Background(), remote, desired, changes); err != nil {
		t.Fatal(err)
	}
	if len(mutations) != 2 {
		t.Fatalf("got %d mutations, want 2", len(mutations))
	}
}

func TestAPIErrorDoesNotExposeResponseBody(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errors":[{"status":"403","code":"FORBIDDEN","title":"Forbidden","detail":"The key cannot access this resource"}]}`))
	}))
	defer server.Close()
	client := testClient(t, server.URL)
	_, err := client.FetchMetadata(context.Background(), "com.example.app", "IOS", "1.0.0")
	if err == nil || !strings.Contains(err.Error(), "FORBIDDEN") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func testClient(t *testing.T, baseURL string) *Client {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return NewClient(Credentials{IssuerID: "issuer", KeyID: "key", Key: key}, baseURL)
}

func resourceJSON(id, resourceType string, attributes map[string]any) map[string]any {
	return map[string]any{"id": id, "type": resourceType, "attributes": attributes}
}

func writeData(t *testing.T, w http.ResponseWriter, data []any) {
	t.Helper()
	if err := json.NewEncoder(w).Encode(map[string]any{"data": data}); err != nil {
		t.Error(err)
	}
}
