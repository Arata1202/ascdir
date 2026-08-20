package appstore

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// BuildStatus is the release-oriented view of an App Store Connect build.
// It deliberately contains no analytics or tester data.
type BuildStatus struct {
	ID              string `json:"id"`
	Version         string `json:"version"`
	UploadedDate    string `json:"uploaded_date,omitempty"`
	ExpirationDate  string `json:"expiration_date,omitempty"`
	ProcessingState string `json:"processing_state,omitempty"`
	Expired         bool   `json:"expired"`
}

type ReviewSubmissionStatus struct {
	ID            string `json:"id"`
	State         string `json:"state"`
	SubmittedDate string `json:"submitted_date,omitempty"`
}

// AppStoreStatus is the current App Store release state for one configured
// platform and version.
type AppStoreStatus struct {
	AppID               string                  `json:"app_id"`
	VersionID           string                  `json:"version_id"`
	Platform            string                  `json:"platform"`
	Version             string                  `json:"version"`
	AppVersionState     string                  `json:"app_version_state,omitempty"`
	AppStoreState       string                  `json:"app_store_state,omitempty"`
	ReleaseType         string                  `json:"release_type,omitempty"`
	EarliestReleaseDate string                  `json:"earliest_release_date,omitempty"`
	Build               *BuildStatus            `json:"build,omitempty"`
	ReviewSubmission    *ReviewSubmissionStatus `json:"review_submission,omitempty"`
}

type TestFlightStatus struct {
	AppID  string        `json:"app_id"`
	Builds []BuildStatus `json:"builds"`
}

// ReleaseOperation is one mutation in a release plan. Plans are deliberately
// serializable so CI can inspect the exact work an execution would perform.
type ReleaseOperation struct {
	Action      string `json:"action"`
	Resource    string `json:"resource"`
	Description string `json:"description"`
}

// AppStoreReleasePlan is the preflight result shared by --dry-run and the
// execution path. The unexported IDs are resolved from App Store Connect and
// prevent execution from acting on a different app or build than the plan.
type AppStoreReleasePlan struct {
	Kind                string             `json:"kind"`
	AppID               string             `json:"app_id"`
	BundleID            string             `json:"bundle_id"`
	Platform            string             `json:"platform"`
	Version             string             `json:"version"`
	Build               string             `json:"build,omitempty"`
	ReleaseType         string             `json:"release_type,omitempty"`
	EarliestReleaseDate string             `json:"earliest_release_date,omitempty"`
	Operations          []ReleaseOperation `json:"operations"`

	versionID          string
	buildID            string
	reviewSubmissionID string
}

type SubmitOptions struct {
	BuildVersion        string
	ReleaseType         string
	EarliestReleaseDate string
}

type nullableRelationshipResponse struct {
	Data *resource `json:"data"`
}

func (c *Client) FetchAppStoreStatus(ctx context.Context, appID, bundleID, platform, version string) (AppStoreStatus, error) {
	app, err := c.resolveApp(ctx, appID, bundleID)
	if err != nil {
		return AppStoreStatus{}, err
	}
	path := fmt.Sprintf("/v1/apps/%s/appStoreVersions?filter%%5Bplatform%%5D=%s&filter%%5BversionString%%5D=%s&fields%%5BappStoreVersions%%5D=platform,versionString,appVersionState,appStoreState,releaseType,earliestReleaseDate&limit=2",
		url.PathEscape(app.ID), url.QueryEscape(platform), url.QueryEscape(version))
	versions, err := c.list(ctx, path)
	if err != nil {
		return AppStoreStatus{}, err
	}
	if len(versions) == 0 {
		return AppStoreStatus{}, fmt.Errorf("version %s for platform %s was not found", version, platform)
	}
	if len(versions) > 1 {
		return AppStoreStatus{}, fmt.Errorf("multiple versions matched %s for platform %s", version, platform)
	}
	remoteVersion := versions[0]
	status := AppStoreStatus{
		AppID: app.ID, VersionID: remoteVersion.ID,
		Platform: stringAttribute(remoteVersion, "platform"), Version: stringAttribute(remoteVersion, "versionString"),
		AppVersionState: stringAttribute(remoteVersion, "appVersionState"), AppStoreState: stringAttribute(remoteVersion, "appStoreState"),
		ReleaseType: stringAttribute(remoteVersion, "releaseType"), EarliestReleaseDate: stringAttribute(remoteVersion, "earliestReleaseDate"),
	}
	if status.Platform == "" {
		status.Platform = platform
	}
	if status.Version == "" {
		status.Version = version
	}

	var buildResponse nullableRelationshipResponse
	buildPath := "/v1/appStoreVersions/" + url.PathEscape(remoteVersion.ID) + "/build?fields%5Bbuilds%5D=version,uploadedDate,expirationDate,processingState,expired"
	if err := c.doJSON(ctx, http.MethodGet, buildPath, nil, &buildResponse); err != nil {
		return AppStoreStatus{}, fmt.Errorf("read build for version %s: %w", version, err)
	}
	if buildResponse.Data != nil {
		build := buildStatus(*buildResponse.Data)
		status.Build = &build
	}

	submissionsPath := fmt.Sprintf("/v1/apps/%s/reviewSubmissions?filter%%5Bplatform%%5D=%s&fields%%5BreviewSubmissions%%5D=state,submittedDate,appStoreVersionForReview&limit=200",
		url.PathEscape(app.ID), url.QueryEscape(platform))
	submissions, err := c.list(ctx, submissionsPath)
	if err != nil {
		return AppStoreStatus{}, fmt.Errorf("read review submissions: %w", err)
	}
	status.ReviewSubmission = selectReviewSubmission(submissions, remoteVersion.ID)
	return status, nil
}

func (c *Client) FetchTestFlightStatus(ctx context.Context, appID, bundleID, platform, version string) (TestFlightStatus, error) {
	app, err := c.resolveApp(ctx, appID, bundleID)
	if err != nil {
		return TestFlightStatus{}, err
	}
	preReleasePath := fmt.Sprintf("/v1/apps/%s/preReleaseVersions?fields%%5BpreReleaseVersions%%5D=platform,version&limit=200", url.PathEscape(app.ID))
	allPreReleaseVersions, err := c.list(ctx, preReleasePath)
	if err != nil {
		return TestFlightStatus{}, err
	}
	preReleaseVersions := make([]resource, 0, 1)
	for _, candidate := range allPreReleaseVersions {
		if stringAttribute(candidate, "platform") == platform && stringAttribute(candidate, "version") == version {
			preReleaseVersions = append(preReleaseVersions, candidate)
		}
	}
	if len(preReleaseVersions) == 0 {
		return TestFlightStatus{AppID: app.ID, Builds: []BuildStatus{}}, nil
	}
	if len(preReleaseVersions) > 1 {
		return TestFlightStatus{}, fmt.Errorf("multiple prerelease versions matched %s for platform %s", version, platform)
	}
	path := "/v1/preReleaseVersions/" + url.PathEscape(preReleaseVersions[0].ID) + "/builds?fields%5Bbuilds%5D=version,uploadedDate,expirationDate,processingState,expired&limit=200"
	resources, err := c.list(ctx, path)
	if err != nil {
		return TestFlightStatus{}, err
	}
	builds := make([]BuildStatus, 0, len(resources))
	for _, item := range resources {
		builds = append(builds, buildStatus(item))
	}
	sort.SliceStable(builds, func(i, j int) bool {
		left, leftErr := time.Parse(time.RFC3339, builds[i].UploadedDate)
		right, rightErr := time.Parse(time.RFC3339, builds[j].UploadedDate)
		if leftErr == nil && rightErr == nil && !left.Equal(right) {
			return left.After(right)
		}
		if builds[i].Version != builds[j].Version {
			return builds[i].Version > builds[j].Version
		}
		return builds[i].ID < builds[j].ID
	})
	return TestFlightStatus{AppID: app.ID, Builds: builds}, nil
}

// PlanAppStoreSubmission performs every read and validation needed before an
// App Store submission. It never mutates App Store Connect.
func (c *Client) PlanAppStoreSubmission(ctx context.Context, appID, bundleID, platform, version string, options SubmitOptions) (AppStoreReleasePlan, error) {
	app, err := c.resolveApp(ctx, appID, bundleID)
	if err != nil {
		return AppStoreReleasePlan{}, err
	}
	options.ReleaseType = strings.ToUpper(strings.TrimSpace(options.ReleaseType))
	if options.ReleaseType != "" && options.ReleaseType != "MANUAL" && options.ReleaseType != "AFTER_APPROVAL" && options.ReleaseType != "SCHEDULED" {
		return AppStoreReleasePlan{}, fmt.Errorf("unsupported release type %q; use MANUAL, AFTER_APPROVAL, or SCHEDULED", options.ReleaseType)
	}
	if options.ReleaseType == "SCHEDULED" {
		if _, err := time.Parse(time.RFC3339, options.EarliestReleaseDate); err != nil {
			return AppStoreReleasePlan{}, fmt.Errorf("scheduled releases require --earliest-release-date in RFC3339 format: %w", err)
		}
	} else if options.EarliestReleaseDate != "" {
		return AppStoreReleasePlan{}, errors.New("--earliest-release-date is only valid with --release-type SCHEDULED")
	}

	plan := AppStoreReleasePlan{Kind: "app-store-submit", AppID: app.ID, BundleID: bundleID, Platform: platform, Version: version, ReleaseType: options.ReleaseType, EarliestReleaseDate: options.EarliestReleaseDate, Operations: []ReleaseOperation{}}
	versions, err := c.findAppStoreVersions(ctx, app.ID, platform, version)
	if err != nil {
		return AppStoreReleasePlan{}, err
	}
	if len(versions) > 1 {
		return AppStoreReleasePlan{}, fmt.Errorf("multiple versions matched %s for platform %s", version, platform)
	}
	if len(versions) == 0 {
		if plan.ReleaseType == "" {
			plan.ReleaseType = "MANUAL"
		}
		plan.Operations = append(plan.Operations, ReleaseOperation{Action: "create", Resource: "app-store-version", Description: fmt.Sprintf("Create %s %s; then sync metadata before submitting", platform, version)})
		return plan, nil
	}
	if len(versions) == 1 && submissionAlreadyStarted(stringAttribute(versions[0], "appVersionState")) {
		plan.versionID = versions[0].ID
		remoteReleaseType := stringAttribute(versions[0], "releaseType")
		remoteEarliestDate := stringAttribute(versions[0], "earliestReleaseDate")
		if options.ReleaseType != "" && !releaseSettingsMatch(options.ReleaseType, options.EarliestReleaseDate, remoteReleaseType, remoteEarliestDate) {
			return AppStoreReleasePlan{}, fmt.Errorf("version %s is already submitted with release type %s; requested release settings can no longer be applied", version, displayState(remoteReleaseType))
		}
		plan.ReleaseType = remoteReleaseType
		plan.EarliestReleaseDate = remoteEarliestDate
		return plan, nil
	}
	remoteVersion := versions[0]
	plan.versionID = remoteVersion.ID
	remoteReleaseType := stringAttribute(remoteVersion, "releaseType")
	remoteEarliestDate := stringAttribute(remoteVersion, "earliestReleaseDate")
	if plan.ReleaseType == "" {
		plan.ReleaseType = remoteReleaseType
		plan.EarliestReleaseDate = remoteEarliestDate
	} else if !releaseSettingsMatch(plan.ReleaseType, plan.EarliestReleaseDate, remoteReleaseType, remoteEarliestDate) {
		plan.Operations = append(plan.Operations, ReleaseOperation{Action: "update", Resource: "release-settings", Description: fmt.Sprintf("Set release type to %s", plan.ReleaseType)})
	}
	var selectedBuild *BuildStatus
	builds, err := c.fetchTestFlightBuilds(ctx, app.ID, platform, version)
	if err != nil {
		return AppStoreReleasePlan{}, fmt.Errorf("read candidate builds: %w", err)
	}
	if options.BuildVersion != "" {
		for index := range builds {
			if builds[index].Version == options.BuildVersion {
				if selectedBuild != nil {
					return AppStoreReleasePlan{}, fmt.Errorf("multiple builds matched build number %q", options.BuildVersion)
				}
				candidate := builds[index]
				selectedBuild = &candidate
			}
		}
		if selectedBuild == nil {
			return AppStoreReleasePlan{}, fmt.Errorf("build %q was not found for %s %s", options.BuildVersion, platform, version)
		}
	} else {
		for index := range builds {
			if builds[index].ProcessingState == "VALID" && !builds[index].Expired {
				candidate := builds[index]
				selectedBuild = &candidate
				break
			}
		}
	}
	if selectedBuild == nil {
		return AppStoreReleasePlan{}, fmt.Errorf("no valid, unexpired build was found for %s %s", platform, version)
	}
	if selectedBuild.ProcessingState != "VALID" || selectedBuild.Expired {
		return AppStoreReleasePlan{}, fmt.Errorf("build %s is not eligible for submission (processing state %s, expired %t)", selectedBuild.Version, selectedBuild.ProcessingState, selectedBuild.Expired)
	}
	plan.Build, plan.buildID = selectedBuild.Version, selectedBuild.ID

	state := stringAttribute(remoteVersion, "appVersionState")
	if state != "PREPARE_FOR_SUBMISSION" && state != "READY_FOR_REVIEW" && state != "" {
		return AppStoreReleasePlan{}, fmt.Errorf("version %s cannot be submitted from state %s", version, state)
	}
	currentBuild, err := c.fetchVersionBuild(ctx, remoteVersion.ID)
	if err != nil {
		return AppStoreReleasePlan{}, err
	}
	if currentBuild == nil {
		plan.Operations = append(plan.Operations, ReleaseOperation{Action: "select", Resource: "build", Description: fmt.Sprintf("Select build %s", selectedBuild.Version)})
	} else if currentBuild.ID != selectedBuild.ID {
		return AppStoreReleasePlan{}, fmt.Errorf("version %s already selects build %s; refusing to replace it with build %s", version, currentBuild.Version, selectedBuild.Version)
	}
	review, err := c.findActiveReviewSubmission(ctx, app.ID, platform, remoteVersion.ID)
	if err != nil {
		return AppStoreReleasePlan{}, err
	}
	if review != nil && (review.State == "WAITING_FOR_REVIEW" || review.State == "IN_REVIEW" || review.State == "COMPLETING") {
		if operationPresent(plan, "update", "release-settings") {
			return AppStoreReleasePlan{}, fmt.Errorf("review submission %s is already %s; requested release settings can no longer be applied", review.ID, review.State)
		}
		return AppStoreReleasePlan{Kind: plan.Kind, AppID: plan.AppID, BundleID: plan.BundleID, Platform: plan.Platform, Version: plan.Version, Build: plan.Build, ReleaseType: plan.ReleaseType, EarliestReleaseDate: plan.EarliestReleaseDate, versionID: plan.versionID, buildID: plan.buildID, reviewSubmissionID: review.ID, Operations: []ReleaseOperation{}}, nil
	}
	if review != nil && review.State != "READY_FOR_REVIEW" {
		return AppStoreReleasePlan{}, fmt.Errorf("review submission %s is %s and cannot be edited; wait for App Store Connect to finish the transition", review.ID, review.State)
	}
	if review == nil {
		plan.Operations = append(plan.Operations, ReleaseOperation{Action: "create", Resource: "review-submission", Description: "Create an App Review submission"})
	} else {
		plan.reviewSubmissionID = review.ID
	}
	itemExists := false
	if plan.reviewSubmissionID != "" {
		itemExists, err = c.reviewSubmissionHasVersion(ctx, plan.reviewSubmissionID, remoteVersion.ID)
		if err != nil {
			return AppStoreReleasePlan{}, err
		}
	}
	if !itemExists {
		plan.Operations = append(plan.Operations, ReleaseOperation{Action: "add", Resource: "review-submission-item", Description: fmt.Sprintf("Add version %s to the submission", version)})
	}
	plan.Operations = append(plan.Operations, ReleaseOperation{Action: "submit", Resource: "review-submission", Description: "Submit for App Review"})
	return plan, nil
}

// ApplyAppStoreSubmission executes a previously validated plan. Callers must
// obtain explicit user confirmation before invoking this method.
func (c *Client) ApplyAppStoreSubmission(ctx context.Context, plan AppStoreReleasePlan) error {
	if plan.Kind != "app-store-submit" || plan.AppID == "" || plan.Version == "" || len(plan.Operations) == 0 {
		return errors.New("invalid App Store submission plan")
	}
	current, err := c.PlanAppStoreSubmission(ctx, plan.AppID, plan.BundleID, plan.Platform, plan.Version, SubmitOptions{
		BuildVersion: plan.Build, ReleaseType: plan.ReleaseType, EarliestReleaseDate: plan.EarliestReleaseDate,
	})
	if err != nil {
		return fmt.Errorf("revalidate App Store submission plan: %w", err)
	}
	if !sameAppStoreReleasePlan(plan, current) {
		return errors.New("remote App Store Connect state changed after planning; review a new dry run before retrying")
	}
	versionID := plan.versionID
	if versionID == "" {
		if len(plan.Operations) != 1 || !operationPresent(plan, "create", "app-store-version") {
			return errors.New("invalid new-version submission plan")
		}
		attributes := map[string]any{"platform": plan.Platform, "versionString": plan.Version, "releaseType": plan.ReleaseType}
		if plan.EarliestReleaseDate != "" {
			attributes["earliestReleaseDate"] = plan.EarliestReleaseDate
		}
		body := map[string]any{"data": map[string]any{"type": "appStoreVersions", "attributes": attributes, "relationships": map[string]any{"app": map[string]any{"data": map[string]string{"type": "apps", "id": plan.AppID}}}}}
		var response singleResponse
		if err := c.doJSON(ctx, http.MethodPost, "/v1/appStoreVersions", body, &response); err != nil {
			return fmt.Errorf("create App Store version: %w", err)
		}
		versionID = response.Data.ID
		if versionID == "" {
			return errors.New("create App Store version response has no resource ID")
		}
		return nil
	}
	if plan.buildID == "" || !operationPresent(plan, "submit", "review-submission") {
		return errors.New("invalid App Store submission operations")
	}
	if operationPresent(plan, "update", "release-settings") {
		attributes := map[string]any{"releaseType": plan.ReleaseType}
		if plan.ReleaseType == "SCHEDULED" {
			attributes["earliestReleaseDate"] = plan.EarliestReleaseDate
		} else {
			attributes["earliestReleaseDate"] = nil
		}
		body := map[string]any{"data": map[string]any{"type": "appStoreVersions", "id": versionID, "attributes": attributes}}
		if err := c.doJSON(ctx, http.MethodPatch, "/v1/appStoreVersions/"+url.PathEscape(versionID), body, nil); err != nil {
			return fmt.Errorf("update release settings: %w", err)
		}
	}
	if operationPresent(plan, "select", "build") {
		body := map[string]any{"data": map[string]string{"type": "builds", "id": plan.buildID}}
		if err := c.doJSON(ctx, http.MethodPatch, "/v1/appStoreVersions/"+url.PathEscape(versionID)+"/relationships/build", body, nil); err != nil {
			return fmt.Errorf("select build %s: %w", plan.Build, err)
		}
	}
	reviewID := plan.reviewSubmissionID
	if reviewID == "" {
		body := map[string]any{"data": map[string]any{"type": "reviewSubmissions", "attributes": map[string]string{"platform": plan.Platform}, "relationships": map[string]any{"app": map[string]any{"data": map[string]string{"type": "apps", "id": plan.AppID}}}}}
		var response singleResponse
		if err := c.doJSON(ctx, http.MethodPost, "/v1/reviewSubmissions", body, &response); err != nil {
			return fmt.Errorf("create review submission: %w", err)
		}
		reviewID = response.Data.ID
		if reviewID == "" {
			return errors.New("create review submission response has no resource ID")
		}
	}
	if operationPresent(plan, "add", "review-submission-item") {
		itemBody := map[string]any{"data": map[string]any{"type": "reviewSubmissionItems", "relationships": map[string]any{
			"reviewSubmission": map[string]any{"data": map[string]string{"type": "reviewSubmissions", "id": reviewID}},
			"appStoreVersion":  map[string]any{"data": map[string]string{"type": "appStoreVersions", "id": versionID}},
		}}}
		if err := c.doJSON(ctx, http.MethodPost, "/v1/reviewSubmissionItems", itemBody, nil); err != nil {
			return fmt.Errorf("add App Store version to review submission: %w", err)
		}
	}
	submitBody := map[string]any{"data": map[string]any{"type": "reviewSubmissions", "id": reviewID, "attributes": map[string]bool{"submitted": true}}}
	if err := c.doJSON(ctx, http.MethodPatch, "/v1/reviewSubmissions/"+url.PathEscape(reviewID), submitBody, nil); err != nil {
		return fmt.Errorf("submit for App Review: %w", err)
	}
	return nil
}

func (c *Client) PlanAppStoreRelease(ctx context.Context, appID, bundleID, platform, version string) (AppStoreReleasePlan, error) {
	status, err := c.FetchAppStoreStatus(ctx, appID, bundleID, platform, version)
	if err != nil {
		return AppStoreReleasePlan{}, err
	}
	plan := AppStoreReleasePlan{Kind: "app-store-release", AppID: status.AppID, BundleID: bundleID, Platform: platform, Version: version, ReleaseType: status.ReleaseType, versionID: status.VersionID, Operations: []ReleaseOperation{}}
	switch status.AppVersionState {
	case "READY_FOR_DISTRIBUTION", "READY_FOR_SALE", "PROCESSING_FOR_DISTRIBUTION":
		return plan, nil
	case "PENDING_DEVELOPER_RELEASE":
		if status.ReleaseType != "" && status.ReleaseType != "MANUAL" {
			return AppStoreReleasePlan{}, fmt.Errorf("version %s uses release type %s, not MANUAL", version, status.ReleaseType)
		}
		plan.Operations = append(plan.Operations, ReleaseOperation{Action: "release", Resource: "app-store-version", Description: fmt.Sprintf("Release %s %s to the App Store", platform, version)})
		return plan, nil
	default:
		return AppStoreReleasePlan{}, fmt.Errorf("version %s cannot be manually released from state %s", version, displayState(status.AppVersionState))
	}
}

func (c *Client) ApplyAppStoreRelease(ctx context.Context, plan AppStoreReleasePlan) error {
	if plan.Kind != "app-store-release" || plan.versionID == "" || len(plan.Operations) != 1 || !operationPresent(plan, "release", "app-store-version") {
		return errors.New("invalid App Store release plan")
	}
	current, err := c.PlanAppStoreRelease(ctx, plan.AppID, plan.BundleID, plan.Platform, plan.Version)
	if err != nil {
		return fmt.Errorf("revalidate App Store release plan: %w", err)
	}
	if !sameAppStoreReleasePlan(plan, current) {
		return errors.New("remote App Store Connect state changed after planning; review a new dry run before retrying")
	}
	body := map[string]any{"data": map[string]any{"type": "appStoreVersionReleaseRequests", "relationships": map[string]any{"appStoreVersion": map[string]any{"data": map[string]string{"type": "appStoreVersions", "id": plan.versionID}}}}}
	if err := c.doJSON(ctx, http.MethodPost, "/v1/appStoreVersionReleaseRequests", body, nil); err != nil {
		return fmt.Errorf("request App Store release: %w", err)
	}
	return nil
}

func sameAppStoreReleasePlan(left, right AppStoreReleasePlan) bool {
	if left.Kind != right.Kind || left.AppID != right.AppID || left.Platform != right.Platform || left.Version != right.Version ||
		left.Build != right.Build || left.ReleaseType != right.ReleaseType || left.EarliestReleaseDate != right.EarliestReleaseDate ||
		left.versionID != right.versionID || left.buildID != right.buildID || left.reviewSubmissionID != right.reviewSubmissionID ||
		len(left.Operations) != len(right.Operations) {
		return false
	}
	for index := range left.Operations {
		if left.Operations[index] != right.Operations[index] {
			return false
		}
	}
	return true
}

func releaseSettingsMatch(leftType, leftDate, rightType, rightDate string) bool {
	if leftType != rightType {
		return false
	}
	if leftType != "SCHEDULED" {
		return true
	}
	left, leftErr := time.Parse(time.RFC3339, leftDate)
	right, rightErr := time.Parse(time.RFC3339, rightDate)
	if leftErr == nil && rightErr == nil {
		return left.Equal(right)
	}
	return leftDate == rightDate
}

func (c *Client) findAppStoreVersions(ctx context.Context, appID, platform, version string) ([]resource, error) {
	path := fmt.Sprintf("/v1/apps/%s/appStoreVersions?filter%%5Bplatform%%5D=%s&filter%%5BversionString%%5D=%s&fields%%5BappStoreVersions%%5D=platform,versionString,appVersionState,releaseType,earliestReleaseDate&limit=2", url.PathEscape(appID), url.QueryEscape(platform), url.QueryEscape(version))
	return c.list(ctx, path)
}

func (c *Client) fetchTestFlightBuilds(ctx context.Context, appID, platform, version string) ([]BuildStatus, error) {
	preReleasePath := fmt.Sprintf("/v1/apps/%s/preReleaseVersions?fields%%5BpreReleaseVersions%%5D=platform,version&limit=200", url.PathEscape(appID))
	items, err := c.list(ctx, preReleasePath)
	if err != nil {
		return nil, err
	}
	var match *resource
	for index := range items {
		if stringAttribute(items[index], "platform") == platform && stringAttribute(items[index], "version") == version {
			if match != nil {
				return nil, fmt.Errorf("multiple prerelease versions matched %s for platform %s", version, platform)
			}
			candidate := items[index]
			match = &candidate
		}
	}
	if match == nil {
		return []BuildStatus{}, nil
	}
	items, err = c.list(ctx, "/v1/preReleaseVersions/"+url.PathEscape(match.ID)+"/builds?fields%5Bbuilds%5D=version,uploadedDate,expirationDate,processingState,expired&limit=200")
	if err != nil {
		return nil, err
	}
	builds := make([]BuildStatus, 0, len(items))
	for _, item := range items {
		builds = append(builds, buildStatus(item))
	}
	sort.SliceStable(builds, func(i, j int) bool {
		left, leftErr := time.Parse(time.RFC3339, builds[i].UploadedDate)
		right, rightErr := time.Parse(time.RFC3339, builds[j].UploadedDate)
		if leftErr == nil && rightErr == nil && !left.Equal(right) {
			return left.After(right)
		}
		if builds[i].Version != builds[j].Version {
			return builds[i].Version > builds[j].Version
		}
		return builds[i].ID < builds[j].ID
	})
	return builds, nil
}

func (c *Client) fetchVersionBuild(ctx context.Context, versionID string) (*BuildStatus, error) {
	var response nullableRelationshipResponse
	if err := c.doJSON(ctx, http.MethodGet, "/v1/appStoreVersions/"+url.PathEscape(versionID)+"/build?fields%5Bbuilds%5D=version,uploadedDate,expirationDate,processingState,expired", nil, &response); err != nil {
		return nil, fmt.Errorf("read selected build: %w", err)
	}
	if response.Data == nil {
		return nil, nil
	}
	result := buildStatus(*response.Data)
	return &result, nil
}

func (c *Client) findActiveReviewSubmission(ctx context.Context, appID, platform, versionID string) (*ReviewSubmissionStatus, error) {
	path := fmt.Sprintf("/v1/apps/%s/reviewSubmissions?filter%%5Bplatform%%5D=%s&fields%%5BreviewSubmissions%%5D=state,submittedDate,appStoreVersionForReview&limit=200", url.PathEscape(appID), url.QueryEscape(platform))
	items, err := c.list(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("read review submissions: %w", err)
	}
	if matching := selectReviewSubmission(items, versionID); matching != nil {
		switch matching.State {
		case "READY_FOR_REVIEW", "WAITING_FOR_REVIEW", "IN_REVIEW", "UNRESOLVED_ISSUES", "CANCELING", "COMPLETING":
			return matching, nil
		}
	}
	// A prior attempt may have created the draft before it could add the
	// version item. Reuse the one active platform submission instead of
	// creating duplicates on retry.
	var draft *ReviewSubmissionStatus
	for _, item := range items {
		state := stringAttribute(item, "state")
		if state != "READY_FOR_REVIEW" {
			continue
		}
		hasVersion, itemCount, err := c.reviewSubmissionVersionItems(ctx, item.ID, versionID)
		if err != nil {
			return nil, err
		}
		if itemCount > 0 && !hasVersion {
			return nil, fmt.Errorf("draft review submission %s contains unrelated items; resolve it before submitting version %s", item.ID, versionID)
		}
		if draft != nil {
			return nil, errors.New("multiple active review submissions exist; resolve them in App Store Connect before retrying")
		}
		candidate := ReviewSubmissionStatus{ID: item.ID, State: state, SubmittedDate: stringAttribute(item, "submittedDate")}
		draft = &candidate
	}
	return draft, nil
}

func (c *Client) reviewSubmissionHasVersion(ctx context.Context, reviewID, versionID string) (bool, error) {
	hasVersion, _, err := c.reviewSubmissionVersionItems(ctx, reviewID, versionID)
	return hasVersion, err
}

func (c *Client) reviewSubmissionVersionItems(ctx context.Context, reviewID, versionID string) (bool, int, error) {
	items, err := c.list(ctx, "/v1/reviewSubmissions/"+url.PathEscape(reviewID)+"/items?include=appStoreVersion&limit=200")
	if err != nil {
		return false, 0, fmt.Errorf("read review submission items: %w", err)
	}
	for _, item := range items {
		if related := item.Relationships["appStoreVersion"].Data; related != nil && related.ID == versionID {
			return true, len(items), nil
		}
	}
	return false, len(items), nil
}

func submissionAlreadyStarted(state string) bool {
	switch state {
	case "WAITING_FOR_REVIEW", "IN_REVIEW", "PENDING_APPLE_RELEASE", "PENDING_DEVELOPER_RELEASE", "PROCESSING_FOR_DISTRIBUTION", "READY_FOR_DISTRIBUTION", "READY_FOR_SALE":
		return true
	default:
		return false
	}
}

func operationPresent(plan AppStoreReleasePlan, action, resource string) bool {
	for _, operation := range plan.Operations {
		if operation.Action == action && operation.Resource == resource {
			return true
		}
	}
	return false
}

func displayState(state string) string {
	if state == "" {
		return "unknown"
	}
	return state
}

func buildStatus(item resource) BuildStatus {
	return BuildStatus{
		ID: item.ID, Version: stringAttribute(item, "version"), UploadedDate: stringAttribute(item, "uploadedDate"),
		ExpirationDate: stringAttribute(item, "expirationDate"), ProcessingState: stringAttribute(item, "processingState"),
		Expired: boolAttribute(item, "expired"),
	}
}

func boolAttribute(item resource, key string) bool {
	value, _ := item.Attributes[key].(bool)
	return value
}

func selectReviewSubmission(items []resource, versionID string) *ReviewSubmissionStatus {
	var matches []ReviewSubmissionStatus
	for _, item := range items {
		relationship, ok := item.Relationships["appStoreVersionForReview"]
		if !ok || relationship.Data == nil || relationship.Data.ID != versionID {
			continue
		}
		matches = append(matches, ReviewSubmissionStatus{ID: item.ID, State: stringAttribute(item, "state"), SubmittedDate: stringAttribute(item, "submittedDate")})
	}
	if len(matches) == 0 {
		return nil
	}
	sort.SliceStable(matches, func(i, j int) bool {
		leftComplete := strings.EqualFold(matches[i].State, "COMPLETE")
		rightComplete := strings.EqualFold(matches[j].State, "COMPLETE")
		if leftComplete != rightComplete {
			return !leftComplete
		}
		return matches[i].SubmittedDate > matches[j].SubmittedDate
	})
	return &matches[0]
}
