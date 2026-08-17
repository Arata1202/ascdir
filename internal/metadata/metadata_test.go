package metadata

import (
	"bytes"
	"path/filepath"
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
	remote := appstore.Metadata{Localizations: map[string]appstore.Localization{
		"en-US": {Values: map[string]string{"name": "Example", "description": "Description", "support_url": "https://example.com/support"}},
	}}
	if err := WriteLocal(cfg, configPath, remote); err != nil {
		t.Fatal(err)
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
