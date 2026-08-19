package appstore

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const assetDeliveryTimeout = 10 * time.Minute

func validateFetchedDeliveryState(item resource, attribute string) error {
	state, detail := deliveryState(item.Attributes[attribute])
	if state == "" || state == "COMPLETE" {
		return nil
	}
	if detail != "" {
		return fmt.Errorf("delivery state is %s: %s", state, detail)
	}
	return fmt.Errorf("delivery state is %s", state)
}

func verifyAssetUnchanged(asset Asset) error {
	if asset.Path == "" {
		if asset.Size != 0 && asset.Size != int64(len(asset.Content)) {
			return fmt.Errorf("size changed from %d to %d bytes", asset.Size, len(asset.Content))
		}
		sum := md5.Sum(asset.Content)
		if asset.Checksum != "" && !strings.EqualFold(asset.Checksum, hex.EncodeToString(sum[:])) {
			return fmt.Errorf("checksum no longer matches")
		}
		return nil
	}
	file, err := os.Open(asset.Path)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("asset is not a regular file")
	}
	if info.Size() != asset.Size {
		return fmt.Errorf("size changed from %d to %d bytes", asset.Size, info.Size())
	}
	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	if asset.Checksum != "" && !strings.EqualFold(asset.Checksum, hex.EncodeToString(hash.Sum(nil))) {
		return fmt.Errorf("checksum no longer matches")
	}
	return nil
}

func (c *Client) waitForAssetDelivery(ctx context.Context, resourceType, resourceID, attribute string) error {
	waitContext, cancel := context.WithTimeout(ctx, assetDeliveryTimeout)
	defer cancel()
	for {
		var response singleResponse
		path := "/v1/" + resourceType + "/" + url.PathEscape(resourceID)
		if err := c.doJSON(waitContext, http.MethodGet, path, nil, &response); err != nil {
			return fmt.Errorf("check delivery state: %w", err)
		}
		state, detail := deliveryState(response.Data.Attributes[attribute])
		switch state {
		case "COMPLETE":
			return nil
		case "FAILED":
			if detail == "" {
				detail = "Apple did not provide an error detail"
			}
			return fmt.Errorf("delivery failed: %s", detail)
		case "":
			return fmt.Errorf("response has no %s state", attribute)
		}
		if err := c.sleep(waitContext, c.assetPollInterval); err != nil {
			return fmt.Errorf("wait for delivery: %w", err)
		}
	}
}

func deliveryState(value any) (string, string) {
	object, ok := value.(map[string]any)
	if !ok {
		return "", ""
	}
	state, _ := object["state"].(string)
	var details []string
	for _, key := range []string{"errors", "warnings"} {
		items, _ := object[key].([]any)
		for _, item := range items {
			entry, _ := item.(map[string]any)
			for _, field := range []string{"description", "detail", "code"} {
				if text, _ := entry[field].(string); text != "" {
					details = append(details, text)
					break
				}
			}
		}
	}
	return strings.ToUpper(state), strings.Join(details, "; ")
}
