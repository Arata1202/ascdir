package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Arata1202/ascdir/internal/atomicfile"
	"gopkg.in/yaml.v3"
)

const CurrentVersion = "2"

type Config struct {
	Version          string                         `yaml:"version"`
	App              App                            `yaml:"app"`
	Metadata         MetadataValues                 `yaml:"metadata,omitempty"`
	Categories       CategoryValues                 `yaml:"categories,omitempty"`
	AgeRating        AgeRatingValues                `yaml:"age_rating,omitempty"`
	Accessibility    map[string]AccessibilityValues `yaml:"accessibility,omitempty"`
	LicenseAgreement *LicenseAgreementValues        `yaml:"license_agreement,omitempty"`
	Assets           AssetPaths                     `yaml:"assets,omitempty"`
	Availability     *AvailabilityValues            `yaml:"availability,omitempty"`
	Pricing          *PricingValues                 `yaml:"pricing,omitempty"`
	Localizations    map[string]Localization        `yaml:"localizations"`
}

// LicenseAgreementValues manages an optional custom EULA. Omitting the
// section leaves the remote agreement unmanaged.
type LicenseAgreementValues struct {
	File        string   `yaml:"file"`
	Territories []string `yaml:"territories"`
}

func (v LicenseAgreementValues) Paths() map[string]string {
	return map[string]string{"license_agreement_text": v.File}
}

func (v LicenseAgreementValues) Map() map[string]string {
	territories := append([]string(nil), v.Territories...)
	sort.Strings(territories)
	return map[string]string{"license_agreement_territories": strings.Join(territories, ",")}
}

type AssetPaths struct {
	Screenshots       string            `yaml:"screenshots,omitempty"`
	AppPreviews       string            `yaml:"app_previews,omitempty"`
	PreviewFrameTimes map[string]string `yaml:"preview_frame_times,omitempty"`
}

type AvailabilityValues struct {
	AvailableInNewTerritories *bool                            `yaml:"available_in_new_territories,omitempty"`
	Territories               map[string]TerritoryAvailability `yaml:"territories"`
}

type TerritoryAvailability struct {
	Available       *bool   `yaml:"available,omitempty"`
	ReleaseDate     *string `yaml:"release_date,omitempty"`
	PreOrderEnabled *bool   `yaml:"pre_order_enabled,omitempty"`
}

func (v AvailabilityValues) Map() map[string]string {
	values := map[string]string{}
	if v.AvailableInNewTerritories != nil {
		values["availability.available_in_new_territories"] = strconv.FormatBool(*v.AvailableInNewTerritories)
	}
	for territory, availability := range v.Territories {
		prefix := "availability.territories." + territory + "."
		if availability.Available != nil {
			values[prefix+"available"] = strconv.FormatBool(*availability.Available)
		}
		if availability.ReleaseDate != nil {
			values[prefix+"release_date"] = *availability.ReleaseDate
		}
		if availability.PreOrderEnabled != nil {
			values[prefix+"pre_order_enabled"] = strconv.FormatBool(*availability.PreOrderEnabled)
		}
	}
	return values
}

func (v *AvailabilityValues) SetManaged(field, value string) {
	if field == "availability.available_in_new_territories" {
		parsed, _ := strconv.ParseBool(value)
		v.AvailableInNewTerritories = &parsed
		return
	}
	parts := strings.Split(field, ".")
	if len(parts) != 4 || parts[0] != "availability" || parts[1] != "territories" {
		return
	}
	if v.Territories == nil {
		v.Territories = map[string]TerritoryAvailability{}
	}
	territory := v.Territories[parts[2]]
	switch parts[3] {
	case "available":
		parsed, _ := strconv.ParseBool(value)
		territory.Available = &parsed
	case "release_date":
		territory.ReleaseDate = stringPointer(value)
	case "pre_order_enabled":
		parsed, _ := strconv.ParseBool(value)
		territory.PreOrderEnabled = &parsed
	}
	v.Territories[parts[2]] = territory
}

type PricingValues struct {
	BaseTerritory   string           `yaml:"base_territory" json:"base_territory"`
	ScheduledPrices []ScheduledPrice `yaml:"scheduled_prices" json:"scheduled_prices"`
}

type ScheduledPrice struct {
	PricePointID string  `yaml:"price_point_id" json:"price_point_id"`
	StartDate    *string `yaml:"start_date,omitempty" json:"start_date,omitempty"`
	EndDate      *string `yaml:"end_date,omitempty" json:"end_date,omitempty"`
}

func (v PricingValues) Map() map[string]string {
	canonical := v
	canonical.ScheduledPrices = append([]ScheduledPrice(nil), v.ScheduledPrices...)
	sort.SliceStable(canonical.ScheduledPrices, func(i, j int) bool {
		return scheduledPriceConfigKey(canonical.ScheduledPrices[i]) < scheduledPriceConfigKey(canonical.ScheduledPrices[j])
	})
	data, _ := json.Marshal(canonical)
	return map[string]string{"pricing.schedule": string(data)}
}

func scheduledPriceConfigKey(price ScheduledPrice) string {
	start, end := "", ""
	if price.StartDate != nil {
		start = *price.StartDate
	}
	if price.EndDate != nil {
		end = *price.EndDate
	}
	return start + "\x00" + end + "\x00" + price.PricePointID
}

func (v *PricingValues) SetManaged(value string) error {
	return json.Unmarshal([]byte(value), v)
}

// AgeRatingValues mirrors Apple's age rating declaration. Pointer values make
// every answer opt-in, so adding ascdir to an existing project never changes
// its declaration unless the corresponding key is present.
type AgeRatingValues struct {
	Advertising                                 *bool   `yaml:"advertising,omitempty"`
	AlcoholTobaccoOrDrugUseOrReferences         *string `yaml:"alcohol_tobacco_or_drug_use_or_references,omitempty"`
	Contests                                    *string `yaml:"contests,omitempty"`
	Gambling                                    *bool   `yaml:"gambling,omitempty"`
	GamblingSimulated                           *string `yaml:"gambling_simulated,omitempty"`
	GunsOrOtherWeapons                          *string `yaml:"guns_or_other_weapons,omitempty"`
	HealthOrWellnessTopics                      *bool   `yaml:"health_or_wellness_topics,omitempty"`
	KidsAgeBand                                 *string `yaml:"kids_age_band,omitempty"`
	LootBox                                     *bool   `yaml:"loot_box,omitempty"`
	MedicalOrTreatmentInformation               *string `yaml:"medical_or_treatment_information,omitempty"`
	MessagingAndChat                            *bool   `yaml:"messaging_and_chat,omitempty"`
	ParentalControls                            *bool   `yaml:"parental_controls,omitempty"`
	ProfanityOrCrudeHumor                       *string `yaml:"profanity_or_crude_humor,omitempty"`
	AgeAssurance                                *bool   `yaml:"age_assurance,omitempty"`
	SexualContentGraphicAndNudity               *string `yaml:"sexual_content_graphic_and_nudity,omitempty"`
	SexualContentOrNudity                       *string `yaml:"sexual_content_or_nudity,omitempty"`
	SocialMedia                                 *bool   `yaml:"social_media,omitempty"`
	SocialMediaAgeRestricted                    *bool   `yaml:"social_media_age_restricted,omitempty"`
	HorrorOrFearThemes                          *string `yaml:"horror_or_fear_themes,omitempty"`
	MatureOrSuggestiveThemes                    *string `yaml:"mature_or_suggestive_themes,omitempty"`
	UnrestrictedWebAccess                       *bool   `yaml:"unrestricted_web_access,omitempty"`
	UserGeneratedContent                        *bool   `yaml:"user_generated_content,omitempty"`
	ViolenceCartoonOrFantasy                    *string `yaml:"violence_cartoon_or_fantasy,omitempty"`
	ViolenceRealisticProlongedGraphicOrSadistic *string `yaml:"violence_realistic_prolonged_graphic_or_sadistic,omitempty"`
	ViolenceRealistic                           *string `yaml:"violence_realistic,omitempty"`
	AgeRatingOverrideV2                         *string `yaml:"age_rating_override,omitempty"`
	KoreaAgeRatingOverride                      *string `yaml:"korea_age_rating_override,omitempty"`
	DeveloperAgeRatingInfoURL                   *string `yaml:"developer_age_rating_info_url,omitempty"`
}

type App struct {
	ID       string `yaml:"id,omitempty"`
	BundleID string `yaml:"bundle_id"`
	Platform string `yaml:"platform"`
	Version  string `yaml:"version"`
}

// MetadataValues contains non-localized, single-value metadata. Pointers
// distinguish an unmanaged field from a managed field whose desired value is
// empty.
type MetadataValues struct {
	Copyright                *string `yaml:"copyright,omitempty"`
	AccessibilityURL         *string `yaml:"accessibility_url,omitempty"`
	ContentRightsDeclaration *string `yaml:"content_rights_declaration,omitempty"`
}

// CategoryValues contains App Store category relationship IDs. Pointers
// distinguish an unmanaged relationship from one intentionally cleared.
type CategoryValues struct {
	PrimaryCategory         *string `yaml:"primary_category,omitempty"`
	PrimarySubcategoryOne   *string `yaml:"primary_subcategory_one,omitempty"`
	PrimarySubcategoryTwo   *string `yaml:"primary_subcategory_two,omitempty"`
	SecondaryCategory       *string `yaml:"secondary_category,omitempty"`
	SecondarySubcategoryOne *string `yaml:"secondary_subcategory_one,omitempty"`
	SecondarySubcategoryTwo *string `yaml:"secondary_subcategory_two,omitempty"`
}

// Localization keeps short scalar values in YAML and long-form content in
// dedicated files. Pointer values distinguish an unmanaged field from a
// managed field whose desired value is empty.
type Localization struct {
	Values      LocaleValues `yaml:"values,omitempty"`
	Files       LocaleFiles  `yaml:"files,omitempty"`
	legacyPaths map[string]string
}

type LocaleValues struct {
	Name              *string `yaml:"name,omitempty"`
	Subtitle          *string `yaml:"subtitle,omitempty"`
	Keywords          *string `yaml:"keywords,omitempty"`
	SupportURL        *string `yaml:"support_url,omitempty"`
	MarketingURL      *string `yaml:"marketing_url,omitempty"`
	PrivacyPolicyURL  *string `yaml:"privacy_policy_url,omitempty"`
	PrivacyChoicesURL *string `yaml:"privacy_choices_url,omitempty"`
}

type LocaleFiles struct {
	Description       string `yaml:"description,omitempty"`
	PromotionalText   string `yaml:"promotional_text,omitempty"`
	WhatsNew          string `yaml:"whats_new,omitempty"`
	PrivacyPolicyText string `yaml:"privacy_policy_text,omitempty"`
}

// localeFilesV1 is the version 1 file-per-field representation. It remains
// loadable so existing projects do not need a flag-day migration.
type localeFilesV1 struct {
	Name              string `yaml:"name"`
	Subtitle          string `yaml:"subtitle"`
	Description       string `yaml:"description"`
	Keywords          string `yaml:"keywords"`
	PromotionalText   string `yaml:"promotional_text"`
	WhatsNew          string `yaml:"whats_new"`
	SupportURL        string `yaml:"support_url"`
	MarketingURL      string `yaml:"marketing_url"`
	PrivacyPolicyURL  string `yaml:"privacy_policy_url"`
	PrivacyChoicesURL string `yaml:"privacy_choices_url"`
	PrivacyPolicyText string `yaml:"privacy_policy_text"`
}

type configV1 struct {
	Version       string                   `yaml:"version"`
	App           App                      `yaml:"app"`
	Localizations map[string]localeFilesV1 `yaml:"localizations"`
}

func New(appID, bundleID, platform, version string, locales []string) Config {
	cfg := Config{
		Version: CurrentVersion,
		App:     App{ID: appID, BundleID: bundleID, Platform: platform, Version: version},
		Metadata: MetadataValues{
			Copyright:                stringPointer(""),
			AccessibilityURL:         stringPointer(""),
			ContentRightsDeclaration: stringPointer(""),
		},
		Categories: CategoryValues{
			PrimaryCategory:         stringPointer(""),
			PrimarySubcategoryOne:   stringPointer(""),
			PrimarySubcategoryTwo:   stringPointer(""),
			SecondaryCategory:       stringPointer(""),
			SecondarySubcategoryOne: stringPointer(""),
			SecondarySubcategoryTwo: stringPointer(""),
		},
		Localizations: make(map[string]Localization, len(locales)),
	}
	for _, locale := range locales {
		base := filepath.ToSlash(filepath.Join("metadata", locale))
		privacyPolicyText := ""
		if platform == "TV_OS" {
			privacyPolicyText = base + "/privacy_policy.md"
		}
		cfg.Localizations[locale] = Localization{
			Values: LocaleValues{
				Name:              stringPointer(""),
				Subtitle:          stringPointer(""),
				Keywords:          stringPointer(""),
				SupportURL:        stringPointer(""),
				MarketingURL:      stringPointer(""),
				PrivacyPolicyURL:  stringPointer(""),
				PrivacyChoicesURL: stringPointer(""),
			},
			Files: LocaleFiles{
				Description:       base + "/description.md",
				PromotionalText:   base + "/promotional_text.md",
				WhatsNew:          base + "/whats_new.md",
				PrivacyPolicyText: privacyPolicyText,
			},
		}
	}
	return cfg
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var header struct {
		Version string `yaml:"version"`
	}
	if err := yaml.Unmarshal(data, &header); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	var cfg Config
	switch header.Version {
	case "1":
		var legacy configV1
		if err := decodeKnown(data, &legacy); err != nil {
			return Config{}, err
		}
		cfg = convertV1(legacy)
	case CurrentVersion:
		if err := decodeKnown(data, &cfg); err != nil {
			return Config{}, err
		}
	default:
		return Config{}, fmt.Errorf("unsupported config version %q", header.Version)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func decodeKnown(data []byte, destination any) error {
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("parse config: multiple YAML documents are not allowed")
		}
		return fmt.Errorf("parse config: %w", err)
	}
	return nil
}

func convertV1(legacy configV1) Config {
	cfg := Config{Version: "1", App: legacy.App, Localizations: make(map[string]Localization, len(legacy.Localizations))}
	for locale, files := range legacy.Localizations {
		cfg.Localizations[locale] = Localization{Files: LocaleFiles{
			Description:       files.Description,
			PromotionalText:   files.PromotionalText,
			WhatsNew:          files.WhatsNew,
			PrivacyPolicyText: files.PrivacyPolicyText,
		}, Values: LocaleValues{}}
		localization := cfg.Localizations[locale]
		localization.legacyPaths = map[string]string{
			"name": files.Name, "subtitle": files.Subtitle, "keywords": files.Keywords,
			"support_url": files.SupportURL, "marketing_url": files.MarketingURL,
			"privacy_policy_url": files.PrivacyPolicyURL, "privacy_choices_url": files.PrivacyChoicesURL,
		}
		cfg.Localizations[locale] = localization
	}
	return cfg
}

func Save(path string, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	data, err := Encode(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Clean(path)), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if err := atomicfile.Write(path, data, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func Encode(cfg Config) ([]byte, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if cfg.Version == "1" {
		return encodeV1(cfg)
	}
	var document yaml.Node
	if err := document.Encode(cfg); err != nil {
		return nil, fmt.Errorf("encode config: %w", err)
	}
	if cfg.Version == CurrentVersion {
		addSchemaComments(&document)
	}
	data, err := yaml.Marshal(&document)
	if err != nil {
		return nil, fmt.Errorf("encode config: %w", err)
	}
	return data, nil
}

func encodeV1(cfg Config) ([]byte, error) {
	legacy := configV1{Version: "1", App: cfg.App, Localizations: make(map[string]localeFilesV1, len(cfg.Localizations))}
	for locale, localization := range cfg.Localizations {
		paths := localization.Paths()
		legacy.Localizations[locale] = localeFilesV1{
			Name:              paths["name"],
			Subtitle:          paths["subtitle"],
			Description:       paths["description"],
			Keywords:          paths["keywords"],
			PromotionalText:   paths["promotional_text"],
			WhatsNew:          paths["whats_new"],
			SupportURL:        paths["support_url"],
			MarketingURL:      paths["marketing_url"],
			PrivacyPolicyURL:  paths["privacy_policy_url"],
			PrivacyChoicesURL: paths["privacy_choices_url"],
			PrivacyPolicyText: paths["privacy_policy_text"],
		}
	}
	data, err := yaml.Marshal(legacy)
	if err != nil {
		return nil, fmt.Errorf("encode config: %w", err)
	}
	return data, nil
}

// EncodeUpdatedValues updates only managed version 2 scalar nodes in an
// existing configuration. Parsing and re-encoding the YAML node tree keeps
// user comments and key order intact. A missing file is treated as a newly
// initialized project and receives the standard generated configuration.
func EncodeUpdatedValues(path string, cfg Config) ([]byte, error) {
	if cfg.Version != CurrentVersion {
		return Encode(cfg)
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Encode(cfg)
	}
	if err != nil {
		return nil, fmt.Errorf("read config for update: %w", err)
	}
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("parse config for update: %w", err)
	}
	root := &document
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		root = root.Content[0]
	}
	localizations := mappingValue(root, "localizations")
	if localizations == nil {
		return nil, errors.New("update config: localizations mapping is missing")
	}
	if err := updateScalarMapping(mappingValue(root, "metadata"), "metadata", cfg.Metadata.Pointers()); err != nil {
		return nil, err
	}
	if err := updateScalarMapping(mappingValue(root, "categories"), "categories", cfg.Categories.Pointers()); err != nil {
		return nil, err
	}
	if err := updateScalarMapping(mappingValue(root, "age_rating"), "age_rating", cfg.AgeRating.Pointers()); err != nil {
		return nil, err
	}
	accessibility := mappingValue(root, "accessibility")
	for _, deviceFamily := range SortedAccessibilityDeviceFamilies(cfg.Accessibility) {
		if err := updateScalarMapping(mappingValue(accessibility, deviceFamily), "accessibility."+deviceFamily, cfg.Accessibility[deviceFamily].Pointers()); err != nil {
			return nil, err
		}
	}
	if cfg.LicenseAgreement != nil {
		if err := replaceMappingValue(root, "license_agreement", cfg.LicenseAgreement); err != nil {
			return nil, err
		}
	}
	if cfg.Assets.AppPreviews != "" {
		assets := mappingValue(root, "assets")
		if assets == nil {
			return nil, errors.New("update config: assets mapping is missing")
		}
		if err := replaceMappingValue(assets, "preview_frame_times", cfg.Assets.PreviewFrameTimes); err != nil {
			return nil, err
		}
	}
	if cfg.Availability != nil {
		if err := replaceMappingValue(root, "availability", cfg.Availability); err != nil {
			return nil, err
		}
	}
	if cfg.Pricing != nil {
		if err := replaceMappingValue(root, "pricing", cfg.Pricing); err != nil {
			return nil, err
		}
	}
	for _, locale := range SortedLocales(cfg.Localizations) {
		localeNode := mappingValue(localizations, locale)
		if localeNode == nil {
			return nil, fmt.Errorf("update config: localization %q is missing", locale)
		}
		if err := updateScalarMapping(mappingValue(localeNode, "values"), locale+".values", cfg.Localizations[locale].Values.Pointers()); err != nil {
			return nil, err
		}
	}
	updated, err := yaml.Marshal(&document)
	if err != nil {
		return nil, fmt.Errorf("encode updated config: %w", err)
	}
	return updated, nil
}

func replaceMappingValue(mapping *yaml.Node, key string, value any) error {
	var encoded yaml.Node
	if err := encoded.Encode(value); err != nil {
		return fmt.Errorf("encode config value %s: %w", key, err)
	}
	node := &encoded
	if encoded.Kind == yaml.DocumentNode && len(encoded.Content) > 0 {
		node = encoded.Content[0]
	}
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			node.HeadComment = mapping.Content[index+1].HeadComment
			node.LineComment = mapping.Content[index+1].LineComment
			mapping.Content[index+1] = node
			return nil
		}
	}
	mapping.Content = append(mapping.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, node)
	return nil
}

func updateScalarMapping(mapping *yaml.Node, label string, values map[string]*string) error {
	for field, pointer := range values {
		if pointer == nil {
			continue
		}
		if mapping == nil {
			return fmt.Errorf("update config: %s mapping is missing", label)
		}
		valueNode := mappingValue(mapping, field)
		if valueNode == nil {
			return fmt.Errorf("update config: managed field %s.%s is missing", label, field)
		}
		if valueNode.Kind != yaml.ScalarNode {
			return fmt.Errorf("update config: managed field %s.%s is not a scalar", label, field)
		}
		valueNode.Value = *pointer
		if valueNode.Tag != "!!bool" {
			valueNode.Tag = "!!str"
		}
	}
	return nil
}

func addSchemaComments(document *yaml.Node) {
	root := document
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		root = root.Content[0]
	}
	addMappingComments(root, map[string]string{
		"version":       "ascdir configuration schema version",
		"localizations": "Per-locale storefront text",
		"age_rating":    "Managed age-rating declaration",
		"accessibility": "Optional Accessibility Nutrition Labels",
		"availability":  "Optional territory availability and preorders",
		"pricing":       "Optional append-only price schedule",
	})
	addMappingComments(mappingValue(root, "app"), map[string]string{
		"id":        "App Store Connect app ID; populated by init",
		"bundle_id": "App bundle identifier",
		"platform":  "IOS, MAC_OS, TV_OS, or VISION_OS",
		"version":   "App Store version to manage",
	})
	addMappingComments(mappingValue(root, "metadata"), map[string]string{
		"copyright":                  "Year and rights holder, for example: 2026 Example, Inc.",
		"accessibility_url":          "Optional public HTTP(S) accessibility information page",
		"content_rights_declaration": "DOES_NOT_USE_THIRD_PARTY_CONTENT or USES_THIRD_PARTY_CONTENT",
	})
	addMappingComments(mappingValue(root, "categories"), map[string]string{
		"primary_category":          "Required top-level App Store category ID",
		"primary_subcategory_one":   "Optional first Games or Stickers subcategory ID",
		"primary_subcategory_two":   "Optional second Games or Stickers subcategory ID",
		"secondary_category":        "Optional secondary top-level App Store category ID",
		"secondary_subcategory_one": "Optional first secondary Games or Stickers subcategory ID",
		"secondary_subcategory_two": "Optional second secondary Games or Stickers subcategory ID",
	})
	addMappingComments(mappingValue(root, "license_agreement"), map[string]string{
		"file":        "Optional plain-text custom EULA; omit this section to leave it unmanaged",
		"territories": "App Store territory IDs where the custom EULA applies",
	})
	addMappingComments(mappingValue(root, "assets"), map[string]string{
		"screenshots":         "Managed screenshot root, structured by locale and display type",
		"app_previews":        "Managed App Preview root, structured by locale and preview type",
		"preview_frame_times": "Optional poster-frame timecodes keyed by preview-relative path",
	})
	availability := mappingValue(root, "availability")
	addMappingComments(availability, map[string]string{
		"available_in_new_territories": "Initial setting for territories Apple adds later",
		"territories":                  "Managed App Store territories; omitted territories remain unmanaged",
	})
	pricing := mappingValue(root, "pricing")
	addMappingComments(pricing, map[string]string{
		"base_territory":   "Three-letter App Store base territory ID",
		"scheduled_prices": "Append-only price schedule using App Store price-point IDs",
	})
}

func addMappingComments(mapping *yaml.Node, comments map[string]string) {
	if mapping == nil {
		return
	}
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		key, value := mapping.Content[index], mapping.Content[index+1]
		comment := comments[key.Value]
		if value.Kind == yaml.ScalarNode {
			value.LineComment = comment
		} else {
			key.HeadComment = comment
		}
	}
}

func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			return mapping.Content[index+1]
		}
	}
	return nil
}

func (c Config) Validate() error {
	if c.Version != "1" && c.Version != CurrentVersion {
		return fmt.Errorf("unsupported config version %q", c.Version)
	}
	if strings.TrimSpace(c.App.BundleID) == "" || strings.TrimSpace(c.App.Version) == "" {
		return errors.New("app.bundle_id and app.version are required")
	}
	switch c.App.Platform {
	case "IOS", "MAC_OS", "TV_OS", "VISION_OS":
	default:
		return fmt.Errorf("unsupported app.platform %q", c.App.Platform)
	}
	if len(c.Localizations) == 0 {
		return errors.New("at least one localization is required")
	}
	validDeviceFamilies := map[string]bool{"IPHONE": true, "IPAD": true, "APPLE_TV": true, "APPLE_WATCH": true, "MAC": true, "VISION": true}
	for deviceFamily, declaration := range c.Accessibility {
		if !validDeviceFamilies[deviceFamily] {
			return fmt.Errorf("unsupported accessibility device family %q", deviceFamily)
		}
		if len(declaration.Map()) == 0 {
			return fmt.Errorf("accessibility.%s must manage at least one field", deviceFamily)
		}
	}
	if c.Availability != nil {
		if len(c.Availability.Territories) == 0 {
			return errors.New("availability.territories must contain at least one managed territory")
		}
		for territory, availability := range c.Availability.Territories {
			if len(territory) != 3 || territory != strings.ToUpper(territory) {
				return fmt.Errorf("invalid availability territory %q; use an App Store territory ID", territory)
			}
			if availability.Available == nil && availability.ReleaseDate == nil && availability.PreOrderEnabled == nil {
				return fmt.Errorf("availability.territories.%s must manage at least one field", territory)
			}
		}
	}
	if c.Pricing != nil {
		if len(c.Pricing.BaseTerritory) != 3 || c.Pricing.BaseTerritory != strings.ToUpper(c.Pricing.BaseTerritory) {
			return fmt.Errorf("invalid pricing.base_territory %q; use an App Store territory ID", c.Pricing.BaseTerritory)
		}
		if len(c.Pricing.ScheduledPrices) == 0 {
			return errors.New("pricing.scheduled_prices must contain at least one price")
		}
		seenPrices := map[string]bool{}
		for index, price := range c.Pricing.ScheduledPrices {
			if strings.TrimSpace(price.PricePointID) == "" {
				return fmt.Errorf("pricing.scheduled_prices[%d].price_point_id is required", index)
			}
			key := scheduledPriceConfigKey(price)
			if seenPrices[key] {
				return fmt.Errorf("pricing.scheduled_prices[%d] duplicates another scheduled price", index)
			}
			seenPrices[key] = true
			var start, end time.Time
			var startSet, endSet bool
			if price.StartDate != nil {
				parsed, err := time.Parse("2006-01-02", *price.StartDate)
				if err != nil {
					return fmt.Errorf("pricing.scheduled_prices[%d].start_date must use YYYY-MM-DD", index)
				}
				start, startSet = parsed, true
			}
			if price.EndDate != nil {
				parsed, err := time.Parse("2006-01-02", *price.EndDate)
				if err != nil {
					return fmt.Errorf("pricing.scheduled_prices[%d].end_date must use YYYY-MM-DD", index)
				}
				end, endSet = parsed, true
			}
			if startSet && endSet && end.Before(start) {
				return fmt.Errorf("pricing.scheduled_prices[%d].end_date must not precede start_date", index)
			}
		}
	}
	seen := map[string]string{}
	if c.LicenseAgreement != nil {
		path := strings.TrimSpace(c.LicenseAgreement.File)
		if path == "" {
			return errors.New("license_agreement.file is required")
		}
		clean := filepath.Clean(path)
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return errors.New("license_agreement.file must be a relative path inside the project")
		}
		seen[clean] = "license_agreement.file"
		territories := map[string]bool{}
		for _, territory := range c.LicenseAgreement.Territories {
			if len(territory) != 3 || territory != strings.ToUpper(territory) {
				return fmt.Errorf("invalid license agreement territory %q; use an App Store territory ID", territory)
			}
			if territories[territory] {
				return fmt.Errorf("duplicate license agreement territory %q", territory)
			}
			territories[territory] = true
		}
	}
	if c.Assets.Screenshots != "" {
		clean := filepath.Clean(c.Assets.Screenshots)
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return errors.New("assets.screenshots must be a relative directory inside the project")
		}
	}
	if c.Assets.AppPreviews != "" {
		clean := filepath.Clean(c.Assets.AppPreviews)
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return errors.New("assets.app_previews must be a relative directory inside the project")
		}
		for path := range c.Assets.PreviewFrameTimes {
			clean := filepath.Clean(path)
			if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
				return fmt.Errorf("assets.preview_frame_times key %q must be relative to assets.app_previews", path)
			}
		}
	} else if len(c.Assets.PreviewFrameTimes) > 0 {
		return errors.New("assets.preview_frame_times requires assets.app_previews")
	}
	for _, locale := range SortedLocales(c.Localizations) {
		localization := c.Localizations[locale]
		if strings.TrimSpace(locale) == "" {
			return errors.New("localization locale cannot be empty")
		}
		managedFields := len(localization.Values.Map())
		paths := localization.Paths()
		fields := make([]string, 0, len(paths))
		for field := range paths {
			fields = append(fields, field)
		}
		sort.Strings(fields)
		for _, field := range fields {
			path := paths[field]
			if path == "" {
				continue
			}
			managedFields++
			clean := filepath.Clean(path)
			if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
				return fmt.Errorf("%s.%s must be a relative path inside the project", locale, field)
			}
			if previous, ok := seen[clean]; ok {
				return fmt.Errorf("%s.%s reuses path %q from %s", locale, field, path, previous)
			}
			seen[clean] = locale + "." + field
		}
		if managedFields == 0 {
			return fmt.Errorf("%s must manage at least one field", locale)
		}
	}
	return nil
}

func (f LocaleFiles) Paths() map[string]string {
	return map[string]string{
		"description": f.Description, "promotional_text": f.PromotionalText,
		"whats_new": f.WhatsNew, "privacy_policy_text": f.PrivacyPolicyText,
	}
}

func (l Localization) Paths() map[string]string {
	paths := l.Files.Paths()
	for field, path := range l.legacyPaths {
		paths[field] = path
	}
	return paths
}

func (v LocaleValues) Map() map[string]string {
	result := map[string]string{}
	for field, value := range v.Pointers() {
		if value != nil {
			result[field] = *value
		}
	}
	return result
}

func (v LocaleValues) Pointers() map[string]*string {
	return map[string]*string{
		"name": v.Name, "subtitle": v.Subtitle, "keywords": v.Keywords,
		"support_url": v.SupportURL, "marketing_url": v.MarketingURL,
		"privacy_policy_url": v.PrivacyPolicyURL, "privacy_choices_url": v.PrivacyChoicesURL,
	}
}

func (v *LocaleValues) SetManaged(field, value string) {
	pointer := stringPointer(value)
	switch field {
	case "name":
		v.Name = pointer
	case "subtitle":
		v.Subtitle = pointer
	case "keywords":
		v.Keywords = pointer
	case "support_url":
		v.SupportURL = pointer
	case "marketing_url":
		v.MarketingURL = pointer
	case "privacy_policy_url":
		v.PrivacyPolicyURL = pointer
	case "privacy_choices_url":
		v.PrivacyChoicesURL = pointer
	}
}

func (v MetadataValues) Map() map[string]string {
	result := map[string]string{}
	for field, value := range v.Pointers() {
		if value != nil {
			result[field] = *value
		}
	}
	return result
}

func (v MetadataValues) Pointers() map[string]*string {
	return map[string]*string{
		"copyright":                  v.Copyright,
		"accessibility_url":          v.AccessibilityURL,
		"content_rights_declaration": v.ContentRightsDeclaration,
	}
}

func (v *MetadataValues) SetManaged(field, value string) {
	pointer := stringPointer(value)
	switch field {
	case "copyright":
		v.Copyright = pointer
	case "accessibility_url":
		v.AccessibilityURL = pointer
	case "content_rights_declaration":
		v.ContentRightsDeclaration = pointer
	}
}

func (v CategoryValues) Map() map[string]string {
	result := map[string]string{}
	for field, value := range v.Pointers() {
		if value != nil {
			result[field] = *value
		}
	}
	return result
}

func (v CategoryValues) Pointers() map[string]*string {
	return map[string]*string{
		"primary_category":          v.PrimaryCategory,
		"primary_subcategory_one":   v.PrimarySubcategoryOne,
		"primary_subcategory_two":   v.PrimarySubcategoryTwo,
		"secondary_category":        v.SecondaryCategory,
		"secondary_subcategory_one": v.SecondarySubcategoryOne,
		"secondary_subcategory_two": v.SecondarySubcategoryTwo,
	}
}

func (v *CategoryValues) SetManaged(field, value string) {
	pointer := stringPointer(value)
	switch field {
	case "primary_category":
		v.PrimaryCategory = pointer
	case "primary_subcategory_one":
		v.PrimarySubcategoryOne = pointer
	case "primary_subcategory_two":
		v.PrimarySubcategoryTwo = pointer
	case "secondary_category":
		v.SecondaryCategory = pointer
	case "secondary_subcategory_one":
		v.SecondarySubcategoryOne = pointer
	case "secondary_subcategory_two":
		v.SecondarySubcategoryTwo = pointer
	}
}

func (v AgeRatingValues) Map() map[string]string {
	result := map[string]string{}
	for field, value := range v.Pointers() {
		if value != nil {
			result[field] = *value
		}
	}
	return result
}

// ManageAll configures every age rating answer from values returned by Apple.
// It is intended for init; existing configurations remain opt-in per field.
func (v *AgeRatingValues) ManageAll(values map[string]string) {
	for _, field := range ageRatingFieldNames() {
		if value, present := values[field]; present {
			v.SetManaged(field, value)
		}
	}
}

func (v AgeRatingValues) Pointers() map[string]*string {
	stringValue := func(value *string) *string { return value }
	boolValue := func(value *bool) *string {
		if value == nil {
			return nil
		}
		text := strconv.FormatBool(*value)
		return &text
	}
	return map[string]*string{
		"advertising": boolValue(v.Advertising), "alcohol_tobacco_or_drug_use_or_references": stringValue(v.AlcoholTobaccoOrDrugUseOrReferences),
		"contests": stringValue(v.Contests), "gambling": boolValue(v.Gambling), "gambling_simulated": stringValue(v.GamblingSimulated),
		"guns_or_other_weapons": stringValue(v.GunsOrOtherWeapons), "health_or_wellness_topics": boolValue(v.HealthOrWellnessTopics),
		"kids_age_band": stringValue(v.KidsAgeBand), "loot_box": boolValue(v.LootBox), "medical_or_treatment_information": stringValue(v.MedicalOrTreatmentInformation),
		"messaging_and_chat": boolValue(v.MessagingAndChat), "parental_controls": boolValue(v.ParentalControls), "profanity_or_crude_humor": stringValue(v.ProfanityOrCrudeHumor),
		"age_assurance": boolValue(v.AgeAssurance), "sexual_content_graphic_and_nudity": stringValue(v.SexualContentGraphicAndNudity),
		"sexual_content_or_nudity": stringValue(v.SexualContentOrNudity), "social_media": boolValue(v.SocialMedia),
		"social_media_age_restricted": boolValue(v.SocialMediaAgeRestricted), "horror_or_fear_themes": stringValue(v.HorrorOrFearThemes),
		"mature_or_suggestive_themes": stringValue(v.MatureOrSuggestiveThemes), "unrestricted_web_access": boolValue(v.UnrestrictedWebAccess),
		"user_generated_content": boolValue(v.UserGeneratedContent), "violence_cartoon_or_fantasy": stringValue(v.ViolenceCartoonOrFantasy),
		"violence_realistic_prolonged_graphic_or_sadistic": stringValue(v.ViolenceRealisticProlongedGraphicOrSadistic),
		"violence_realistic": stringValue(v.ViolenceRealistic), "age_rating_override": stringValue(v.AgeRatingOverrideV2),
		"korea_age_rating_override": stringValue(v.KoreaAgeRatingOverride), "developer_age_rating_info_url": stringValue(v.DeveloperAgeRatingInfoURL),
	}
}

func (v *AgeRatingValues) SetManaged(field, value string) {
	sp := stringPointer(value)
	bp := func() *bool { parsed, _ := strconv.ParseBool(value); return &parsed }
	switch field {
	case "advertising":
		v.Advertising = bp()
	case "alcohol_tobacco_or_drug_use_or_references":
		v.AlcoholTobaccoOrDrugUseOrReferences = sp
	case "contests":
		v.Contests = sp
	case "gambling":
		v.Gambling = bp()
	case "gambling_simulated":
		v.GamblingSimulated = sp
	case "guns_or_other_weapons":
		v.GunsOrOtherWeapons = sp
	case "health_or_wellness_topics":
		v.HealthOrWellnessTopics = bp()
	case "kids_age_band":
		v.KidsAgeBand = sp
	case "loot_box":
		v.LootBox = bp()
	case "medical_or_treatment_information":
		v.MedicalOrTreatmentInformation = sp
	case "messaging_and_chat":
		v.MessagingAndChat = bp()
	case "parental_controls":
		v.ParentalControls = bp()
	case "profanity_or_crude_humor":
		v.ProfanityOrCrudeHumor = sp
	case "age_assurance":
		v.AgeAssurance = bp()
	case "sexual_content_graphic_and_nudity":
		v.SexualContentGraphicAndNudity = sp
	case "sexual_content_or_nudity":
		v.SexualContentOrNudity = sp
	case "social_media":
		v.SocialMedia = bp()
	case "social_media_age_restricted":
		v.SocialMediaAgeRestricted = bp()
	case "horror_or_fear_themes":
		v.HorrorOrFearThemes = sp
	case "mature_or_suggestive_themes":
		v.MatureOrSuggestiveThemes = sp
	case "unrestricted_web_access":
		v.UnrestrictedWebAccess = bp()
	case "user_generated_content":
		v.UserGeneratedContent = bp()
	case "violence_cartoon_or_fantasy":
		v.ViolenceCartoonOrFantasy = sp
	case "violence_realistic_prolonged_graphic_or_sadistic":
		v.ViolenceRealisticProlongedGraphicOrSadistic = sp
	case "violence_realistic":
		v.ViolenceRealistic = sp
	case "age_rating_override":
		v.AgeRatingOverrideV2 = sp
	case "korea_age_rating_override":
		v.KoreaAgeRatingOverride = sp
	case "developer_age_rating_info_url":
		v.DeveloperAgeRatingInfoURL = sp
	}
}

func ageRatingFieldNames() []string {
	return []string{
		"advertising", "alcohol_tobacco_or_drug_use_or_references", "contests", "gambling", "gambling_simulated",
		"guns_or_other_weapons", "health_or_wellness_topics", "kids_age_band", "loot_box", "medical_or_treatment_information",
		"messaging_and_chat", "parental_controls", "profanity_or_crude_humor", "age_assurance",
		"sexual_content_graphic_and_nudity", "sexual_content_or_nudity", "social_media", "social_media_age_restricted",
		"horror_or_fear_themes", "mature_or_suggestive_themes", "unrestricted_web_access", "user_generated_content",
		"violence_cartoon_or_fantasy", "violence_realistic_prolonged_graphic_or_sadistic", "violence_realistic",
		"age_rating_override", "korea_age_rating_override", "developer_age_rating_info_url",
	}
}

func stringPointer(value string) *string { return &value }

func SortedLocales(localizations map[string]Localization) []string {
	locales := make([]string, 0, len(localizations))
	for locale := range localizations {
		locales = append(locales, locale)
	}
	sort.Strings(locales)
	return locales
}
