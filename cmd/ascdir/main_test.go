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
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/Arata1202/ascdir/internal/appstore"
	"github.com/Arata1202/ascdir/internal/authconfig"
	"github.com/Arata1202/ascdir/internal/config"
	"github.com/Arata1202/ascdir/internal/metadata"
)

type mockStoreClient struct {
	checkAuth                   func(context.Context) error
	fetchMetadata               func(context.Context, string, string, string, string, appstore.FetchOptions) (appstore.Metadata, error)
	applyMetadata               func(context.Context, appstore.Metadata, []string, []appstore.Change) error
	listPricePoints             func(context.Context, string, string) ([]appstore.PricePoint, error)
	resolveAppID                func(context.Context, string, string) (string, error)
	fetchAppStoreStatus         func(context.Context, string, string, string, string) (appstore.AppStoreStatus, error)
	fetchTestFlightStatus       func(context.Context, string, string, string, string) (appstore.TestFlightStatus, error)
	planAppStoreSubmission      func(context.Context, string, string, string, string, appstore.SubmitOptions) (appstore.AppStoreReleasePlan, error)
	applyAppStoreSubmission     func(context.Context, appstore.AppStoreReleasePlan) error
	planAppStoreRelease         func(context.Context, string, string, string, string) (appstore.AppStoreReleasePlan, error)
	applyAppStoreRelease        func(context.Context, appstore.AppStoreReleasePlan) error
	planTestFlightDistribution  func(context.Context, string, string, string, string, appstore.TestFlightDistributionOptions) (appstore.TestFlightDistributionPlan, error)
	applyTestFlightDistribution func(context.Context, appstore.TestFlightDistributionPlan) error
}

func (m mockStoreClient) PlanTestFlightDistribution(ctx context.Context, appID, bundleID, platform, version string, options appstore.TestFlightDistributionOptions) (appstore.TestFlightDistributionPlan, error) {
	if m.planTestFlightDistribution == nil {
		return appstore.TestFlightDistributionPlan{}, errors.New("PlanTestFlightDistribution should not be called")
	}
	return m.planTestFlightDistribution(ctx, appID, bundleID, platform, version, options)
}

func (m mockStoreClient) ApplyTestFlightDistribution(ctx context.Context, plan appstore.TestFlightDistributionPlan) error {
	if m.applyTestFlightDistribution == nil {
		return errors.New("ApplyTestFlightDistribution should not be called")
	}
	return m.applyTestFlightDistribution(ctx, plan)
}

func (m mockStoreClient) PlanAppStoreSubmission(ctx context.Context, appID, bundleID, platform, version string, options appstore.SubmitOptions) (appstore.AppStoreReleasePlan, error) {
	if m.planAppStoreSubmission == nil {
		return appstore.AppStoreReleasePlan{}, errors.New("PlanAppStoreSubmission should not be called")
	}
	return m.planAppStoreSubmission(ctx, appID, bundleID, platform, version, options)
}

func (m mockStoreClient) ApplyAppStoreSubmission(ctx context.Context, plan appstore.AppStoreReleasePlan) error {
	if m.applyAppStoreSubmission == nil {
		return errors.New("ApplyAppStoreSubmission should not be called")
	}
	return m.applyAppStoreSubmission(ctx, plan)
}

func (m mockStoreClient) PlanAppStoreRelease(ctx context.Context, appID, bundleID, platform, version string) (appstore.AppStoreReleasePlan, error) {
	if m.planAppStoreRelease == nil {
		return appstore.AppStoreReleasePlan{}, errors.New("PlanAppStoreRelease should not be called")
	}
	return m.planAppStoreRelease(ctx, appID, bundleID, platform, version)
}

func (m mockStoreClient) ApplyAppStoreRelease(ctx context.Context, plan appstore.AppStoreReleasePlan) error {
	if m.applyAppStoreRelease == nil {
		return errors.New("ApplyAppStoreRelease should not be called")
	}
	return m.applyAppStoreRelease(ctx, plan)
}

func (m mockStoreClient) FetchAppStoreStatus(ctx context.Context, appID, bundleID, platform, version string) (appstore.AppStoreStatus, error) {
	if m.fetchAppStoreStatus == nil {
		return appstore.AppStoreStatus{}, errors.New("FetchAppStoreStatus should not be called")
	}
	return m.fetchAppStoreStatus(ctx, appID, bundleID, platform, version)
}

func (m mockStoreClient) FetchTestFlightStatus(ctx context.Context, appID, bundleID, platform, version string) (appstore.TestFlightStatus, error) {
	if m.fetchTestFlightStatus == nil {
		return appstore.TestFlightStatus{}, errors.New("FetchTestFlightStatus should not be called")
	}
	return m.fetchTestFlightStatus(ctx, appID, bundleID, platform, version)
}

func (m mockStoreClient) ResolveAppID(ctx context.Context, appID, bundleID string) (string, error) {
	if m.resolveAppID != nil {
		return m.resolveAppID(ctx, appID, bundleID)
	}
	if appID != "" {
		return appID, nil
	}
	return "", errors.New("ResolveAppID should not be called")
}

func (m mockStoreClient) ListPricePoints(ctx context.Context, appID, territory string) ([]appstore.PricePoint, error) {
	if m.listPricePoints == nil {
		return nil, errors.New("ListPricePoints should not be called")
	}
	return m.listPricePoints(ctx, appID, territory)
}

func (m mockStoreClient) CheckAuth(ctx context.Context) error {
	return m.checkAuth(ctx)
}

func (m mockStoreClient) FetchMetadata(ctx context.Context, appID, bundleID, platform, version string, options appstore.FetchOptions) (appstore.Metadata, error) {
	return m.fetchMetadata(ctx, appID, bundleID, platform, version, options)
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

func TestRunAppStoreStatusJSON(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ascdir.yaml")
	if err := config.Save(path, config.New("app-1", "com.example.app", "IOS", "1.2.0", []string{"en-US"})); err != nil {
		t.Fatal(err)
	}
	client := mockStoreClient{fetchAppStoreStatus: func(_ context.Context, appID, bundleID, platform, version string) (appstore.AppStoreStatus, error) {
		if appID != "app-1" || bundleID != "com.example.app" || platform != "IOS" || version != "1.2.0" {
			t.Fatalf("unexpected request: %s %s %s %s", appID, bundleID, platform, version)
		}
		return appstore.AppStoreStatus{AppID: appID, VersionID: "version-1", Platform: platform, Version: version, AppVersionState: "READY_FOR_REVIEW"}, nil
	}}
	environment, stdout, _ := testEnvironment(client)
	if err := runWithEnvironment(context.Background(), []string{"app-store", "status", "--config", path, "--json"}, environment); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"version": "1.2.0"`, `"app_version_state": "READY_FOR_REVIEW"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("output = %q, missing %q", stdout.String(), want)
		}
	}
}

func TestRunTestFlightStatusHumanReadable(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ascdir.yaml")
	if err := config.Save(path, config.New("app-1", "com.example.app", "IOS", "1.2.0", []string{"en-US"})); err != nil {
		t.Fatal(err)
	}
	client := mockStoreClient{fetchTestFlightStatus: func(_ context.Context, _, _, _, _ string) (appstore.TestFlightStatus, error) {
		return appstore.TestFlightStatus{AppID: "app-1", Builds: []appstore.BuildStatus{{ID: "build-1", Version: "42", ProcessingState: "VALID"}}}, nil
	}}
	environment, stdout, _ := testEnvironment(client)
	if err := runWithEnvironment(context.Background(), []string{"testflight", "status", "--config", path}, environment); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"TestFlight IOS 1.2.0", "- 42: VALID"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("output = %q, missing %q", stdout.String(), want)
		}
	}
}

func TestRunTestFlightDistributeDryRunPassesRepeatedGroups(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ascdir.yaml")
	if err := config.Save(path, config.New("app-1", "com.example.app", "IOS", "1.2.0", []string{"en-US"})); err != nil {
		t.Fatal(err)
	}
	client := mockStoreClient{planTestFlightDistribution: func(_ context.Context, _, _, _, _ string, options appstore.TestFlightDistributionOptions) (appstore.TestFlightDistributionPlan, error) {
		if options.BuildVersion != "42" || !slices.Equal(options.GroupNames, []string{"Team", "Public"}) {
			t.Fatalf("options = %#v", options)
		}
		return appstore.TestFlightDistributionPlan{Kind: "testflight-distribute", Platform: "IOS", Version: "1.2.0", Build: "42", Groups: []appstore.TestFlightGroup{{Name: "Public"}, {Name: "Team", Internal: true}}, Operations: []appstore.ReleaseOperation{{Action: "attach", Resource: "beta-group:Public", Description: "Attach build 42 to beta group Public"}}}, nil
	}}
	environment, stdout, _ := testEnvironment(client)
	err := runWithEnvironment(context.Background(), []string{"testflight", "distribute", "--config", path, "--build", "42", "--group", "Team", "--group", "Public", "--dry-run"}, environment)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "Public (external, pending)") || !strings.Contains(stdout.String(), "Dry run") {
		t.Fatalf("output = %q", stdout.String())
	}
}

func TestRunTestFlightDistributeRequiresVersionConfirmation(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ascdir.yaml")
	if err := config.Save(path, config.New("app-1", "com.example.app", "IOS", "1.2.0", []string{"en-US"})); err != nil {
		t.Fatal(err)
	}
	applied := 0
	client := mockStoreClient{
		planTestFlightDistribution: func(_ context.Context, _, _, _, _ string, _ appstore.TestFlightDistributionOptions) (appstore.TestFlightDistributionPlan, error) {
			return appstore.TestFlightDistributionPlan{Kind: "testflight-distribute", Platform: "IOS", Version: "1.2.0", Build: "42", Groups: []appstore.TestFlightGroup{{Name: "Team", Internal: true}}, Operations: []appstore.ReleaseOperation{{Action: "attach", Resource: "beta-group:Team"}}}, nil
		},
		applyTestFlightDistribution: func(_ context.Context, _ appstore.TestFlightDistributionPlan) error { applied++; return nil },
	}
	environment, _, _ := testEnvironment(client)
	base := []string{"testflight", "distribute", "--config", path, "--group", "Team"}
	if err := runWithEnvironment(context.Background(), base, environment); err == nil || !strings.Contains(err.Error(), "--confirm 1.2.0") {
		t.Fatalf("error = %v", err)
	}
	if applied != 0 {
		t.Fatal("distribution applied without confirmation")
	}
	if err := runWithEnvironment(context.Background(), append(base, "--confirm", "1.2.0"), environment); err != nil {
		t.Fatal(err)
	}
	if applied != 1 {
		t.Fatalf("apply calls = %d", applied)
	}
}

func TestRunAppStoreSubmitDryRunUsesExecutionPlanWithoutApplying(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ascdir.yaml")
	if err := config.Save(path, config.New("app-1", "com.example.app", "IOS", "1.2.0", []string{"en-US"})); err != nil {
		t.Fatal(err)
	}
	client := mockStoreClient{planAppStoreSubmission: func(_ context.Context, _, _, _, _ string, options appstore.SubmitOptions) (appstore.AppStoreReleasePlan, error) {
		if options.BuildVersion != "42" || options.ReleaseType != "" {
			t.Fatalf("options = %#v", options)
		}
		return appstore.AppStoreReleasePlan{Kind: "app-store-submit", Platform: "IOS", Version: "1.2.0", Build: "42", Operations: []appstore.ReleaseOperation{{Action: "submit", Resource: "review-submission", Description: "Submit for App Review"}}}, nil
	}}
	environment, stdout, _ := testEnvironment(client)
	err := runWithEnvironment(context.Background(), []string{"app-store", "submit", "--config", path, "--build", "42", "--dry-run"}, environment)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "Submit for App Review") || !strings.Contains(stdout.String(), "Dry run") {
		t.Fatalf("output = %q", stdout.String())
	}
}

func TestRunAppStoreSubmitRequiresVersionBoundConfirmation(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ascdir.yaml")
	if err := config.Save(path, config.New("app-1", "com.example.app", "IOS", "1.2.0", []string{"en-US"})); err != nil {
		t.Fatal(err)
	}
	applied := 0
	client := mockStoreClient{
		planAppStoreSubmission: func(_ context.Context, _, _, _, _ string, _ appstore.SubmitOptions) (appstore.AppStoreReleasePlan, error) {
			return appstore.AppStoreReleasePlan{Kind: "app-store-submit", Platform: "IOS", Version: "1.2.0", Build: "42", Operations: []appstore.ReleaseOperation{{Action: "submit", Resource: "review-submission", Description: "Submit for App Review"}}}, nil
		},
		applyAppStoreSubmission: func(_ context.Context, _ appstore.AppStoreReleasePlan) error { applied++; return nil },
	}
	environment, _, _ := testEnvironment(client)
	base := []string{"app-store", "submit", "--config", path}
	if err := runWithEnvironment(context.Background(), base, environment); err == nil || !strings.Contains(err.Error(), "--confirm 1.2.0") {
		t.Fatalf("error = %v", err)
	}
	if err := runWithEnvironment(context.Background(), append(base, "--confirm", "wrong"), environment); err == nil {
		t.Fatal("wrong confirmation succeeded")
	}
	if applied != 0 {
		t.Fatalf("applied before confirmation: %d", applied)
	}
	if err := runWithEnvironment(context.Background(), append(base, "--confirm", "1.2.0"), environment); err != nil {
		t.Fatal(err)
	}
	if applied != 1 {
		t.Fatalf("apply calls = %d", applied)
	}
}

func TestRunAppStoreReleaseRequiresConfirmation(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ascdir.yaml")
	if err := config.Save(path, config.New("app-1", "com.example.app", "IOS", "1.2.0", []string{"en-US"})); err != nil {
		t.Fatal(err)
	}
	applied := 0
	client := mockStoreClient{
		planAppStoreRelease: func(_ context.Context, _, _, _, _ string) (appstore.AppStoreReleasePlan, error) {
			return appstore.AppStoreReleasePlan{Kind: "app-store-release", Platform: "IOS", Version: "1.2.0", Operations: []appstore.ReleaseOperation{{Action: "release", Resource: "app-store-version", Description: "Release IOS 1.2.0"}}}, nil
		},
		applyAppStoreRelease: func(_ context.Context, _ appstore.AppStoreReleasePlan) error { applied++; return nil },
	}
	environment, _, _ := testEnvironment(client)
	base := []string{"app-store", "release", "--config", path}
	if err := runWithEnvironment(context.Background(), append(base, "--dry-run"), environment); err != nil {
		t.Fatal(err)
	}
	if applied != 0 {
		t.Fatal("dry-run applied release")
	}
	if err := runWithEnvironment(context.Background(), append(base, "--confirm", "1.2.0"), environment); err != nil {
		t.Fatal(err)
	}
	if applied != 1 {
		t.Fatalf("apply calls = %d", applied)
	}
}

func TestRunPricePoints(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ascdir.yaml")
	cfg := config.New("app-1", "com.example.app", "IOS", "1.0.0", []string{"en-US"})
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	client := mockStoreClient{listPricePoints: func(_ context.Context, appID, territory string) ([]appstore.PricePoint, error) {
		if appID != "app-1" || territory != "USA" {
			t.Fatalf("unexpected lookup: %s %s", appID, territory)
		}
		return []appstore.PricePoint{{ID: "point-1", CustomerPrice: "0.99", Proceeds: "0.70"}}, nil
	}}
	environment, stdout, _ := testEnvironment(client)
	if err := runWithEnvironment(context.Background(), []string{"price-points", "--config", path, "--territory", "USA"}, environment); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"CUSTOMER_PRICE", "0.99", "point-1"} {
		if !strings.Contains(stdout.String(), expected) {
			t.Fatalf("output %q does not contain %q", stdout.String(), expected)
		}
	}
}

func TestRunPricePointsResolvesBundleIDOnlyConfig(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ascdir.yaml")
	cfg := config.New("", "com.example.app", "IOS", "1.0.0", []string{"en-US"})
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	client := mockStoreClient{
		resolveAppID: func(_ context.Context, appID, bundleID string) (string, error) {
			if appID != "" || bundleID != "com.example.app" {
				t.Fatalf("identity = %q %q", appID, bundleID)
			}
			return "resolved-app", nil
		},
		listPricePoints: func(_ context.Context, appID, _ string) ([]appstore.PricePoint, error) {
			if appID != "resolved-app" {
				t.Fatalf("app ID = %q", appID)
			}
			return nil, nil
		},
	}
	environment, _, _ := testEnvironment(client)
	if err := runWithEnvironment(context.Background(), []string{"price-points", "--config", path, "--territory", "USA"}, environment); err != nil {
		t.Fatal(err)
	}
}

func TestRunCheck(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	configPath := filepath.Join(directory, "ascdir.yaml")
	cfg := config.New("app-1", "com.example.app", "IOS", "1.0.0", []string{"en-US"})
	cfg.Metadata = config.MetadataValues{}
	cfg.Categories = config.CategoryValues{}
	localization := cfg.Localizations["en-US"]
	localization.Values.Subtitle = nil
	localization.Values.Keywords = nil
	localization.Values.MarketingURL = nil
	localization.Values.PrivacyPolicyURL = nil
	localization.Values.PrivacyChoicesURL = nil
	localization.Files.PromotionalText = ""
	localization.Files.WhatsNew = ""
	localization.Files.PrivacyPolicyText = ""
	cfg.Localizations["en-US"] = localization
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

func TestFetchOptionsFollowManagedSections(t *testing.T) {
	t.Parallel()
	cfg := config.New("app-1", "com.example.app", "IOS", "1.0.0", []string{"en-US"})
	if fetchOptions(cfg).AgeRating {
		t.Fatal("unmanaged age rating was requested")
	}
	value := false
	cfg.AgeRating.Advertising = &value
	if !fetchOptions(cfg).AgeRating {
		t.Fatal("managed age rating was not requested")
	}
	if fetchOptions(cfg).Accessibility {
		t.Fatal("unmanaged accessibility declarations were requested")
	}
	accessible := true
	cfg.Accessibility = map[string]config.AccessibilityValues{"IPHONE": {SupportsVoiceover: &accessible}}
	if !fetchOptions(cfg).Accessibility {
		t.Fatal("managed accessibility declarations were not requested")
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
	isolateUserConfigDir(t, configHome)
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

func TestRunAuthLogout(t *testing.T) {
	configHome := t.TempDir()
	isolateUserConfigDir(t, configHome)
	keyPath := filepath.Join(t.TempDir(), "AuthKey_TEST.p8")
	if err := os.WriteFile(keyPath, []byte("key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := authconfig.Save(authconfig.Config{IssuerID: "issuer", KeyID: "key", PrivateKeyPath: keyPath}); err != nil {
		t.Fatal(err)
	}
	environment, stdout, _ := testEnvironment(nil)
	if err := runWithEnvironment(context.Background(), []string{"auth", "logout"}, environment); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "Removed stored credentials") {
		t.Fatalf("unexpected output: %q", stdout.String())
	}
}

func isolateUserConfigDir(t *testing.T, directory string) {
	t.Helper()
	t.Setenv("HOME", directory)
	t.Setenv("XDG_CONFIG_HOME", directory)
	t.Setenv("AppData", directory)
}

func TestRunInitPullAndPush(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	configPath := filepath.Join(directory, "ascdir.yaml")
	remote := appstore.Metadata{
		AppID: "app-1", AppInfoID: "info-1", VersionID: "version-1",
		Values: map[string]string{"copyright": "2026 Example, Inc.", "accessibility_url": "https://example.com/accessibility", "content_rights_declaration": "DOES_NOT_USE_THIRD_PARTY_CONTENT", "primary_category": "PRODUCTIVITY"},
		Localizations: map[string]appstore.Localization{
			"en-US": {Values: map[string]string{
				"name": "Example", "description": "Description", "support_url": "https://example.com/support",
			}},
		},
	}
	var applied []appstore.Change
	client := mockStoreClient{
		fetchMetadata: func(_ context.Context, appID, bundleID, platform, version string, options appstore.FetchOptions) (appstore.Metadata, error) {
			if bundleID != "com.example.app" || platform != "IOS" || version != "1.0.0" {
				t.Fatalf("unexpected lookup: %q %q %q", bundleID, platform, version)
			}
			if appID == "" && !options.AgeRating {
				t.Fatal("init did not request age rating metadata")
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
	for _, expected := range []string{"Next steps:", "empty managed values", "categories:", "localizations.en-US.values:", "files:", "ascdir check --config", "ascdir push --config", "/blob/main/examples/ascdir.minimal.yaml"} {
		if !strings.Contains(stdout.String(), expected) {
			t.Fatalf("init output is missing %q: %q", expected, stdout.String())
		}
	}
	if err := runWithEnvironment(context.Background(), []string{
		"init", "--bundle-id", "com.example.app", "--version", "1.0.0", "--config", configPath,
	}, environment); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("unexpected duplicate init error: %v", err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	localization := cfg.Localizations["en-US"]
	changed := "Changed"
	localization.Values.Name = &changed
	cfg.Localizations["en-US"] = localization
	if err := config.Save(configPath, cfg); err != nil {
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
	cfg, err = config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := *cfg.Localizations["en-US"].Values.Name; got != "Example" {
		t.Fatalf("name = %q", got)
	}
}

func TestPrintInitNextStepsQuotesConfigPathForShell(t *testing.T) {
	var output bytes.Buffer
	printInitNextSteps(&output, "configs/release candidate's ascdir.yaml", nil)
	want := `--config 'configs/release candidate'"'"'s ascdir.yaml'`
	if count := strings.Count(output.String(), want); count != 2 {
		t.Fatalf("quoted config path count = %d, want 2; output = %q", count, output.String())
	}
}

func TestRunInitForceReplacesVersionOneConfiguration(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	configPath := filepath.Join(directory, "ascdir.yaml")
	legacy := `version: "1"
app:
  bundle_id: com.example.app
  platform: IOS
  version: 0.9.0
localizations:
  en-US:
    description: metadata/en-US/description.md
`
	if err := os.WriteFile(configPath, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	remote := appstore.Metadata{AppID: "app-1", Values: map[string]string{"copyright": "2026 Example, Inc."}, Localizations: map[string]appstore.Localization{
		"en-US": {Values: map[string]string{"name": "Example", "description": "Description"}},
	}}
	client := mockStoreClient{
		fetchMetadata: func(context.Context, string, string, string, string, appstore.FetchOptions) (appstore.Metadata, error) {
			return remote, nil
		},
	}
	environment, _, _ := testEnvironment(client)
	if err := runWithEnvironment(context.Background(), []string{
		"init", "--force", "--bundle-id", "com.example.app", "--version", "1.0.0", "--config", configPath,
	}, environment); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Version != config.CurrentVersion || cfg.Localizations["en-US"].Values.Name == nil || *cfg.Localizations["en-US"].Values.Name != "Example" {
		t.Fatalf("forced configuration = %#v", cfg)
	}
}

func TestRunPullRequiresPermissionToDeleteLocalAssets(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	configPath := filepath.Join(directory, "ascdir.yaml")
	cfg := config.New("app-1", "com.example.app", "IOS", "1.0.0", []string{"en-US"})
	cfg.Metadata, cfg.Categories, cfg.AgeRating = config.MetadataValues{}, config.CategoryValues{}, config.AgeRatingValues{}
	localization := cfg.Localizations["en-US"]
	name := "Example"
	localization.Values, localization.Files = config.LocaleValues{Name: &name}, config.LocaleFiles{}
	cfg.Localizations["en-US"] = localization
	cfg.Assets.Screenshots = "assets/screenshots"
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatal(err)
	}
	assetPath := filepath.Join(directory, "assets", "screenshots", "en-US", "APP_IPHONE_67", "01.png")
	if err := os.MkdirAll(filepath.Dir(assetPath), 0o755); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(assetPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	remote := appstore.Metadata{
		AppID: "app-1", AppInfoID: "info-1", VersionID: "version-1",
		Localizations: map[string]appstore.Localization{"en-US": {Values: map[string]string{"name": "Example"}}},
		Screenshots:   map[string]map[string][]appstore.Asset{},
	}
	client := mockStoreClient{fetchMetadata: func(context.Context, string, string, string, string, appstore.FetchOptions) (appstore.Metadata, error) {
		return remote, nil
	}}
	environment, _, _ := testEnvironment(client)
	args := []string{"pull", "--config", configPath}
	if err := runWithEnvironment(context.Background(), args, environment); err == nil || !strings.Contains(err.Error(), "--allow-local-asset-deletions") {
		t.Fatalf("error = %v", err)
	}
	if _, err := os.Stat(assetPath); err != nil {
		t.Fatalf("asset changed without permission: %v", err)
	}
	if err := runWithEnvironment(context.Background(), append(args, "--allow-local-asset-deletions"), environment); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(assetPath); !os.IsNotExist(err) {
		t.Fatalf("asset was not removed: %v", err)
	}
}

func TestRunPullBootstrapsMissingAssetDirectory(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	configPath := filepath.Join(directory, "ascdir.yaml")
	cfg := config.New("app-1", "com.example.app", "IOS", "1.0.0", []string{"en-US"})
	cfg.Metadata, cfg.Categories, cfg.AgeRating = config.MetadataValues{}, config.CategoryValues{}, config.AgeRatingValues{}
	localization := cfg.Localizations["en-US"]
	name := "Example"
	localization.Values, localization.Files = config.LocaleValues{Name: &name}, config.LocaleFiles{}
	cfg.Localizations["en-US"] = localization
	cfg.Assets.Screenshots = "assets/screenshots"
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatal(err)
	}
	remote := appstore.Metadata{
		AppID: "app-1", AppInfoID: "info-1", VersionID: "version-1",
		Localizations: map[string]appstore.Localization{"en-US": {Values: map[string]string{"name": "Example"}}},
		Screenshots: map[string]map[string][]appstore.Asset{"en-US": {"APP_IPHONE_67": {{
			FileName: "01.png", Content: []byte("downloaded screenshot"),
		}}}},
	}
	client := mockStoreClient{fetchMetadata: func(context.Context, string, string, string, string, appstore.FetchOptions) (appstore.Metadata, error) {
		return remote, nil
	}}
	environment, _, _ := testEnvironment(client)
	if err := runWithEnvironment(context.Background(), []string{"pull", "--config", configPath}, environment); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "assets", "screenshots", "en-US", "APP_IPHONE_67", "01.png")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("remote asset was not bootstrapped: %v", err)
	}
}

func TestRunPushRequiresExplicitPermissionToClear(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	configPath := filepath.Join(directory, "ascdir.yaml")
	cfg := config.New("app-1", "com.example.app", "IOS", "1.0.0", []string{"en-US"})
	cfg.Metadata = config.MetadataValues{}
	cfg.Categories = config.CategoryValues{}
	localization := cfg.Localizations["en-US"]
	empty := ""
	localization.Values = config.LocaleValues{Subtitle: &empty}
	localization.Files = config.LocaleFiles{}
	cfg.Localizations["en-US"] = localization
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
		fetchMetadata: func(context.Context, string, string, string, string, appstore.FetchOptions) (appstore.Metadata, error) {
			return remote, nil
		},
		applyMetadata: func(context.Context, appstore.Metadata, []string, []appstore.Change) error { applyCalls++; return nil },
	}
	environment, _, _ := testEnvironment(client)
	args := []string{"push", "--config", configPath}
	if err := runWithEnvironment(context.Background(), append(args, "--dry-run"), environment); err == nil || !strings.Contains(err.Error(), "--allow-empty") {
		t.Fatalf("dry-run did not preflight safeguards: %v", err)
	}
	if err := runWithEnvironment(context.Background(), append(args, "--dry-run", "--allow-empty"), environment); err != nil {
		t.Fatal(err)
	}
	if applyCalls != 0 {
		t.Fatalf("dry-run apply calls = %d", applyCalls)
	}
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

func TestRunPushRequiresExplicitPermissionForMadeForKids(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	configPath := filepath.Join(directory, "ascdir.yaml")
	cfg := config.New("app-1", "com.example.app", "IOS", "1.0.0", []string{"en-US"})
	cfg.Metadata = config.MetadataValues{}
	cfg.Categories = config.CategoryValues{}
	kidsAgeBand := "FIVE_AND_UNDER"
	cfg.AgeRating = config.AgeRatingValues{KidsAgeBand: &kidsAgeBand}
	localization := cfg.Localizations["en-US"]
	name := "Example"
	localization.Values = config.LocaleValues{Name: &name}
	localization.Files = config.LocaleFiles{}
	cfg.Localizations["en-US"] = localization
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatal(err)
	}
	remote := appstore.Metadata{
		AppID: "app-1", AppInfoID: "info-1", VersionID: "version-1", AgeRatingID: "rating-1",
		Values:        map[string]string{"kids_age_band": ""},
		Localizations: map[string]appstore.Localization{"en-US": {AppInfoLocalizationID: "info-loc-1", VersionLocalizationID: "version-loc-1", Values: map[string]string{"name": "Example"}}},
	}
	applyCalls := 0
	client := mockStoreClient{
		fetchMetadata: func(context.Context, string, string, string, string, appstore.FetchOptions) (appstore.Metadata, error) {
			return remote, nil
		},
		applyMetadata: func(context.Context, appstore.Metadata, []string, []appstore.Change) error { applyCalls++; return nil },
	}
	environment, _, _ := testEnvironment(client)
	args := []string{"push", "--config", configPath}
	if err := runWithEnvironment(context.Background(), args, environment); err == nil || !strings.Contains(err.Error(), "--allow-irreversible") {
		t.Fatalf("unexpected error: %v", err)
	}
	if applyCalls != 0 {
		t.Fatalf("apply calls = %d", applyCalls)
	}
	if err := runWithEnvironment(context.Background(), append(args, "--allow-irreversible"), environment); err != nil {
		t.Fatal(err)
	}
	if applyCalls != 1 {
		t.Fatalf("apply calls = %d", applyCalls)
	}
}

func TestRunPushProtectsAccessibilityPublicationState(t *testing.T) {
	t.Parallel()
	makeConfig := func(t *testing.T, published bool) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "ascdir.yaml")
		cfg := config.New("app-1", "com.example.app", "IOS", "1.0.0", []string{"en-US"})
		cfg.Metadata, cfg.Categories = config.MetadataValues{}, config.CategoryValues{}
		cfg.Accessibility = map[string]config.AccessibilityValues{"IPHONE": {Published: &published}}
		localization := cfg.Localizations["en-US"]
		name := "Example"
		localization.Values, localization.Files = config.LocaleValues{Name: &name}, config.LocaleFiles{}
		cfg.Localizations["en-US"] = localization
		if err := config.Save(path, cfg); err != nil {
			t.Fatal(err)
		}
		return path
	}
	remoteWithPublished := func(published bool) appstore.Metadata {
		return appstore.Metadata{
			AppID: "app-1", AppInfoID: "info-1", VersionID: "version-1",
			Accessibility: map[string]appstore.AccessibilityDeclaration{"IPHONE": {ID: "declaration-1", Values: map[string]string{"published": fmt.Sprintf("%t", published)}}},
			Localizations: map[string]appstore.Localization{"en-US": {AppInfoLocalizationID: "info-loc-1", VersionLocalizationID: "version-loc-1", Values: map[string]string{"name": "Example"}}},
		}
	}
	t.Run("publish requires confirmation", func(t *testing.T) {
		path, remote, applyCalls := makeConfig(t, true), remoteWithPublished(false), 0
		client := mockStoreClient{
			fetchMetadata: func(context.Context, string, string, string, string, appstore.FetchOptions) (appstore.Metadata, error) {
				return remote, nil
			},
			applyMetadata: func(context.Context, appstore.Metadata, []string, []appstore.Change) error { applyCalls++; return nil },
		}
		environment, _, _ := testEnvironment(client)
		args := []string{"push", "--config", path}
		if err := runWithEnvironment(context.Background(), args, environment); err == nil || !strings.Contains(err.Error(), "--allow-irreversible") {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := runWithEnvironment(context.Background(), append(args, "--allow-irreversible"), environment); err != nil {
			t.Fatal(err)
		}
		if applyCalls != 1 {
			t.Fatalf("apply calls = %d", applyCalls)
		}
	})
	t.Run("published declaration cannot be reverted", func(t *testing.T) {
		path, remote := makeConfig(t, false), remoteWithPublished(true)
		client := mockStoreClient{fetchMetadata: func(context.Context, string, string, string, string, appstore.FetchOptions) (appstore.Metadata, error) {
			return remote, nil
		}}
		environment, _, _ := testEnvironment(client)
		err := runWithEnvironment(context.Background(), []string{"push", "--config", path}, environment)
		if err == nil || !strings.Contains(err.Error(), "cannot be unpublished") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestRunPushRequiresAvailabilityConfirmation(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ascdir.yaml")
	cfg := config.New("app-1", "com.example.app", "IOS", "1.0.0", []string{"en-US"})
	cfg.Metadata, cfg.Categories = config.MetadataValues{}, config.CategoryValues{}
	available := false
	cfg.Availability = &config.AvailabilityValues{Territories: map[string]config.TerritoryAvailability{
		"USA": {Available: &available},
	}}
	localization := cfg.Localizations["en-US"]
	name := "Example"
	localization.Values, localization.Files = config.LocaleValues{Name: &name}, config.LocaleFiles{}
	cfg.Localizations["en-US"] = localization
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	remote := appstore.Metadata{
		AppID: "app-1", AppInfoID: "info-1", VersionID: "version-1", AvailabilityID: "availability-1",
		TerritoryAvailabilityIDs: map[string]string{"USA": "territory-1"},
		Values:                   map[string]string{"availability.territories.USA.available": "true"},
		Localizations:            map[string]appstore.Localization{"en-US": {AppInfoLocalizationID: "info-loc-1", VersionLocalizationID: "version-loc-1", Values: map[string]string{"name": "Example"}}},
	}
	applyCalls := 0
	client := mockStoreClient{
		fetchMetadata: func(context.Context, string, string, string, string, appstore.FetchOptions) (appstore.Metadata, error) {
			return remote, nil
		},
		applyMetadata: func(context.Context, appstore.Metadata, []string, []appstore.Change) error { applyCalls++; return nil },
	}
	environment, _, _ := testEnvironment(client)
	args := []string{"push", "--config", path}
	if err := runWithEnvironment(context.Background(), args, environment); err == nil || !strings.Contains(err.Error(), "--allow-availability-changes") {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := runWithEnvironment(context.Background(), append(args, "--allow-availability-changes"), environment); err != nil {
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
