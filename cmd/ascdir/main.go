package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/Arata1202/ascdir/internal/appstore"
	"github.com/Arata1202/ascdir/internal/authconfig"
	"github.com/Arata1202/ascdir/internal/config"
	"github.com/Arata1202/ascdir/internal/metadata"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

type storeClient interface {
	CheckAuth(context.Context) error
	FetchMetadata(context.Context, string, string, string, string, appstore.FetchOptions) (appstore.Metadata, error)
	ApplyMetadata(context.Context, appstore.Metadata, []string, []appstore.Change) error
}

type commandEnvironment struct {
	stdin     io.Reader
	stdout    io.Writer
	stderr    io.Writer
	newClient func() (storeClient, error)
}

func defaultCommandEnvironment() commandEnvironment {
	return commandEnvironment{
		stdin:  os.Stdin,
		stdout: os.Stdout,
		stderr: os.Stderr,
		newClient: func() (storeClient, error) {
			return newClient(os.Stderr)
		},
	}
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	return runWithEnvironment(ctx, args, defaultCommandEnvironment())
}

func runWithEnvironment(ctx context.Context, args []string, environment commandEnvironment) error {
	if len(args) == 0 {
		usage(environment.stdout)
		return nil
	}

	switch args[0] {
	case "help", "-h", "--help":
		usage(environment.stdout)
		return nil
	case "version", "--version":
		fmt.Fprintln(environment.stdout, versionString())
		return nil
	case "auth":
		return runAuth(ctx, args[1:], environment)
	case "init":
		return runInit(ctx, args[1:], environment)
	case "pull":
		return runPull(ctx, args[1:], environment)
	case "push":
		return runPush(ctx, args[1:], environment)
	case "check":
		return runCheckWithEnvironment(args[1:], environment)
	case "completion":
		return runCompletion(args[1:], environment.stdout)
	default:
		return fmt.Errorf("unknown command %q; run 'ascdir help'", args[0])
	}
}

func versionString() string {
	resolvedVersion := version
	if resolvedVersion == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" && !strings.HasPrefix(info.Main.Version, "v0.0.0-") {
			resolvedVersion = info.Main.Version
		}
	}
	result := "ascdir " + resolvedVersion
	var details []string
	if commit != "unknown" && commit != "" {
		details = append(details, "commit "+commit)
	}
	if date != "unknown" && date != "" {
		details = append(details, "built "+date)
	}
	if len(details) > 0 {
		result += " (" + strings.Join(details, ", ") + ")"
	}
	return result
}

func usage(writer io.Writer) {
	fmt.Fprint(writer, `ascdir manages App Store Connect metadata as YAML and Markdown.

Usage:
  ascdir init  --bundle-id ID --version VERSION [--platform IOS] [--locale en-US]
  ascdir auth login
  ascdir auth check
  ascdir auth logout
  ascdir pull  [--config ascdir.yaml] [--dry-run]
  ascdir push  [--config ascdir.yaml] [--dry-run] [--allow-empty] [--allow-irreversible]
  ascdir check [--config ascdir.yaml]
  ascdir completion <bash|zsh|fish|powershell>
  ascdir version

Authentication:
  ASC_ISSUER_ID        App Store Connect API issuer ID
  ASC_KEY_ID           App Store Connect API key ID
  ASC_PRIVATE_KEY_PATH Path to the AuthKey_*.p8 private key
  ASCDIR_TIMEOUT       HTTP timeout (default: 30s)
`)
}

func runAuth(ctx context.Context, args []string, environment commandEnvironment) error {
	if len(args) == 1 && args[0] == "login" {
		return runAuthLogin(environment)
	}
	if len(args) == 1 && args[0] == "logout" {
		return runAuthLogout(environment)
	}
	if len(args) != 1 || args[0] != "check" {
		return errors.New("usage: ascdir auth <login|check|logout>")
	}
	client, err := environment.newClient()
	if err != nil {
		return err
	}
	if err := client.CheckAuth(ctx); err != nil {
		return err
	}
	fmt.Fprintln(environment.stdout, "Authentication succeeded.")
	return nil
}

func runAuthLogin(environment commandEnvironment) error {
	reader := bufio.NewReader(environment.stdin)
	read := func(label string) (string, error) {
		fmt.Fprint(environment.stdout, label+": ")
		value, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		return strings.TrimSpace(value), nil
	}
	issuerID, err := read("Issuer ID")
	if err != nil {
		return err
	}
	keyID, err := read("Key ID")
	if err != nil {
		return err
	}
	keyPath, err := read("Private key path")
	if err != nil {
		return err
	}
	if strings.HasPrefix(keyPath, "~/") {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return fmt.Errorf("expand private key path: %w", homeErr)
		}
		keyPath = filepath.Join(home, strings.TrimPrefix(keyPath, "~/"))
	}
	keyPath, err = filepath.Abs(keyPath)
	if err != nil {
		return fmt.Errorf("resolve private key path: %w", err)
	}
	if _, err := appstore.CredentialsFromValues(issuerID, keyID, keyPath); err != nil {
		return err
	}
	if warning := appstore.PrivateKeyPermissionWarning(keyPath); warning != "" {
		fmt.Fprintln(environment.stderr, "warning:", warning)
	}
	path, err := authconfig.Save(authconfig.Config{IssuerID: issuerID, KeyID: keyID, PrivateKeyPath: keyPath})
	if err != nil {
		return err
	}
	fmt.Fprintf(environment.stdout, "Credentials saved to %s.\n", path)
	return nil
}

func runAuthLogout(environment commandEnvironment) error {
	path, removed, err := authconfig.Remove()
	if err != nil {
		return err
	}
	if !removed {
		fmt.Fprintln(environment.stdout, "No stored credentials.")
		return nil
	}
	fmt.Fprintf(environment.stdout, "Removed stored credentials from %s.\n", path)
	if os.Getenv("ASC_ISSUER_ID") != "" || os.Getenv("ASC_KEY_ID") != "" || os.Getenv("ASC_PRIVATE_KEY_PATH") != "" {
		fmt.Fprintln(environment.stderr, "warning: ASC_ISSUER_ID, ASC_KEY_ID, or ASC_PRIVATE_KEY_PATH is still set and takes precedence")
	}
	return nil
}

func runInit(ctx context.Context, args []string, environment commandEnvironment) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	bundleID := fs.String("bundle-id", "", "app bundle identifier")
	appVersion := fs.String("version", "", "App Store version")
	platform := fs.String("platform", "IOS", "platform (IOS, MAC_OS, TV_OS, or VISION_OS)")
	locale := fs.String("locale", "en-US", "fallback locale when the app has no localization")
	configPath := fs.String("config", "ascdir.yaml", "configuration file")
	force := fs.Bool("force", false, "overwrite an existing configuration")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireNoArgs(fs); err != nil {
		return err
	}
	if *bundleID == "" || *appVersion == "" {
		return errors.New("--bundle-id and --version are required")
	}
	normalizedBundleID := strings.TrimSpace(*bundleID)
	normalizedVersion := strings.TrimSpace(*appVersion)
	normalizedPlatform := strings.ToUpper(strings.TrimSpace(*platform))
	normalizedLocale := strings.TrimSpace(*locale)
	if err := config.New("", normalizedBundleID, normalizedPlatform, normalizedVersion, []string{normalizedLocale}).Validate(); err != nil {
		return fmt.Errorf("invalid init options: %w", err)
	}
	if !*force {
		if info, err := os.Stat(*configPath); err == nil {
			if info.IsDir() {
				return fmt.Errorf("configuration path %s is a directory", *configPath)
			}
			return fmt.Errorf("%s already exists; use --force to replace it", *configPath)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("check existing configuration: %w", err)
		}
	} else if info, err := os.Stat(*configPath); err == nil && info.IsDir() {
		return fmt.Errorf("configuration path %s is a directory", *configPath)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("check existing configuration: %w", err)
	}

	client, err := environment.newClient()
	if err != nil {
		return err
	}
	remote, err := client.FetchMetadata(ctx, "", normalizedBundleID, normalizedPlatform, normalizedVersion, appstore.FetchOptions{AgeRating: true, Accessibility: true})
	if err != nil {
		return err
	}
	locales := remote.Locales()
	if len(locales) == 0 {
		locales = []string{normalizedLocale}
	}
	cfg := config.New(remote.AppID, normalizedBundleID, normalizedPlatform, normalizedVersion, locales)
	cfg.AgeRating.ManageAll(remote.Values)
	for deviceFamily, remoteDeclaration := range remote.Accessibility {
		if cfg.Accessibility == nil {
			cfg.Accessibility = map[string]config.AccessibilityValues{}
		}
		declaration := config.AccessibilityValues{}
		for field, value := range remoteDeclaration.Values {
			declaration.SetManaged(field, value)
		}
		cfg.Accessibility[deviceFamily] = declaration
	}
	if err := metadata.WriteLocalNew(cfg, *configPath, remote); err != nil {
		return err
	}
	fmt.Fprintf(environment.stdout, "Created %s with %d localization(s).\n", *configPath, len(locales))
	return nil
}

func runPull(ctx context.Context, args []string, environment commandEnvironment) error {
	fs := flag.NewFlagSet("pull", flag.ContinueOnError)
	configPath := fs.String("config", "ascdir.yaml", "configuration file")
	dryRun := fs.Bool("dry-run", false, "show changes without overwriting local metadata")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireNoArgs(fs); err != nil {
		return err
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	client, err := environment.newClient()
	if err != nil {
		return err
	}
	remote, err := client.FetchMetadata(ctx, cfg.App.ID, cfg.App.BundleID, cfg.App.Platform, cfg.App.Version, fetchOptions(cfg))
	if err != nil {
		return err
	}
	if *dryRun {
		local, err := metadata.ReadLocal(cfg, *configPath)
		if err != nil {
			return err
		}
		changes := metadata.Diff(metadata.Select(cfg, remote), local)
		metadata.PrintChanges(environment.stdout, changes)
		if len(changes) == 0 {
			fmt.Fprintln(environment.stdout, "No changes.")
			return nil
		}
		fmt.Fprintf(environment.stdout, "Dry run: %d local change(s) not written.\n", len(changes))
		return nil
	}
	if err := metadata.WriteLocal(cfg, *configPath, remote); err != nil {
		return err
	}
	fmt.Fprintf(environment.stdout, "Pulled %d localization(s).\n", len(cfg.Localizations))
	return nil
}

func runPush(ctx context.Context, args []string, environment commandEnvironment) error {
	fs := flag.NewFlagSet("push", flag.ContinueOnError)
	configPath := fs.String("config", "ascdir.yaml", "configuration file")
	dryRun := fs.Bool("dry-run", false, "show changes without updating App Store Connect")
	allowEmpty := fs.Bool("allow-empty", false, "allow clearing non-empty remote fields")
	allowIrreversible := fs.Bool("allow-irreversible", false, "allow changes Apple may make irreversible, such as Made for Kids")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireNoArgs(fs); err != nil {
		return err
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	desired, err := metadata.ReadLocal(cfg, *configPath)
	if err != nil {
		return err
	}
	if problems := metadata.Validate(desired); len(problems) > 0 {
		for _, problem := range problems {
			fmt.Fprintln(environment.stderr, "-", problem)
		}
		return fmt.Errorf("validation failed with %d problem(s)", len(problems))
	}
	client, err := environment.newClient()
	if err != nil {
		return err
	}
	remote, err := client.FetchMetadata(ctx, cfg.App.ID, cfg.App.BundleID, cfg.App.Platform, cfg.App.Version, fetchOptions(cfg))
	if err != nil {
		return err
	}
	changes := metadata.Diff(desired, remote)
	metadata.PrintChanges(environment.stdout, changes)
	missingLocalizations := appstore.MissingLocalizationResources(remote, desired.Locales())
	for _, missing := range missingLocalizations {
		fmt.Fprintf(environment.stdout, "+ %s localization resource\n", missing)
	}
	operationCount := len(changes) + len(missingLocalizations)
	if operationCount == 0 {
		fmt.Fprintln(environment.stdout, "No changes.")
		return nil
	}
	if *dryRun {
		fmt.Fprintf(environment.stdout, "Dry run: %d operation(s) not applied.\n", operationCount)
		return nil
	}
	clears := metadata.ClearingChanges(changes)
	if len(clears) > 0 && !*allowEmpty {
		return fmt.Errorf("%d change(s) would clear non-empty remote fields; review with --dry-run and rerun with --allow-empty", len(clears))
	}
	if !*allowIrreversible {
		for _, change := range changes {
			if change.Locale == "" && (change.Field == "kids_age_band" || change.DeviceFamily != "" && change.Field == "published" && change.After == "true") {
				return errors.New("the change may be irreversible after App Review or publication; review with --dry-run and rerun with --allow-irreversible")
			}
		}
	}
	for _, change := range changes {
		if change.DeviceFamily != "" && change.Field == "published" && change.Before == "true" && change.After == "false" {
			return fmt.Errorf("accessibility.%s.published cannot be unpublished through App Store Connect", change.DeviceFamily)
		}
	}
	if err := client.ApplyMetadata(ctx, remote, desired.Locales(), changes); err != nil {
		return err
	}
	fmt.Fprintf(environment.stdout, "Applied %d operation(s).\n", operationCount)
	return nil
}

func runCheck(args []string) error {
	return runCheckWithEnvironment(args, defaultCommandEnvironment())
}

func runCheckWithEnvironment(args []string, environment commandEnvironment) error {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	configPath := fs.String("config", "ascdir.yaml", "configuration file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireNoArgs(fs); err != nil {
		return err
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	values, err := metadata.ReadLocal(cfg, *configPath)
	if err != nil {
		return err
	}
	problems := metadata.Validate(values)
	for _, problem := range problems {
		fmt.Fprintln(environment.stdout, "-", problem)
	}
	if len(problems) > 0 {
		return fmt.Errorf("validation failed with %d problem(s)", len(problems))
	}
	fmt.Fprintf(environment.stdout, "Configuration and %d localization(s) are valid.\n", len(values.Localizations))
	return nil
}

func requireNoArgs(fs *flag.FlagSet) error {
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	return nil
}

func fetchOptions(cfg config.Config) appstore.FetchOptions {
	return appstore.FetchOptions{AgeRating: len(cfg.AgeRating.Map()) > 0, Accessibility: len(cfg.Accessibility) > 0, LicenseAgreement: cfg.LicenseAgreement != nil}
}

func newClient(stderr io.Writer) (*appstore.Client, error) {
	issuerID, keyID, keyPath := os.Getenv("ASC_ISSUER_ID"), os.Getenv("ASC_KEY_ID"), os.Getenv("ASC_PRIVATE_KEY_PATH")
	var credentials appstore.Credentials
	var err error
	if issuerID != "" || keyID != "" || keyPath != "" {
		credentials, err = appstore.CredentialsFromEnv()
	} else {
		stored, loadErr := authconfig.Load()
		if loadErr != nil {
			return nil, loadErr
		}
		keyPath = stored.PrivateKeyPath
		credentials, err = appstore.CredentialsFromValues(stored.IssuerID, stored.KeyID, stored.PrivateKeyPath)
	}
	if err != nil {
		return nil, err
	}
	if warning := appstore.PrivateKeyPermissionWarning(keyPath); warning != "" {
		fmt.Fprintln(stderr, "warning:", warning)
	}
	timeout := 30 * time.Second
	if value := os.Getenv("ASCDIR_TIMEOUT"); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil || parsed <= 0 {
			return nil, fmt.Errorf("ASCDIR_TIMEOUT must be a positive duration such as 30s")
		}
		timeout = parsed
	}
	return appstore.NewClient(credentials, "", appstore.WithTimeout(timeout)), nil
}
