package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Arata1202/ascdir/internal/atomicfile"
	"gopkg.in/yaml.v3"
)

const CurrentVersion = "2"

type Config struct {
	Version       string                  `yaml:"version"`
	App           App                     `yaml:"app"`
	Metadata      MetadataValues          `yaml:"metadata,omitempty"`
	Categories    CategoryValues          `yaml:"categories,omitempty"`
	Localizations map[string]Localization `yaml:"localizations"`
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
				PrivacyPolicyText: base + "/privacy_policy.md",
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
		valueNode.Tag = "!!str"
	}
	return nil
}

func addSchemaComments(document *yaml.Node) {
	root := document
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		root = root.Content[0]
	}
	localizations := mappingValue(root, "localizations")
	if localizations == nil {
		return
	}
	valueComments := map[string]string{
		"name":                "App Store display name, up to 30 characters",
		"subtitle":            "Short summary displayed below the name, up to 30 characters",
		"keywords":            "Comma-separated search keywords, up to 100 bytes",
		"support_url":         "Public HTTP(S) support page",
		"marketing_url":       "Optional public HTTP(S) marketing page",
		"privacy_policy_url":  "Public HTTP(S) privacy policy",
		"privacy_choices_url": "Optional public HTTP(S) privacy choices page",
	}
	fileComments := map[string]string{
		"description":         "Markdown file containing the long app description",
		"promotional_text":    "Markdown file containing promotional text",
		"whats_new":           "Markdown file containing release notes",
		"privacy_policy_text": "Markdown file containing the tvOS privacy policy",
	}
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
	for index := 0; index+1 < len(localizations.Content); index += 2 {
		locale := localizations.Content[index+1]
		addMappingComments(mappingValue(locale, "values"), valueComments)
		addMappingComments(mappingValue(locale, "files"), fileComments)
	}
}

func addMappingComments(mapping *yaml.Node, comments map[string]string) {
	if mapping == nil {
		return
	}
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		key, value := mapping.Content[index], mapping.Content[index+1]
		value.LineComment = comments[key.Value]
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
	seen := map[string]string{}
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

func stringPointer(value string) *string { return &value }

func SortedLocales(localizations map[string]Localization) []string {
	locales := make([]string, 0, len(localizations))
	for locale := range localizations {
		locales = append(locales, locale)
	}
	sort.Strings(locales)
	return locales
}
