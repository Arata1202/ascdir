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
	"sync/atomic"
	"testing"
	"time"
)

func TestFetchAndApplyMetadata(t *testing.T) {
	t.Parallel()
	var mutations []map[string]any
	var mutationPaths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Error("missing bearer token")
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/apps":
			writeData(t, w, []any{resourceJSON("app-1", "apps", map[string]any{"bundleId": "com.example.app", "accessibilityUrl": "https://example.com/accessibility", "contentRightsDeclaration": "DOES_NOT_USE_THIRD_PARTY_CONTENT"})})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/apps/app-1/appInfos":
			writeData(t, w, []any{resourceJSON("info-1", "appInfos", nil)})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/appInfos/info-1/appInfoLocalizations":
			writeData(t, w, []any{resourceJSON("info-loc-1", "appInfoLocalizations", map[string]any{"locale": "en-US", "name": "Old Name", "subtitle": "Subtitle", "privacyPolicyUrl": "https://example.com/privacy"})})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/apps/app-1/appStoreVersions":
			writeData(t, w, []any{resourceJSON("version-1", "appStoreVersions", map[string]any{"copyright": "2025 Example, Inc."})})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			writeData(t, w, []any{resourceJSON("version-loc-1", "appStoreVersionLocalizations", map[string]any{"locale": "en-US", "description": "Old description", "supportUrl": "https://example.com/support"})})
		case r.Method == http.MethodPatch:
			mutationPaths = append(mutationPaths, r.URL.Path)
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
	remote, err := client.FetchMetadata(context.Background(), "", "com.example.app", "IOS", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if remote.AppID != "app-1" || remote.AppInfoID != "info-1" || remote.VersionID != "version-1" {
		t.Fatalf("unexpected IDs: %#v", remote)
	}
	if got := remote.Localizations["en-US"].Values["description"]; got != "Old description" {
		t.Fatalf("description = %q", got)
	}
	if remote.Values["copyright"] != "2025 Example, Inc." || remote.Values["accessibility_url"] != "https://example.com/accessibility" {
		t.Fatalf("global metadata = %#v", remote.Values)
	}
	if remote.Values["content_rights_declaration"] != "DOES_NOT_USE_THIRD_PARTY_CONTENT" {
		t.Fatalf("content rights declaration = %q", remote.Values["content_rights_declaration"])
	}
	changes := []Change{
		{Field: "copyright", Before: "2025 Example, Inc.", After: "2026 Example, Inc."},
		{Field: "accessibility_url", Before: "https://example.com/accessibility", After: "https://example.com/a11y"},
		{Field: "content_rights_declaration", Before: "DOES_NOT_USE_THIRD_PARTY_CONTENT", After: "USES_THIRD_PARTY_CONTENT"},
		{Locale: "en-US", Field: "name", Before: "Old Name", After: "New Name"},
		{Locale: "en-US", Field: "description", Before: "Old description", After: "New description"},
	}
	if err := client.ApplyMetadata(context.Background(), remote, []string{"en-US"}, changes); err != nil {
		t.Fatal(err)
	}
	if len(mutations) != 4 {
		t.Fatalf("got %d mutations, want 4", len(mutations))
	}
	wantPaths := []string{"/v1/apps/app-1", "/v1/appStoreVersions/version-1", "/v1/appInfoLocalizations/info-loc-1", "/v1/appStoreVersionLocalizations/version-loc-1"}
	for index := range wantPaths {
		if mutationPaths[index] != wantPaths[index] {
			t.Fatalf("mutation paths = %#v, want %#v", mutationPaths, wantPaths)
		}
	}
	data := mutations[0]["data"].(map[string]any)
	attributes := data["attributes"].(map[string]any)
	if attributes["contentRightsDeclaration"] != "USES_THIRD_PARTY_CONTENT" {
		t.Fatalf("app attributes = %#v", attributes)
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
	_, err := client.FetchMetadata(context.Background(), "", "com.example.app", "IOS", "1.0.0")
	if err == nil || !strings.Contains(err.Error(), "FORBIDDEN") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListFollowsSameOriginPagination(t *testing.T) {
	t.Parallel()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") == "2" {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{resourceJSON("app-2", "apps", nil)}, "links": map[string]any{}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":  []any{resourceJSON("app-1", "apps", nil)},
			"links": map[string]any{"next": server.URL + "/v1/apps?page=2"},
		})
	}))
	defer server.Close()
	client := testClient(t, server.URL)
	resources, err := client.list(context.Background(), "/v1/apps")
	if err != nil {
		t.Fatal(err)
	}
	if len(resources) != 2 {
		t.Fatalf("got %d resources, want 2", len(resources))
	}
}

func TestListRejectsCrossOriginPagination(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}, "links": map[string]any{"next": "https://example.com/steal-token"}})
	}))
	defer server.Close()
	client := testClient(t, server.URL)
	_, err := client.list(context.Background(), "/v1/apps")
	if err == nil || !strings.Contains(err.Error(), "changed origin") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRetriesRateLimit(t *testing.T) {
	t.Parallel()
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempt := attempts.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if attempt == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		writeData(t, w, []any{})
	}))
	defer server.Close()
	client := testClient(t, server.URL)
	client.sleep = func(context.Context, time.Duration) error { return nil }
	if _, err := client.list(context.Background(), "/v1/apps"); err != nil {
		t.Fatal(err)
	}
	if attempts.Load() != 2 {
		t.Fatalf("attempts = %d, want 2", attempts.Load())
	}
}

func TestResolveAppUsesConfiguredIDAndVerifiesBundleID(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/apps/app-1" {
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": resourceJSON("app-1", "apps", map[string]any{"bundleId": "com.other.app"})})
	}))
	defer server.Close()
	client := testClient(t, server.URL)
	_, err := client.resolveApp(context.Background(), "app-1", "com.example.app")
	if err == nil || !strings.Contains(err.Error(), "com.other.app") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSelectAppInfoMatchesVersionState(t *testing.T) {
	t.Parallel()
	infos := []resource{
		{ID: "live", Attributes: map[string]any{"state": "READY_FOR_DISTRIBUTION"}},
		{ID: "draft", Attributes: map[string]any{"state": "PREPARE_FOR_SUBMISSION"}},
	}
	selected, err := selectAppInfo(infos, "PREPARE_FOR_SUBMISSION")
	if err != nil {
		t.Fatal(err)
	}
	if selected.ID != "draft" {
		t.Fatalf("selected app info = %q", selected.ID)
	}
	if _, err := selectAppInfo(infos, "WAITING_FOR_REVIEW"); err == nil {
		t.Fatal("ambiguous app infos were accepted")
	}
}

func TestApplyMetadataCreatesMatchingLocalizationsInSafeOrder(t *testing.T) {
	t.Parallel()
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		var body struct {
			Data struct {
				Attributes map[string]string `json:"attributes"`
			} `json:"data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.Data.Attributes["locale"] != "fr-FR" {
			t.Errorf("locale = %q", body.Data.Attributes["locale"])
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()
	client := testClient(t, server.URL)
	remote := Metadata{AppInfoID: "info-1", VersionID: "version-1", Localizations: map[string]Localization{}}
	changes := []Change{
		{Locale: "fr-FR", Field: "name", After: "Exemple"},
		{Locale: "fr-FR", Field: "description", After: "Description"},
	}
	if err := client.ApplyMetadata(context.Background(), remote, []string{"fr-FR"}, changes); err != nil {
		t.Fatal(err)
	}
	want := []string{"/v1/appInfoLocalizations", "/v1/appStoreVersionLocalizations"}
	if len(paths) != len(want) {
		t.Fatalf("paths = %#v", paths)
	}
	for index := range want {
		if paths[index] != want[index] {
			t.Fatalf("paths = %#v, want %#v", paths, want)
		}
	}
}

func TestApplyMetadataCreatesMissingCounterpartLocalization(t *testing.T) {
	t.Parallel()
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()
	client := testClient(t, server.URL)
	remote := Metadata{AppInfoID: "info-1", VersionID: "version-1", Localizations: map[string]Localization{}}
	changes := []Change{{Locale: "fr-FR", Field: "description", After: "Description"}}
	if err := client.ApplyMetadata(context.Background(), remote, []string{"fr-FR"}, changes); err != nil {
		t.Fatal(err)
	}
	want := []string{"/v1/appInfoLocalizations", "/v1/appStoreVersionLocalizations"}
	if len(paths) != len(want) {
		t.Fatalf("paths = %#v, want %#v", paths, want)
	}
	for index := range want {
		if paths[index] != want[index] {
			t.Fatalf("paths = %#v, want %#v", paths, want)
		}
	}
}

func TestApplyMetadataRepairsLocaleWithoutValueChanges(t *testing.T) {
	t.Parallel()
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()
	client := testClient(t, server.URL)
	remote := Metadata{
		AppInfoID: "info-1", VersionID: "version-1",
		Localizations: map[string]Localization{"fr-FR": {AppInfoLocalizationID: "info-loc-1"}},
	}
	missing := MissingLocalizationResources(remote, []string{"fr-FR"})
	if len(missing) != 1 || missing[0] != "fr-FR.version" {
		t.Fatalf("missing = %#v", missing)
	}
	if err := client.ApplyMetadata(context.Background(), remote, []string{"fr-FR"}, nil); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != "/v1/appStoreVersionLocalizations" {
		t.Fatalf("paths = %#v", paths)
	}
}

func TestApplyMetadataRejectsUnknownField(t *testing.T) {
	t.Parallel()
	client := testClient(t, "https://example.invalid")
	err := client.ApplyMetadata(context.Background(), Metadata{}, []string{"en-US"}, []Change{{Locale: "en-US", Field: "unknown"}})
	if err == nil || !strings.Contains(err.Error(), "unsupported metadata field") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApplyMetadataClearsNullableAccessibilityURL(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/v1/apps/app-1" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected request", http.StatusNotFound)
			return
		}
		var body struct {
			Data struct {
				Attributes map[string]any `json:"attributes"`
			} `json:"data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		value, exists := body.Data.Attributes["accessibilityUrl"]
		if !exists || value != nil {
			t.Errorf("accessibilityUrl = %#v, exists = %t", value, exists)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := testClient(t, server.URL)
	err := client.ApplyMetadata(context.Background(), Metadata{AppID: "app-1"}, nil, []Change{
		{Field: "accessibility_url", Before: "https://example.com/accessibility", After: ""},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestApplyMetadataErrorIdentifiesPartialProgress(t *testing.T) {
	t.Parallel()
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		if requests == 1 {
			w.WriteHeader(http.StatusCreated)
			return
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"errors":[{"code":"INVALID","detail":"rejected"}]}`))
	}))
	defer server.Close()
	client := testClient(t, server.URL)
	remote := Metadata{AppInfoID: "info-1", VersionID: "version-1", Localizations: map[string]Localization{}}
	err := client.ApplyMetadata(context.Background(), remote, []string{"fr-FR"}, []Change{
		{Locale: "fr-FR", Field: "name", After: "Exemple"},
		{Locale: "fr-FR", Field: "description", After: "Description"},
	})
	if err == nil {
		t.Fatal("partial failure succeeded")
	}
	for _, want := range []string{"fr-FR.version", "1 successful", "INVALID"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not contain %q", err, want)
		}
	}
}

func TestAPIErrorIncludesAllErrorsAndRequestID(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-ID", "request-123")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"errors":[{"code":"FIRST","title":"First","detail":"one"},{"code":"SECOND","title":"Second","detail":"two"}]}`))
	}))
	defer server.Close()
	client := testClient(t, server.URL)
	err := client.CheckAuth(context.Background())
	if err == nil {
		t.Fatal("request succeeded")
	}
	for _, want := range []string{"FIRST", "SECOND", "request-123"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not contain %q", err, want)
		}
	}
}

func TestRetryDelayHonorsReasonableRetryAfter(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	if got := retryDelay(0, "90", now); got != 90*time.Second {
		t.Fatalf("seconds delay = %s", got)
	}
	if got := retryDelay(0, now.Add(2*time.Minute).Format(http.TimeFormat), now); got != 2*time.Minute {
		t.Fatalf("date delay = %s", got)
	}
	if got := retryDelay(2, "invalid", now); got != 4*time.Second {
		t.Fatalf("backoff delay = %s", got)
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
