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
	"os"
	"path/filepath"
	"sort"
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

type uploadOperation struct {
	Method  string
	URL     string
	Offset  int64
	Length  int64
	Headers http.Header
}

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
	size := asset.Size
	if size == 0 {
		size = int64(len(asset.Content))
	}
	body := map[string]any{"data": map[string]any{
		"type":          "appScreenshots",
		"attributes":    map[string]any{"fileName": asset.FileName, "fileSize": size},
		"relationships": map[string]any{"appScreenshotSet": map[string]any{"data": map[string]string{"type": "appScreenshotSets", "id": setID}}},
	}}
	var response singleResponse
	if err := c.doJSON(ctx, http.MethodPost, "/v1/appScreenshots", body, &response); err != nil {
		return "", fmt.Errorf("reserve %s: %w", asset.FileName, err)
	}
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
	attributes := map[string]any{"uploaded": true, "sourceFileChecksum": checksum}
	if err := c.patchResource(ctx, "appScreenshots", response.Data.ID, attributes); err != nil {
		return "", fmt.Errorf("commit %s: %w", asset.FileName, err)
	}
	return response.Data.ID, nil
}

func (c *Client) performUploadOperation(ctx context.Context, asset Asset, operation uploadOperation) error {
	parsed, err := url.Parse(operation.URL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("invalid HTTPS upload URL")
	}
	var reader io.Reader
	var file *os.File
	if asset.Path != "" {
		file, err = os.Open(asset.Path)
		if err != nil {
			return err
		}
		defer file.Close()
		reader = io.NewSectionReader(file, operation.Offset, operation.Length)
	} else {
		reader = bytes.NewReader(asset.Content[operation.Offset : operation.Offset+operation.Length])
	}
	req, err := http.NewRequestWithContext(ctx, operation.Method, operation.URL, reader)
	if err != nil {
		return err
	}
	req.ContentLength = operation.Length
	for name, values := range operation.Headers {
		for _, value := range values {
			req.Header.Add(name, value)
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

func validatedUploadOperations(value any, size int64) ([]uploadOperation, error) {
	rawOperations, ok := value.([]any)
	if !ok || len(rawOperations) == 0 {
		return nil, fmt.Errorf("response has no upload operations")
	}
	operations := make([]uploadOperation, 0, len(rawOperations))
	for index, raw := range rawOperations {
		item, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("upload operation %d is malformed", index)
		}
		offset, offsetOK := exactInt64(item["offset"])
		length, lengthOK := exactInt64(item["length"])
		method, methodOK := item["method"].(string)
		uploadURL, urlOK := item["url"].(string)
		if !offsetOK || !lengthOK || !methodOK || !urlOK || offset < 0 || length <= 0 || offset > size-length {
			return nil, fmt.Errorf("upload operation %d is invalid", index)
		}
		if method != http.MethodPut && method != http.MethodPost {
			return nil, fmt.Errorf("upload operation %d uses unsupported method %q", index, method)
		}
		headers := http.Header{}
		if rawHeaders, exists := item["requestHeaders"]; exists {
			list, ok := rawHeaders.([]any)
			if !ok {
				return nil, fmt.Errorf("upload operation %d has malformed headers", index)
			}
			for _, rawHeader := range list {
				header, ok := rawHeader.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("upload operation %d has a malformed header", index)
				}
				name, nameOK := header["name"].(string)
				value, valueOK := header["value"].(string)
				if !nameOK || !valueOK || strings.TrimSpace(name) == "" {
					return nil, fmt.Errorf("upload operation %d has an invalid header", index)
				}
				headers.Add(name, value)
			}
		}
		operations = append(operations, uploadOperation{Method: method, URL: uploadURL, Offset: offset, Length: length, Headers: headers})
	}
	sort.Slice(operations, func(i, j int) bool { return operations[i].Offset < operations[j].Offset })
	next := int64(0)
	for _, operation := range operations {
		if operation.Offset != next {
			return nil, fmt.Errorf("upload operations do not cover the file exactly")
		}
		next += operation.Length
	}
	if next != size {
		return nil, fmt.Errorf("upload operations do not cover the file exactly")
	}
	return operations, nil
}

func exactInt64(value any) (int64, bool) {
	number, ok := value.(float64)
	if !ok || number < 0 || number != float64(int64(number)) {
		return 0, false
	}
	return int64(number), true
}
