package appstore

import (
	"context"
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
