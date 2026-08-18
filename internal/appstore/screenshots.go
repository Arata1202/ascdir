package appstore

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
)

func (c *Client) fetchScreenshots(ctx context.Context, result *Metadata, download bool) error {
	for locale, localization := range result.Localizations {
		if localization.VersionLocalizationID == "" {
			continue
		}
		sets, err := c.list(ctx, "/v1/appStoreVersionLocalizations/"+url.PathEscape(localization.VersionLocalizationID)+"/appScreenshotSets?limit=200")
		if err != nil {
			return fmt.Errorf("list screenshots for %s: %w", locale, err)
		}
		for _, set := range sets {
			displayType := stringAttribute(set, "screenshotDisplayType")
			if result.ScreenshotSetIDs[locale] == nil {
				result.ScreenshotSetIDs[locale] = map[string]string{}
			}
			result.ScreenshotSetIDs[locale][displayType] = set.ID
			items, err := c.list(ctx, "/v1/appScreenshotSets/"+url.PathEscape(set.ID)+"/appScreenshots?limit=50")
			if err != nil {
				return fmt.Errorf("list screenshots for %s/%s: %w", locale, displayType, err)
			}
			assets := make([]Asset, 0, len(items))
			for _, item := range items {
				asset := Asset{ID: item.ID, FileName: stringAttribute(item, "fileName"), Checksum: strings.ToLower(stringAttribute(item, "sourceFileChecksum"))}
				if download {
					downloadURL, err := imageAssetURL(item)
					if err != nil {
						return fmt.Errorf("resolve screenshot %s/%s/%s: %w", locale, displayType, asset.FileName, err)
					}
					asset.Content, err = c.downloadAsset(ctx, downloadURL)
					if err != nil {
						return fmt.Errorf("download screenshot %s/%s/%s: %w", locale, displayType, asset.FileName, err)
					}
				}
				assets = append(assets, asset)
			}
			if result.Screenshots[locale] == nil {
				result.Screenshots[locale] = map[string][]Asset{}
			}
			result.Screenshots[locale][displayType] = assets
		}
	}
	return nil
}

func imageAssetURL(item resource) (string, error) {
	image, ok := item.Attributes["imageAsset"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("response has no image asset")
	}
	template, _ := image["templateUrl"].(string)
	width := numberString(image["width"])
	height := numberString(image["height"])
	if template == "" || width == "" || height == "" {
		return "", fmt.Errorf("response has an incomplete image asset")
	}
	format := strings.TrimPrefix(strings.ToLower(filepath.Ext(stringAttribute(item, "fileName"))), ".")
	if format == "jpg" {
		format = "jpeg"
	}
	replacer := strings.NewReplacer("{w}", width, "{h}", height, "{c}", "", "{f}", format)
	return replacer.Replace(template), nil
}

func numberString(value any) string {
	switch value := value.(type) {
	case float64:
		return strconv.FormatInt(int64(value), 10)
	case int:
		return strconv.Itoa(value)
	case jsonNumber:
		return string(value)
	default:
		return ""
	}
}

// jsonNumber is kept local to avoid enabling UseNumber for every API response.
type jsonNumber string

func (c *Client) downloadAsset(ctx context.Context, assetURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, assetURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("asset server returned %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 100<<20))
	if err != nil {
		return nil, err
	}
	if len(data) == 100<<20 {
		return nil, fmt.Errorf("asset exceeds 100 MiB")
	}
	return data, nil
}

func (c *Client) applyScreenshotChanges(ctx context.Context, changes []Change) error {
	for _, change := range changes {
		if change.AssetSet == nil || change.AssetSet.Kind != "screenshots" {
			continue
		}
		if err := c.applyScreenshotSet(ctx, *change.AssetSet); err != nil {
			return fmt.Errorf("apply screenshots.%s.%s: %w", change.AssetSet.Locale, change.AssetSet.DisplayType, err)
		}
	}
	return nil
}

func (c *Client) applyScreenshotSet(ctx context.Context, change AssetSetChange) error {
	setID := change.SetID
	if setID == "" {
		if len(change.After) == 0 {
			return nil
		}
		body := map[string]any{"data": map[string]any{
			"type":       "appScreenshotSets",
			"attributes": map[string]string{"screenshotDisplayType": change.DisplayType},
			"relationships": map[string]any{"appStoreVersionLocalization": map[string]any{"data": map[string]string{
				"type": "appStoreVersionLocalizations", "id": change.After[0].Path,
			}}},
		}}
		var response singleResponse
		if err := c.doJSON(ctx, http.MethodPost, "/v1/appScreenshotSets", body, &response); err != nil {
			return err
		}
		setID = response.Data.ID
	}
	available := map[string][]Asset{}
	for _, asset := range change.Before {
		available[asset.Checksum] = append(available[asset.Checksum], asset)
	}
	orderedIDs := make([]string, 0, len(change.After))
	kept := map[string]bool{}
	for _, desired := range change.After {
		matches := available[desired.Checksum]
		if len(matches) > 0 {
			match := matches[0]
			available[desired.Checksum] = matches[1:]
			orderedIDs = append(orderedIDs, match.ID)
			kept[match.ID] = true
			continue
		}
		id, err := c.uploadScreenshot(ctx, setID, desired)
		if err != nil {
			return err
		}
		orderedIDs = append(orderedIDs, id)
	}
	linkages := make([]map[string]string, 0, len(orderedIDs))
	for _, id := range orderedIDs {
		linkages = append(linkages, map[string]string{"type": "appScreenshots", "id": id})
	}
	body := map[string]any{"data": linkages}
	if err := c.doJSON(ctx, http.MethodPatch, "/v1/appScreenshotSets/"+url.PathEscape(setID)+"/relationships/appScreenshots", body, nil); err != nil {
		return fmt.Errorf("update display order: %w", err)
	}
	for _, existing := range change.Before {
		if !kept[existing.ID] {
			if err := c.doJSON(ctx, http.MethodDelete, "/v1/appScreenshots/"+url.PathEscape(existing.ID), nil, nil); err != nil {
				return fmt.Errorf("delete replaced screenshot %s: %w", existing.FileName, err)
			}
		}
	}
	return nil
}

func (c *Client) uploadScreenshot(ctx context.Context, setID string, asset Asset) (string, error) {
	body := map[string]any{"data": map[string]any{
		"type":          "appScreenshots",
		"attributes":    map[string]any{"fileName": asset.FileName, "fileSize": len(asset.Content)},
		"relationships": map[string]any{"appScreenshotSet": map[string]any{"data": map[string]string{"type": "appScreenshotSets", "id": setID}}},
	}}
	var response singleResponse
	if err := c.doJSON(ctx, http.MethodPost, "/v1/appScreenshots", body, &response); err != nil {
		return "", fmt.Errorf("reserve %s: %w", asset.FileName, err)
	}
	operations, _ := response.Data.Attributes["uploadOperations"].([]any)
	for _, raw := range operations {
		operation, _ := raw.(map[string]any)
		if err := c.performUploadOperation(ctx, asset.Content, operation); err != nil {
			return "", fmt.Errorf("upload %s: %w", asset.FileName, err)
		}
	}
	checksum := asset.Checksum
	if checksum == "" {
		sum := md5.Sum(asset.Content)
		checksum = hex.EncodeToString(sum[:])
	}
	attributes := map[string]any{"uploaded": true, "sourceFileChecksum": checksum}
	if err := c.patchResource(ctx, "appScreenshots", response.Data.ID, attributes); err != nil {
		return "", fmt.Errorf("commit %s: %w", asset.FileName, err)
	}
	return response.Data.ID, nil
}

func (c *Client) performUploadOperation(ctx context.Context, content []byte, operation map[string]any) error {
	offsetValue, offsetOK := operation["offset"].(float64)
	lengthValue, lengthOK := operation["length"].(float64)
	if !offsetOK || !lengthOK {
		return fmt.Errorf("upload operation has no byte range")
	}
	offset, length := int(offsetValue), int(lengthValue)
	if offset < 0 || length < 0 || offset+length > len(content) {
		return fmt.Errorf("invalid upload byte range")
	}
	method, _ := operation["method"].(string)
	uploadURL, _ := operation["url"].(string)
	parsed, err := url.Parse(uploadURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("invalid HTTPS upload URL")
	}
	req, err := http.NewRequestWithContext(ctx, method, uploadURL, bytes.NewReader(content[offset:offset+length]))
	if err != nil {
		return err
	}
	if headers, ok := operation["requestHeaders"].([]any); ok {
		for _, raw := range headers {
			header, _ := raw.(map[string]any)
			name, _ := header["name"].(string)
			value, _ := header["value"].(string)
			if name != "" {
				req.Header.Set(name, value)
			}
		}
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("upload server returned %s", resp.Status)
	}
	return nil
}
