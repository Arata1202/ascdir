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

func TestPlanTestFlightDistributionInternalGroupNeedsNoBetaReview(t *testing.T) {
	t.Parallel()
	server := distributionTestServer(t, distributionFixture{groups: []distributionGroupFixture{{id: "group-internal", name: "Team", internal: true}}})
	defer server.Close()
	plan, err := testClient(t, server.URL).PlanTestFlightDistribution(context.Background(), "app-1", "com.example.app", "IOS", "1.2.0", TestFlightDistributionOptions{GroupNames: []string{"Team"}})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Build != "42" || len(plan.Groups) != 1 || !plan.Groups[0].Internal || len(plan.Operations) != 1 || plan.Operations[0].Resource != "beta-group:Team" {
		t.Fatalf("plan = %#v", plan)
	}
}

func TestPlanTestFlightDistributionExternalGroupSubmitsBetaReview(t *testing.T) {
	t.Parallel()
	server := distributionTestServer(t, distributionFixture{groups: []distributionGroupFixture{{id: "group-external", name: "Public", internal: false}}, reviewNotFound: true})
	defer server.Close()
	plan, err := testClient(t, server.URL).PlanTestFlightDistribution(context.Background(), "app-1", "com.example.app", "IOS", "1.2.0", TestFlightDistributionOptions{BuildVersion: "42", GroupNames: []string{"Public"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Operations) != 2 || plan.Operations[0].Resource != "beta-group:Public" || plan.Operations[1].Resource != "beta-app-review" {
		t.Fatalf("operations = %#v", plan.Operations)
	}
}

func TestPlanTestFlightDistributionReusesActiveBetaReview(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"WAITING_FOR_REVIEW", "IN_REVIEW", "APPROVED"} {
		state := state
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			server := distributionTestServer(t, distributionFixture{groups: []distributionGroupFixture{{id: "group-external", name: "Public", internal: false}}, reviewState: state})
			defer server.Close()
			plan, err := testClient(t, server.URL).PlanTestFlightDistribution(context.Background(), "app-1", "com.example.app", "IOS", "1.2.0", TestFlightDistributionOptions{GroupNames: []string{"Public"}})
			if err != nil {
				t.Fatal(err)
			}
			if len(plan.Operations) != 1 || plan.Operations[0].Resource != "beta-group:Public" || plan.BetaReviewState != state {
				t.Fatalf("plan = %#v", plan)
			}
		})
	}
}

func TestPlanTestFlightDistributionResubmitsRejectedBuild(t *testing.T) {
	t.Parallel()
	server := distributionTestServer(t, distributionFixture{groups: []distributionGroupFixture{{id: "group-external", name: "Public", internal: false}}, reviewState: "REJECTED"})
	defer server.Close()
	plan, err := testClient(t, server.URL).PlanTestFlightDistribution(context.Background(), "app-1", "com.example.app", "IOS", "1.2.0", TestFlightDistributionOptions{GroupNames: []string{"Public"}})
	if err != nil || len(plan.Operations) != 2 || plan.Operations[1].Resource != "beta-app-review" {
		t.Fatalf("plan=%#v err=%v", plan, err)
	}
}

func TestPlanTestFlightDistributionRejectsInternalOnlyBuildForExternalGroup(t *testing.T) {
	t.Parallel()
	server := distributionTestServer(t, distributionFixture{
		groups: []distributionGroupFixture{{id: "group-external", name: "Public", internal: false}}, buildAudienceType: "INTERNAL_ONLY",
	})
	defer server.Close()
	_, err := testClient(t, server.URL).PlanTestFlightDistribution(context.Background(), "app-1", "com.example.app", "IOS", "1.2.0", TestFlightDistributionOptions{GroupNames: []string{"Public"}})
	if err == nil || !strings.Contains(err.Error(), "cannot be distributed to external groups") {
		t.Fatalf("error = %v", err)
	}
}

func TestPlanTestFlightDistributionIsIdempotentWhenAttachedAndApproved(t *testing.T) {
	t.Parallel()
	server := distributionTestServer(t, distributionFixture{
		groups: []distributionGroupFixture{{id: "group-external", name: "Public", internal: false, attached: true}}, reviewState: "APPROVED",
	})
	defer server.Close()
	plan, err := testClient(t, server.URL).PlanTestFlightDistribution(context.Background(), "app-1", "com.example.app", "IOS", "1.2.0", TestFlightDistributionOptions{GroupNames: []string{"Public"}})
	if err != nil || len(plan.Operations) != 0 || plan.BetaReviewState != "APPROVED" {
		t.Fatalf("plan=%#v err=%v", plan, err)
	}
}

func TestApplyTestFlightDistributionUsesExactRequestBodies(t *testing.T) {
	t.Parallel()
	var mutations []string
	fixture := distributionFixture{groups: []distributionGroupFixture{{id: "group-external", name: "Public", internal: false}}, reviewNotFound: true, mutation: func(r *http.Request, body []byte) {
		mutations = append(mutations, r.Method+" "+r.URL.Path+" "+string(body))
	}}
	server := distributionTestServer(t, fixture)
	defer server.Close()
	client := testClient(t, server.URL)
	plan, err := client.PlanTestFlightDistribution(context.Background(), "app-1", "com.example.app", "IOS", "1.2.0", TestFlightDistributionOptions{BuildVersion: "42", GroupNames: []string{"Public"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.ApplyTestFlightDistribution(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	want := []string{
		`POST /v1/betaGroups/group-external/relationships/builds {"data":[{"id":"build-42","type":"builds"}]}`,
		`POST /v1/betaAppReviewSubmissions {"data":{"relationships":{"build":{"data":{"id":"build-42","type":"builds"}}},"type":"betaAppReviewSubmissions"}}`,
	}
	if !slices.Equal(mutations, want) {
		t.Fatalf("mutations = %#v, want %#v", mutations, want)
	}
}

func TestApplyTestFlightDistributionRejectsConcurrentMembershipChange(t *testing.T) {
	t.Parallel()
	attached := false
	mutations := 0
	server := distributionTestServer(t, distributionFixture{
		groups:           []distributionGroupFixture{{id: "group-internal", name: "Team", internal: true}},
		attachedOverride: func() bool { return attached },
		mutation:         func(_ *http.Request, _ []byte) { mutations++ },
	})
	defer server.Close()
	client := testClient(t, server.URL)
	plan, err := client.PlanTestFlightDistribution(context.Background(), "app-1", "com.example.app", "IOS", "1.2.0", TestFlightDistributionOptions{GroupNames: []string{"Team"}})
	if err != nil {
		t.Fatal(err)
	}
	attached = true
	err = client.ApplyTestFlightDistribution(context.Background(), plan)
	if err == nil || !strings.Contains(err.Error(), "changed after planning") || mutations != 0 {
		t.Fatalf("err=%v mutations=%d", err, mutations)
	}
}

func TestPlanTestFlightDistributionRejectsUnknownGroup(t *testing.T) {
	t.Parallel()
	server := distributionTestServer(t, distributionFixture{})
	defer server.Close()
	_, err := testClient(t, server.URL).PlanTestFlightDistribution(context.Background(), "app-1", "com.example.app", "IOS", "1.2.0", TestFlightDistributionOptions{GroupNames: []string{"Missing"}})
	if err == nil || !strings.Contains(err.Error(), "does not create groups") {
		t.Fatalf("error = %v", err)
	}
}

type distributionGroupFixture struct {
	id, name           string
	internal, attached bool
}

type distributionFixture struct {
	groups            []distributionGroupFixture
	buildAudienceType string
	reviewState       string
	reviewNotFound    bool
	attachedOverride  func() bool
	mutation          func(*http.Request, []byte)
}

func distributionTestServer(t *testing.T, fixture distributionFixture) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/apps/app-1":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": resourceJSON("app-1", "apps", map[string]any{"bundleId": "com.example.app"})})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/apps/app-1/preReleaseVersions":
			writeData(t, w, []any{resourceJSON("pre-1", "preReleaseVersions", map[string]any{"platform": "IOS", "version": "1.2.0"})})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/preReleaseVersions/pre-1/builds":
			audienceType := fixture.buildAudienceType
			if audienceType == "" {
				audienceType = "APP_STORE_ELIGIBLE"
			}
			writeData(t, w, []any{resourceJSON("build-42", "builds", map[string]any{"version": "42", "uploadedDate": "2026-08-20T00:00:00Z", "processingState": "VALID", "buildAudienceType": audienceType, "expired": false})})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/apps/app-1/betaGroups":
			items := make([]any, 0, len(fixture.groups))
			for _, group := range fixture.groups {
				items = append(items, resourceJSON(group.id, "betaGroups", map[string]any{"name": group.name, "isInternalGroup": group.internal}))
			}
			writeData(t, w, items)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/betaGroups/") && strings.HasSuffix(r.URL.Path, "/builds"):
			attached := false
			for _, group := range fixture.groups {
				if r.URL.Path == "/v1/betaGroups/"+group.id+"/builds" {
					attached = group.attached
				}
			}
			if fixture.attachedOverride != nil {
				attached = fixture.attachedOverride()
			}
			if attached {
				writeData(t, w, []any{resourceJSON("build-42", "builds", map[string]any{"version": "42"})})
			} else {
				writeData(t, w, []any{})
			}
		case r.Method == http.MethodGet && r.URL.Path == "/v1/builds/build-42/betaAppReviewSubmission":
			if fixture.reviewNotFound || fixture.reviewState == "" {
				http.Error(w, `{"errors":[{"status":"404","detail":"not found"}]}`, http.StatusNotFound)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": resourceJSON("review-1", "betaAppReviewSubmissions", map[string]any{"betaReviewState": fixture.reviewState})})
		case r.Method == http.MethodPost:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			if !json.Valid(body) {
				t.Fatalf("invalid JSON body: %q", body)
			}
			if fixture.mutation != nil {
				fixture.mutation(r, body)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "unexpected request: "+r.Method+" "+r.URL.String(), http.StatusNotFound)
		}
	}))
}
