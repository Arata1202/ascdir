package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Arata1202/ascdir/internal/appstore"
	"github.com/Arata1202/ascdir/internal/config"
	"github.com/Arata1202/ascdir/internal/metadata"
)

func TestVersionString(t *testing.T) {
	originalVersion, originalCommit, originalDate := version, commit, date
	t.Cleanup(func() { version, commit, date = originalVersion, originalCommit, originalDate })
	version, commit, date = "v1.0.0", "abc123", "2026-08-17"
	got := versionString()
	for _, want := range []string{"v1.0.0", "abc123", "2026-08-17"} {
		if !strings.Contains(got, want) {
			t.Fatalf("versionString() = %q, missing %q", got, want)
		}
	}
}

func TestRunCheck(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	configPath := filepath.Join(directory, "ascdir.yaml")
	cfg := config.New("app-1", "com.example.app", "IOS", "1.0.0", []string{"en-US"})
	files := cfg.Localizations["en-US"]
	files.Subtitle = ""
	files.Keywords = ""
	files.PromotionalText = ""
	files.WhatsNew = ""
	files.MarketingURL = ""
	files.PrivacyPolicyURL = ""
	files.PrivacyChoicesURL = ""
	files.PrivacyPolicyText = ""
	cfg.Localizations["en-US"] = files
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatal(err)
	}
	remote := appstore.Metadata{Localizations: map[string]appstore.Localization{
		"en-US": {Values: map[string]string{"name": "Example", "description": "Description", "support_url": "https://example.com/support"}},
	}}
	if err := metadata.WriteLocal(cfg, configPath, remote); err != nil {
		t.Fatal(err)
	}
	if err := runCheck([]string{"--config", configPath}); err != nil {
		t.Fatal(err)
	}
}

func TestRunCheckRejectsUnexpectedArgument(t *testing.T) {
	t.Parallel()
	if err := runCheck([]string{"extra"}); err == nil || !strings.Contains(err.Error(), "unexpected argument") {
		t.Fatalf("unexpected error: %v", err)
	}
}
