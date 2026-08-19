package metadata

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Arata1202/ascdir/internal/appstore"
	"github.com/Arata1202/ascdir/internal/config"
)

func readLocalAppPreviews(cfg config.Config, base string, allowMissing bool) (map[string]map[string][]appstore.Asset, error) {
	result := map[string]map[string][]appstore.Asset{}
	if cfg.Assets.AppPreviews == "" {
		return result, nil
	}
	candidate := filepath.Join(base, filepath.FromSlash(cfg.Assets.AppPreviews))
	if _, err := os.Stat(candidate); os.IsNotExist(err) {
		if allowMissing {
			return result, nil
		}
		return nil, fmt.Errorf("assets.app_previews directory does not exist: %s", cfg.Assets.AppPreviews)
	} else if err != nil {
		return nil, fmt.Errorf("stat assets.app_previews: %w", err)
	}
	root, err := managedPath(base, cfg.Assets.AppPreviews, true)
	if err != nil {
		return nil, fmt.Errorf("resolve assets.app_previews: %w", err)
	}
	portablePaths := map[string]string{}
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		parts := strings.Split(filepath.ToSlash(relative), "/")
		if entry.IsDir() {
			if len(parts) == 2 {
				if result[parts[0]] == nil {
					result[parts[0]] = map[string][]appstore.Asset{}
				}
				result[parts[0]][parts[1]] = []appstore.Asset{}
			}
			return nil
		}
		if ignoredAssetHelperFile(entry.Name()) {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("app preview %s must not be a symbolic link", path)
		}
		if len(parts) != 3 {
			return fmt.Errorf("app preview %s must use <locale>/<preview-type>/<file>", relative)
		}
		locale, previewType := parts[0], parts[1]
		for _, component := range []struct{ kind, value string }{{"app preview locale", locale}, {"app preview type", previewType}, {"app preview file name", entry.Name()}} {
			if err := validateRemotePathComponent(component.kind, component.value); err != nil {
				return err
			}
		}
		portableKey := portablePathKey(relative)
		if previous, exists := portablePaths[portableKey]; exists {
			return fmt.Errorf("app preview %s collides with %s on a supported filesystem", relative, previous)
		}
		portablePaths[portableKey] = relative
		if _, ok := cfg.Localizations[locale]; !ok {
			return fmt.Errorf("app preview locale %q is not configured", locale)
		}
		extension := strings.ToLower(filepath.Ext(path))
		if extension != ".mov" && extension != ".mp4" && extension != ".m4v" {
			return fmt.Errorf("app preview %s must be MOV, MP4, or M4V", relative)
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		hash := md5.New()
		if _, err := io.Copy(hash, file); err != nil {
			file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		mimeType := mime.TypeByExtension(extension)
		if extension == ".mov" {
			mimeType = "video/quicktime"
		} else if mimeType == "" {
			mimeType = "video/mp4"
		}
		key := filepath.ToSlash(filepath.Join(locale, previewType, entry.Name()))
		asset := appstore.Asset{FileName: entry.Name(), Path: path, Checksum: hex.EncodeToString(hash.Sum(nil)), Size: info.Size(), MIMEType: mimeType, PreviewFrameTimeCode: cfg.Assets.PreviewFrameTimes[key]}
		if result[locale] == nil {
			result[locale] = map[string][]appstore.Asset{}
		}
		result[locale][previewType] = append(result[locale][previewType], asset)
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, sets := range result {
		for previewType := range sets {
			sort.SliceStable(sets[previewType], func(i, j int) bool { return sets[previewType][i].FileName < sets[previewType][j].FileName })
		}
	}
	return result, nil
}

func ignoredAssetHelperFile(name string) bool {
	return name == ".DS_Store" || name == ".gitkeep"
}

func appPreviewChanges(desired, remote appstore.Metadata) []appstore.Change {
	var changes []appstore.Change
	locales := make([]string, 0, len(desired.Localizations))
	for locale := range desired.Localizations {
		locales = append(locales, locale)
	}
	sort.Strings(locales)
	for _, locale := range locales {
		previewTypeSet := map[string]bool{}
		for previewType := range desired.AppPreviews[locale] {
			previewTypeSet[previewType] = true
		}
		for previewType := range remote.AppPreviews[locale] {
			previewTypeSet[previewType] = true
		}
		previewTypes := make([]string, 0, len(previewTypeSet))
		for previewType := range previewTypeSet {
			previewTypes = append(previewTypes, previewType)
		}
		sort.Strings(previewTypes)
		for _, previewType := range previewTypes {
			before, after := remote.AppPreviews[locale][previewType], desired.AppPreviews[locale][previewType]
			if previewAssetsEqual(before, after) {
				continue
			}
			changes = append(changes, appstore.Change{AssetSet: &appstore.AssetSetChange{
				Kind: "app_previews", Locale: locale, DisplayType: previewType,
				LocalizationID: remote.Localizations[locale].VersionLocalizationID,
				SetID:          remote.AppPreviewSetIDs[locale][previewType], Before: before, After: after,
			}})
		}
	}
	return changes
}

func previewAssetsEqual(before, after []appstore.Asset) bool {
	if len(before) != len(after) {
		return false
	}
	for index := range before {
		if before[index].Checksum != after[index].Checksum || before[index].FileName != after[index].FileName || before[index].PreviewFrameTimeCode != after[index].PreviewFrameTimeCode {
			return false
		}
	}
	return true
}
