package appstore

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

func TestPlanAppStoreSubmissionForNewVersionIsReadOnly(t *testing.T) {
	t.Parallel()
	var mutations int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet {
			mutations++
			t.Fatalf("planning sent mutation %s %s", r.Method, r.URL.Path)
		}
		switch r.URL.Path {
		case "/v1/apps/app-1":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": resourceJSON("app-1", "apps", map[string]any{"bundleId": "com.example.app"})})
		case "/v1/apps/app-1/appStoreVersions":
			writeData(t, w, []any{})
		default:
			http.Error(w, "unexpected request: "+r.URL.String(), http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := testClient(t, server.URL)
	plan, err := client.PlanAppStoreSubmission(context.Background(), "app-1", "com.example.app", "IOS", "1.2.0", SubmitOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if mutations != 0 || plan.Build != "" || plan.ReleaseType != "MANUAL" || len(plan.Operations) != 1 || plan.Operations[0].Resource != "app-store-version" {
		t.Fatalf("plan=%#v mutations=%d", plan, mutations)
	}
}

func TestApplyAppStoreSubmissionCreatesOnlyNewVersion(t *testing.T) {
	t.Parallel()
	var mutations []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/apps/app-1":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": resourceJSON("app-1", "apps", map[string]any{"bundleId": "com.example.app"})})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/apps/app-1/appStoreVersions":
			writeData(t, w, []any{})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/appStoreVersions":
			mutations = append(mutations, r.URL.Path)
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			if !json.Valid(body) || !strings.Contains(string(body), `"versionString":"1.2.0"`) ||
				!strings.Contains(string(body), `"releaseType":"MANUAL"`) || !strings.Contains(string(body), `"id":"app-1"`) {
				t.Fatalf("create body = %s", body)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": resourceJSON("version-1", "appStoreVersions", nil)})
		default:
			http.Error(w, "unexpected request: "+r.Method+" "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := testClient(t, server.URL)
	plan, err := client.PlanAppStoreSubmission(context.Background(), "app-1", "com.example.app", "IOS", "1.2.0", SubmitOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.ApplyAppStoreSubmission(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(mutations, []string{"/v1/appStoreVersions"}) {
		t.Fatalf("mutations = %#v", mutations)
	}
}

func TestApplyAppStoreSubmissionUsesPlannedResources(t *testing.T) {
	t.Parallel()
	wantPaths := []string{
		"GET /v1/apps/app-1",
		"GET /v1/apps/app-1/appStoreVersions",
		"GET /v1/apps/app-1/preReleaseVersions",
		"GET /v1/preReleaseVersions/pre-1/builds",
		"GET /v1/appStoreVersions/version-1/build",
		"GET /v1/apps/app-1/reviewSubmissions",
		"PATCH /v1/appStoreVersions/version-1",
		"PATCH /v1/appStoreVersions/version-1/relationships/build",
		"POST /v1/reviewSubmissions",
		"POST /v1/reviewSubmissionItems",
		"PATCH /v1/reviewSubmissions/review-1",
	}
	var gotPaths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		gotPaths = append(gotPaths, r.Method+" "+r.URL.Path)
		bodyText := ""
		if r.Method != http.MethodGet {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			if !json.Valid(body) {
				t.Fatalf("invalid JSON body: %q", body)
			}
			bodyText = string(body)
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/apps/app-1":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": resourceJSON("app-1", "apps", map[string]any{"bundleId": "com.example.app"})})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/apps/app-1/appStoreVersions":
			writeData(t, w, []any{resourceJSON("version-1", "appStoreVersions", map[string]any{"appVersionState": "PREPARE_FOR_SUBMISSION", "releaseType": "MANUAL"})})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/apps/app-1/preReleaseVersions":
			writeData(t, w, []any{resourceJSON("pre-1", "preReleaseVersions", map[string]any{"platform": "IOS", "version": "1.2.0"})})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/preReleaseVersions/pre-1/builds":
			writeData(t, w, []any{resourceJSON("build-42", "builds", map[string]any{"version": "42", "uploadedDate": "2026-08-20T00:00:00Z", "processingState": "VALID"})})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/appStoreVersions/version-1/build":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			writeData(t, w, []any{})
		case r.Method == http.MethodPatch && r.URL.Path == "/v1/appStoreVersions/version-1":
			assertJSONContains(t, bodyText, `"type":"appStoreVersions"`, `"id":"version-1"`, `"releaseType":"AFTER_APPROVAL"`, `"earliestReleaseDate":null`)
		case r.Method == http.MethodPatch && r.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			assertJSONContains(t, bodyText, `"type":"builds"`, `"id":"build-42"`)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/reviewSubmissions":
			assertJSONContains(t, bodyText, `"type":"reviewSubmissions"`, `"platform":"IOS"`, `"type":"apps"`, `"id":"app-1"`)
			_ = json.NewEncoder(w).Encode(map[string]any{"data": resourceJSON("review-1", "reviewSubmissions", nil)})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/reviewSubmissionItems":
			assertJSONContains(t, bodyText, `"type":"reviewSubmissionItems"`, `"id":"review-1"`, `"id":"version-1"`)
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPatch && r.URL.Path == "/v1/reviewSubmissions/review-1":
			assertJSONContains(t, bodyText, `"type":"reviewSubmissions"`, `"id":"review-1"`, `"submitted":true`)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()
	client := testClient(t, server.URL)
	plan := AppStoreReleasePlan{
		Kind: "app-store-submit", AppID: "app-1", BundleID: "com.example.app", Platform: "IOS", Version: "1.2.0", Build: "42", ReleaseType: "AFTER_APPROVAL", versionID: "version-1", buildID: "build-42",
		Operations: []ReleaseOperation{{Action: "update", Resource: "release-settings", Description: "Set release type to AFTER_APPROVAL"}, {Action: "select", Resource: "build", Description: "Select build 42"}, {Action: "create", Resource: "review-submission", Description: "Create an App Review submission"}, {Action: "add", Resource: "review-submission-item", Description: "Add version 1.2.0 to the submission"}, {Action: "submit", Resource: "review-submission", Description: "Submit for App Review"}},
	}
	if err := client.ApplyAppStoreSubmission(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(gotPaths, wantPaths) {
		t.Fatalf("requests = %#v, want %#v", gotPaths, wantPaths)
	}
}

func assertJSONContains(t *testing.T, body string, fragments ...string) {
	t.Helper()
	for _, fragment := range fragments {
		if !strings.Contains(body, fragment) {
			t.Fatalf("JSON body %s does not contain %s", body, fragment)
		}
	}
}

func TestApplyAppStoreSubmissionRejectsConcurrentBuildChange(t *testing.T) {
	t.Parallel()
	selectedOther := false
	mutations := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet {
			mutations++
			t.Fatalf("unexpected mutation %s %s", r.Method, r.URL.Path)
		}
		switch r.URL.Path {
		case "/v1/apps/app-1":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": resourceJSON("app-1", "apps", map[string]any{"bundleId": "com.example.app"})})
		case "/v1/apps/app-1/appStoreVersions":
			writeData(t, w, []any{resourceJSON("version-1", "appStoreVersions", map[string]any{"appVersionState": "PREPARE_FOR_SUBMISSION", "releaseType": "MANUAL"})})
		case "/v1/apps/app-1/preReleaseVersions":
			writeData(t, w, []any{resourceJSON("pre-1", "preReleaseVersions", map[string]any{"platform": "IOS", "version": "1.2.0"})})
		case "/v1/preReleaseVersions/pre-1/builds":
			writeData(t, w, []any{resourceJSON("build-42", "builds", map[string]any{"version": "42", "uploadedDate": "2026-08-20T00:00:00Z", "processingState": "VALID"})})
		case "/v1/appStoreVersions/version-1/build":
			if selectedOther {
				_ = json.NewEncoder(w).Encode(map[string]any{"data": resourceJSON("build-41", "builds", map[string]any{"version": "41", "processingState": "VALID"})})
			} else {
				_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
			}
		case "/v1/apps/app-1/reviewSubmissions":
			writeData(t, w, []any{})
		default:
			http.Error(w, "unexpected request: "+r.URL.String(), http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := testClient(t, server.URL)
	plan, err := client.PlanAppStoreSubmission(context.Background(), "app-1", "com.example.app", "IOS", "1.2.0", SubmitOptions{BuildVersion: "42"})
	if err != nil {
		t.Fatal(err)
	}
	selectedOther = true
	if err := client.ApplyAppStoreSubmission(context.Background(), plan); err == nil || !strings.Contains(err.Error(), "refusing to replace") {
		t.Fatalf("error = %v", err)
	}
	if mutations != 0 {
		t.Fatalf("mutations = %d", mutations)
	}
}

func TestPlanAppStoreSubmissionIsIdempotentAfterSubmission(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/apps/app-1":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": resourceJSON("app-1", "apps", map[string]any{"bundleId": "com.example.app"})})
		case "/v1/apps/app-1/appStoreVersions":
			writeData(t, w, []any{resourceJSON("version-1", "appStoreVersions", map[string]any{"appVersionState": "WAITING_FOR_REVIEW"})})
		default:
			t.Fatalf("idempotent plan made unnecessary request %s", r.URL.Path)
		}
	}))
	defer server.Close()
	plan, err := testClient(t, server.URL).PlanAppStoreSubmission(context.Background(), "app-1", "com.example.app", "IOS", "1.2.0", SubmitOptions{})
	if err != nil || len(plan.Operations) != 0 {
		t.Fatalf("plan=%#v err=%v", plan, err)
	}
}

func TestPlanAppStoreSubmissionRejectsIgnoredReleaseIntent(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/apps/app-1":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": resourceJSON("app-1", "apps", map[string]any{"bundleId": "com.example.app"})})
		case "/v1/apps/app-1/appStoreVersions":
			writeData(t, w, []any{resourceJSON("version-1", "appStoreVersions", map[string]any{"appVersionState": "WAITING_FOR_REVIEW", "releaseType": "AFTER_APPROVAL"})})
		default:
			t.Fatalf("unexpected request %s", r.URL.Path)
		}
	}))
	defer server.Close()
	_, err := testClient(t, server.URL).PlanAppStoreSubmission(context.Background(), "app-1", "com.example.app", "IOS", "1.2.0", SubmitOptions{ReleaseType: "MANUAL"})
	if err == nil || !strings.Contains(err.Error(), "already submitted with release type AFTER_APPROVAL") {
		t.Fatalf("error = %v", err)
	}
}

func TestPlanAndApplyAppStoreRelease(t *testing.T) {
	t.Parallel()
	var released bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/apps/app-1":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": resourceJSON("app-1", "apps", map[string]any{"bundleId": "com.example.app"})})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/apps/app-1/appStoreVersions":
			state := "PENDING_DEVELOPER_RELEASE"
			if released {
				state = "PROCESSING_FOR_DISTRIBUTION"
			}
			writeData(t, w, []any{resourceJSON("version-1", "appStoreVersions", map[string]any{"platform": "IOS", "versionString": "1.2.0", "appVersionState": state, "releaseType": "MANUAL"})})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/appStoreVersions/version-1/build":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			writeData(t, w, []any{})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/appStoreVersionReleaseRequests":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			assertJSONContains(t, string(body), `"type":"appStoreVersionReleaseRequests"`, `"type":"appStoreVersions"`, `"id":"version-1"`)
			released = true
			w.WriteHeader(http.StatusCreated)
		default:
			http.Error(w, "unexpected request: "+r.Method+" "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := testClient(t, server.URL)
	plan, err := client.PlanAppStoreRelease(context.Background(), "app-1", "com.example.app", "IOS", "1.2.0")
	if err != nil || len(plan.Operations) != 1 {
		t.Fatalf("plan=%#v err=%v", plan, err)
	}
	if err := client.ApplyAppStoreRelease(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	second, err := client.PlanAppStoreRelease(context.Background(), "app-1", "com.example.app", "IOS", "1.2.0")
	if err != nil || len(second.Operations) != 0 {
		t.Fatalf("second plan=%#v err=%v", second, err)
	}
}

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
