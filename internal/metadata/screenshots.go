package metadata

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Arata1202/ascdir/internal/appstore"
	"github.com/Arata1202/ascdir/internal/config"
)

func readLocalScreenshots(cfg config.Config, base string, allowMissing bool) (map[string]map[string][]appstore.Asset, error) {
	result := map[string]map[string][]appstore.Asset{}
	if cfg.Assets.Screenshots == "" {
		return result, nil
	}
	candidate := filepath.Join(base, filepath.FromSlash(cfg.Assets.Screenshots))
	if _, err := os.Stat(candidate); os.IsNotExist(err) {
		if allowMissing {
			return result, nil
		}
		return nil, fmt.Errorf("assets.screenshots directory does not exist: %s", cfg.Assets.Screenshots)
	} else if err != nil {
		return nil, fmt.Errorf("stat assets.screenshots: %w", err)
	}
	root, err := managedPath(base, cfg.Assets.Screenshots, true)
	if err != nil {
		return nil, fmt.Errorf("resolve assets.screenshots: %w", err)
	}
	portablePaths := map[string]string{}
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root {
				relative, err := filepath.Rel(root, path)
				if err != nil {
					return err
				}
				parts := strings.Split(filepath.ToSlash(relative), "/")
				if len(parts) == 2 {
					if result[parts[0]] == nil {
						result[parts[0]] = map[string][]appstore.Asset{}
					}
					if _, exists := result[parts[0]][parts[1]]; !exists {
						result[parts[0]][parts[1]] = []appstore.Asset{}
					}
				}
			}
			return nil
		}
		if ignoredAssetHelperFile(entry.Name()) {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("screenshot %s must not be a symbolic link", path)
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		parts := strings.Split(filepath.ToSlash(relative), "/")
		if len(parts) != 3 {
			return fmt.Errorf("screenshot %s must use <locale>/<display-type>/<file>", relative)
		}
		locale, displayType := parts[0], parts[1]
		for _, component := range []struct{ kind, value string }{{"screenshot locale", locale}, {"screenshot display type", displayType}, {"screenshot file name", entry.Name()}} {
			if err := validateRemotePathComponent(component.kind, component.value); err != nil {
				return err
			}
		}
		portableKey := portablePathKey(relative)
		if previous, exists := portablePaths[portableKey]; exists {
			return fmt.Errorf("screenshot %s collides with %s on a supported filesystem", relative, previous)
		}
		portablePaths[portableKey] = relative
		if _, ok := cfg.Localizations[locale]; !ok {
			return fmt.Errorf("screenshot locale %q is not configured", locale)
		}
		extension := strings.ToLower(filepath.Ext(path))
		if extension != ".png" && extension != ".jpg" && extension != ".jpeg" {
			return fmt.Errorf("screenshot %s must be PNG or JPEG", relative)
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		if _, _, err := image.DecodeConfig(file); err != nil {
			file.Close()
			return fmt.Errorf("decode screenshot %s: %w", relative, err)
		}
		if _, err := file.Seek(0, 0); err != nil {
			file.Close()
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
		asset := appstore.Asset{FileName: entry.Name(), Path: path, Checksum: hex.EncodeToString(hash.Sum(nil)), Size: info.Size()}
		if result[locale] == nil {
			result[locale] = map[string][]appstore.Asset{}
		}
		result[locale][displayType] = append(result[locale][displayType], asset)
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, sets := range result {
		for displayType := range sets {
			sort.SliceStable(sets[displayType], func(i, j int) bool { return sets[displayType][i].FileName < sets[displayType][j].FileName })
		}
	}
	return result, nil
}

func screenshotChanges(desired, remote appstore.Metadata) []appstore.Change {
	var changes []appstore.Change
	locales := make([]string, 0, len(desired.Localizations))
	for locale := range desired.Localizations {
		locales = append(locales, locale)
	}
	sort.Strings(locales)
	for _, locale := range locales {
		displaySet := map[string]bool{}
		for displayType := range desired.Screenshots[locale] {
			displaySet[displayType] = true
		}
		for displayType := range remote.Screenshots[locale] {
			displaySet[displayType] = true
		}
		displayTypes := make([]string, 0, len(displaySet))
		for displayType := range displaySet {
			displayTypes = append(displayTypes, displayType)
		}
		sort.Strings(displayTypes)
		for _, displayType := range displayTypes {
			before, after := remote.Screenshots[locale][displayType], desired.Screenshots[locale][displayType]
			if assetsEqual(before, after) {
				continue
			}
			setID := remote.ScreenshotSetIDs[locale][displayType]
			changes = append(changes, appstore.Change{AssetSet: &appstore.AssetSetChange{
				Kind: "screenshots", Locale: locale, DisplayType: displayType,
				LocalizationID: remote.Localizations[locale].VersionLocalizationID,
				SetID:          setID, Before: before, After: after,
			}})
		}
	}
	return changes
}

func assetsEqual(before, after []appstore.Asset) bool {
	if len(before) != len(after) {
		return false
	}
	for index := range before {
		if before[index].Checksum != after[index].Checksum || before[index].FileName != after[index].FileName {
			return false
		}
	}
	return true
}

func AssetSetDeletesRemoteFiles(change appstore.AssetSetChange) bool {
	remaining := map[string]int{}
	for _, asset := range change.After {
		remaining[assetDeletionKey(change.Kind, asset)]++
	}
	for _, asset := range change.Before {
		key := assetDeletionKey(change.Kind, asset)
		if remaining[key] == 0 {
			return true
		}
		remaining[key]--
	}
	return false
}

func assetDeletionKey(kind string, asset appstore.Asset) string {
	return asset.Checksum + "\x00" + asset.FileName
}
