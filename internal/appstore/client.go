package appstore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.appstoreconnect.apple.com"

type Client struct {
	credentials Credentials
	baseURL     string
	httpClient  *http.Client
	now         func() time.Time
	maxRetries  int
	sleep       func(context.Context, time.Duration) error
	jitter      func(time.Duration) time.Duration
}

type Option func(*Client)

func WithTimeout(timeout time.Duration) Option {
	return func(client *Client) { client.httpClient.Timeout = timeout }
}

type Metadata struct {
	AppID                    string
	AppInfoID                string
	VersionID                string
	AgeRatingID              string
	LicenseAgreementID       string
	Accessibility            map[string]AccessibilityDeclaration
	Screenshots              map[string]map[string][]Asset
	ScreenshotSetIDs         map[string]map[string]string
	AppPreviews              map[string]map[string][]Asset
	AppPreviewSetIDs         map[string]map[string]string
	AvailabilityID           string
	TerritoryAvailabilityIDs map[string]string
	Values                   map[string]string
	Localizations            map[string]Localization
}

// FetchOptions limits optional App Store Connect resources to the features a
// configuration manages. This keeps existing projects usable with least-
// privilege API keys that cannot access newly supported resource families.
type FetchOptions struct {
	AgeRating        bool
	Accessibility    bool
	LicenseAgreement bool
	Screenshots      bool
	DownloadAssets   bool
	AppPreviews      bool
	Availability     bool
}

type Asset struct {
	ID                   string
	FileName             string
	Path                 string
	Checksum             string
	Size                 int64
	Content              []byte
	MIMEType             string
	PreviewFrameTimeCode string
}

type AssetSetChange struct {
	Kind        string
	Locale      string
	DisplayType string
	SetID       string
	Before      []Asset
	After       []Asset
}

type Localization struct {
	AppInfoLocalizationID string
	VersionLocalizationID string
	Values                map[string]string
}

type AccessibilityDeclaration struct {
	ID     string
	Values map[string]string
}

type resource struct {
	ID            string                  `json:"id"`
	Type          string                  `json:"type"`
	Attributes    map[string]any          `json:"attributes"`
	Relationships map[string]relationship `json:"relationships"`
}

type relationship struct {
	Data *resourceIdentifier `json:"data"`
}

type resourceIdentifier struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

type listResponse struct {
	Data  []resource `json:"data"`
	Links struct {
		Next string `json:"next"`
	} `json:"links"`
}

type singleResponse struct {
	Data resource `json:"data"`
}

type errorResponse struct {
	Errors []struct {
		Status string `json:"status"`
		Code   string `json:"code"`
		Title  string `json:"title"`
		Detail string `json:"detail"`
	} `json:"errors"`
}

func NewClient(credentials Credentials, baseURL string, options ...Option) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	client := &Client{
		credentials: credentials,
		baseURL:     strings.TrimRight(baseURL, "/"),
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		now:         time.Now,
		maxRetries:  3,
		sleep:       sleepContext,
		jitter:      retryJitter,
	}
	for _, option := range options {
		option(client)
	}
	return client
}

func (c *Client) CheckAuth(ctx context.Context) error {
	var response listResponse
	return c.doJSON(ctx, http.MethodGet, "/v1/apps?limit=1", nil, &response)
}

func (m Metadata) Locales() []string {
	locales := make([]string, 0, len(m.Localizations))
	for locale := range m.Localizations {
		locales = append(locales, locale)
	}
	sort.Strings(locales)
	return locales
}

func (c *Client) FetchMetadata(ctx context.Context, appID, bundleID, platform, version string, options FetchOptions) (Metadata, error) {
	app, err := c.resolveApp(ctx, appID, bundleID)
	if err != nil {
		return Metadata{}, err
	}
	result := Metadata{AppID: app.ID, Values: map[string]string{}, Accessibility: map[string]AccessibilityDeclaration{}, Screenshots: map[string]map[string][]Asset{}, ScreenshotSetIDs: map[string]map[string]string{}, AppPreviews: map[string]map[string][]Asset{}, AppPreviewSetIDs: map[string]map[string]string{}, TerritoryAvailabilityIDs: map[string]string{}, Localizations: map[string]Localization{}}
	copyAttributes(result.Values, app.Attributes, appFields)
	if options.LicenseAgreement {
		if err := c.fetchLicenseAgreement(ctx, &result); err != nil {
			return Metadata{}, err
		}
	}
	if options.Availability {
		if err := c.fetchAvailability(ctx, &result); err != nil {
			return Metadata{}, err
		}
	}
	if options.Accessibility {
		accessibilityDeclarations, err := c.list(ctx, fmt.Sprintf("/v1/apps/%s/accessibilityDeclarations?limit=200", result.AppID))
		if err != nil {
			return Metadata{}, err
		}
		for _, declaration := range accessibilityDeclarations {
			deviceFamily := stringAttribute(declaration, "deviceFamily")
			if deviceFamily == "" {
				continue
			}
			values := map[string]string{}
			for local, remote := range accessibilityFields {
				if value, ok := declaration.Attributes[remote].(bool); ok {
					values[local] = strconv.FormatBool(value)
				}
			}
			state := stringAttribute(declaration, "state")
			values["published"] = strconv.FormatBool(state == "PUBLISHED" || state == "REPLACED")
			result.Accessibility[deviceFamily] = AccessibilityDeclaration{ID: declaration.ID, Values: values}
		}
	}

	infos, err := c.list(ctx, fmt.Sprintf("/v1/apps/%s/appInfos?fields%%5BappInfos%%5D=state&limit=10", result.AppID))
	if err != nil {
		return Metadata{}, err
	}

	versionsPath := fmt.Sprintf("/v1/apps/%s/appStoreVersions?filter%%5Bplatform%%5D=%s&filter%%5BversionString%%5D=%s&fields%%5BappStoreVersions%%5D=appVersionState,copyright&limit=2", result.AppID, url.QueryEscape(platform), url.QueryEscape(version))
	versions, err := c.list(ctx, versionsPath)
	if err != nil {
		return Metadata{}, err
	}
	if len(versions) == 0 {
		return Metadata{}, fmt.Errorf("version %s for platform %s was not found", version, platform)
	}
	if len(versions) > 1 {
		return Metadata{}, fmt.Errorf("multiple versions matched %s for platform %s", version, platform)
	}
	result.VersionID = versions[0].ID
	copyAttributes(result.Values, versions[0].Attributes, appStoreVersionFields)
	appInfo, err := selectAppInfo(infos, stringAttribute(versions[0], "appVersionState"))
	if err != nil {
		return Metadata{}, err
	}
	result.AppInfoID = appInfo.ID
	if options.AgeRating {
		var ageRatingResponse singleResponse
		ageRatingPath := fmt.Sprintf("/v1/appInfos/%s/ageRatingDeclaration", result.AppInfoID)
		if err := c.doJSON(ctx, http.MethodGet, ageRatingPath, nil, &ageRatingResponse); err != nil {
			return Metadata{}, err
		}
		result.AgeRatingID = ageRatingResponse.Data.ID
		copyAgeRatingAttributes(result.Values, ageRatingResponse.Data.Attributes)
	}
	var appInfoResponse singleResponse
	categoryIncludes := strings.Join(sortedRemoteFields(categoryFields), ",")
	appInfoPath := fmt.Sprintf("/v1/appInfos/%s?include=%s", result.AppInfoID, categoryIncludes)
	if err := c.doJSON(ctx, http.MethodGet, appInfoPath, nil, &appInfoResponse); err != nil {
		return Metadata{}, err
	}
	for field, remoteField := range categoryFields {
		if related := appInfoResponse.Data.Relationships[remoteField].Data; related != nil {
			result.Values[field] = related.ID
		} else {
			result.Values[field] = ""
		}
	}

	infoLocs, err := c.list(ctx, fmt.Sprintf("/v1/appInfos/%s/appInfoLocalizations?limit=200", result.AppInfoID))
	if err != nil {
		return Metadata{}, err
	}
	for _, item := range infoLocs {
		locale := stringAttribute(item, "locale")
		loc := result.Localizations[locale]
		loc.AppInfoLocalizationID = item.ID
		if loc.Values == nil {
			loc.Values = map[string]string{}
		}
		copyAttributes(loc.Values, item.Attributes, infoFields)
		result.Localizations[locale] = loc
	}

	versionLocs, err := c.list(ctx, fmt.Sprintf("/v1/appStoreVersions/%s/appStoreVersionLocalizations?limit=200", result.VersionID))
	if err != nil {
		return Metadata{}, err
	}
	for _, item := range versionLocs {
		locale := stringAttribute(item, "locale")
		loc := result.Localizations[locale]
		loc.VersionLocalizationID = item.ID
		if loc.Values == nil {
			loc.Values = map[string]string{}
		}
		copyAttributes(loc.Values, item.Attributes, versionFields)
		result.Localizations[locale] = loc
	}
	if options.Screenshots {
		if err := c.fetchScreenshots(ctx, &result, options.DownloadAssets); err != nil {
			return Metadata{}, err
		}
	}
	if options.AppPreviews {
		if err := c.fetchAppPreviews(ctx, &result, options.DownloadAssets); err != nil {
			return Metadata{}, err
		}
	}
	return result, nil
}

func selectAppInfo(infos []resource, versionState string) (resource, error) {
	if len(infos) == 0 {
		return resource{}, errors.New("app has no app info resource")
	}
	if len(infos) == 1 {
		return infos[0], nil
	}
	var matches []resource
	var states []string
	for _, info := range infos {
		state := stringAttribute(info, "state")
		states = append(states, state)
		if versionState != "" && state == versionState {
			matches = append(matches, info)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	sort.Strings(states)
	return resource{}, fmt.Errorf("could not select one app info for version state %q from app info states %q", versionState, states)
}

var infoFields = map[string]string{
	"name":                "name",
	"subtitle":            "subtitle",
	"privacy_policy_url":  "privacyPolicyUrl",
	"privacy_choices_url": "privacyChoicesUrl",
	"privacy_policy_text": "privacyPolicyText",
}

var appFields = map[string]string{
	"accessibility_url":          "accessibilityUrl",
	"content_rights_declaration": "contentRightsDeclaration",
}

var appStoreVersionFields = map[string]string{
	"copyright": "copyright",
}

var categoryFields = map[string]string{
	"primary_category":          "primaryCategory",
	"primary_subcategory_one":   "primarySubcategoryOne",
	"primary_subcategory_two":   "primarySubcategoryTwo",
	"secondary_category":        "secondaryCategory",
	"secondary_subcategory_one": "secondarySubcategoryOne",
	"secondary_subcategory_two": "secondarySubcategoryTwo",
}

var versionFields = map[string]string{
	"description":      "description",
	"keywords":         "keywords",
	"promotional_text": "promotionalText",
	"whats_new":        "whatsNew",
	"support_url":      "supportUrl",
	"marketing_url":    "marketingUrl",
}

func (c *Client) ApplyMetadata(ctx context.Context, remote Metadata, locales []string, changes []Change) error {
	grouped := map[string]map[string]string{}
	global := map[string]map[string]any{}
	accessibilityChanges := map[string]map[string]string{}
	touchedLocales := map[string]bool{}
	for _, locale := range locales {
		touchedLocales[locale] = true
	}
	for _, change := range changes {
		if change.Locale == "" && change.DeviceFamily == "" && strings.HasPrefix(change.Field, "license_agreement_") {
			continue
		}
		if change.AssetSet != nil {
			continue
		}
		if strings.HasPrefix(change.Field, "availability.") {
			continue
		}
		if change.Locale == "" {
			if change.DeviceFamily != "" {
				deviceFamily, field := change.DeviceFamily, change.Field
				if accessibilityChanges[deviceFamily] == nil {
					accessibilityChanges[deviceFamily] = map[string]string{}
				}
				accessibilityChanges[deviceFamily][field] = change.After
				continue
			}
			group, fields, ok := globalFieldGroup(change.Field)
			if !ok {
				return fmt.Errorf("unsupported metadata field %q", change.Field)
			}
			if global[group] == nil {
				global[group] = map[string]any{}
			}
			value := any(change.After)
			if group == "app_info_categories" {
				var data any
				if change.After != "" {
					data = map[string]string{"type": "appCategories", "id": change.After}
				}
				value = map[string]any{"data": data}
			} else if group == "age_rating" {
				if change.After == "" {
					value = nil
				} else if ageRatingBooleanFields[change.Field] {
					parsed, err := strconv.ParseBool(change.After)
					if err != nil {
						return fmt.Errorf("invalid boolean value for %s: %w", change.Field, err)
					}
					value = parsed
				}
			} else if (change.Field == "accessibility_url" || change.Field == "content_rights_declaration") && change.After == "" {
				value = nil
			}
			global[group][fields[change.Field]] = value
			continue
		}
		group, ok := fieldGroup(change.Field)
		if !ok {
			return fmt.Errorf("unsupported metadata field %q", change.Field)
		}
		key := change.Locale + "\x00" + group
		if grouped[key] == nil {
			grouped[key] = map[string]string{}
		}
		grouped[key][change.Field] = change.After
		touchedLocales[change.Locale] = true
	}
	if err := c.applyLicenseAgreementChanges(ctx, remote, changes); err != nil {
		return err
	}
	if err := c.applyScreenshotChanges(ctx, changes); err != nil {
		return err
	}
	if err := c.applyAppPreviewChanges(ctx, changes); err != nil {
		return err
	}
	if err := c.applyAvailabilityChanges(ctx, remote, changes); err != nil {
		return err
	}
	globalGroups := make([]string, 0, len(global))
	for group := range global {
		globalGroups = append(globalGroups, group)
	}
	sort.Strings(globalGroups)
	for index, group := range globalGroups {
		resourceType, resourceID := "apps", remote.AppID
		section := "attributes"
		if group == "app_info_categories" {
			resourceType, resourceID = "appInfos", remote.AppInfoID
			section = "relationships"
		}
		if group == "app_store_version" {
			resourceType, resourceID = "appStoreVersions", remote.VersionID
		}
		if group == "age_rating" {
			resourceType, resourceID = "ageRatingDeclarations", remote.AgeRatingID
		}
		if err := c.patchResourceSection(ctx, resourceType, resourceID, section, global[group]); err != nil {
			if index == 0 {
				return fmt.Errorf("apply %s metadata: %w", group, err)
			}
			return fmt.Errorf("apply %s metadata after %d successful request(s): %w", group, index, err)
		}
	}
	if err := c.applyAccessibilityChanges(ctx, remote, accessibilityChanges); err != nil {
		return err
	}
	// Apple requires app-info and version localizations to contain the same
	// locale set. Add an empty group when necessary so a touched locale is
	// always created on both resources, even when only one side is managed.
	for locale := range touchedLocales {
		localization := remote.Localizations[locale]
		if localization.AppInfoLocalizationID == "" && grouped[locale+"\x00info"] == nil {
			grouped[locale+"\x00info"] = map[string]string{}
		}
		if localization.VersionLocalizationID == "" && grouped[locale+"\x00version"] == nil {
			grouped[locale+"\x00version"] = map[string]string{}
		}
	}
	keys := make([]string, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for index, key := range keys {
		parts := strings.SplitN(key, "\x00", 2)
		locale, group := parts[0], parts[1]
		loc := remote.Localizations[locale]
		fields := infoFields
		resourceType := "appInfoLocalizations"
		parentType, parentID := "appInfos", remote.AppInfoID
		resourceID := loc.AppInfoLocalizationID
		if group == "version" {
			fields = versionFields
			resourceType = "appStoreVersionLocalizations"
			parentType, parentID = "appStoreVersions", remote.VersionID
			resourceID = loc.VersionLocalizationID
		}
		attributes := map[string]string{}
		for field, value := range grouped[key] {
			remoteField, ok := fields[field]
			if !ok {
				return fmt.Errorf("metadata field %q does not belong to the %s localization", field, group)
			}
			attributes[remoteField] = value
		}
		if resourceID == "" {
			attributes["locale"] = locale
			if err := c.createLocalization(ctx, resourceType, parentType, parentID, attributes); err != nil {
				return applyLocalizationError(locale, group, len(globalGroups)+index, err)
			}
		} else if err := c.patchResource(ctx, resourceType, resourceID, attributes); err != nil {
			return applyLocalizationError(locale, group, len(globalGroups)+index, err)
		}
	}
	return nil
}

func applyLocalizationError(locale, group string, completed int, err error) error {
	if completed == 0 {
		return fmt.Errorf("apply %s.%s localization: %w", locale, group, err)
	}
	return fmt.Errorf("apply %s.%s localization after %d successful localization request(s): %w", locale, group, completed, err)
}

func MissingLocalizationResources(remote Metadata, locales []string) []string {
	var missing []string
	for _, locale := range locales {
		localization := remote.Localizations[locale]
		if localization.AppInfoLocalizationID == "" {
			missing = append(missing, locale+".app_info")
		}
		if localization.VersionLocalizationID == "" {
			missing = append(missing, locale+".version")
		}
	}
	sort.Strings(missing)
	return missing
}

func (c *Client) resolveApp(ctx context.Context, appID, bundleID string) (resource, error) {
	if appID != "" {
		var response singleResponse
		if err := c.doJSON(ctx, http.MethodGet, "/v1/apps/"+url.PathEscape(appID)+"?fields%5Bapps%5D=bundleId,accessibilityUrl,contentRightsDeclaration", nil, &response); err != nil {
			return resource{}, err
		}
		if actual := stringAttribute(response.Data, "bundleId"); actual != "" && actual != bundleID {
			return resource{}, fmt.Errorf("configured app ID belongs to bundle ID %q, not %q", actual, bundleID)
		}
		return response.Data, nil
	}
	apps, err := c.list(ctx, "/v1/apps?filter%5BbundleId%5D="+url.QueryEscape(bundleID)+"&fields%5Bapps%5D=bundleId,accessibilityUrl,contentRightsDeclaration&limit=2")
	if err != nil {
		return resource{}, err
	}
	if len(apps) == 0 {
		return resource{}, fmt.Errorf("app with bundle ID %q was not found", bundleID)
	}
	if len(apps) > 1 {
		return resource{}, fmt.Errorf("multiple apps matched bundle ID %q", bundleID)
	}
	return apps[0], nil
}

type Change struct {
	Locale       string
	DeviceFamily string
	Field        string
	Before       string
	After        string
	AssetSet     *AssetSetChange
}

func fieldGroup(field string) (string, bool) {
	if _, ok := infoFields[field]; ok {
		return "info", true
	}
	if _, ok := versionFields[field]; ok {
		return "version", true
	}
	return "", false
}

func globalFieldGroup(field string) (string, map[string]string, bool) {
	if _, ok := appFields[field]; ok {
		return "app", appFields, true
	}
	if _, ok := appStoreVersionFields[field]; ok {
		return "app_store_version", appStoreVersionFields, true
	}
	if _, ok := categoryFields[field]; ok {
		return "app_info_categories", categoryFields, true
	}
	if _, ok := ageRatingFields[field]; ok {
		return "age_rating", ageRatingFields, true
	}
	return "", nil, false
}

func (c *Client) createLocalization(ctx context.Context, resourceType, parentType, parentID string, attributes map[string]string) error {
	body := map[string]any{"data": map[string]any{
		"type": resourceType, "attributes": attributes,
		"relationships": map[string]any{singular(parentType): map[string]any{"data": map[string]string{"type": parentType, "id": parentID}}},
	}}
	return c.doJSON(ctx, http.MethodPost, "/v1/"+resourceType, body, nil)
}

func (c *Client) patchResource(ctx context.Context, resourceType, resourceID string, attributes any) error {
	return c.patchResourceSection(ctx, resourceType, resourceID, "attributes", attributes)
}

func (c *Client) patchResourceSection(ctx context.Context, resourceType, resourceID, section string, values any) error {
	body := map[string]any{"data": map[string]any{"type": resourceType, "id": resourceID, section: values}}
	return c.doJSON(ctx, http.MethodPatch, "/v1/"+resourceType+"/"+resourceID, body, nil)
}

func sortedRemoteFields(fields map[string]string) []string {
	values := make([]string, 0, len(fields))
	for _, value := range fields {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}

func singular(parentType string) string {
	if parentType == "appInfos" {
		return "appInfo"
	}
	return "appStoreVersion"
}

func (c *Client) list(ctx context.Context, path string) ([]resource, error) {
	var resources []resource
	next := path
	seen := map[string]bool{}
	for page := 0; next != ""; page++ {
		if page >= 100 {
			return nil, errors.New("pagination exceeded 100 pages")
		}
		if seen[next] {
			return nil, errors.New("pagination returned a repeated next link")
		}
		seen[next] = true
		var response listResponse
		if err := c.doJSON(ctx, http.MethodGet, next, nil, &response); err != nil {
			return nil, err
		}
		resources = append(resources, response.Data...)
		next = response.Links.Next
	}
	return resources, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, requestBody, responseBody any) error {
	var requestData []byte
	if requestBody != nil {
		data, err := json.Marshal(requestBody)
		if err != nil {
			return err
		}
		requestData = data
	}
	requestURL, err := c.resolveURL(path)
	if err != nil {
		return err
	}
	for attempt := 0; ; attempt++ {
		var body io.Reader
		if requestData != nil {
			body = bytes.NewReader(requestData)
		}
		req, err := http.NewRequestWithContext(ctx, method, requestURL, body)
		if err != nil {
			return err
		}
		token, err := c.credentials.Token(c.now())
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "ascdir")
		if requestData != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			if attempt < c.maxRetries && methodRetryable(method) {
				if sleepErr := c.sleep(ctx, c.jitter(retryDelay(attempt, "", c.now()))); sleepErr != nil {
					return sleepErr
				}
				continue
			}
			return fmt.Errorf("request to App Store Connect failed: %w", err)
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		if readErr != nil {
			return readErr
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if responseBody != nil && len(data) > 0 {
				if err := json.Unmarshal(data, responseBody); err != nil {
					return fmt.Errorf("decode response from App Store Connect: %w", err)
				}
			}
			return nil
		}
		if attempt < c.maxRetries && statusRetryable(method, resp.StatusCode) {
			retryAfter := resp.Header.Get("Retry-After")
			delay := retryDelay(attempt, retryAfter, c.now())
			if retryAfter == "" {
				delay = c.jitter(delay)
			}
			if err := c.sleep(ctx, delay); err != nil {
				return err
			}
			continue
		}
		var apiError errorResponse
		if json.Unmarshal(data, &apiError) == nil && len(apiError.Errors) > 0 {
			messages := make([]string, 0, len(apiError.Errors))
			for _, item := range apiError.Errors {
				message := fmt.Sprintf("%s (%s): %s", item.Title, item.Code, item.Detail)
				messages = append(messages, strings.TrimSpace(message))
			}
			if requestID := resp.Header.Get("X-Request-ID"); requestID != "" {
				messages = append(messages, "request ID "+requestID)
			}
			return fmt.Errorf("the App Store Connect API reported %s", strings.Join(messages, "; "))
		}
		return fmt.Errorf("the App Store Connect API returned %s", resp.Status)
	}
}

func (c *Client) resolveURL(path string) (string, error) {
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return "", err
	}
	reference, err := url.Parse(path)
	if err != nil {
		return "", err
	}
	resolved := base.ResolveReference(reference)
	if resolved.Scheme != base.Scheme || resolved.Host != base.Host {
		return "", errors.New("pagination link from App Store Connect changed origin")
	}
	return resolved.String(), nil
}

func methodRetryable(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodPatch
}

func statusRetryable(method string, status int) bool {
	if status == http.StatusTooManyRequests {
		return true
	}
	if !methodRetryable(method) {
		return false
	}
	return status == http.StatusInternalServerError || status == http.StatusBadGateway || status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout
}

func retryDelay(attempt int, retryAfter string, now time.Time) time.Duration {
	const maximumRetryAfter = 5 * time.Minute
	if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds >= 0 {
		if seconds >= int(maximumRetryAfter/time.Second) {
			return maximumRetryAfter
		}
		return time.Duration(seconds) * time.Second
	}
	if parsed, err := http.ParseTime(retryAfter); err == nil {
		return min(max(parsed.Sub(now), 0), maximumRetryAfter)
	}
	delay := time.Second << attempt
	return min(delay, 30*time.Second)
}

func retryJitter(delay time.Duration) time.Duration {
	if delay <= 0 {
		return 0
	}
	return delay + time.Duration(rand.Int64N(int64(delay/2)+1))
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func stringAttribute(item resource, key string) string {
	value, _ := item.Attributes[key].(string)
	return value
}

func copyAttributes(target map[string]string, source map[string]any, fields map[string]string) {
	for local, remote := range fields {
		if value, ok := source[remote].(string); ok {
			target[local] = value
		}
	}
}
