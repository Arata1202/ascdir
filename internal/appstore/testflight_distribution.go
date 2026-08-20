package appstore

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

// TestFlightGroup is a resolved beta group in a distribution plan.
type TestFlightGroup struct {
	Name       string `json:"name"`
	Internal   bool   `json:"internal"`
	Attached   bool   `json:"attached"`
	resourceID string
}

// TestFlightDistributionPlan is the read-only preflight shared by dry-run and
// execution. Resource IDs remain private so only a plan created by this client
// can be applied.
type TestFlightDistributionPlan struct {
	Kind            string             `json:"kind"`
	AppID           string             `json:"app_id"`
	BundleID        string             `json:"bundle_id"`
	Platform        string             `json:"platform"`
	Version         string             `json:"version"`
	Build           string             `json:"build"`
	BetaReviewState string             `json:"beta_review_state,omitempty"`
	Groups          []TestFlightGroup  `json:"groups"`
	Operations      []ReleaseOperation `json:"operations"`
	buildID         string
}

type TestFlightDistributionOptions struct {
	BuildVersion string
	GroupNames   []string
}

func (c *Client) PlanTestFlightDistribution(ctx context.Context, appID, bundleID, platform, version string, options TestFlightDistributionOptions) (TestFlightDistributionPlan, error) {
	app, err := c.resolveApp(ctx, appID, bundleID)
	if err != nil {
		return TestFlightDistributionPlan{}, err
	}
	groupNames, err := normalizeGroupNames(options.GroupNames)
	if err != nil {
		return TestFlightDistributionPlan{}, err
	}
	builds, err := c.fetchTestFlightBuilds(ctx, app.ID, platform, version)
	if err != nil {
		return TestFlightDistributionPlan{}, fmt.Errorf("read candidate builds: %w", err)
	}
	build, err := selectDistributionBuild(builds, strings.TrimSpace(options.BuildVersion), platform, version)
	if err != nil {
		return TestFlightDistributionPlan{}, err
	}
	plan := TestFlightDistributionPlan{
		Kind: "testflight-distribute", AppID: app.ID, BundleID: bundleID, Platform: platform,
		Version: version, Build: build.Version, Groups: []TestFlightGroup{}, Operations: []ReleaseOperation{}, buildID: build.ID,
	}

	groups, err := c.list(ctx, "/v1/apps/"+url.PathEscape(app.ID)+"/betaGroups?fields%5BbetaGroups%5D=name,isInternalGroup&limit=200")
	if err != nil {
		return TestFlightDistributionPlan{}, fmt.Errorf("read beta groups: %w", err)
	}
	byName := map[string][]resource{}
	for _, group := range groups {
		name := stringAttribute(group, "name")
		byName[name] = append(byName[name], group)
	}
	hasExternal := false
	for _, name := range groupNames {
		matches := byName[name]
		if len(matches) == 0 {
			return TestFlightDistributionPlan{}, fmt.Errorf("beta group %q was not found for the configured app; ascdir does not create groups", name)
		}
		if len(matches) > 1 {
			return TestFlightDistributionPlan{}, fmt.Errorf("multiple beta groups are named %q; use unique group names in App Store Connect", name)
		}
		group := TestFlightGroup{Name: name, Internal: boolAttribute(matches[0], "isInternalGroup"), resourceID: matches[0].ID}
		group.Attached, err = c.betaGroupHasBuild(ctx, group.resourceID, build.ID)
		if err != nil {
			return TestFlightDistributionPlan{}, err
		}
		if !group.Internal {
			hasExternal = true
		}
		if !group.Attached {
			plan.Operations = append(plan.Operations, ReleaseOperation{Action: "attach", Resource: "beta-group:" + name, Description: fmt.Sprintf("Attach build %s to beta group %s", build.Version, name)})
		}
		plan.Groups = append(plan.Groups, group)
	}
	if hasExternal {
		if build.AudienceType != "APP_STORE_ELIGIBLE" {
			return TestFlightDistributionPlan{}, fmt.Errorf("build %s has audience type %s and cannot be distributed to external groups; upload an APP_STORE_ELIGIBLE build", build.Version, displayState(build.AudienceType))
		}
		submission, err := c.fetchBetaAppReviewSubmission(ctx, build.ID)
		if err != nil {
			return TestFlightDistributionPlan{}, err
		}
		if submission != nil {
			plan.BetaReviewState = submission.State
		}
		switch plan.BetaReviewState {
		case "APPROVED", "WAITING_FOR_REVIEW", "IN_REVIEW":
			// The existing submission is sufficient. Group attachment remains
			// independently idempotent.
		case "", "REJECTED":
			plan.Operations = append(plan.Operations, ReleaseOperation{Action: "submit", Resource: "beta-app-review", Description: fmt.Sprintf("Submit build %s for Beta App Review", build.Version)})
		default:
			return TestFlightDistributionPlan{}, fmt.Errorf("build %s has unsupported Beta App Review state %s", build.Version, plan.BetaReviewState)
		}
	}
	return plan, nil
}

func (c *Client) ApplyTestFlightDistribution(ctx context.Context, plan TestFlightDistributionPlan) error {
	if err := validateTestFlightDistributionPlan(plan); err != nil {
		return err
	}
	groupNames := make([]string, 0, len(plan.Groups))
	for _, group := range plan.Groups {
		groupNames = append(groupNames, group.Name)
	}
	current, err := c.PlanTestFlightDistribution(ctx, plan.AppID, plan.BundleID, plan.Platform, plan.Version, TestFlightDistributionOptions{BuildVersion: plan.Build, GroupNames: groupNames})
	if err != nil {
		return fmt.Errorf("revalidate TestFlight distribution plan: %w", err)
	}
	if !sameTestFlightDistributionPlan(plan, current) {
		return errors.New("remote App Store Connect state changed after planning; review a new dry run before retrying")
	}
	for _, group := range plan.Groups {
		if group.Attached {
			continue
		}
		body := map[string]any{"data": []map[string]string{{"type": "builds", "id": plan.buildID}}}
		path := "/v1/betaGroups/" + url.PathEscape(group.resourceID) + "/relationships/builds"
		if err := c.doJSON(ctx, http.MethodPost, path, body, nil); err != nil {
			return fmt.Errorf("attach build %s to beta group %s: %w", plan.Build, group.Name, err)
		}
	}
	if operationPresentInDistribution(plan, "submit", "beta-app-review") {
		body := map[string]any{"data": map[string]any{"type": "betaAppReviewSubmissions", "relationships": map[string]any{"build": map[string]any{"data": map[string]string{"type": "builds", "id": plan.buildID}}}}}
		if err := c.doJSON(ctx, http.MethodPost, "/v1/betaAppReviewSubmissions", body, nil); err != nil {
			return fmt.Errorf("submit build %s for Beta App Review: %w", plan.Build, err)
		}
	}
	return nil
}

func normalizeGroupNames(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, errors.New("at least one --group is required")
	}
	seen := map[string]bool{}
	names := make([]string, 0, len(values))
	for _, value := range values {
		name := strings.TrimSpace(value)
		if name == "" {
			return nil, errors.New("--group cannot be empty")
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func selectDistributionBuild(builds []BuildStatus, requested, platform, version string) (BuildStatus, error) {
	if requested != "" {
		var matches []BuildStatus
		for _, build := range builds {
			if build.Version == requested {
				matches = append(matches, build)
			}
		}
		if len(matches) == 0 {
			return BuildStatus{}, fmt.Errorf("build %q was not found for %s %s", requested, platform, version)
		}
		if len(matches) > 1 {
			return BuildStatus{}, fmt.Errorf("multiple builds matched build number %q", requested)
		}
		if matches[0].ProcessingState != "VALID" || matches[0].Expired {
			return BuildStatus{}, fmt.Errorf("build %s is not eligible for distribution (processing state %s, expired %t)", matches[0].Version, matches[0].ProcessingState, matches[0].Expired)
		}
		return matches[0], nil
	}
	for _, build := range builds {
		if build.ProcessingState == "VALID" && !build.Expired {
			return build, nil
		}
	}
	return BuildStatus{}, fmt.Errorf("no valid, unexpired build was found for %s %s", platform, version)
}

func (c *Client) betaGroupHasBuild(ctx context.Context, groupID, buildID string) (bool, error) {
	items, err := c.list(ctx, "/v1/betaGroups/"+url.PathEscape(groupID)+"/builds?fields%5Bbuilds%5D=version&limit=200")
	if err != nil {
		return false, fmt.Errorf("read builds for beta group %s: %w", groupID, err)
	}
	for _, item := range items {
		if item.ID == buildID {
			return true, nil
		}
	}
	return false, nil
}

func (c *Client) fetchBetaAppReviewSubmission(ctx context.Context, buildID string) (*ReviewSubmissionStatus, error) {
	var response nullableRelationshipResponse
	path := "/v1/builds/" + url.PathEscape(buildID) + "/betaAppReviewSubmission?fields%5BbetaAppReviewSubmissions%5D=betaReviewState,submittedDate"
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		if isAPIStatus(err, http.StatusNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("read Beta App Review submission: %w", err)
	}
	if response.Data == nil {
		return nil, nil
	}
	return &ReviewSubmissionStatus{ID: response.Data.ID, State: stringAttribute(*response.Data, "betaReviewState"), SubmittedDate: stringAttribute(*response.Data, "submittedDate")}, nil
}

func validateTestFlightDistributionPlan(plan TestFlightDistributionPlan) error {
	if plan.Kind != "testflight-distribute" || plan.AppID == "" || plan.Version == "" || plan.Build == "" || plan.buildID == "" || len(plan.Groups) == 0 || len(plan.Operations) == 0 {
		return errors.New("invalid TestFlight distribution plan")
	}
	wantsReview := false
	attachCounts := map[string]int{}
	for _, operation := range plan.Operations {
		switch {
		case operation.Action == "submit" && operation.Resource == "beta-app-review":
			if wantsReview {
				return errors.New("invalid duplicate Beta App Review operation")
			}
			wantsReview = true
		case operation.Action == "attach" && strings.HasPrefix(operation.Resource, "beta-group:"):
			attachCounts[strings.TrimPrefix(operation.Resource, "beta-group:")]++
		default:
			return fmt.Errorf("invalid TestFlight distribution operation %s %s", operation.Action, operation.Resource)
		}
	}
	for _, group := range plan.Groups {
		if group.resourceID == "" {
			return errors.New("invalid TestFlight beta group plan")
		}
		count := attachCounts[group.Name]
		delete(attachCounts, group.Name)
		if group.Attached && count != 0 {
			return fmt.Errorf("invalid attach operation for already attached beta group %s", group.Name)
		}
		if !group.Attached && count != 1 {
			return fmt.Errorf("invalid attach operations for beta group %s", group.Name)
		}
	}
	if len(attachCounts) != 0 {
		return errors.New("invalid attach operation for an unresolved beta group")
	}
	return nil
}

func sameTestFlightDistributionPlan(left, right TestFlightDistributionPlan) bool {
	if left.Kind != right.Kind || left.AppID != right.AppID || left.BundleID != right.BundleID || left.Platform != right.Platform || left.Version != right.Version || left.Build != right.Build || left.BetaReviewState != right.BetaReviewState || left.buildID != right.buildID || len(left.Groups) != len(right.Groups) || len(left.Operations) != len(right.Operations) {
		return false
	}
	for index := range left.Groups {
		if left.Groups[index] != right.Groups[index] {
			return false
		}
	}
	for index := range left.Operations {
		if left.Operations[index] != right.Operations[index] {
			return false
		}
	}
	return true
}

func operationPresentInDistribution(plan TestFlightDistributionPlan, action, resource string) bool {
	for _, operation := range plan.Operations {
		if operation.Action == action && operation.Resource == resource {
			return true
		}
	}
	return false
}
