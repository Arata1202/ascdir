package appstore

import (
	"strings"
	"testing"
)

func TestValidatePlanRejectsInvalidOperationBeforeMutation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		remote  Metadata
		locales []string
		changes []Change
		want    string
	}{
		{
			name:   "missing localization name",
			remote: Metadata{Localizations: map[string]Localization{}}, locales: []string{"fr-FR"},
			changes: []Change{{Locale: "fr-FR", Field: "description", After: "Description"}},
			want:    "fr-FR.name is required",
		},
		{
			name:    "published accessibility attributes",
			remote:  Metadata{Accessibility: map[string]AccessibilityDeclaration{"IPHONE": {ID: "a-1", State: "PUBLISHED"}}},
			changes: []Change{{DeviceFamily: "IPHONE", Field: "supports_voiceover", After: "true"}},
			want:    "only be changed while DRAFT",
		},
		{
			name:    "immutable availability option",
			remote:  Metadata{AvailabilityID: "availability-1"},
			changes: []Change{{Field: "availability.available_in_new_territories", Before: "true", After: "false"}},
			want:    "cannot be changed",
		},
		{
			name:    "scheduled price removal",
			remote:  Metadata{Values: map[string]string{"pricing.schedule": `{"base_territory":"USA","scheduled_prices":[{"price_point_id":"p1"}]}`}},
			changes: []Change{{Field: "pricing.schedule", After: `{"base_territory":"USA","scheduled_prices":[]}`}},
			want:    "cannot be removed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePlan(tt.remote, tt.locales, tt.changes)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestSelectAppInfoMapsDistinctStateEnums(t *testing.T) {
	t.Parallel()
	infos := []resource{
		{ID: "current", Attributes: map[string]any{"state": "READY_FOR_SALE"}},
		{ID: "pending", Attributes: map[string]any{"state": "PENDING_RELEASE"}},
	}
	selected, err := selectAppInfo(infos, "PENDING_DEVELOPER_RELEASE")
	if err != nil {
		t.Fatal(err)
	}
	if selected.ID != "pending" {
		t.Fatalf("selected = %s", selected.ID)
	}
}

func TestAccessibilityDraftWinsRegardlessOfResponseOrder(t *testing.T) {
	t.Parallel()
	if accessibilityStatePriority("DRAFT") <= accessibilityStatePriority("PUBLISHED") {
		t.Fatal("DRAFT must be preferred")
	}
}
