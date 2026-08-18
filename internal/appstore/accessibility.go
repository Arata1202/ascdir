package appstore

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
)

var accessibilityFields = map[string]string{
	"supports_audio_descriptions":                "supportsAudioDescriptions",
	"supports_captions":                          "supportsCaptions",
	"supports_dark_interface":                    "supportsDarkInterface",
	"supports_differentiate_without_color_alone": "supportsDifferentiateWithoutColorAlone",
	"supports_larger_text":                       "supportsLargerText",
	"supports_reduced_motion":                    "supportsReducedMotion",
	"supports_sufficient_contrast":               "supportsSufficientContrast",
	"supports_voice_control":                     "supportsVoiceControl",
	"supports_voiceover":                         "supportsVoiceover",
}

func (c *Client) applyAccessibilityChanges(ctx context.Context, remote Metadata, changes map[string]map[string]string) error {
	deviceFamilies := make([]string, 0, len(changes))
	for deviceFamily := range changes {
		deviceFamilies = append(deviceFamilies, deviceFamily)
	}
	sort.Strings(deviceFamilies)
	for index, deviceFamily := range deviceFamilies {
		fields := changes[deviceFamily]
		attributes := map[string]any{}
		for field, text := range fields {
			if field == "published" {
				continue
			}
			remoteField, ok := accessibilityFields[field]
			if !ok {
				return fmt.Errorf("unsupported accessibility field %q", field)
			}
			value, err := strconv.ParseBool(text)
			if err != nil {
				return fmt.Errorf("invalid boolean value for accessibility.%s.%s: %w", deviceFamily, field, err)
			}
			attributes[remoteField] = value
		}
		resourceID := remote.Accessibility[deviceFamily].ID
		if resourceID == "" {
			attributes["deviceFamily"] = deviceFamily
			body := map[string]any{"data": map[string]any{
				"type": "accessibilityDeclarations", "attributes": attributes,
				"relationships": map[string]any{"app": map[string]any{"data": map[string]string{"type": "apps", "id": remote.AppID}}},
			}}
			var response singleResponse
			if err := c.doJSON(ctx, http.MethodPost, "/v1/accessibilityDeclarations", body, &response); err != nil {
				return fmt.Errorf("create accessibility.%s declaration after %d successful declaration(s): %w", deviceFamily, index, err)
			}
			resourceID = response.Data.ID
		} else if len(attributes) > 0 {
			if err := c.patchResource(ctx, "accessibilityDeclarations", resourceID, attributes); err != nil {
				return fmt.Errorf("update accessibility.%s declaration after %d successful declaration(s): %w", deviceFamily, index, err)
			}
		}
		if fields["published"] == "true" {
			if err := c.patchResource(ctx, "accessibilityDeclarations", resourceID, map[string]any{"publish": true}); err != nil {
				return fmt.Errorf("publish accessibility.%s declaration: %w", deviceFamily, err)
			}
		}
	}
	return nil
}
