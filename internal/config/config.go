package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Version       string                 `yaml:"version"`
	App           App                    `yaml:"app"`
	Localizations map[string]LocaleFiles `yaml:"localizations"`
}

type App struct {
	ID       string `yaml:"id,omitempty"`
	BundleID string `yaml:"bundle_id"`
	Platform string `yaml:"platform"`
	Version  string `yaml:"version"`
}

type LocaleFiles struct {
	Name             string `yaml:"name"`
	Subtitle         string `yaml:"subtitle"`
	Description      string `yaml:"description"`
	Keywords         string `yaml:"keywords"`
	PromotionalText  string `yaml:"promotional_text"`
	WhatsNew         string `yaml:"whats_new"`
	SupportURL       string `yaml:"support_url"`
	MarketingURL     string `yaml:"marketing_url"`
	PrivacyPolicyURL string `yaml:"privacy_policy_url"`
}

func New(appID, bundleID, platform, version string, locales []string) Config {
	cfg := Config{
		Version:       "1",
		App:           App{ID: appID, BundleID: bundleID, Platform: platform, Version: version},
		Localizations: make(map[string]LocaleFiles, len(locales)),
	}
	for _, locale := range locales {
		base := filepath.ToSlash(filepath.Join("metadata", locale))
		cfg.Localizations[locale] = LocaleFiles{
			Name: base + "/name.txt", Subtitle: base + "/subtitle.txt",
			Description: base + "/description.md", Keywords: base + "/keywords.txt",
			PromotionalText: base + "/promotional_text.txt", WhatsNew: base + "/whats_new.md",
			SupportURL: base + "/support_url.txt", MarketingURL: base + "/marketing_url.txt",
			PrivacyPolicyURL: base + "/privacy_policy_url.txt",
		}
	}
	return cfg
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Clean(path)), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func (c Config) Validate() error {
	if c.Version != "1" {
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
	for locale, files := range c.Localizations {
		if strings.TrimSpace(locale) == "" {
			return errors.New("localization locale cannot be empty")
		}
		for field, path := range files.Paths() {
			if path == "" {
				continue
			}
			clean := filepath.Clean(path)
			if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
				return fmt.Errorf("%s.%s must be a relative path inside the project", locale, field)
			}
			if previous, ok := seen[clean]; ok {
				return fmt.Errorf("%s.%s reuses path %q from %s", locale, field, path, previous)
			}
			seen[clean] = locale + "." + field
		}
	}
	return nil
}

func (f LocaleFiles) Paths() map[string]string {
	return map[string]string{
		"name": f.Name, "subtitle": f.Subtitle, "description": f.Description,
		"keywords": f.Keywords, "promotional_text": f.PromotionalText,
		"whats_new": f.WhatsNew, "support_url": f.SupportURL,
		"marketing_url": f.MarketingURL, "privacy_policy_url": f.PrivacyPolicyURL,
	}
}

func SortedLocales(localizations map[string]LocaleFiles) []string {
	locales := make([]string, 0, len(localizations))
	for locale := range localizations {
		locales = append(locales, locale)
	}
	sort.Strings(locales)
	return locales
}
