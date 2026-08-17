package appstore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.appstoreconnect.apple.com"

type Client struct {
	credentials Credentials
	baseURL     string
	httpClient  *http.Client
	now         func() time.Time
}

type Metadata struct {
	AppID         string
	AppInfoID     string
	VersionID     string
	Localizations map[string]Localization
}

type Localization struct {
	AppInfoLocalizationID string
	VersionLocalizationID string
	Values                map[string]string
}

type resource struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Attributes map[string]any `json:"attributes"`
}

type listResponse struct {
	Data []resource `json:"data"`
}

type errorResponse struct {
	Errors []struct {
		Status string `json:"status"`
		Code   string `json:"code"`
		Title  string `json:"title"`
		Detail string `json:"detail"`
	} `json:"errors"`
}

func NewClient(credentials Credentials, baseURL string) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		credentials: credentials,
		baseURL:     strings.TrimRight(baseURL, "/"),
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		now:         time.Now,
	}
}

func (m Metadata) Locales() []string {
	locales := make([]string, 0, len(m.Localizations))
	for locale := range m.Localizations {
		locales = append(locales, locale)
	}
	sort.Strings(locales)
	return locales
}

func (c *Client) FetchMetadata(ctx context.Context, bundleID, platform, version string) (Metadata, error) {
	apps, err := c.list(ctx, "/v1/apps?filter%5BbundleId%5D="+url.QueryEscape(bundleID)+"&limit=2")
	if err != nil {
		return Metadata{}, err
	}
	if len(apps) == 0 {
		return Metadata{}, fmt.Errorf("app with bundle ID %q was not found", bundleID)
	}
	if len(apps) > 1 {
		return Metadata{}, fmt.Errorf("multiple apps matched bundle ID %q", bundleID)
	}
	result := Metadata{AppID: apps[0].ID, Localizations: map[string]Localization{}}

	infos, err := c.list(ctx, fmt.Sprintf("/v1/apps/%s/appInfos?limit=10", result.AppID))
	if err != nil {
		return Metadata{}, err
	}
	if len(infos) == 0 {
		return Metadata{}, errors.New("app has no app info resource")
	}
	result.AppInfoID = infos[0].ID
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

	versionsPath := fmt.Sprintf("/v1/apps/%s/appStoreVersions?filter%%5Bplatform%%5D=%s&filter%%5BversionString%%5D=%s&limit=2", result.AppID, url.QueryEscape(platform), url.QueryEscape(version))
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
	return result, nil
}

var infoFields = map[string]string{
	"name": "name", "subtitle": "subtitle", "privacy_policy_url": "privacyPolicyUrl",
}

var versionFields = map[string]string{
	"description": "description", "keywords": "keywords", "promotional_text": "promotionalText",
	"whats_new": "whatsNew", "support_url": "supportUrl", "marketing_url": "marketingUrl",
}

func (c *Client) ApplyMetadata(ctx context.Context, remote, desired Metadata, changes []Change) error {
	grouped := map[string]map[string]string{}
	for _, change := range changes {
		key := change.Locale + "\x00" + fieldGroup(change.Field)
		if grouped[key] == nil {
			grouped[key] = map[string]string{}
		}
		grouped[key][change.Field] = change.After
	}
	keys := make([]string, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
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
			attributes[fields[field]] = value
		}
		if resourceID == "" {
			attributes["locale"] = locale
			if err := c.createLocalization(ctx, resourceType, parentType, parentID, attributes); err != nil {
				return err
			}
		} else if err := c.patchLocalization(ctx, resourceType, resourceID, attributes); err != nil {
			return err
		}
	}
	return nil
}

type Change struct {
	Locale string
	Field  string
	Before string
	After  string
}

func fieldGroup(field string) string {
	if _, ok := infoFields[field]; ok {
		return "info"
	}
	return "version"
}

func (c *Client) createLocalization(ctx context.Context, resourceType, parentType, parentID string, attributes map[string]string) error {
	body := map[string]any{"data": map[string]any{
		"type": resourceType, "attributes": attributes,
		"relationships": map[string]any{singular(parentType): map[string]any{"data": map[string]string{"type": parentType, "id": parentID}}},
	}}
	return c.doJSON(ctx, http.MethodPost, "/v1/"+resourceType, body, nil)
}

func (c *Client) patchLocalization(ctx context.Context, resourceType, resourceID string, attributes map[string]string) error {
	body := map[string]any{"data": map[string]any{"type": resourceType, "id": resourceID, "attributes": attributes}}
	return c.doJSON(ctx, http.MethodPatch, "/v1/"+resourceType+"/"+resourceID, body, nil)
}

func singular(parentType string) string {
	if parentType == "appInfos" {
		return "appInfo"
	}
	return "appStoreVersion"
}

func (c *Client) list(ctx context.Context, path string) ([]resource, error) {
	var response listResponse
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return nil, err
	}
	return response.Data, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, requestBody, responseBody any) error {
	var body io.Reader
	if requestBody != nil {
		data, err := json.Marshal(requestBody)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	token, err := c.credentials.Token(c.now())
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("App Store Connect request failed: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiError errorResponse
		if json.Unmarshal(data, &apiError) == nil && len(apiError.Errors) > 0 {
			item := apiError.Errors[0]
			return fmt.Errorf("App Store Connect API %s (%s): %s", item.Title, item.Code, item.Detail)
		}
		return fmt.Errorf("App Store Connect API returned %s", resp.Status)
	}
	if responseBody != nil && len(data) > 0 {
		if err := json.Unmarshal(data, responseBody); err != nil {
			return fmt.Errorf("decode App Store Connect response: %w", err)
		}
	}
	return nil
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
