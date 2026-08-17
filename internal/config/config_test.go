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
	files := cfg.Localizations["en-US"]
	files.Name = "../name.txt"
	cfg.Localizations["en-US"] = files
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "relative path") {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg = New("123", "com.example.app", "IOS", "1.2.0", []string{"en-US"})
	files = cfg.Localizations["en-US"]
	files.Subtitle = files.Name
	cfg.Localizations["en-US"] = files
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "reuses path") {
		t.Fatalf("unexpected error: %v", err)
	}
}
