package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveAndLoad(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ascdir.yaml")
	want := New("123", "com.example.app", "IOS", "1.2.0", []string{"ja", "en-US"})
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.App != want.App || len(got.Localizations) != 2 {
		t.Fatalf("loaded config mismatch: %#v", got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"version: \"2\"", "metadata:", "copyright:", "accessibility_url:", "content_rights_declaration:", "categories:", "primary_category:", "secondary_category:", "values:", "files:", "# App Store display name", "# Markdown file containing the long app description"} {
		if !strings.Contains(string(data), expected) {
			t.Fatalf("generated config is missing %q:\n%s", expected, data)
		}
	}
}

func TestLoadVersionOneConfiguration(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ascdir.yaml")
	contents := `version: "1"
app:
  id: "123"
  bundle_id: com.example.app
  platform: IOS
  version: 1.0.0
localizations:
  en-US:
    name: metadata/en-US/name.txt
    subtitle: metadata/en-US/subtitle.txt
    description: metadata/en-US/description.md
    keywords: metadata/en-US/keywords.txt
    promotional_text: metadata/en-US/promotional_text.txt
    whats_new: metadata/en-US/whats_new.md
    support_url: metadata/en-US/support_url.txt
    marketing_url: metadata/en-US/marketing_url.txt
    privacy_policy_url: metadata/en-US/privacy_policy_url.txt
    privacy_choices_url: metadata/en-US/privacy_choices_url.txt
    privacy_policy_text: metadata/en-US/privacy_policy.md
`
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Version != "1" {
		t.Fatalf("version = %q", cfg.Version)
	}
	paths := cfg.Localizations["en-US"].Paths()
	if paths["name"] != "metadata/en-US/name.txt" || paths["description"] != "metadata/en-US/description.md" {
		t.Fatalf("legacy paths = %#v", paths)
	}
	if len(cfg.Localizations["en-US"].Values.Map()) != 0 {
		t.Fatalf("legacy values unexpectedly became inline: %#v", cfg.Localizations["en-US"].Values.Map())
	}
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(saved), "values:") || !strings.Contains(string(saved), "name: metadata/en-US/name.txt") {
		t.Fatalf("version 1 was not saved in its original schema:\n%s", saved)
	}
	roundTripped, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := roundTripped.Localizations["en-US"].Paths()["name"]; got != "metadata/en-US/name.txt" {
		t.Fatalf("round-tripped name path = %q", got)
	}
}

func TestLoadVersionTwoWithoutMetadataLeavesFieldsUnmanaged(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ascdir.yaml")
	contents := `version: "2"
app:
  bundle_id: com.example.app
  platform: IOS
  version: 1.0.0
localizations:
  en-US:
    values:
      name: Example
`
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Metadata.Map()) != 0 {
		t.Fatalf("metadata unexpectedly became managed: %#v", cfg.Metadata.Map())
	}
	if len(cfg.Categories.Map()) != 0 {
		t.Fatalf("categories unexpectedly became managed: %#v", cfg.Categories.Map())
	}
}

func TestEncodeUpdatedValuesPreservesCommentsAndKeyOrder(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ascdir.yaml")
	contents := `# project comment
version: "2"
app:
  bundle_id: com.example.app
  platform: IOS
  version: 1.0.0
metadata:
  copyright: 2025 Example, Inc. # custom copyright comment
localizations:
  en-US: # locale comment
    values:
      name: Before # custom name comment
    files:
      description: metadata/en-US/description.md
`
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	localization := cfg.Localizations["en-US"]
	localization.Values.SetManaged("name", "After")
	cfg.Localizations["en-US"] = localization
	cfg.Metadata.SetManaged("copyright", "2026 Example, Inc.")
	updated, err := EncodeUpdatedValues(path, cfg)
	if err != nil {
		t.Fatal(err)
	}
	text := string(updated)
	for _, expected := range []string{"# project comment", "# locale comment", "copyright: 2026 Example, Inc. # custom copyright comment", "name: After # custom name comment"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("updated config is missing %q:\n%s", expected, text)
		}
	}
	if strings.Contains(text, "subtitle:") {
		t.Fatalf("update added an unmanaged field:\n%s", text)
	}
	if strings.Index(text, "values:") > strings.Index(text, "files:") {
		t.Fatalf("update changed key order:\n%s", text)
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ascdir.yaml")
	contents := "version: \"1\"\napp:\n  bundle_id: com.example.app\n  platform: IOS\n  version: 1.0.0\n  typo: value\nlocalizations:\n  en-US:\n    description: metadata/en-US/description.md\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "field typo not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateRejectsEscapingAndDuplicatePaths(t *testing.T) {
	t.Parallel()
	cfg := New("123", "com.example.app", "IOS", "1.2.0", []string{"en-US"})
	localization := cfg.Localizations["en-US"]
	localization.Files.Description = "../description.md"
	cfg.Localizations["en-US"] = localization
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "relative path") {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg = New("123", "com.example.app", "IOS", "1.2.0", []string{"en-US"})
	localization = cfg.Localizations["en-US"]
	localization.Files.WhatsNew = localization.Files.Description
	cfg.Localizations["en-US"] = localization
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "reuses path") {
		t.Fatalf("unexpected error: %v", err)
	}
}
