package appstore

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

func (c *Client) fetchAvailability(ctx context.Context, result *Metadata) error {
	var response singleResponse
	path := "/v1/apps/" + url.PathEscape(result.AppID) + "/appAvailabilityV2"
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		if isAPIStatus(err, http.StatusNotFound) {
			return nil
		}
		return fmt.Errorf("read app availability: %w", err)
	}
	result.AvailabilityID = response.Data.ID
	if value, ok := response.Data.Attributes["availableInNewTerritories"].(bool); ok {
		result.Values["availability.available_in_new_territories"] = strconv.FormatBool(value)
	}
	territories, err := c.list(ctx, "/v2/appAvailabilities/"+url.PathEscape(result.AvailabilityID)+"/territoryAvailabilities?include=territory&limit=200")
	if err != nil {
		return fmt.Errorf("list territory availability: %w", err)
	}
	for _, item := range territories {
		related := item.Relationships["territory"].Data
		if related == nil {
			continue
		}
		territory := related.ID
		result.TerritoryAvailabilityIDs[territory] = item.ID
		prefix := "availability.territories." + territory + "."
		if value, ok := item.Attributes["available"].(bool); ok {
			result.Values[prefix+"available"] = strconv.FormatBool(value)
		}
		result.Values[prefix+"release_date"] = stringAttribute(item, "releaseDate")
		if value, ok := item.Attributes["preOrderEnabled"].(bool); ok {
			result.Values[prefix+"pre_order_enabled"] = strconv.FormatBool(value)
		}
	}
	return nil
}

func (c *Client) applyAvailabilityChanges(ctx context.Context, remote Metadata, changes []Change) error {
	availabilityChanges := map[string]string{}
	for _, change := range changes {
		if strings.HasPrefix(change.Field, "availability.") {
			availabilityChanges[change.Field] = change.After
		}
	}
	if len(availabilityChanges) == 0 {
		return nil
	}
	if remote.AvailabilityID == "" {
		return c.createAvailability(ctx, remote.AppID, availabilityChanges)
	}
	if _, changed := availabilityChanges["availability.available_in_new_territories"]; changed {
		return fmt.Errorf("available_in_new_territories cannot be changed after App Store Connect creates the availability resource")
	}
	byTerritory := map[string]map[string]string{}
	for field, value := range availabilityChanges {
		parts := strings.Split(field, ".")
		if len(parts) != 4 || parts[1] != "territories" {
			return fmt.Errorf("unsupported availability field %q", field)
		}
		if byTerritory[parts[2]] == nil {
			byTerritory[parts[2]] = map[string]string{}
		}
		byTerritory[parts[2]][parts[3]] = value
	}
	territories := make([]string, 0, len(byTerritory))
	for territory := range byTerritory {
		territories = append(territories, territory)
	}
	sort.Strings(territories)
	for index, territory := range territories {
		resourceID := remote.TerritoryAvailabilityIDs[territory]
		if resourceID == "" {
			return fmt.Errorf("no availability resource was returned for territory %s", territory)
		}
		attributes, err := availabilityAttributes(byTerritory[territory])
		if err != nil {
			return err
		}
		body := map[string]any{"data": map[string]any{"type": "territoryAvailabilities", "id": resourceID, "attributes": attributes}}
		if err := c.doJSON(ctx, http.MethodPatch, "/v1/territoryAvailabilities/"+url.PathEscape(resourceID), body, nil); err != nil {
			return fmt.Errorf("update territory %s after %d successful request(s): %w", territory, index, err)
		}
	}
	return nil
}

func (c *Client) createAvailability(ctx context.Context, appID string, changes map[string]string) error {
	newTerritories, ok := changes["availability.available_in_new_territories"]
	if !ok {
		return fmt.Errorf("availability.available_in_new_territories is required when creating availability")
	}
	parsedNewTerritories, err := strconv.ParseBool(newTerritories)
	if err != nil {
		return err
	}
	byTerritory := map[string]map[string]string{}
	for field, value := range changes {
		parts := strings.Split(field, ".")
		if len(parts) == 4 && parts[1] == "territories" {
			if byTerritory[parts[2]] == nil {
				byTerritory[parts[2]] = map[string]string{}
			}
			byTerritory[parts[2]][parts[3]] = value
		}
	}
	territories := make([]string, 0, len(byTerritory))
	for territory := range byTerritory {
		territories = append(territories, territory)
	}
	sort.Strings(territories)
	linkages := make([]map[string]string, 0, len(territories))
	included := make([]map[string]any, 0, len(territories))
	for index, territory := range territories {
		id := fmt.Sprintf("territory-%d", index)
		attributes, err := availabilityAttributes(byTerritory[territory])
		if err != nil {
			return err
		}
		linkages = append(linkages, map[string]string{"type": "territoryAvailabilities", "id": id})
		included = append(included, map[string]any{
			"type": "territoryAvailabilities", "id": id, "attributes": attributes,
			"relationships": map[string]any{"territory": map[string]any{"data": map[string]string{"type": "territories", "id": territory}}},
		})
	}
	body := map[string]any{
		"data": map[string]any{
			"type": "appAvailabilities", "attributes": map[string]bool{"availableInNewTerritories": parsedNewTerritories},
			"relationships": map[string]any{
				"app":                     map[string]any{"data": map[string]string{"type": "apps", "id": appID}},
				"territoryAvailabilities": map[string]any{"data": linkages},
			},
		},
		"included": included,
	}
	return c.doJSON(ctx, http.MethodPost, "/v2/appAvailabilities", body, nil)
}

func availabilityAttributes(values map[string]string) (map[string]any, error) {
	attributes := map[string]any{}
	for field, value := range values {
		switch field {
		case "available", "pre_order_enabled":
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				return nil, fmt.Errorf("invalid %s value %q", field, value)
			}
			remoteField := map[string]string{"available": "available", "pre_order_enabled": "preOrderEnabled"}[field]
			attributes[remoteField] = parsed
		case "release_date":
			if value == "" {
				attributes["releaseDate"] = nil
			} else {
				attributes["releaseDate"] = value
			}
		default:
			return nil, fmt.Errorf("unsupported territory availability field %q", field)
		}
	}
	return attributes, nil
}
