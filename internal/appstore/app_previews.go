package appstore

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const maximumAppPreviewDownloadSize = 1 << 30

func (c *Client) fetchAppPreviews(ctx context.Context, result *Metadata, download bool) (err error) {
	var downloaded []string
	defer func() {
		if err != nil {
			for _, path := range downloaded {
				_ = os.Remove(path)
			}
		}
	}()
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
					asset.Path, asset.Size, err = c.downloadAssetToFile(ctx, videoURL)
					if err != nil {
						return fmt.Errorf("download app preview %s/%s/%s: %w", locale, previewType, asset.FileName, err)
					}
					downloaded = append(downloaded, asset.Path)
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

func (c *Client) applyAppPreviewSet(ctx context.Context, change AssetSetChange) (err error) {
	setID := change.SetID
	createdSet := false
	if setID == "" {
		if len(change.After) == 0 {
			return nil
		}
		if change.LocalizationID == "" {
			return fmt.Errorf("version localization ID is missing")
		}
		body := map[string]any{"data": map[string]any{
			"type": "appPreviewSets", "attributes": map[string]string{"previewType": change.DisplayType},
			"relationships": map[string]any{"appStoreVersionLocalization": map[string]any{"data": map[string]string{
				"type": "appStoreVersionLocalizations", "id": change.LocalizationID,
			}}},
		}}
		var response singleResponse
		if err := c.doJSON(ctx, http.MethodPost, "/v1/appPreviewSets", body, &response); err != nil {
			return err
		}
		setID = response.Data.ID
		if setID == "" {
			return errors.New("create app preview set: response has no resource ID")
		}
		createdSet = true
	}
	available := map[string][]Asset{}
	for _, asset := range change.Before {
		key := asset.Checksum
		available[key] = append(available[key], asset)
	}
	orderedIDs := make([]string, 0, len(change.After))
	var uploadedIDs []string
	orderUpdated := false
	defer func() {
		if err == nil || orderUpdated {
			return
		}
		for _, id := range uploadedIDs {
			if cleanupErr := c.cleanupReservedResource(ctx, "appPreviews", id); cleanupErr != nil {
				err = errors.Join(err, fmt.Errorf("clean up uploaded app preview %s: %w", id, cleanupErr))
			}
		}
		if createdSet {
			if cleanupErr := c.cleanupReservedResource(ctx, "appPreviewSets", setID); cleanupErr != nil {
				err = errors.Join(err, fmt.Errorf("clean up app preview set %s: %w", setID, cleanupErr))
			}
		}
	}()
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
		uploadedIDs = append(uploadedIDs, id)
	}
	linkages := make([]map[string]string, 0, len(orderedIDs))
	for _, id := range orderedIDs {
		linkages = append(linkages, map[string]string{"type": "appPreviews", "id": id})
	}
	if err := c.doJSON(ctx, http.MethodPatch, "/v1/appPreviewSets/"+url.PathEscape(setID)+"/relationships/appPreviews", map[string]any{"data": linkages}, nil); err != nil {
		return fmt.Errorf("update display order: %w", err)
	}
	orderUpdated = true
	for _, existing := range change.Before {
		if !kept[existing.ID] {
			if err := c.doJSON(ctx, http.MethodDelete, "/v1/appPreviews/"+url.PathEscape(existing.ID), nil, nil); err != nil {
				return fmt.Errorf("delete replaced app preview %s: %w", existing.FileName, err)
			}
		}
	}
	return nil
}

func (c *Client) uploadAppPreview(ctx context.Context, setID string, asset Asset) (_ string, err error) {
	size := asset.Size
	if size == 0 {
		size = int64(len(asset.Content))
	}
	attributes := map[string]any{"fileName": asset.FileName, "fileSize": size, "mimeType": asset.MIMEType}
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
	if response.Data.ID == "" {
		return "", fmt.Errorf("reserve %s: response has no resource ID", asset.FileName)
	}
	committed := false
	defer func() {
		if err == nil || committed || response.Data.ID == "" {
			return
		}
		if cleanupErr := c.cleanupReservedResource(ctx, "appPreviews", response.Data.ID); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("clean up reserved app preview %s: %w", response.Data.ID, cleanupErr))
		}
	}()
	operations, err := validatedUploadOperations(response.Data.Attributes["uploadOperations"], size)
	if err != nil {
		return "", fmt.Errorf("reserve %s: %w", asset.FileName, err)
	}
	for _, operation := range operations {
		if err := c.performUploadOperation(ctx, asset, operation); err != nil {
			return "", fmt.Errorf("upload %s: %w", asset.FileName, err)
		}
	}
	checksum := asset.Checksum
	if checksum == "" && len(asset.Content) > 0 {
		sum := md5.Sum(asset.Content)
		checksum = hex.EncodeToString(sum[:])
	}
	if checksum == "" {
		return "", fmt.Errorf("commit %s: source checksum is missing", asset.FileName)
	}
	commit := map[string]any{"uploaded": true, "sourceFileChecksum": checksum}
	if asset.PreviewFrameTimeCode != "" {
		commit["previewFrameTimeCode"] = asset.PreviewFrameTimeCode
	}
	if err := c.patchResource(ctx, "appPreviews", response.Data.ID, commit); err != nil {
		return "", fmt.Errorf("commit %s: %w", asset.FileName, err)
	}
	committed = true
	return response.Data.ID, nil
}

func (c *Client) downloadAssetToFile(ctx context.Context, assetURL string) (_ string, _ int64, err error) {
	resp, err := c.getAsset(ctx, assetURL)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", 0, fmt.Errorf("asset server returned %s", resp.Status)
	}
	temporary, err := os.CreateTemp("", "ascdir-app-preview-*")
	if err != nil {
		return "", 0, err
	}
	path := temporary.Name()
	defer func() {
		if err != nil {
			_ = temporary.Close()
			_ = os.Remove(path)
		}
	}()
	written, err := copyWithLimit(temporary, resp.Body, maximumAppPreviewDownloadSize)
	if err != nil {
		return "", 0, err
	}
	if err := temporary.Close(); err != nil {
		return "", 0, err
	}
	return path, written, nil
}

func validateAssetDownloadURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("parse asset URL: %w", err)
	}
	if parsed.Scheme != "https" || parsed.Host == "" {
		return errors.New("asset download URL must use HTTPS")
	}
	return nil
}

func (c *Client) getAsset(ctx context.Context, assetURL string) (*http.Response, error) {
	if err := validateAssetDownloadURL(assetURL); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, assetURL, nil)
	if err != nil {
		return nil, err
	}
	client := *c.httpClient
	previousCheckRedirect := client.CheckRedirect
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if err := validateAssetDownloadURL(req.URL.String()); err != nil {
			return err
		}
		if previousCheckRedirect != nil {
			return previousCheckRedirect(req, via)
		}
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
	return client.Do(req)
}

func copyWithLimit(destination io.Writer, source io.Reader, maximum int64) (int64, error) {
	written, err := io.Copy(destination, io.LimitReader(source, maximum+1))
	if err != nil {
		return 0, err
	}
	if written > maximum {
		return 0, fmt.Errorf("asset exceeds %d bytes", maximum)
	}
	return written, nil
}
