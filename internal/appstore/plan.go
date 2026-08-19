package appstore

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ValidatePlan checks every locally-detectable App Store Connect constraint
// before ApplyMetadata performs its first mutation. It is also used by CLI
// dry-runs, so a successful dry-run is a meaningful execution preflight.
func ValidatePlan(remote Metadata, locales []string, changes []Change) error {
	localeNames := map[string]string{}
	availabilityChanges := map[string]string{}
	accessibilityChanges := map[string]map[string]string{}
	for _, change := range changes {
		if change.AssetSet != nil || strings.HasPrefix(change.Field, "license_agreement_") {
			continue
		}
		if strings.HasPrefix(change.Field, "availability.") {
			availabilityChanges[change.Field] = change.After
			continue
		}
		if change.Field == "pricing.schedule" {
			if err := validatePricingChange(remote, change.After); err != nil {
				return err
			}
			continue
		}
		if change.DeviceFamily != "" {
			if accessibilityChanges[change.DeviceFamily] == nil {
				accessibilityChanges[change.DeviceFamily] = map[string]string{}
			}
			accessibilityChanges[change.DeviceFamily][change.Field] = change.After
			continue
		}
		if change.Locale != "" {
			group, ok := fieldGroup(change.Field)
			if !ok {
				return fmt.Errorf("unsupported metadata field %q", change.Field)
			}
			if group == "info" && change.Field == "name" {
				localeNames[change.Locale] = change.After
			}
			continue
		}
		group, fields, ok := globalFieldGroup(change.Field)
		if !ok {
			return fmt.Errorf("unsupported metadata field %q", change.Field)
		}
		if group == "age_rating" && ageRatingBooleanFields[change.Field] && change.After != "" {
			if _, err := strconv.ParseBool(change.After); err != nil {
				return fmt.Errorf("invalid boolean value for %s: %w", change.Field, err)
			}
		}
		if fields[change.Field] == "" {
			return fmt.Errorf("unsupported metadata field %q", change.Field)
		}
	}
	for _, locale := range locales {
		if remote.Localizations[locale].AppInfoLocalizationID != "" {
			continue
		}
		name := localeNames[locale]
		if name == "" {
			name = remote.Localizations[locale].Values["name"]
		}
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("%s.name is required to create the App Info localization", locale)
		}
	}
	if err := validateAccessibilityChanges(remote, accessibilityChanges); err != nil {
		return err
	}
	return validateAvailabilityChange(remote, availabilityChanges)
}

func validateAccessibilityChanges(remote Metadata, changes map[string]map[string]string) error {
	for deviceFamily, fields := range changes {
		declaration := remote.Accessibility[deviceFamily]
		for field, text := range fields {
			if field == "published" {
				if _, err := strconv.ParseBool(text); err != nil {
					return fmt.Errorf("invalid boolean value for accessibility.%s.published: %w", deviceFamily, err)
				}
				continue
			}
			if _, ok := accessibilityFields[field]; !ok {
				return fmt.Errorf("unsupported accessibility field %q", field)
			}
			if _, err := strconv.ParseBool(text); err != nil {
				return fmt.Errorf("invalid boolean value for accessibility.%s.%s: %w", deviceFamily, field, err)
			}
			if declaration.ID != "" && declaration.State != "" && declaration.State != "DRAFT" {
				return fmt.Errorf("accessibility.%s declaration is %s; feature attributes can only be changed while DRAFT", deviceFamily, declaration.State)
			}
		}
	}
	return nil
}

func validateAvailabilityChange(remote Metadata, changes map[string]string) error {
	if len(changes) == 0 {
		return nil
	}
	if remote.AvailabilityID == "" {
		if _, ok := changes["availability.available_in_new_territories"]; !ok {
			return fmt.Errorf("availability.available_in_new_territories is required when creating availability")
		}
	} else if _, changed := changes["availability.available_in_new_territories"]; changed {
		return fmt.Errorf("available_in_new_territories cannot be changed after App Store Connect creates the availability resource")
	}
	for field, value := range changes {
		if field == "availability.available_in_new_territories" {
			if _, err := strconv.ParseBool(value); err != nil {
				return fmt.Errorf("invalid available_in_new_territories value %q", value)
			}
			continue
		}
		parts := strings.Split(field, ".")
		if len(parts) != 4 || parts[1] != "territories" {
			return fmt.Errorf("unsupported availability field %q", field)
		}
		if remote.AvailabilityID != "" && remote.TerritoryAvailabilityIDs[parts[2]] == "" {
			return fmt.Errorf("no availability resource was returned for territory %s", parts[2])
		}
		if _, err := availabilityAttributes(map[string]string{parts[3]: value}); err != nil {
			return err
		}
	}
	return nil
}

func validatePricingChange(remote Metadata, desiredJSON string) error {
	var desired, existing pricingSchedule
	if err := json.Unmarshal([]byte(desiredJSON), &desired); err != nil {
		return fmt.Errorf("decode desired pricing schedule: %w", err)
	}
	if value := remote.Values["pricing.schedule"]; value != "" {
		if err := json.Unmarshal([]byte(value), &existing); err != nil {
			return fmt.Errorf("decode remote pricing schedule: %w", err)
		}
	}
	existingCounts, desiredCounts := map[string]int{}, map[string]int{}
	for _, price := range existing.ScheduledPrices {
		existingCounts[scheduledPriceKey(price)]++
	}
	for _, price := range desired.ScheduledPrices {
		desiredCounts[scheduledPriceKey(price)]++
	}
	for key, count := range existingCounts {
		if desiredCounts[key] < count {
			return fmt.Errorf("existing scheduled prices cannot be removed through the App Store Connect API")
		}
	}
	return nil
}
