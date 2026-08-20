package appstore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchAppStoreStatus(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/apps":
			writeData(t, w, []any{resourceJSON("app-1", "apps", map[string]any{"bundleId": "com.example.app"})})
		case "/v1/apps/app-1/appStoreVersions":
			writeData(t, w, []any{resourceJSON("version-1", "appStoreVersions", map[string]any{
				"platform": "IOS", "versionString": "1.2.0", "appVersionState": "WAITING_FOR_REVIEW", "releaseType": "MANUAL",
			})})
		case "/v1/appStoreVersions/version-1/build":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": resourceJSON("build-1", "builds", map[string]any{
				"version": "42", "processingState": "VALID", "uploadedDate": "2026-08-20T10:00:00Z", "expired": false,
			})})
		case "/v1/apps/app-1/reviewSubmissions":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{
				map[string]any{"type": "reviewSubmissions", "id": "review-old", "attributes": map[string]any{"state": "COMPLETE", "submittedDate": "2026-01-01T00:00:00Z"}, "relationships": map[string]any{"appStoreVersionForReview": map[string]any{"data": map[string]any{"type": "appStoreVersions", "id": "version-1"}}}},
				map[string]any{"type": "reviewSubmissions", "id": "review-current", "attributes": map[string]any{"state": "WAITING_FOR_REVIEW", "submittedDate": "2026-08-20T11:00:00Z"}, "relationships": map[string]any{"appStoreVersionForReview": map[string]any{"data": map[string]any{"type": "appStoreVersions", "id": "version-1"}}}},
			}})
		default:
			http.Error(w, "unexpected request: "+r.URL.String(), http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := testClient(t, server.URL)
	status, err := client.FetchAppStoreStatus(context.Background(), "", "com.example.app", "IOS", "1.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if status.VersionID != "version-1" || status.AppVersionState != "WAITING_FOR_REVIEW" || status.ReleaseType != "MANUAL" {
		t.Fatalf("status = %#v", status)
	}
	if status.Build == nil || status.Build.ID != "build-1" || status.Build.Version != "42" {
		t.Fatalf("build = %#v", status.Build)
	}
	if status.ReviewSubmission == nil || status.ReviewSubmission.ID != "review-current" {
		t.Fatalf("review = %#v", status.ReviewSubmission)
	}
}

func TestFetchTestFlightStatusSortsBuildsNewestFirst(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/apps":
			writeData(t, w, []any{resourceJSON("app-1", "apps", map[string]any{"bundleId": "com.example.app"})})
		case "/v1/apps/app-1/preReleaseVersions":
			if r.URL.Query().Has("filter[platform]") || r.URL.Query().Has("filter[version]") {
				t.Fatal("relationship endpoint does not accept platform or version filters")
			}
			writeData(t, w, []any{
				resourceJSON("pre-mac", "preReleaseVersions", map[string]any{"platform": "MAC_OS", "version": "1.2.0"}),
				resourceJSON("pre-old", "preReleaseVersions", map[string]any{"platform": "IOS", "version": "1.1.0"}),
				resourceJSON("pre-1", "preReleaseVersions", map[string]any{"platform": "IOS", "version": "1.2.0"}),
			})
		case "/v1/preReleaseVersions/pre-1/builds":
			writeData(t, w, []any{
				resourceJSON("build-41", "builds", map[string]any{"version": "41", "uploadedDate": "2026-08-19T10:00:00Z", "processingState": "VALID"}),
				resourceJSON("build-42", "builds", map[string]any{"version": "42", "uploadedDate": "2026-08-20T10:00:00Z", "processingState": "PROCESSING"}),
			})
		default:
			http.Error(w, "unexpected request: "+r.URL.String(), http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := testClient(t, server.URL)
	status, err := client.FetchTestFlightStatus(context.Background(), "", "com.example.app", "IOS", "1.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Builds) != 2 || status.Builds[0].Version != "42" || status.Builds[1].Version != "41" {
		t.Fatalf("builds = %#v", status.Builds)
	}
}

func TestFetchTestFlightStatusAllowsMissingPrereleaseVersion(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/apps":
			writeData(t, w, []any{resourceJSON("app-1", "apps", map[string]any{"bundleId": "com.example.app"})})
		case "/v1/apps/app-1/preReleaseVersions":
			writeData(t, w, []any{})
		default:
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := testClient(t, server.URL)
	status, err := client.FetchTestFlightStatus(context.Background(), "", "com.example.app", "IOS", "1.2.0")
	if err != nil || len(status.Builds) != 0 {
		t.Fatalf("status=%#v err=%v", status, err)
	}
}
