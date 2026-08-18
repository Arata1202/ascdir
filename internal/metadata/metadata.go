package metadata

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/Arata1202/ascdir/internal/appstore"
	"github.com/Arata1202/ascdir/internal/atomicfile"
	"github.com/Arata1202/ascdir/internal/config"
)

func ReadLocal(cfg config.Config, configPath string) (appstore.Metadata, error) {
	result := appstore.Metadata{AppID: cfg.App.ID, Localizations: map[string]appstore.Localization{}}
	base := filepath.Dir(filepath.Clean(configPath))
	for _, locale := range config.SortedLocales(cfg.Localizations) {
		localization := cfg.Localizations[locale]
		values := localization.Values.Map()
		paths := localization.Paths()
		for _, field := range sortedFields(paths) {
			path := paths[field]
			if path == "" {
				continue
			}
			fullPath, err := managedPath(base, path, true)
			if err != nil {
				return appstore.Metadata{}, fmt.Errorf("resolve %s.%s: %w", locale, field, err)
			}
			data, err := os.ReadFile(fullPath)
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
	type writeOperation struct {
		locale string
		field  string
		path   string
		data   []byte
	}
	var operations []writeOperation
	resolvedPaths := map[string]string{}
	for _, locale := range config.SortedLocales(cfg.Localizations) {
		localization := cfg.Localizations[locale]
		remoteLocale := remote.Localizations[locale]
		paths := localization.Paths()
		for _, field := range sortedFields(paths) {
			path := paths[field]
			if path == "" {
				continue
			}
			fullPath, err := prepareManagedPath(base, path)
			if err != nil {
				return fmt.Errorf("resolve %s.%s: %w", locale, field, err)
			}
			value := remoteLocale.Values[field]
			if value != "" {
				value += "\n"
			}
			if previous, ok := resolvedPaths[fullPath]; ok {
				return fmt.Errorf("%s.%s resolves to the same metadata file as %s", locale, field, previous)
			}
			resolvedPaths[fullPath] = locale + "." + field
			operations = append(operations, writeOperation{locale: locale, field: field, path: fullPath, data: []byte(value)})
		}
	}
	if cfg.Version == config.CurrentVersion {
		updated := cfg
		updated.Localizations = make(map[string]config.Localization, len(cfg.Localizations))
		for locale, localization := range cfg.Localizations {
			for field, pointer := range localization.Values.Pointers() {
				if pointer != nil {
					localization.Values.SetManaged(field, remote.Localizations[locale].Values[field])
				}
			}
			updated.Localizations[locale] = localization
		}
		data, err := config.Encode(updated)
		if err != nil {
			return fmt.Errorf("encode configuration: %w", err)
		}
		operations = append(operations, writeOperation{field: "configuration", path: filepath.Clean(configPath), data: data})
	}
	// Resolve, validate, and fully stage every destination before replacing any
	// files. This prevents configuration, permission, and write errors from
	// modifying the existing metadata tree. Each replacement remains atomic.
	pending := make([]*atomicfile.Pending, 0, len(operations))
	defer func() {
		for _, file := range pending {
			file.Cleanup()
		}
	}()
	for _, operation := range operations {
		file, err := atomicfile.Prepare(operation.path, operation.data, 0o644)
		if err != nil {
			if operation.locale == "" {
				return fmt.Errorf("write %s: %w", operation.field, err)
			}
			return fmt.Errorf("write %s.%s: %w", operation.locale, operation.field, err)
		}
		pending = append(pending, file)
	}
	for index, file := range pending {
		if err := file.Commit(); err != nil {
			operation := operations[index]
			if operation.locale == "" {
				return fmt.Errorf("write %s: %w", operation.field, err)
			}
			return fmt.Errorf("write %s.%s: %w", operation.locale, operation.field, err)
		}
	}
	return nil
}

func prepareManagedPath(base, relative string) (string, error) {
	canonicalBase, err := filepath.EvalSymlinks(base)
	if err != nil {
		return "", fmt.Errorf("resolve configuration directory: %w", err)
	}
	parent := filepath.Dir(filepath.Join(canonicalBase, filepath.FromSlash(relative)))
	ancestor := parent
	for {
		if _, err := os.Lstat(ancestor); err == nil {
			break
		} else if !os.IsNotExist(err) {
			return "", err
		}
		next := filepath.Dir(ancestor)
		if next == ancestor {
			return "", errors.New("metadata path has no existing ancestor")
		}
		ancestor = next
	}
	resolvedAncestor, err := filepath.EvalSymlinks(ancestor)
	if err != nil {
		return "", err
	}
	if err := ensureWithin(canonicalBase, resolvedAncestor); err != nil {
		return "", err
	}
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", fmt.Errorf("create metadata directory: %w", err)
	}
	return managedPath(canonicalBase, relative, false)
}

func managedPath(base, relative string, requireExisting bool) (string, error) {
	canonicalBase, err := filepath.EvalSymlinks(base)
	if err != nil {
		return "", fmt.Errorf("resolve configuration directory: %w", err)
	}
	candidate := filepath.Join(canonicalBase, filepath.FromSlash(relative))
	var resolved string
	if requireExisting {
		resolved, err = filepath.EvalSymlinks(candidate)
	} else {
		resolvedParent, parentErr := filepath.EvalSymlinks(filepath.Dir(candidate))
		if parentErr != nil {
			err = parentErr
		} else {
			resolved = filepath.Join(resolvedParent, filepath.Base(candidate))
		}
	}
	if err != nil {
		return "", err
	}
	if err := ensureWithin(canonicalBase, resolved); err != nil {
		return "", err
	}
	return resolved, nil
}

func ensureWithin(base, path string) error {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return errors.New("metadata path resolves outside the configuration directory")
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

func Select(cfg config.Config, source appstore.Metadata) appstore.Metadata {
	selected := appstore.Metadata{AppID: source.AppID, AppInfoID: source.AppInfoID, VersionID: source.VersionID, Localizations: map[string]appstore.Localization{}}
	for locale, localization := range cfg.Localizations {
		values := map[string]string{}
		for field := range localization.Values.Map() {
			values[field] = source.Localizations[locale].Values[field]
		}
		for field, path := range localization.Paths() {
			if path != "" {
				values[field] = source.Localizations[locale].Values[field]
			}
		}
		selected.Localizations[locale] = appstore.Localization{Values: values}
	}
	return selected
}

func ClearingChanges(changes []appstore.Change) []appstore.Change {
	clears := make([]appstore.Change, 0)
	for _, change := range changes {
		if change.Before != "" && change.After == "" {
			clears = append(clears, change)
		}
	}
	return clears
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

func sortedFields(paths map[string]string) []string {
	fields := make([]string, 0, len(paths))
	for field := range paths {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	return fields
}

func Validate(values appstore.Metadata) []string {
	limits := map[string]int{
		"name": 30, "subtitle": 30, "description": 4000,
		"promotional_text": 170, "whats_new": 4000,
	}
	urlFields := map[string]bool{"support_url": true, "marketing_url": true, "privacy_policy_url": true, "privacy_choices_url": true}
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
		if value, configured := fields["keywords"]; configured && len([]byte(value)) > 100 {
			problems = append(problems, fmt.Sprintf("%s.keywords is %d bytes; maximum is 100", locale, len([]byte(value))))
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
		if value, configured := fields["description"]; configured && strings.TrimSpace(value) == "" {
			problems = append(problems, fmt.Sprintf("%s.description is empty", locale))
		}
		if value, configured := fields["name"]; configured && strings.TrimSpace(value) == "" {
			problems = append(problems, fmt.Sprintf("%s.name is empty", locale))
		}
		if value, configured := fields["support_url"]; configured && strings.TrimSpace(value) == "" {
			problems = append(problems, fmt.Sprintf("%s.support_url is empty", locale))
		}
	}
	sort.Strings(problems)
	return problems
}
