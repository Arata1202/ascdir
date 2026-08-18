package appstore

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

type linkageResponse struct {
	Data *resourceIdentifier `json:"data"`
}

func (c *Client) fetchLicenseAgreement(ctx context.Context, result *Metadata) error {
	var linkage linkageResponse
	path := fmt.Sprintf("/v1/apps/%s/relationships/endUserLicenseAgreement", result.AppID)
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &linkage); err != nil {
		return fmt.Errorf("read custom license agreement linkage: %w", err)
	}
	result.Values["license_agreement_text"] = ""
	result.Values["license_agreement_territories"] = ""
	if linkage.Data == nil {
		return nil
	}
	result.LicenseAgreementID = linkage.Data.ID
	var agreement singleResponse
	if err := c.doJSON(ctx, http.MethodGet, "/v1/endUserLicenseAgreements/"+linkage.Data.ID, nil, &agreement); err != nil {
		return fmt.Errorf("read custom license agreement: %w", err)
	}
	result.Values["license_agreement_text"] = stringAttribute(agreement.Data, "agreementText")
	territories, err := c.list(ctx, "/v1/endUserLicenseAgreements/"+linkage.Data.ID+"/territories?limit=200")
	if err != nil {
		return fmt.Errorf("list custom license agreement territories: %w", err)
	}
	ids := make([]string, 0, len(territories))
	for _, territory := range territories {
		ids = append(ids, territory.ID)
	}
	sort.Strings(ids)
	result.Values["license_agreement_territories"] = strings.Join(ids, ",")
	return nil
}

func (c *Client) applyLicenseAgreementChanges(ctx context.Context, remote Metadata, changes []Change) error {
	text, territories := remote.Values["license_agreement_text"], remote.Values["license_agreement_territories"]
	changed := false
	for _, change := range changes {
		switch change.Field {
		case "license_agreement_text":
			text, changed = change.After, true
		case "license_agreement_territories":
			territories, changed = change.After, true
		}
	}
	if !changed {
		return nil
	}
	if text == "" && territories == "" {
		if remote.LicenseAgreementID == "" {
			return nil
		}
		return c.doJSON(ctx, http.MethodDelete, "/v1/endUserLicenseAgreements/"+remote.LicenseAgreementID, nil, nil)
	}
	if strings.TrimSpace(text) == "" || strings.TrimSpace(territories) == "" {
		return fmt.Errorf("custom license agreement requires both text and at least one territory")
	}
	ids := splitTerritoryIDs(territories)
	relationships := make([]map[string]string, 0, len(ids))
	for _, id := range ids {
		relationships = append(relationships, map[string]string{"type": "territories", "id": id})
	}
	if remote.LicenseAgreementID == "" {
		body := map[string]any{"data": map[string]any{
			"type":       "endUserLicenseAgreements",
			"attributes": map[string]string{"agreementText": text},
			"relationships": map[string]any{
				"app":         map[string]any{"data": map[string]string{"type": "apps", "id": remote.AppID}},
				"territories": map[string]any{"data": relationships},
			},
		}}
		return c.doJSON(ctx, http.MethodPost, "/v1/endUserLicenseAgreements", body, nil)
	}
	body := map[string]any{"data": map[string]any{
		"type": "endUserLicenseAgreements", "id": remote.LicenseAgreementID,
		"attributes":    map[string]string{"agreementText": text},
		"relationships": map[string]any{"territories": map[string]any{"data": relationships}},
	}}
	return c.doJSON(ctx, http.MethodPatch, "/v1/endUserLicenseAgreements/"+remote.LicenseAgreementID, body, nil)
}

func splitTerritoryIDs(value string) []string {
	parts := strings.Split(value, ",")
	ids := make([]string, 0, len(parts))
	for _, part := range parts {
		if id := strings.TrimSpace(part); id != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}
