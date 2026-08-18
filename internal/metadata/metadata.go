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
	"time"
	"unicode/utf8"

	"github.com/Arata1202/ascdir/internal/appstore"
	"github.com/Arata1202/ascdir/internal/atomicfile"
	"github.com/Arata1202/ascdir/internal/config"
)

func ReadLocal(cfg config.Config, configPath string) (appstore.Metadata, error) {
	result := appstore.Metadata{AppID: cfg.App.ID, Values: cfg.Metadata.Map(), Accessibility: map[string]appstore.AccessibilityDeclaration{}, Screenshots: map[string]map[string][]appstore.Asset{}, ScreenshotSetIDs: map[string]map[string]string{}, AppPreviews: map[string]map[string][]appstore.Asset{}, AppPreviewSetIDs: map[string]map[string]string{}, Localizations: map[string]appstore.Localization{}}
	for field, value := range cfg.Categories.Map() {
		result.Values[field] = value
	}
	for field, value := range cfg.AgeRating.Map() {
		result.Values[field] = value
	}
	if cfg.Availability != nil {
		for field, value := range cfg.Availability.Map() {
			result.Values[field] = value
		}
	}
	for deviceFamily, declaration := range cfg.Accessibility {
		result.Accessibility[deviceFamily] = appstore.AccessibilityDeclaration{Values: declaration.Map()}
	}
	base := filepath.Dir(filepath.Clean(configPath))
	if cfg.LicenseAgreement != nil {
		for field, value := range cfg.LicenseAgreement.Map() {
			result.Values[field] = value
		}
		fullPath, err := managedPath(base, cfg.LicenseAgreement.File, true)
		if err != nil {
			return appstore.Metadata{}, fmt.Errorf("resolve license_agreement.file: %w", err)
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return appstore.Metadata{}, fmt.Errorf("read license_agreement.file: %w", err)
		}
		result.Values["license_agreement_text"] = strings.TrimSuffix(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	}
	screenshots, err := readLocalScreenshots(cfg, base)
	if err != nil {
		return appstore.Metadata{}, err
	}
	result.Screenshots = screenshots
	previews, err := readLocalAppPreviews(cfg, base)
	if err != nil {
		return appstore.Metadata{}, err
	}
	result.AppPreviews = previews
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
	return writeLocal(cfg, configPath, remote, false)
}

// WriteLocalNew writes a newly initialized project. Unlike pull, init owns the
// complete configuration and intentionally replaces any file accepted through
// --force instead of attempting to preserve its previous schema or comments.
func WriteLocalNew(cfg config.Config, configPath string, remote appstore.Metadata) error {
	return writeLocal(cfg, configPath, remote, true)
}

func writeLocal(cfg config.Config, configPath string, remote appstore.Metadata, replaceConfig bool) error {
	base := filepath.Dir(filepath.Clean(configPath))
	type writeOperation struct {
		locale string
		field  string
		path   string
		data   []byte
		source string
	}
	var operations []writeOperation
	resolvedPaths := map[string]string{}
	if cfg.LicenseAgreement != nil {
		fullPath, err := prepareManagedPath(base, cfg.LicenseAgreement.File)
		if err != nil {
			return fmt.Errorf("resolve license_agreement.file: %w", err)
		}
		value := remote.Values["license_agreement_text"]
		if value != "" {
			value += "\n"
		}
		resolvedPaths[fullPath] = "license_agreement.file"
		operations = append(operations, writeOperation{field: "license_agreement", path: fullPath, data: []byte(value)})
	}
	var staleScreenshots []string
	var staleAppPreviews []string
	if cfg.Assets.Screenshots != "" {
		root, err := prepareManagedPath(base, filepath.ToSlash(filepath.Join(cfg.Assets.Screenshots, ".keep")))
		if err != nil {
			return fmt.Errorf("resolve assets.screenshots: %w", err)
		}
		root = filepath.Dir(root)
		wanted := map[string]bool{}
		for locale, sets := range remote.Screenshots {
			for displayType, assets := range sets {
				for _, asset := range assets {
					if len(asset.Content) == 0 && asset.Path == "" {
						return fmt.Errorf("screenshot %s/%s/%s was not downloaded", locale, displayType, asset.FileName)
					}
					relative := filepath.ToSlash(filepath.Join(cfg.Assets.Screenshots, locale, displayType, asset.FileName))
					fullPath, err := prepareManagedPath(base, relative)
					if err != nil {
						return fmt.Errorf("resolve screenshot %s: %w", relative, err)
					}
					wanted[fullPath] = true
					operations = append(operations, writeOperation{field: "screenshot", path: fullPath, data: asset.Content, source: asset.Path})
					if asset.Path != "" {
						defer os.Remove(asset.Path)
					}
				}
			}
		}
		if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || wanted[path] {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
				staleScreenshots = append(staleScreenshots, path)
			}
			return nil
		}); err != nil {
			return fmt.Errorf("scan assets.screenshots: %w", err)
		}
	}
	if cfg.Assets.AppPreviews != "" {
		root, err := prepareManagedPath(base, filepath.ToSlash(filepath.Join(cfg.Assets.AppPreviews, ".keep")))
		if err != nil {
			return fmt.Errorf("resolve assets.app_previews: %w", err)
		}
		root = filepath.Dir(root)
		wanted := map[string]bool{}
		for locale, sets := range remote.AppPreviews {
			for previewType, assets := range sets {
				for _, asset := range assets {
					if len(asset.Content) == 0 && asset.Path == "" {
						return fmt.Errorf("app preview %s/%s/%s was not downloaded", locale, previewType, asset.FileName)
					}
					relative := filepath.ToSlash(filepath.Join(cfg.Assets.AppPreviews, locale, previewType, asset.FileName))
					fullPath, err := prepareManagedPath(base, relative)
					if err != nil {
						return fmt.Errorf("resolve app preview %s: %w", relative, err)
					}
					wanted[fullPath] = true
					operations = append(operations, writeOperation{field: "app_preview", path: fullPath, data: asset.Content, source: asset.Path})
					if asset.Path != "" {
						defer os.Remove(asset.Path)
					}
				}
			}
		}
		if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || wanted[path] {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext == ".mov" || ext == ".mp4" || ext == ".m4v" {
				staleAppPreviews = append(staleAppPreviews, path)
			}
			return nil
		}); err != nil {
			return fmt.Errorf("scan assets.app_previews: %w", err)
		}
	}
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
		if cfg.Assets.AppPreviews != "" {
			updated.Assets.PreviewFrameTimes = map[string]string{}
			for locale, sets := range remote.AppPreviews {
				for previewType, assets := range sets {
					for _, asset := range assets {
						if asset.PreviewFrameTimeCode != "" {
							key := filepath.ToSlash(filepath.Join(locale, previewType, asset.FileName))
							updated.Assets.PreviewFrameTimes[key] = asset.PreviewFrameTimeCode
						}
					}
				}
			}
		}
		for field, pointer := range cfg.Metadata.Pointers() {
			if pointer != nil {
				updated.Metadata.SetManaged(field, remote.Values[field])
			}
		}
		for field, pointer := range cfg.Categories.Pointers() {
			if pointer != nil {
				updated.Categories.SetManaged(field, remote.Values[field])
			}
		}
		for field, pointer := range cfg.AgeRating.Pointers() {
			if pointer != nil {
				updated.AgeRating.SetManaged(field, remote.Values[field])
			}
		}
		if cfg.Availability != nil {
			for field := range cfg.Availability.Map() {
				updated.Availability.SetManaged(field, remote.Values[field])
			}
		}
		for deviceFamily, declaration := range cfg.Accessibility {
			for field, pointer := range declaration.Pointers() {
				if pointer != nil {
					declaration.SetManaged(field, remote.Accessibility[deviceFamily].Values[field])
				}
			}
			if updated.Accessibility == nil {
				updated.Accessibility = map[string]config.AccessibilityValues{}
			}
			updated.Accessibility[deviceFamily] = declaration
		}
		if cfg.LicenseAgreement != nil {
			territories := remote.Values["license_agreement_territories"]
			if territories == "" {
				updated.LicenseAgreement.Territories = nil
			} else {
				updated.LicenseAgreement.Territories = strings.Split(territories, ",")
			}
		}
		updated.Localizations = make(map[string]config.Localization, len(cfg.Localizations))
		for locale, localization := range cfg.Localizations {
			for field, pointer := range localization.Values.Pointers() {
				if pointer != nil {
					localization.Values.SetManaged(field, remote.Localizations[locale].Values[field])
				}
			}
			updated.Localizations[locale] = localization
		}
		var data []byte
		var err error
		if replaceConfig {
			data, err = config.Encode(updated)
		} else {
			data, err = config.EncodeUpdatedValues(configPath, updated)
		}
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
		var file *atomicfile.Pending
		var err error
		if operation.source == "" {
			file, err = atomicfile.Prepare(operation.path, operation.data, 0o644)
		} else {
			source, openErr := os.Open(operation.source)
			if openErr != nil {
				err = openErr
			} else {
				file, err = atomicfile.PrepareReader(operation.path, source, 0o644)
				closeErr := source.Close()
				if err == nil {
					err = closeErr
				}
			}
		}
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
	for _, path := range staleScreenshots {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove stale screenshot %s: %w", path, err)
		}
	}
	for _, path := range staleAppPreviews {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove stale app preview %s: %w", path, err)
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
	globalFields := make([]string, 0, len(desired.Values))
	for field := range desired.Values {
		globalFields = append(globalFields, field)
	}
	sort.Strings(globalFields)
	for _, field := range globalFields {
		if desired.Values[field] != remote.Values[field] {
			changes = append(changes, appstore.Change{Field: field, Before: remote.Values[field], After: desired.Values[field]})
		}
	}
	deviceFamilies := make([]string, 0, len(desired.Accessibility))
	for deviceFamily := range desired.Accessibility {
		deviceFamilies = append(deviceFamilies, deviceFamily)
	}
	sort.Strings(deviceFamilies)
	for _, deviceFamily := range deviceFamilies {
		fields := make([]string, 0, len(desired.Accessibility[deviceFamily].Values))
		for field := range desired.Accessibility[deviceFamily].Values {
			fields = append(fields, field)
		}
		sort.Strings(fields)
		for _, field := range fields {
			before, after := remote.Accessibility[deviceFamily].Values[field], desired.Accessibility[deviceFamily].Values[field]
			if before != after {
				changes = append(changes, appstore.Change{DeviceFamily: deviceFamily, Field: field, Before: before, After: after})
			}
		}
	}
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
	changes = append(changes, screenshotChanges(desired, remote)...)
	changes = append(changes, appPreviewChanges(desired, remote)...)
	return changes
}

func Select(cfg config.Config, source appstore.Metadata) appstore.Metadata {
	selected := appstore.Metadata{AppID: source.AppID, AppInfoID: source.AppInfoID, VersionID: source.VersionID, AgeRatingID: source.AgeRatingID, AvailabilityID: source.AvailabilityID, TerritoryAvailabilityIDs: source.TerritoryAvailabilityIDs, Accessibility: map[string]appstore.AccessibilityDeclaration{}, Screenshots: map[string]map[string][]appstore.Asset{}, ScreenshotSetIDs: map[string]map[string]string{}, AppPreviews: map[string]map[string][]appstore.Asset{}, AppPreviewSetIDs: map[string]map[string]string{}, Values: map[string]string{}, Localizations: map[string]appstore.Localization{}}
	for field := range cfg.Metadata.Map() {
		selected.Values[field] = source.Values[field]
	}
	for field := range cfg.Categories.Map() {
		selected.Values[field] = source.Values[field]
	}
	for field := range cfg.AgeRating.Map() {
		selected.Values[field] = source.Values[field]
	}
	if cfg.LicenseAgreement != nil {
		selected.LicenseAgreementID = source.LicenseAgreementID
		selected.Values["license_agreement_text"] = source.Values["license_agreement_text"]
		selected.Values["license_agreement_territories"] = source.Values["license_agreement_territories"]
	}
	if cfg.Availability != nil {
		for field := range cfg.Availability.Map() {
			selected.Values[field] = source.Values[field]
		}
	}
	for deviceFamily, declaration := range cfg.Accessibility {
		values := map[string]string{}
		for field := range declaration.Map() {
			values[field] = source.Accessibility[deviceFamily].Values[field]
		}
		selected.Accessibility[deviceFamily] = appstore.AccessibilityDeclaration{ID: source.Accessibility[deviceFamily].ID, Values: values}
	}
	if cfg.Assets.Screenshots != "" {
		selected.Screenshots = source.Screenshots
		selected.ScreenshotSetIDs = source.ScreenshotSetIDs
	}
	if cfg.Assets.AppPreviews != "" {
		selected.AppPreviews = source.AppPreviews
		selected.AppPreviewSetIDs = source.AppPreviewSetIDs
	}
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
		if change.AssetSet != nil {
			fmt.Fprintf(w, "assets.%s.%s.%s\n", change.AssetSet.Kind, change.AssetSet.Locale, change.AssetSet.DisplayType)
			fmt.Fprintf(w, "- %d asset(s)\n", len(change.AssetSet.Before))
			fmt.Fprintf(w, "+ %d asset(s)\n", len(change.AssetSet.After))
			continue
		}
		if change.Locale == "" {
			if change.DeviceFamily != "" {
				fmt.Fprintf(w, "accessibility.%s.%s\n", change.DeviceFamily, change.Field)
				fmt.Fprintf(w, "- %s\n", summarize(change.Before))
				fmt.Fprintf(w, "+ %s\n", summarize(change.After))
				continue
			}
			prefix := "metadata"
			if isCategoryField(change.Field) {
				prefix = "categories"
			} else if isAgeRatingField(change.Field) {
				prefix = "age_rating"
			} else if strings.HasPrefix(change.Field, "availability.") {
				prefix = "availability"
			}
			fmt.Fprintf(w, "%s.%s\n", prefix, change.Field)
		} else {
			fmt.Fprintf(w, "%s.%s\n", change.Locale, change.Field)
		}
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
	for locale, sets := range values.Screenshots {
		for displayType, assets := range sets {
			if len(assets) > 10 {
				problems = append(problems, fmt.Sprintf("assets.screenshots.%s.%s has %d screenshots; maximum is 10", locale, displayType, len(assets)))
			}
		}
	}
	for locale, sets := range values.AppPreviews {
		for previewType, assets := range sets {
			if len(assets) > 3 {
				problems = append(problems, fmt.Sprintf("assets.app_previews.%s.%s has %d previews; maximum is 3", locale, previewType, len(assets)))
			}
		}
	}
	for field, value := range values.Values {
		if strings.HasPrefix(field, "availability.territories.") && strings.HasSuffix(field, ".release_date") && value != "" {
			if _, err := time.Parse("2006-01-02", value); err != nil {
				problems = append(problems, fmt.Sprintf("%s must use YYYY-MM-DD", field))
			}
		}
	}
	if value, configured := values.Values["copyright"]; configured && strings.TrimSpace(value) == "" {
		problems = append(problems, "metadata.copyright is empty")
	}
	if value, configured := values.Values["accessibility_url"]; configured && value != "" && !validHTTPURL(value) {
		problems = append(problems, "metadata.accessibility_url is not a valid HTTP(S) URL")
	}
	if value, configured := values.Values["content_rights_declaration"]; configured && value != "" && value != "DOES_NOT_USE_THIRD_PARTY_CONTENT" && value != "USES_THIRD_PARTY_CONTENT" {
		problems = append(problems, "metadata.content_rights_declaration must be DOES_NOT_USE_THIRD_PARTY_CONTENT or USES_THIRD_PARTY_CONTENT")
	}
	licenseText, managesLicense := values.Values["license_agreement_text"]
	licenseTerritories := values.Values["license_agreement_territories"]
	if managesLicense && (strings.TrimSpace(licenseText) == "") != (strings.TrimSpace(licenseTerritories) == "") {
		problems = append(problems, "license_agreement requires both non-empty text and at least one territory, or both empty to use Apple's standard EULA")
	}
	frequencyValues := map[string]bool{"NONE": true, "INFREQUENT_OR_MILD": true, "FREQUENT_OR_INTENSE": true, "INFREQUENT": true, "FREQUENT": true}
	for _, field := range ageRatingFrequencyFields() {
		if value, configured := values.Values[field]; configured && value != "" && !frequencyValues[value] {
			problems = append(problems, fmt.Sprintf("age_rating.%s has unsupported value %q", field, value))
		}
	}
	validateAgeRatingEnum := func(field string, allowed map[string]bool) {
		if value, configured := values.Values[field]; configured && value != "" && !allowed[value] {
			problems = append(problems, fmt.Sprintf("age_rating.%s has unsupported value %q", field, value))
		}
	}
	validateAgeRatingEnum("kids_age_band", map[string]bool{"FIVE_AND_UNDER": true, "SIX_TO_EIGHT": true, "NINE_TO_ELEVEN": true})
	validateAgeRatingEnum("age_rating_override", map[string]bool{"NONE": true, "NINE_PLUS": true, "THIRTEEN_PLUS": true, "SIXTEEN_PLUS": true, "EIGHTEEN_PLUS": true, "UNRATED": true})
	validateAgeRatingEnum("korea_age_rating_override", map[string]bool{"NONE": true, "FIFTEEN_PLUS": true, "NINETEEN_PLUS": true})
	if value, configured := values.Values["developer_age_rating_info_url"]; configured && value != "" && !validHTTPURL(value) {
		problems = append(problems, "age_rating.developer_age_rating_info_url is not a valid HTTP(S) URL")
	}
	if _, managed := values.Values["primary_category"]; managed && strings.TrimSpace(values.Values["primary_category"]) == "" {
		problems = append(problems, "categories.primary_category is empty")
	}
	if primary, secondary := values.Values["primary_category"], values.Values["secondary_category"]; primary != "" && primary == secondary {
		problems = append(problems, "categories.primary_category and categories.secondary_category must differ")
	}
	if values.Values["primary_category"] == "" && (values.Values["primary_subcategory_one"] != "" || values.Values["primary_subcategory_two"] != "") {
		problems = append(problems, "primary subcategories require categories.primary_category")
	}
	if values.Values["primary_subcategory_one"] == "" && values.Values["primary_subcategory_two"] != "" {
		problems = append(problems, "categories.primary_subcategory_two requires primary_subcategory_one")
	}
	if values.Values["secondary_category"] == "" && (values.Values["secondary_subcategory_one"] != "" || values.Values["secondary_subcategory_two"] != "") {
		problems = append(problems, "secondary subcategories require categories.secondary_category")
	}
	if values.Values["secondary_subcategory_one"] == "" && values.Values["secondary_subcategory_two"] != "" {
		problems = append(problems, "categories.secondary_subcategory_two requires secondary_subcategory_one")
	}
	for field, value := range values.Values {
		if isCategoryField(field) && value != "" && strings.TrimSpace(value) != value {
			problems = append(problems, fmt.Sprintf("categories.%s contains leading or trailing whitespace", field))
		}
	}
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
			if !validHTTPURL(value) {
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

func validHTTPURL(value string) bool {
	parsed, err := url.ParseRequestURI(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != ""
}

func isCategoryField(field string) bool {
	switch field {
	case "primary_category", "primary_subcategory_one", "primary_subcategory_two", "secondary_category", "secondary_subcategory_one", "secondary_subcategory_two":
		return true
	default:
		return false
	}
}

func ageRatingFrequencyFields() []string {
	return []string{"alcohol_tobacco_or_drug_use_or_references", "contests", "gambling_simulated", "guns_or_other_weapons", "medical_or_treatment_information", "profanity_or_crude_humor", "sexual_content_graphic_and_nudity", "sexual_content_or_nudity", "horror_or_fear_themes", "mature_or_suggestive_themes", "violence_cartoon_or_fantasy", "violence_realistic_prolonged_graphic_or_sadistic", "violence_realistic"}
}

func isAgeRatingField(field string) bool {
	for _, candidate := range ageRatingFrequencyFields() {
		if field == candidate {
			return true
		}
	}
	switch field {
	case "advertising", "gambling", "health_or_wellness_topics", "kids_age_band", "loot_box", "messaging_and_chat", "parental_controls", "age_assurance", "social_media", "social_media_age_restricted", "unrestricted_web_access", "user_generated_content", "age_rating_override", "korea_age_rating_override", "developer_age_rating_info_url":
		return true
	default:
		return false
	}
}
