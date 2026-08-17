package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Arata1202/ascdir/internal/appstore"
	"github.com/Arata1202/ascdir/internal/authconfig"
	"github.com/Arata1202/ascdir/internal/config"
	"github.com/Arata1202/ascdir/internal/metadata"
)

type mockStoreClient struct {
	checkAuth     func(context.Context) error
	fetchMetadata func(context.Context, string, string, string, string) (appstore.Metadata, error)
	applyMetadata func(context.Context, appstore.Metadata, []string, []appstore.Change) error
}

func (m mockStoreClient) CheckAuth(ctx context.Context) error {
	return m.checkAuth(ctx)
}

func (m mockStoreClient) FetchMetadata(ctx context.Context, appID, bundleID, platform, version string) (appstore.Metadata, error) {
	return m.fetchMetadata(ctx, appID, bundleID, platform, version)
}

func (m mockStoreClient) ApplyMetadata(ctx context.Context, remote appstore.Metadata, locales []string, changes []appstore.Change) error {
	return m.applyMetadata(ctx, remote, locales, changes)
}

func testEnvironment(client storeClient) (commandEnvironment, *bytes.Buffer, *bytes.Buffer) {
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	return commandEnvironment{
		stdin:  strings.NewReader(""),
		stdout: stdout,
		stderr: stderr,
		newClient: func() (storeClient, error) {
			if client == nil {
				return nil, errors.New("client should not be created")
			}
			return client, nil
		},
	}, stdout, stderr
}

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

func TestRunDispatchAndAuth(t *testing.T) {
	t.Parallel()
	client := mockStoreClient{checkAuth: func(context.Context) error { return nil }}
	environment, stdout, _ := testEnvironment(client)
	for _, args := range [][]string{nil, {"help"}, {"version"}, {"auth", "check"}} {
		stdout.Reset()
		if err := runWithEnvironment(context.Background(), args, environment); err != nil {
			t.Fatalf("run(%v): %v", args, err)
		}
		if stdout.Len() == 0 {
			t.Fatalf("run(%v) produced no output", args)
		}
	}
	if err := runWithEnvironment(context.Background(), []string{"unknown"}, environment); err == nil {
		t.Fatal("unknown command succeeded")
	}
	if err := runWithEnvironment(context.Background(), []string{"auth"}, environment); err == nil {
		t.Fatal("invalid auth command succeeded")
	}
}

func TestRunAuthLogin(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(t.TempDir(), "AuthKey_TEST.p8")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded}), 0o600); err != nil {
		t.Fatal(err)
	}
	configHome := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("AppData", configHome)
	} else {
		t.Setenv("XDG_CONFIG_HOME", configHome)
	}
	environment, stdout, _ := testEnvironment(nil)
	environment.stdin = strings.NewReader("issuer\nkey-id\n" + keyPath + "\n")
	if err := runWithEnvironment(context.Background(), []string{"auth", "login"}, environment); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "Credentials saved") {
		t.Fatalf("unexpected output: %q", stdout.String())
	}
	stored, err := authconfig.Load()
	if err != nil {
		t.Fatal(err)
	}
	if stored.IssuerID != "issuer" || stored.KeyID != "key-id" || stored.PrivateKeyPath != keyPath {
		t.Fatalf("stored config = %#v", stored)
	}
}

func TestRunInitPullAndPush(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	configPath := filepath.Join(directory, "ascdir.yaml")
	remote := appstore.Metadata{
		AppID: "app-1", AppInfoID: "info-1", VersionID: "version-1",
		Localizations: map[string]appstore.Localization{
			"en-US": {Values: map[string]string{
				"name": "Example", "description": "Description", "support_url": "https://example.com/support",
			}},
		},
	}
	var applied []appstore.Change
	client := mockStoreClient{
		fetchMetadata: func(_ context.Context, _, bundleID, platform, version string) (appstore.Metadata, error) {
			if bundleID != "com.example.app" || platform != "IOS" || version != "1.0.0" {
				t.Fatalf("unexpected lookup: %q %q %q", bundleID, platform, version)
			}
			return remote, nil
		},
		applyMetadata: func(_ context.Context, _ appstore.Metadata, _ []string, changes []appstore.Change) error {
			applied = append(applied, changes...)
			return nil
		},
	}
	environment, stdout, _ := testEnvironment(client)
	if err := runWithEnvironment(context.Background(), []string{
		"init", "--bundle-id", "com.example.app", "--version", "1.0.0", "--config", configPath,
	}, environment); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "Created") {
		t.Fatalf("output = %q", stdout.String())
	}
	if err := runWithEnvironment(context.Background(), []string{
		"init", "--bundle-id", "com.example.app", "--version", "1.0.0", "--config", configPath,
	}, environment); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("unexpected duplicate init error: %v", err)
	}

	namePath := filepath.Join(directory, "metadata", "en-US", "name.txt")
	if err := os.WriteFile(namePath, []byte("Changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	if err := runWithEnvironment(context.Background(), []string{"push", "--config", configPath, "--dry-run"}, environment); err != nil {
		t.Fatal(err)
	}
	if len(applied) != 0 || !strings.Contains(stdout.String(), "Dry run") {
		t.Fatalf("applied=%v output=%q", applied, stdout.String())
	}
	if err := runWithEnvironment(context.Background(), []string{"push", "--config", configPath}, environment); err != nil {
		t.Fatal(err)
	}
	if len(applied) != 1 || applied[0].Field != "name" {
		t.Fatalf("applied = %#v", applied)
	}

	stdout.Reset()
	if err := runWithEnvironment(context.Background(), []string{"pull", "--config", configPath, "--dry-run"}, environment); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "Dry run") {
		t.Fatalf("output = %q", stdout.String())
	}
	if err := runWithEnvironment(context.Background(), []string{"pull", "--config", configPath}, environment); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(namePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "Example\n" {
		t.Fatalf("name = %q", data)
	}
}

func TestRunPushRequiresExplicitPermissionToClear(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	configPath := filepath.Join(directory, "ascdir.yaml")
	cfg := config.New("app-1", "com.example.app", "IOS", "1.0.0", []string{"en-US"})
	files := cfg.Localizations["en-US"]
	files.Name, files.Description, files.Keywords = "", "", ""
	files.PromotionalText, files.WhatsNew, files.SupportURL = "", "", ""
	files.MarketingURL, files.PrivacyPolicyURL = "", ""
	files.PrivacyChoicesURL, files.PrivacyPolicyText = "", ""
	cfg.Localizations["en-US"] = files
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatal(err)
	}
	remote := appstore.Metadata{
		AppID: "app-1", AppInfoID: "info-1", VersionID: "version-1",
		Localizations: map[string]appstore.Localization{"en-US": {
			AppInfoLocalizationID: "info-loc-1", VersionLocalizationID: "version-loc-1",
			Values: map[string]string{"subtitle": "Existing"},
		}},
	}
	if err := metadata.WriteLocal(cfg, configPath, appstore.Metadata{Localizations: map[string]appstore.Localization{
		"en-US": {Values: map[string]string{"subtitle": ""}},
	}}); err != nil {
		t.Fatal(err)
	}
	var applyCalls int
	client := mockStoreClient{
		fetchMetadata: func(context.Context, string, string, string, string) (appstore.Metadata, error) { return remote, nil },
		applyMetadata: func(context.Context, appstore.Metadata, []string, []appstore.Change) error { applyCalls++; return nil },
	}
	environment, _, _ := testEnvironment(client)
	args := []string{"push", "--config", configPath}
	if err := runWithEnvironment(context.Background(), args, environment); err == nil || !strings.Contains(err.Error(), "--allow-empty") {
		t.Fatalf("unexpected error: %v", err)
	}
	if applyCalls != 0 {
		t.Fatalf("apply calls = %d", applyCalls)
	}
	if err := runWithEnvironment(context.Background(), append(args, "--allow-empty"), environment); err != nil {
		t.Fatal(err)
	}
	if applyCalls != 1 {
		t.Fatalf("apply calls = %d", applyCalls)
	}
}

func TestRunInitReportsUnexpectedStatError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation commonly requires elevated privileges on Windows")
	}
	t.Parallel()
	directory := t.TempDir()
	configPath := filepath.Join(directory, "loop")
	if err := os.Symlink("loop", configPath); err != nil {
		t.Fatal(err)
	}
	environment, _, _ := testEnvironment(nil)
	err := runWithEnvironment(context.Background(), []string{
		"init", "--bundle-id", "com.example.app", "--version", "1.0.0", "--config", configPath,
	}, environment)
	if err == nil || !strings.Contains(err.Error(), "check existing configuration") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunInitValidatesOptionsBeforeCreatingClient(t *testing.T) {
	t.Parallel()
	environment, _, _ := testEnvironment(nil)
	err := runWithEnvironment(context.Background(), []string{
		"init", "--bundle-id", "com.example.app", "--version", "1.0.0", "--platform", "invalid",
	}, environment)
	if err == nil || !strings.Contains(err.Error(), "invalid init options") {
		t.Fatalf("unexpected error: %v", err)
	}
}
