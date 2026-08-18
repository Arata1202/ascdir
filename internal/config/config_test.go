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
	for _, expected := range []string{"version: \"2\"", "values:", "files:", "# App Store display name", "# Markdown file containing the long app description"} {
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
