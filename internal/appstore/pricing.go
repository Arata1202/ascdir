package appstore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
)

type pricingSchedule struct {
	BaseTerritory   string           `json:"base_territory"`
	ScheduledPrices []scheduledPrice `json:"scheduled_prices"`
}

type scheduledPrice struct {
	PricePointID string  `json:"price_point_id"`
	StartDate    *string `json:"start_date,omitempty"`
	EndDate      *string `json:"end_date,omitempty"`
}

func (c *Client) fetchPricing(ctx context.Context, result *Metadata) error {
	var scheduleResponse singleResponse
	if err := c.doJSON(ctx, http.MethodGet, "/v1/apps/"+url.PathEscape(result.AppID)+"/appPriceSchedule", nil, &scheduleResponse); err != nil {
		return fmt.Errorf("read app price schedule: %w", err)
	}
	result.PriceScheduleID = scheduleResponse.Data.ID
	var baseResponse singleResponse
	if err := c.doJSON(ctx, http.MethodGet, "/v1/appPriceSchedules/"+url.PathEscape(result.PriceScheduleID)+"/baseTerritory", nil, &baseResponse); err != nil {
		return fmt.Errorf("read app price base territory: %w", err)
	}
	schedule := pricingSchedule{BaseTerritory: baseResponse.Data.ID}
	path := "/v1/appPriceSchedules/" + url.PathEscape(result.PriceScheduleID) + "/manualPrices?include=appPricePoint,territory&limit=200"
	prices, err := c.list(ctx, path)
	if err != nil {
		return fmt.Errorf("list app price schedule: %w", err)
	}
	for _, price := range prices {
		territory := price.Relationships["territory"].Data
		point := price.Relationships["appPricePoint"].Data
		if territory == nil || point == nil || territory.ID != schedule.BaseTerritory {
			continue
		}
		item := scheduledPrice{PricePointID: point.ID}
		if value := stringAttribute(price, "startDate"); value != "" {
			item.StartDate = &value
		}
		if value := stringAttribute(price, "endDate"); value != "" {
			item.EndDate = &value
		}
		schedule.ScheduledPrices = append(schedule.ScheduledPrices, item)
	}
	sortScheduledPrices(schedule.ScheduledPrices)
	data, err := json.Marshal(schedule)
	if err != nil {
		return err
	}
	result.Values["pricing.schedule"] = string(data)
	return nil
}

func (c *Client) applyPricingChanges(ctx context.Context, remote Metadata, changes []Change) error {
	var desiredJSON string
	for _, change := range changes {
		if change.Field == "pricing.schedule" {
			desiredJSON = change.After
		}
	}
	if desiredJSON == "" {
		return nil
	}
	var desired, existing pricingSchedule
	if err := json.Unmarshal([]byte(desiredJSON), &desired); err != nil {
		return fmt.Errorf("decode desired pricing schedule: %w", err)
	}
	if value := remote.Values["pricing.schedule"]; value != "" {
		if err := json.Unmarshal([]byte(value), &existing); err != nil {
			return fmt.Errorf("decode remote pricing schedule: %w", err)
		}
	}
	existingCounts := map[string]int{}
	for _, price := range existing.ScheduledPrices {
		existingCounts[scheduledPriceKey(price)]++
	}
	desiredCounts := map[string]int{}
	for _, price := range desired.ScheduledPrices {
		desiredCounts[scheduledPriceKey(price)]++
	}
	for key, count := range existingCounts {
		if desiredCounts[key] < count {
			return fmt.Errorf("existing scheduled prices cannot be removed through the App Store Connect API")
		}
	}
	remaining := map[string]int{}
	for key, count := range existingCounts {
		remaining[key] = count
	}
	var additions []scheduledPrice
	for _, price := range desired.ScheduledPrices {
		key := scheduledPriceKey(price)
		if remaining[key] > 0 {
			remaining[key]--
			continue
		}
		additions = append(additions, price)
	}
	if len(additions) == 0 && desired.BaseTerritory == existing.BaseTerritory {
		return nil
	}
	if len(additions) == 0 {
		additions = append(additions, desired.ScheduledPrices...)
	}
	return c.createPriceSchedule(ctx, remote.AppID, desired.BaseTerritory, additions)
}

func (c *Client) createPriceSchedule(ctx context.Context, appID, baseTerritory string, prices []scheduledPrice) error {
	linkages := make([]map[string]string, 0, len(prices))
	included := make([]map[string]any, 0, len(prices))
	for index, price := range prices {
		id := fmt.Sprintf("price-%d", index)
		attributes := map[string]any{}
		if price.StartDate != nil {
			attributes["startDate"] = *price.StartDate
		}
		if price.EndDate != nil {
			attributes["endDate"] = *price.EndDate
		}
		linkages = append(linkages, map[string]string{"type": "appPrices", "id": id})
		included = append(included, map[string]any{
			"type": "appPrices", "id": id, "attributes": attributes,
			"relationships": map[string]any{"appPricePoint": map[string]any{"data": map[string]string{"type": "appPricePoints", "id": price.PricePointID}}},
		})
	}
	body := map[string]any{
		"data": map[string]any{
			"type": "appPriceSchedules",
			"relationships": map[string]any{
				"app":           map[string]any{"data": map[string]string{"type": "apps", "id": appID}},
				"baseTerritory": map[string]any{"data": map[string]string{"type": "territories", "id": baseTerritory}},
				"manualPrices":  map[string]any{"data": linkages},
			},
		},
		"included": included,
	}
	return c.doJSON(ctx, http.MethodPost, "/v1/appPriceSchedules", body, nil)
}

func scheduledPriceKey(price scheduledPrice) string {
	start, end := "", ""
	if price.StartDate != nil {
		start = *price.StartDate
	}
	if price.EndDate != nil {
		end = *price.EndDate
	}
	return start + "\x00" + end + "\x00" + price.PricePointID
}

func sortScheduledPrices(prices []scheduledPrice) {
	sort.SliceStable(prices, func(i, j int) bool { return scheduledPriceKey(prices[i]) < scheduledPriceKey(prices[j]) })
}
