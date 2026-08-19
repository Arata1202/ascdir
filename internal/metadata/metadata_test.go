package metadata

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Arata1202/ascdir/internal/appstore"
	"github.com/Arata1202/ascdir/internal/atomicfile"
	"github.com/Arata1202/ascdir/internal/config"
)

func TestWriteReadAndDiff(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "ascdir.yaml")
	cfg := config.New("app-1", "com.example.app", "IOS", "1.0.0", []string{"en-US"})
	configured := cfg.Localizations["en-US"]
	configured.Values.Subtitle = nil
	cfg.Localizations["en-US"] = configured
	remote := appstore.Metadata{Values: map[string]string{
		"copyright": "2026 Example, Inc.", "accessibility_url": "https://example.com/accessibility", "primary_category": "PRODUCTIVITY",
	}, Localizations: map[string]appstore.Localization{
		"en-US": {Values: map[string]string{"name": "Example", "subtitle": "Not managed", "description": "Description", "support_url": "https://example.com/support"}},
	}}
	if err := WriteLocal(cfg, configPath, remote); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "metadata", "en-US", "name.txt")); !os.IsNotExist(err) {
		t.Fatalf("short metadata was written to a separate file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "metadata", "en-US", "description.md")); err != nil {
		t.Fatalf("long-form metadata file was not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "metadata", "en-US", "privacy_policy.md")); !os.IsNotExist(err) {
		t.Fatalf("iOS project unexpectedly wrote tvOS privacy policy text: %v", err)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Localizations["en-US"].Values.Subtitle != nil {
		t.Fatal("pull added an unmanaged inline value")
	}
	local, err := ReadLocal(cfg, configPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := local.Localizations["en-US"].Values["name"]; got != "Example" {
		t.Fatalf("name = %q", got)
	}
	if local.Values["copyright"] != "2026 Example, Inc." || local.Values["accessibility_url"] != "https://example.com/accessibility" {
		t.Fatalf("global metadata = %#v", local.Values)
	}
	if local.Values["primary_category"] != "PRODUCTIVITY" {
		t.Fatalf("categories = %#v", local.Values)
	}
	localization := local.Localizations["en-US"]
	localization.Values["name"] = "Updated"
	local.Localizations["en-US"] = localization
	changes := Diff(local, remote)
	if len(changes) != 1 || changes[0].Field != "name" {
		t.Fatalf("unexpected changes: %#v", changes)
	}
	var output bytes.Buffer
	PrintChanges(&output, changes)
	if !strings.Contains(output.String(), "- Example") || !strings.Contains(output.String(), "+ Updated") {
		t.Fatalf("unexpected diff output: %s", output.String())
	}
}

func TestWriteLocalCreatesPrivacyPolicyTextForTVOS(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "ascdir.yaml")
	cfg := config.New("app-1", "com.example.app", "TV_OS", "1.0.0", []string{"en-US"})
	remote := appstore.Metadata{Localizations: map[string]appstore.Localization{
		"en-US": {Values: map[string]string{"privacy_policy_text": "Privacy policy"}},
	}}

	if err := WriteLocal(cfg, configPath, remote); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "metadata", "en-US", "privacy_policy.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("tvOS privacy policy text was not written: %v", err)
	}
	if got := string(data); got != "Privacy policy\n" {
		t.Fatalf("privacy policy text = %q", got)
	}
}

func TestWriteLocalNewCreatesNestedConfigurationDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "some", "new", "path", "ascdir.yaml")
	cfg := config.New("app-1", "com.example.app", "IOS", "1.0", []string{"en-US"})
	remote := appstore.Metadata{Localizations: map[string]appstore.Localization{"en-US": {Values: map[string]string{"name": "Example"}}}}
	if err := WriteLocalNew(cfg, configPath, remote); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("nested config was not created: %v", err)
	}
}

func TestWriteLocalRejectsUnsafeRemoteAssetFileName(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := config.New("app-1", "com.example.app", "IOS", "1.0", []string{"en-US"})
	cfg.Assets.Screenshots = "assets/screenshots"
	remote := appstore.Metadata{
		Localizations: map[string]appstore.Localization{"en-US": {}},
		Screenshots:   map[string]map[string][]appstore.Asset{"en-US": {"APP_IPHONE_67": {{FileName: "../../../../victim.png", Content: []byte("image")}}}},
	}
	if err := WriteLocal(cfg, filepath.Join(dir, "ascdir.yaml"), remote); err == nil || !strings.Contains(err.Error(), "safe file name") {
		t.Fatalf("error = %v", err)
	}
}

func TestWriteLocalRejectsPortableAssetNameCollisions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := config.New("app-1", "com.example.app", "IOS", "1.0", []string{"en-US"})
	cfg.Assets.Screenshots = "assets/screenshots"
	remote := appstore.Metadata{
		Localizations: map[string]appstore.Localization{"en-US": {}},
		Screenshots: map[string]map[string][]appstore.Asset{"en-US": {"APP_IPHONE_67": {
			{FileName: "é.png", Content: []byte("first")},
			{FileName: "e\u0301.png", Content: []byte("second")},
		}}},
	}
	if err := WriteLocal(cfg, filepath.Join(dir, "ascdir.yaml"), remote); err == nil || !strings.Contains(err.Error(), "collision") {
		t.Fatalf("error = %v", err)
	}
}

func TestWriteLocalRejectsWindowsReservedAssetNameWithExtraExtension(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := config.New("app-1", "com.example.app", "IOS", "1.0", []string{"en-US"})
	cfg.Assets.Screenshots = "assets/screenshots"
	remote := appstore.Metadata{
		Localizations: map[string]appstore.Localization{"en-US": {}},
		Screenshots:   map[string]map[string][]appstore.Asset{"en-US": {"APP_IPHONE_67": {{FileName: "CON.foo.png", Content: []byte("image")}}}},
	}
	if err := WriteLocal(cfg, filepath.Join(dir, "ascdir.yaml"), remote); err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("error = %v", err)
	}
}

func TestWriteLocalRejectsMetadataCollisionWithConfiguration(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "ascdir.yaml")
	contents := `version: "1"
app:
  id: app-1
  bundle_id: com.example.app
  platform: IOS
  version: 1.0
localizations:
  en-US:
    description: ascdir.yaml
`
	if err := os.WriteFile(configPath, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	remote := appstore.Metadata{Localizations: map[string]appstore.Localization{"en-US": {Values: map[string]string{"description": "replacement"}}}}
	if err := WriteLocal(cfg, configPath, remote); err == nil || !strings.Contains(err.Error(), "configuration") {
		t.Fatalf("error = %v", err)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != contents {
		t.Fatal("configuration was overwritten")
	}
}

func TestWriteLocalIgnoresAssetsForUnconfiguredLocale(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := config.New("app-1", "com.example.app", "IOS", "1.0", []string{"en-US"})
	cfg.Assets.Screenshots = "assets/screenshots"
	remote := appstore.Metadata{
		Localizations: map[string]appstore.Localization{"en-US": {}, "ja": {}},
		Screenshots:   map[string]map[string][]appstore.Asset{"ja": {"APP_IPHONE_67": {{FileName: "01.png", Content: []byte("image")}}}},
	}
	if err := WriteLocal(cfg, filepath.Join(dir, "ascdir.yaml"), remote); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "assets", "screenshots", "ja")); !os.IsNotExist(err) {
		t.Fatalf("unconfigured locale was written: %v", err)
	}
}

func TestWriteLocalIgnoresPreviewFrameTimesForUnconfiguredLocale(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "ascdir.yaml")
	cfg := config.New("app-1", "com.example.app", "IOS", "1.0", []string{"en-US"})
	cfg.Assets.AppPreviews = "assets/app-previews"
	remote := appstore.Metadata{
		Localizations: map[string]appstore.Localization{"en-US": {}, "ja": {}},
		AppPreviews: map[string]map[string][]appstore.Asset{"ja": {"IPHONE_67": {{
			FileName: "01.mp4", Content: []byte("video"), PreviewFrameTimeCode: "00:00:05",
		}}}},
	}
	if err := WriteLocal(cfg, configPath, remote); err != nil {
		t.Fatal(err)
	}
	updated, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Assets.PreviewFrameTimes) != 0 {
		t.Fatalf("unconfigured frame times = %#v", updated.Assets.PreviewFrameTimes)
	}
}

func TestWriteLocalRejectsDownloadedAssetChecksumMismatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := config.New("app-1", "com.example.app", "IOS", "1.0", []string{"en-US"})
	cfg.Assets.Screenshots = "assets/screenshots"
	remote := appstore.Metadata{
		Localizations: map[string]appstore.Localization{"en-US": {}},
		Screenshots:   map[string]map[string][]appstore.Asset{"en-US": {"APP_IPHONE_67": {{FileName: "01.png", Content: []byte("image"), Checksum: "00000000000000000000000000000000"}}}},
	}
	if err := WriteLocal(cfg, filepath.Join(dir, "ascdir.yaml"), remote); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("error = %v", err)
	}
}

func TestReadLocalRejectsInvalidUTF8(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "metadata", "en-US", "description.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte{0xff, 0xfe}, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.New("app-1", "com.example.app", "IOS", "1.0", []string{"en-US"})
	if _, err := ReadLocal(cfg, filepath.Join(dir, "ascdir.yaml")); err == nil || !strings.Contains(err.Error(), "UTF-8") {
		t.Fatalf("error = %v", err)
	}
}

func TestCommitFileTransactionRollsBackEarlierReplacements(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	first := filepath.Join(dir, "first.txt")
	if err := os.WriteFile(first, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	second := filepath.Join(dir, "second.txt")
	if err := os.Mkdir(second, 0o755); err != nil {
		t.Fatal(err)
	}
	firstPending, err := atomicfile.Prepare(first, []byte("replacement"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer firstPending.Cleanup()
	secondPending, err := atomicfile.Prepare(second, []byte("replacement"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer secondPending.Cleanup()
	if _, err := commitFileTransaction([]*atomicfile.Pending{firstPending, secondPending}, []string{first, second}, nil); err == nil {
		t.Fatal("transaction unexpectedly succeeded")
	}
	data, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "original" {
		t.Fatalf("first file was not rolled back: %q", data)
	}
}

func TestCommitFileTransactionReportsAndKeepsBackupWhenRestoreFails(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.txt")
	if err := os.WriteFile(first, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	second := filepath.Join(dir, "second.txt")
	if err := os.Mkdir(second, 0o755); err != nil {
		t.Fatal(err)
	}
	firstPending, err := atomicfile.Prepare(first, []byte("replacement"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer firstPending.Cleanup()
	secondPending, err := atomicfile.Prepare(second, []byte("replacement"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer secondPending.Cleanup()
	ops := transactionFileOps{
		remove: os.Remove,
		rename: func(source, destination string) error {
			if strings.Contains(filepath.Base(source), ".ascdir-backup-") {
				return errors.New("injected restore failure")
			}
			return os.Rename(source, destination)
		},
	}
	_, err = commitFileTransactionWithOps([]*atomicfile.Pending{firstPending, secondPending}, []string{first, second}, nil, ops)
	if err == nil || !strings.Contains(err.Error(), "restore") || !strings.Contains(err.Error(), "backup") {
		t.Fatalf("error = %v", err)
	}
	backups, err := filepath.Glob(filepath.Join(dir, ".ascdir-backup-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 {
		t.Fatalf("recoverable backups = %#v", backups)
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()
	values := appstore.Metadata{Localizations: map[string]appstore.Localization{
		"en-US": {Values: map[string]string{
			"name": strings.Repeat("a", 31), "description": "Description",
			"support_url": "not a URL", "marketing_url": "https://example.com",
		}},
	}}
	problems := Validate(values)
	if len(problems) != 2 {
		t.Fatalf("problems = %#v", problems)
	}
}

func TestValidateKeywordsUsesAppStoreByteLimit(t *testing.T) {
	t.Parallel()
	values := appstore.Metadata{Localizations: map[string]appstore.Localization{
		"ja": {Values: map[string]string{"keywords": strings.Repeat("あ", 34)}},
	}}
	problems := Validate(values)
	if len(problems) != 1 || !strings.Contains(problems[0], "102 bytes; maximum is 100") {
		t.Fatalf("problems = %#v", problems)
	}
}

func TestValidateRejectsShortNameAndControlCharacters(t *testing.T) {
	t.Parallel()
	values := appstore.Metadata{
		Values: map[string]string{"copyright": "2026\x00Example"},
		Localizations: map[string]appstore.Localization{"en-US": {Values: map[string]string{
			"name": "A", "description": "Description", "support_url": "https://example.com",
		}}},
	}
	problems := strings.Join(Validate(values), "\n")
	for _, want := range []string{"minimum is 2", "disallowed control character"} {
		if !strings.Contains(problems, want) {
			t.Fatalf("problems = %q, missing %q", problems, want)
		}
	}
}

func TestValidateGlobalMetadata(t *testing.T) {
	t.Parallel()
	values := appstore.Metadata{Values: map[string]string{
		"copyright": " ", "accessibility_url": "not a URL", "content_rights_declaration": "UNKNOWN",
	}, Localizations: map[string]appstore.Localization{}}
	problems := Validate(values)
	if len(problems) != 3 || !strings.Contains(strings.Join(problems, "\n"), "metadata.copyright") || !strings.Contains(strings.Join(problems, "\n"), "metadata.accessibility_url") || !strings.Contains(strings.Join(problems, "\n"), "metadata.content_rights_declaration") {
		t.Fatalf("problems = %#v", problems)
	}
}

func TestValidateCategories(t *testing.T) {
	t.Parallel()
	values := appstore.Metadata{Values: map[string]string{
		"primary_category":          "GAMES",
		"secondary_category":        "GAMES",
		"primary_subcategory_two":   "GAMES_ACTION",
		"secondary_subcategory_one": "GAMES_CASINO",
	}}
	problems := Validate(values)
	joined := strings.Join(problems, "\n")
	for _, expected := range []string{"must differ", "primary_subcategory_two requires"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("problems = %#v; missing %q", problems, expected)
		}
	}
	missingParent := Validate(appstore.Metadata{Values: map[string]string{"secondary_subcategory_one": "GAMES_CASINO"}})
	if !strings.Contains(strings.Join(missingParent, "\n"), "secondary subcategories require") {
		t.Fatalf("problems = %#v", missingParent)
	}
}

func TestValidateAgeRating(t *testing.T) {
	t.Parallel()
	values := appstore.Metadata{Values: map[string]string{
		"kids_age_band": "TODDLER", "violence_cartoon_or_fantasy": "OFTEN",
		"developer_age_rating_info_url": "not a URL",
	}}
	problems := Validate(values)
	joined := strings.Join(problems, "\n")
	for _, expected := range []string{"kids_age_band", "violence_cartoon_or_fantasy", "developer_age_rating_info_url"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("problems = %#v; missing %q", problems, expected)
		}
	}
}

func TestDiffAndPrintGlobalMetadata(t *testing.T) {
	t.Parallel()
	desired := appstore.Metadata{Values: map[string]string{"copyright": "2026 Example, Inc.", "primary_category": "BUSINESS"}}
	remote := appstore.Metadata{Values: map[string]string{"copyright": "2025 Example, Inc.", "primary_category": "PRODUCTIVITY"}}
	changes := Diff(desired, remote)
	if len(changes) != 2 || changes[0].Locale != "" || changes[0].Field != "copyright" {
		t.Fatalf("changes = %#v", changes)
	}
	var output bytes.Buffer
	PrintChanges(&output, changes)
	if !strings.Contains(output.String(), "metadata.copyright") || !strings.Contains(output.String(), "categories.primary_category") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestAccessibilityDiffSelectAndPrintUseTypedStructure(t *testing.T) {
	t.Parallel()
	voiceover, published := true, false
	cfg := config.New("app-1", "com.example.app", "IOS", "1.0.0", []string{"en-US"})
	cfg.Accessibility = map[string]config.AccessibilityValues{"IPHONE": {SupportsVoiceover: &voiceover, Published: &published}}
	desired := appstore.Metadata{Accessibility: map[string]appstore.AccessibilityDeclaration{"IPHONE": {Values: map[string]string{"supports_voiceover": "true", "published": "false"}}}}
	remote := appstore.Metadata{Accessibility: map[string]appstore.AccessibilityDeclaration{"IPHONE": {ID: "declaration-1", Values: map[string]string{"supports_voiceover": "false", "published": "false"}}}}
	changes := Diff(desired, remote)
	if len(changes) != 1 || changes[0].DeviceFamily != "IPHONE" || changes[0].Field != "supports_voiceover" {
		t.Fatalf("changes = %#v", changes)
	}
	selected := Select(cfg, remote)
	if selected.Accessibility["IPHONE"].ID != "declaration-1" || selected.Accessibility["IPHONE"].Values["supports_voiceover"] != "false" {
		t.Fatalf("selected = %#v", selected.Accessibility)
	}
	var output bytes.Buffer
	PrintChanges(&output, changes)
	if !strings.Contains(output.String(), "accessibility.IPHONE.supports_voiceover") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestClearingChanges(t *testing.T) {
	t.Parallel()
	changes := []appstore.Change{
		{Locale: "en-US", Field: "subtitle", Before: "Old", After: ""},
		{Locale: "en-US", Field: "name", Before: "Old", After: "New"},
		{Locale: "ja", Field: "subtitle", Before: "", After: ""},
	}
	clears := ClearingChanges(changes)
	if len(clears) != 1 || clears[0].Field != "subtitle" {
		t.Fatalf("unexpected clearing changes: %#v", clears)
	}
}

func TestSelectKeepsOnlyConfiguredFields(t *testing.T) {
	t.Parallel()
	cfg := config.New("app-1", "com.example.app", "IOS", "1.0.0", []string{"en-US"})
	localization := cfg.Localizations["en-US"]
	localization.Values.Subtitle = nil
	cfg.Localizations["en-US"] = localization
	source := appstore.Metadata{
		AppID: "app-1", AppInfoID: "info-1", VersionID: "version-1",
		Localizations: map[string]appstore.Localization{"en-US": {Values: map[string]string{"name": "Example", "subtitle": "Ignored"}}},
	}
	selected := Select(cfg, source)
	if selected.AppInfoID != "info-1" || selected.VersionID != "version-1" {
		t.Fatalf("IDs were not preserved: %#v", selected)
	}
	values := selected.Localizations["en-US"].Values
	if values["name"] != "Example" {
		t.Fatalf("name = %q", values["name"])
	}
	if _, ok := values["subtitle"]; ok {
		t.Fatalf("unmanaged subtitle was selected: %#v", values)
	}
}

func TestReadLocalRejectsSymlinkOutsideProject(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation commonly requires elevated privileges on Windows")
	}
	t.Parallel()
	project := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	metadataDirectory := filepath.Join(project, "metadata", "en-US")
	if err := os.MkdirAll(metadataDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(metadataDirectory, "description.md")); err != nil {
		t.Fatal(err)
	}
	cfg := config.New("app-1", "com.example.app", "IOS", "1.0.0", []string{"en-US"})
	localization := cfg.Localizations["en-US"]
	localization.Values = config.LocaleValues{}
	localization.Files.PromotionalText = ""
	localization.Files.WhatsNew = ""
	localization.Files.PrivacyPolicyText = ""
	cfg.Localizations["en-US"] = localization
	_, err := ReadLocal(cfg, filepath.Join(project, "ascdir.yaml"))
	if err == nil || !strings.Contains(err.Error(), "outside the configuration directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWriteLocalRejectsSymlinkedDirectoryOutsideProject(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation commonly requires elevated privileges on Windows")
	}
	t.Parallel()
	project := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(project, "metadata")); err != nil {
		t.Fatal(err)
	}
	cfg := config.New("app-1", "com.example.app", "IOS", "1.0.0", []string{"en-US"})
	remote := appstore.Metadata{Localizations: map[string]appstore.Localization{"en-US": {Values: map[string]string{"name": "Example"}}}}
	err := WriteLocal(cfg, filepath.Join(project, "ascdir.yaml"), remote)
	if err == nil || !strings.Contains(err.Error(), "outside the configuration directory") {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "en-US")); !os.IsNotExist(err) {
		t.Fatalf("write created a directory outside the project: %v", err)
	}
}

func TestWriteLocalValidatesAllPathsBeforeReplacingFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation commonly requires elevated privileges on Windows")
	}
	t.Parallel()
	project := t.TempDir()
	metadataDirectory := filepath.Join(project, "metadata")
	if err := os.MkdirAll(filepath.Join(metadataDirectory, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	descriptionPath := filepath.Join(metadataDirectory, "a", "description.md")
	if err := os.WriteFile(descriptionPath, []byte("original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(metadataDirectory, "z")); err != nil {
		t.Fatal(err)
	}
	cfg := config.New("app-1", "com.example.app", "IOS", "1.0.0", []string{"en-US"})
	localization := cfg.Localizations["en-US"]
	localization.Values = config.LocaleValues{}
	localization.Files.Description = "metadata/a/description.md"
	localization.Files.PromotionalText = "metadata/z/promotional_text.md"
	localization.Files.WhatsNew = ""
	localization.Files.PrivacyPolicyText = ""
	cfg.Localizations["en-US"] = localization
	remote := appstore.Metadata{Localizations: map[string]appstore.Localization{
		"en-US": {Values: map[string]string{"description": "replacement", "promotional_text": "replacement"}},
	}}
	if err := WriteLocal(cfg, filepath.Join(project, "ascdir.yaml"), remote); err == nil {
		t.Fatal("unsafe write succeeded")
	}
	data, err := os.ReadFile(descriptionPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "original\n" {
		t.Fatalf("description changed before path validation completed: %q", data)
	}
}
