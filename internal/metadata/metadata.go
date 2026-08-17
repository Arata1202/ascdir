package metadata

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/Arata1202/ascdir/internal/appstore"
	"github.com/Arata1202/ascdir/internal/config"
)

func ReadLocal(cfg config.Config, configPath string) (appstore.Metadata, error) {
	result := appstore.Metadata{AppID: cfg.App.ID, Localizations: map[string]appstore.Localization{}}
	base := filepath.Dir(filepath.Clean(configPath))
	for _, locale := range config.SortedLocales(cfg.Localizations) {
		files := cfg.Localizations[locale]
		values := map[string]string{}
		for field, path := range files.Paths() {
			if path == "" {
				continue
			}
			data, err := os.ReadFile(filepath.Join(base, filepath.FromSlash(path)))
			if err != nil {
				return appstore.Metadata{}, fmt.Errorf("read %s.%s: %w", locale, field, err)
			}
			values[field] = strings.TrimSuffix(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
		}
		result.Localizations[locale] = appstore.Localization{Values: values}
	}
	return result, nil
}

func WriteLocal(cfg config.Config, configPath string, remote appstore.Metadata) error {
	base := filepath.Dir(filepath.Clean(configPath))
	for _, locale := range config.SortedLocales(cfg.Localizations) {
		files := cfg.Localizations[locale]
		remoteLocale := remote.Localizations[locale]
		for field, path := range files.Paths() {
			if path == "" {
				continue
			}
			fullPath := filepath.Join(base, filepath.FromSlash(path))
			if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
				return fmt.Errorf("create metadata directory: %w", err)
			}
			value := remoteLocale.Values[field]
			if value != "" {
				value += "\n"
			}
			if err := os.WriteFile(fullPath, []byte(value), 0o644); err != nil {
				return fmt.Errorf("write %s.%s: %w", locale, field, err)
			}
		}
	}
	return nil
}

func Diff(desired, remote appstore.Metadata) []appstore.Change {
	var changes []appstore.Change
	locales := make([]string, 0, len(desired.Localizations))
	for locale := range desired.Localizations {
		locales = append(locales, locale)
	}
	sort.Strings(locales)
	for _, locale := range locales {
		fields := make([]string, 0, len(desired.Localizations[locale].Values))
		for field := range desired.Localizations[locale].Values {
			fields = append(fields, field)
		}
		sort.Strings(fields)
		for _, field := range fields {
			before := remote.Localizations[locale].Values[field]
			after := desired.Localizations[locale].Values[field]
			if before != after {
				changes = append(changes, appstore.Change{Locale: locale, Field: field, Before: before, After: after})
			}
		}
	}
	return changes
}

func PrintChanges(w io.Writer, changes []appstore.Change) {
	for _, change := range changes {
		fmt.Fprintf(w, "%s.%s\n", change.Locale, change.Field)
		fmt.Fprintf(w, "- %s\n", summarize(change.Before))
		fmt.Fprintf(w, "+ %s\n", summarize(change.After))
	}
}

func summarize(value string) string {
	value = strings.ReplaceAll(value, "\n", "\\n")
	const limit = 120
	if utf8.RuneCountInString(value) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit]) + "…"
}

func Validate(values appstore.Metadata) []string {
	limits := map[string]int{
		"name": 30, "subtitle": 30, "description": 4000,
		"keywords": 100, "promotional_text": 170, "whats_new": 4000,
	}
	urlFields := map[string]bool{"support_url": true, "marketing_url": true, "privacy_policy_url": true}
	var problems []string
	locales := values.Locales()
	for _, locale := range locales {
		fields := values.Localizations[locale].Values
		for field, limit := range limits {
			value, configured := fields[field]
			if !configured {
				continue
			}
			length := utf8.RuneCountInString(value)
			if length > limit {
				problems = append(problems, fmt.Sprintf("%s.%s is %d characters; maximum is %d", locale, field, length, limit))
			}
		}
		for field := range urlFields {
			value, configured := fields[field]
			if !configured || value == "" {
				continue
			}
			parsed, err := url.ParseRequestURI(value)
			if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
				problems = append(problems, fmt.Sprintf("%s.%s is not a valid HTTP(S) URL", locale, field))
			}
		}
		if strings.TrimSpace(fields["description"]) == "" {
			problems = append(problems, fmt.Sprintf("%s.description is empty", locale))
		}
		if strings.TrimSpace(fields["support_url"]) == "" {
			problems = append(problems, fmt.Sprintf("%s.support_url is empty", locale))
		}
	}
	sort.Strings(problems)
	return problems
}
