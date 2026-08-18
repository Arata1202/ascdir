package metadata

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Arata1202/ascdir/internal/appstore"
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
	remote := appstore.Metadata{Localizations: map[string]appstore.Localization{
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
