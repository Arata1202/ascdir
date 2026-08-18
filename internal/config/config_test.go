package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveAndLoad(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ascdir.yaml")
	want := New("123", "com.example.app", "IOS", "1.2.0", []string{"ja", "en-US"})
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.App != want.App || len(got.Localizations) != 2 {
		t.Fatalf("loaded config mismatch: %#v", got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"version: \"2\"", "# ascdir configuration schema version", "# App bundle identifier", "metadata:", "copyright:", "accessibility_url:", "content_rights_declaration:", "categories:", "primary_category:", "secondary_category:", "values:", "files:", "# App Store display name", "# Required plain-text product description; Markdown is not rendered"} {
		if !strings.Contains(string(data), expected) {
			t.Fatalf("generated config is missing %q:\n%s", expected, data)
		}
	}
	if strings.Contains(string(data), "privacy_policy_text:") {
		t.Fatalf("iOS config unexpectedly manages tvOS privacy policy text:\n%s", data)
	}
}

func TestNewIncludesPrivacyPolicyTextOnlyForTVOS(t *testing.T) {
	t.Parallel()
	for _, platform := range []string{"IOS", "MAC_OS", "VISION_OS"} {
		cfg := New("123", "com.example.app", platform, "1.0", []string{"en-US"})
		if got := cfg.Localizations["en-US"].Files.PrivacyPolicyText; got != "" {
			t.Errorf("%s privacy policy text path = %q", platform, got)
		}
	}
	cfg := New("123", "com.example.app", "TV_OS", "1.0", []string{"en-US"})
	if got := cfg.Localizations["en-US"].Files.PrivacyPolicyText; got != "metadata/en-US/privacy_policy.md" {
		t.Fatalf("TV_OS privacy policy text path = %q", got)
	}
	path := filepath.Join(t.TempDir(), "ascdir.yaml")
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "privacy_policy_text: metadata/en-US/privacy_policy.md # Required plain-text tvOS privacy policy; TV_OS only") {
		t.Fatalf("generated tvOS config lacks the privacy policy explanation:\n%s", data)
	}
}

func TestEncodeUpdatedValuesUpdatesLicenseTerritories(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ascdir.yaml")
	cfg := New("123", "com.example.app", "IOS", "1.0", []string{"en-US"})
	cfg.LicenseAgreement = &LicenseAgreementValues{File: "metadata/license.md", Territories: []string{"USA"}}
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	cfg.LicenseAgreement.Territories = []string{"JPN", "USA"}
	data, err := EncodeUpdatedValues(path, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "- JPN") || !strings.Contains(string(data), "- USA") {
		t.Fatalf("updated config =\n%s", data)
	}
}

func TestSaveCommentsOptionalSections(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ascdir.yaml")
	availableInNewTerritories := false
	available := true
	cfg := New("123", "com.example.app", "IOS", "1.0", []string{"en-US"})
	cfg.LicenseAgreement = &LicenseAgreementValues{File: "metadata/license_agreement.md", Territories: []string{"USA"}}
	cfg.Assets = AssetPaths{Screenshots: "assets/screenshots", AppPreviews: "assets/app-previews", PreviewFrameTimes: map[string]string{"en-US/IPHONE_67/01.mp4": "00:00:05"}}
	cfg.Availability = &AvailabilityValues{AvailableInNewTerritories: &availableInNewTerritories, Territories: map[string]TerritoryAvailability{"USA": {Available: &available}}}
	cfg.Pricing = &PricingValues{BaseTerritory: "USA", ScheduledPrices: []ScheduledPrice{{PricePointID: "point-1"}}}
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"# Optional plain-text custom EULA; omit this section to leave it unmanaged",
		"# Managed screenshot root, structured by locale and display type",
		"# Initial setting for territories Apple adds later",
		"# Whether the app is available in this territory",
		"# Three-letter App Store base territory ID",
		"# App Store Connect price-point ID",
	} {
		if !strings.Contains(string(data), expected) {
			t.Fatalf("generated optional section comments are missing %q:\n%s", expected, data)
		}
	}
}

func TestEncodeUpdatedValuesUpdatesPreviewFrameTimes(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ascdir.yaml")
	cfg := New("123", "com.example.app", "IOS", "1.0", []string{"en-US"})
	cfg.Assets.AppPreviews = "assets/app-previews"
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	cfg.Assets.PreviewFrameTimes = map[string]string{"en-US/IPHONE_67/01.mp4": "00:00:05"}
	data, err := EncodeUpdatedValues(path, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `en-US/IPHONE_67/01.mp4: "00:00:05"`) {
		t.Fatalf("updated config =\n%s", data)
	}
}

func TestPricingRoundTripAndDateValidation(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ascdir.yaml")
	start := "2026-09-01"
	cfg := New("123", "com.example.app", "IOS", "1.0", []string{"en-US"})
	cfg.Pricing = &PricingValues{BaseTerritory: "USA", ScheduledPrices: []ScheduledPrice{{PricePointID: "point-1", StartDate: &start}}}
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Pricing == nil || loaded.Pricing.ScheduledPrices[0].PricePointID != "point-1" {
		t.Fatalf("pricing = %#v", loaded.Pricing)
	}
	invalid := "09/01/2026"
	loaded.Pricing.ScheduledPrices[0].StartDate = &invalid
	if err := loaded.Validate(); err == nil || !strings.Contains(err.Error(), "YYYY-MM-DD") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadVersionOneConfiguration(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ascdir.yaml")
	contents := `version: "1"
app:
  id: "123"
  bundle_id: com.example.app
  platform: IOS
  version: 1.0.0
localizations:
  en-US:
    name: metadata/en-US/name.txt
    subtitle: metadata/en-US/subtitle.txt
    description: metadata/en-US/description.md
    keywords: metadata/en-US/keywords.txt
    promotional_text: metadata/en-US/promotional_text.txt
    whats_new: metadata/en-US/whats_new.md
    support_url: metadata/en-US/support_url.txt
    marketing_url: metadata/en-US/marketing_url.txt
    privacy_policy_url: metadata/en-US/privacy_policy_url.txt
    privacy_choices_url: metadata/en-US/privacy_choices_url.txt
    privacy_policy_text: metadata/en-US/privacy_policy.md
`
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Version != "1" {
		t.Fatalf("version = %q", cfg.Version)
	}
	paths := cfg.Localizations["en-US"].Paths()
	if paths["name"] != "metadata/en-US/name.txt" || paths["description"] != "metadata/en-US/description.md" {
		t.Fatalf("legacy paths = %#v", paths)
	}
	if len(cfg.Localizations["en-US"].Values.Map()) != 0 {
		t.Fatalf("legacy values unexpectedly became inline: %#v", cfg.Localizations["en-US"].Values.Map())
	}
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(saved), "values:") || !strings.Contains(string(saved), "name: metadata/en-US/name.txt") {
		t.Fatalf("version 1 was not saved in its original schema:\n%s", saved)
	}
	roundTripped, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := roundTripped.Localizations["en-US"].Paths()["name"]; got != "metadata/en-US/name.txt" {
		t.Fatalf("round-tripped name path = %q", got)
	}
}

func TestLoadVersionTwoWithoutMetadataLeavesFieldsUnmanaged(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ascdir.yaml")
	contents := `version: "2"
app:
  bundle_id: com.example.app
  platform: IOS
  version: 1.0.0
localizations:
  en-US:
    values:
      name: Example
`
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Metadata.Map()) != 0 {
		t.Fatalf("metadata unexpectedly became managed: %#v", cfg.Metadata.Map())
	}
	if len(cfg.Categories.Map()) != 0 {
		t.Fatalf("categories unexpectedly became managed: %#v", cfg.Categories.Map())
	}
	if len(cfg.AgeRating.Map()) != 0 {
		t.Fatalf("age rating unexpectedly became managed: %#v", cfg.AgeRating.Map())
	}
}

func TestLoadExistingVersionTwoPrivacyPolicyText(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ascdir.yaml")
	contents := `version: "2"
app:
  bundle_id: com.example.app
  platform: IOS
  version: 1.0.0
localizations:
  en-US:
    files:
      privacy_policy_text: metadata/en-US/privacy_policy.md
`
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Localizations["en-US"].Files.PrivacyPolicyText; got != "metadata/en-US/privacy_policy.md" {
		t.Fatalf("privacy policy text path = %q", got)
	}
}

func TestCompleteExampleLoadsAndValidates(t *testing.T) {
	t.Parallel()
	path := filepath.Join("..", "..", "examples", "ascdir.full.yaml")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("complete example is invalid: %v", err)
	}
}

func TestAgeRatingBooleansRemainTypedWhenUpdated(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ascdir.yaml")
	contents := `version: "2"
app:
  bundle_id: com.example.app
  platform: IOS
  version: 1.0.0
age_rating:
  advertising: false # keep me
  violence_cartoon_or_fantasy: NONE
localizations:
  en-US:
    values:
      name: Example
`
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.AgeRating.SetManaged("advertising", "true")
	updated, err := EncodeUpdatedValues(path, cfg)
	if err != nil {
		t.Fatal(err)
	}
	text := string(updated)
	if !strings.Contains(text, "advertising: true # keep me") || strings.Contains(text, `advertising: "true"`) {
		t.Fatalf("updated boolean was not preserved as a YAML boolean:\n%s", text)
	}
}

func TestAgeRatingFieldRegistryIsComplete(t *testing.T) {
	t.Parallel()
	remote := map[string]string{}
	for _, field := range ageRatingFieldNames() {
		remote[field] = "true"
	}
	var values AgeRatingValues
	values.ManageAll(remote)
	if got, want := len(values.Map()), len(ageRatingFieldNames()); got != want || want != 28 {
		t.Fatalf("managed age rating fields = %d, registry = %d; want 28", got, want)
	}
}

func TestAccessibilityValuesRoundTrip(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ascdir.yaml")
	cfg := New("123", "com.example.app", "IOS", "1.0.0", []string{"en-US"})
	value := true
	cfg.Accessibility = map[string]AccessibilityValues{"IPHONE": {SupportsVoiceover: &value, Published: &value}}
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Accessibility["IPHONE"].Map()["supports_voiceover"] != "true" {
		t.Fatalf("accessibility = %#v", loaded.Accessibility)
	}
	declaration := loaded.Accessibility["IPHONE"]
	declaration.SetManaged("published", "false")
	loaded.Accessibility["IPHONE"] = declaration
	updated, err := EncodeUpdatedValues(path, loaded)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(updated), `published: "false"`) || !strings.Contains(string(updated), "published: false") {
		t.Fatalf("updated YAML =\n%s", updated)
	}
}

func TestEncodeUpdatedValuesPreservesCommentsAndKeyOrder(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ascdir.yaml")
	contents := `# project comment
version: "2"
app:
  bundle_id: com.example.app
  platform: IOS
  version: 1.0.0
metadata:
  copyright: 2025 Example, Inc. # custom copyright comment
localizations:
  en-US: # locale comment
    values:
      name: Before # custom name comment
    files:
      description: metadata/en-US/description.md
`
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	localization := cfg.Localizations["en-US"]
	localization.Values.SetManaged("name", "After")
	cfg.Localizations["en-US"] = localization
	cfg.Metadata.SetManaged("copyright", "2026 Example, Inc.")
	updated, err := EncodeUpdatedValues(path, cfg)
	if err != nil {
		t.Fatal(err)
	}
	text := string(updated)
	for _, expected := range []string{"# project comment", "# locale comment", "copyright: 2026 Example, Inc. # custom copyright comment", "name: After # custom name comment"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("updated config is missing %q:\n%s", expected, text)
		}
	}
	if strings.Contains(text, "subtitle:") {
		t.Fatalf("update added an unmanaged field:\n%s", text)
	}
	if strings.Index(text, "values:") > strings.Index(text, "files:") {
		t.Fatalf("update changed key order:\n%s", text)
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ascdir.yaml")
	contents := "version: \"1\"\napp:\n  bundle_id: com.example.app\n  platform: IOS\n  version: 1.0.0\n  typo: value\nlocalizations:\n  en-US:\n    description: metadata/en-US/description.md\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "field typo not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateRejectsEscapingAndDuplicatePaths(t *testing.T) {
	t.Parallel()
	cfg := New("123", "com.example.app", "IOS", "1.2.0", []string{"en-US"})
	localization := cfg.Localizations["en-US"]
	localization.Files.Description = "../description.md"
	cfg.Localizations["en-US"] = localization
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "relative path") {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg = New("123", "com.example.app", "IOS", "1.2.0", []string{"en-US"})
	localization = cfg.Localizations["en-US"]
	localization.Files.WhatsNew = localization.Files.Description
	cfg.Localizations["en-US"] = localization
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "reuses path") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLicenseAgreementValuesMapsDeterministically(t *testing.T) {
	t.Parallel()
	values := LicenseAgreementValues{File: "metadata/license.md", Territories: []string{"USA", "JPN"}}
	if got := values.Paths()["license_agreement_text"]; got != "metadata/license.md" {
		t.Fatalf("license agreement path = %q", got)
	}
	if got := values.Map()["license_agreement_territories"]; got != "JPN,USA" {
		t.Fatalf("license agreement territories = %q", got)
	}
}

func TestAvailabilityValuesMapAndSetManaged(t *testing.T) {
	t.Parallel()
	availableInNewTerritories, available, preOrder := true, false, true
	releaseDate := "2026-09-01"
	values := AvailabilityValues{
		AvailableInNewTerritories: &availableInNewTerritories,
		Territories: map[string]TerritoryAvailability{"USA": {
			Available: &available, ReleaseDate: &releaseDate, PreOrderEnabled: &preOrder,
		}},
	}
	mapped := values.Map()
	for field, expected := range map[string]string{
		"availability.available_in_new_territories":      "true",
		"availability.territories.USA.available":         "false",
		"availability.territories.USA.release_date":      "2026-09-01",
		"availability.territories.USA.pre_order_enabled": "true",
	} {
		if mapped[field] != expected {
			t.Fatalf("%s = %q", field, mapped[field])
		}
	}
	var managed AvailabilityValues
	for field, value := range mapped {
		managed.SetManaged(field, value)
	}
	if got := managed.Map(); len(got) != len(mapped) {
		t.Fatalf("managed availability = %#v", got)
	}
}
