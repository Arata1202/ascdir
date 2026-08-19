package metadata

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Arata1202/ascdir/internal/appstore"
	"github.com/Arata1202/ascdir/internal/config"
)

func TestReadAndDiffLocalAppPreviews(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	root := filepath.Join(dir, "assets", "app-previews", "en-US", "IPHONE_67")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "01.mp4"), []byte("video one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "02.mov"), []byte("video two"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.New("app-1", "com.example.app", "IOS", "1.0", []string{"en-US"})
	cfg.Assets.AppPreviews = "assets/app-previews"
	cfg.Assets.PreviewFrameTimes = map[string]string{"en-US/IPHONE_67/01.mp4": "00:00:05"}
	previews, err := readLocalAppPreviews(cfg, dir, false)
	if err != nil {
		t.Fatal(err)
	}
	assets := previews["en-US"]["IPHONE_67"]
	if len(assets) != 2 || assets[0].FileName != "01.mp4" || assets[0].MIMEType != "video/mp4" || assets[0].PreviewFrameTimeCode != "00:00:05" {
		t.Fatalf("assets = %#v", assets)
	}
	remoteAssets := append([]appstore.Asset(nil), assets...)
	remoteAssets[0].PreviewFrameTimeCode = "00:00:01"
	desired := appstore.Metadata{Localizations: map[string]appstore.Localization{"en-US": {}}, AppPreviews: previews}
	remote := appstore.Metadata{
		Localizations:    map[string]appstore.Localization{"en-US": {VersionLocalizationID: "loc-1"}},
		AppPreviews:      map[string]map[string][]appstore.Asset{"en-US": {"IPHONE_67": remoteAssets}},
		AppPreviewSetIDs: map[string]map[string]string{"en-US": {"IPHONE_67": "set-1"}},
	}
	changes := appPreviewChanges(desired, remote)
	if len(changes) != 1 || changes[0].AssetSet == nil || changes[0].AssetSet.SetID != "set-1" {
		t.Fatalf("changes = %#v", changes)
	}
	if changes[0].AssetSet.LocalizationID != "loc-1" || changes[0].AssetSet.After[0].Path != assets[0].Path {
		t.Fatalf("asset change lost its localization or local path: %#v", changes[0].AssetSet)
	}
	if AssetSetDeletesRemoteFiles(*changes[0].AssetSet) {
		t.Fatal("poster-frame-only update was treated as a video deletion")
	}
}

func TestReadLocalAppPreviewsRejectsUnsupportedFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	root := filepath.Join(dir, "assets", "app-previews", "en-US", "IPHONE_67")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("not a video"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.New("app-1", "com.example.app", "IOS", "1.0", []string{"en-US"})
	cfg.Assets.AppPreviews = "assets/app-previews"
	if _, err := readLocalAppPreviews(cfg, dir, false); err == nil {
		t.Fatal("unsupported preview file was accepted")
	}
}

func TestPreviewAssetsEqual(t *testing.T) {
	t.Parallel()
	asset := appstore.Asset{Checksum: "abc", PreviewFrameTimeCode: "00:00:05"}
	if !previewAssetsEqual([]appstore.Asset{asset}, []appstore.Asset{asset}) {
		t.Fatal("identical preview assets differ")
	}
	if previewAssetsEqual([]appstore.Asset{asset}, nil) {
		t.Fatal("different preview asset counts compare equal")
	}
}

func TestWriteLocalStreamsDownloadedAppPreview(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "ascdir.yaml")
	cfg := config.New("app-1", "com.example.app", "IOS", "1.0", []string{"en-US"})
	cfg.Assets.AppPreviews = "assets/app-previews"
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "downloaded-preview")
	content := []byte("streamed video")
	if err := os.WriteFile(source, content, 0o600); err != nil {
		t.Fatal(err)
	}
	remote := appstore.Metadata{
		Values:        map[string]string{},
		Localizations: map[string]appstore.Localization{"en-US": {Values: map[string]string{}}},
		AppPreviews:   map[string]map[string][]appstore.Asset{"en-US": {"IPHONE_67": {{FileName: "01.mp4", Path: source, Size: int64(len(content))}}}},
	}
	if err := WriteLocal(cfg, configPath, remote); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(dir, "assets", "app-previews", "en-US", "IPHONE_67", "01.mp4")
	written, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != string(content) {
		t.Fatalf("content = %q", written)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("temporary source remains: %v", err)
	}
}
