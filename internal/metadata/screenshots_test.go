package metadata

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/Arata1202/ascdir/internal/appstore"
	"github.com/Arata1202/ascdir/internal/config"
)

func TestReadLocalScreenshotsUsesDirectoryOrderAndChecksums(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	root := filepath.Join(dir, "assets", "screenshots", "en-US", "APP_IPHONE_67")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"02.png", "01.png"} {
		file, err := os.Create(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		picture := image.NewRGBA(image.Rect(0, 0, 2, 2))
		if name == "01.png" {
			picture.Set(0, 0, color.RGBA{R: 255, A: 255})
		} else {
			picture.Set(0, 0, color.RGBA{B: 255, A: 255})
		}
		if err := png.Encode(file, picture); err != nil {
			t.Fatal(err)
		}
		_ = file.Close()
	}
	cfg := config.New("app-1", "com.example.app", "IOS", "1.0", []string{"en-US"})
	cfg.Assets.Screenshots = "assets/screenshots"
	screenshots, err := readLocalScreenshots(cfg, dir)
	if err != nil {
		t.Fatal(err)
	}
	assets := screenshots["en-US"]["APP_IPHONE_67"]
	if len(assets) != 2 || assets[0].FileName != "01.png" || assets[0].Checksum == "" {
		t.Fatalf("assets = %#v", assets)
	}
	remote := appstore.Metadata{
		Localizations:    map[string]appstore.Localization{"en-US": {VersionLocalizationID: "loc-1"}},
		Screenshots:      map[string]map[string][]appstore.Asset{"en-US": {"APP_IPHONE_67": {assets[1], assets[0]}}},
		ScreenshotSetIDs: map[string]map[string]string{"en-US": {"APP_IPHONE_67": "set-1"}},
	}
	desired := appstore.Metadata{Screenshots: screenshots, Localizations: map[string]appstore.Localization{"en-US": {}}}
	changes := screenshotChanges(desired, remote)
	if len(changes) != 1 || changes[0].AssetSet == nil || changes[0].AssetSet.SetID != "set-1" {
		t.Fatalf("changes = %#v", changes)
	}
	if changes[0].AssetSet.LocalizationID != "loc-1" || changes[0].AssetSet.After[0].Path != assets[0].Path {
		t.Fatalf("asset change lost its localization or local path: %#v", changes[0].AssetSet)
	}
}

func TestAssetSetDeletionDetection(t *testing.T) {
	t.Parallel()
	change := appstore.AssetSetChange{Before: []appstore.Asset{{Checksum: "old"}}, After: []appstore.Asset{{Checksum: "new"}}}
	if !AssetSetDeletesRemoteFiles(change) {
		t.Fatal("replacement was not recognized as a remote deletion")
	}
}
