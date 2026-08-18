package appstore

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

func (c *Client) fetchAppPreviews(ctx context.Context, result *Metadata, download bool) error {
	for locale, localization := range result.Localizations {
		if localization.VersionLocalizationID == "" {
			continue
		}
		sets, err := c.list(ctx, "/v1/appStoreVersionLocalizations/"+url.PathEscape(localization.VersionLocalizationID)+"/appPreviewSets?limit=200")
		if err != nil {
			return fmt.Errorf("list app previews for %s: %w", locale, err)
		}
		for _, set := range sets {
			previewType := stringAttribute(set, "previewType")
			if result.AppPreviewSetIDs[locale] == nil {
				result.AppPreviewSetIDs[locale] = map[string]string{}
			}
			result.AppPreviewSetIDs[locale][previewType] = set.ID
			items, err := c.list(ctx, "/v1/appPreviewSets/"+url.PathEscape(set.ID)+"/appPreviews?limit=50")
			if err != nil {
				return fmt.Errorf("list app previews for %s/%s: %w", locale, previewType, err)
			}
			assets := make([]Asset, 0, len(items))
			for _, item := range items {
				asset := Asset{
					ID: item.ID, FileName: stringAttribute(item, "fileName"),
					Checksum: strings.ToLower(stringAttribute(item, "sourceFileChecksum")),
					MIMEType: stringAttribute(item, "mimeType"), PreviewFrameTimeCode: stringAttribute(item, "previewFrameTimeCode"),
				}
				if download {
					videoURL := stringAttribute(item, "videoUrl")
					if videoURL == "" {
						return fmt.Errorf("app preview %s/%s/%s has no video URL", locale, previewType, asset.FileName)
					}
					asset.Content, err = c.downloadAsset(ctx, videoURL)
					if err != nil {
						return fmt.Errorf("download app preview %s/%s/%s: %w", locale, previewType, asset.FileName, err)
					}
				}
				assets = append(assets, asset)
			}
			if result.AppPreviews[locale] == nil {
				result.AppPreviews[locale] = map[string][]Asset{}
			}
			result.AppPreviews[locale][previewType] = assets
		}
	}
	return nil
}

func (c *Client) applyAppPreviewChanges(ctx context.Context, changes []Change) error {
	for _, change := range changes {
		if change.AssetSet == nil || change.AssetSet.Kind != "app_previews" {
			continue
		}
		if err := c.applyAppPreviewSet(ctx, *change.AssetSet); err != nil {
			return fmt.Errorf("apply app_previews.%s.%s: %w", change.AssetSet.Locale, change.AssetSet.DisplayType, err)
		}
	}
	return nil
}

func (c *Client) applyAppPreviewSet(ctx context.Context, change AssetSetChange) error {
	setID := change.SetID
	if setID == "" {
		if len(change.After) == 0 {
			return nil
		}
		body := map[string]any{"data": map[string]any{
			"type": "appPreviewSets", "attributes": map[string]string{"previewType": change.DisplayType},
			"relationships": map[string]any{"appStoreVersionLocalization": map[string]any{"data": map[string]string{
				"type": "appStoreVersionLocalizations", "id": change.After[0].Path,
			}}},
		}}
		var response singleResponse
		if err := c.doJSON(ctx, http.MethodPost, "/v1/appPreviewSets", body, &response); err != nil {
			return err
		}
		setID = response.Data.ID
	}
	available := map[string][]Asset{}
	for _, asset := range change.Before {
		key := asset.Checksum
		available[key] = append(available[key], asset)
	}
	orderedIDs := make([]string, 0, len(change.After))
	kept := map[string]bool{}
	for _, desired := range change.After {
		key := desired.Checksum
		matches := available[key]
		if len(matches) > 0 {
			match := matches[0]
			if match.PreviewFrameTimeCode != desired.PreviewFrameTimeCode {
				var frame any = desired.PreviewFrameTimeCode
				if desired.PreviewFrameTimeCode == "" {
					frame = nil
				}
				if err := c.patchResource(ctx, "appPreviews", match.ID, map[string]any{"previewFrameTimeCode": frame}); err != nil {
					return fmt.Errorf("update poster frame for %s: %w", desired.FileName, err)
				}
			}
			orderedIDs = append(orderedIDs, match.ID)
			kept[match.ID] = true
			available[key] = matches[1:]
			continue
		}
		id, err := c.uploadAppPreview(ctx, setID, desired)
		if err != nil {
			return err
		}
		orderedIDs = append(orderedIDs, id)
	}
	linkages := make([]map[string]string, 0, len(orderedIDs))
	for _, id := range orderedIDs {
		linkages = append(linkages, map[string]string{"type": "appPreviews", "id": id})
	}
	if err := c.doJSON(ctx, http.MethodPatch, "/v1/appPreviewSets/"+url.PathEscape(setID)+"/relationships/appPreviews", map[string]any{"data": linkages}, nil); err != nil {
		return fmt.Errorf("update display order: %w", err)
	}
	for _, existing := range change.Before {
		if !kept[existing.ID] {
			if err := c.doJSON(ctx, http.MethodDelete, "/v1/appPreviews/"+url.PathEscape(existing.ID), nil, nil); err != nil {
				return fmt.Errorf("delete replaced app preview %s: %w", existing.FileName, err)
			}
		}
	}
	return nil
}

func (c *Client) uploadAppPreview(ctx context.Context, setID string, asset Asset) (string, error) {
	attributes := map[string]any{"fileName": asset.FileName, "fileSize": len(asset.Content), "mimeType": asset.MIMEType}
	if asset.PreviewFrameTimeCode != "" {
		attributes["previewFrameTimeCode"] = asset.PreviewFrameTimeCode
	}
	body := map[string]any{"data": map[string]any{
		"type": "appPreviews", "attributes": attributes,
		"relationships": map[string]any{"appPreviewSet": map[string]any{"data": map[string]string{"type": "appPreviewSets", "id": setID}}},
	}}
	var response singleResponse
	if err := c.doJSON(ctx, http.MethodPost, "/v1/appPreviews", body, &response); err != nil {
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
	commit := map[string]any{"uploaded": true, "sourceFileChecksum": checksum}
	if asset.PreviewFrameTimeCode != "" {
		commit["previewFrameTimeCode"] = asset.PreviewFrameTimeCode
	}
	if err := c.patchResource(ctx, "appPreviews", response.Data.ID, commit); err != nil {
		return "", fmt.Errorf("commit %s: %w", asset.FileName, err)
	}
	return response.Data.ID, nil
}

func sortedPreviewTypes(sets map[string][]Asset) []string {
	values := make([]string, 0, len(sets))
	for value := range sets {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}
