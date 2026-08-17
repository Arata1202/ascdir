package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/Arata1202/ascdir/internal/appstore"
	"github.com/Arata1202/ascdir/internal/config"
	"github.com/Arata1202/ascdir/internal/metadata"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

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
	if len(args) == 0 {
		usage()
		return nil
	}

	switch args[0] {
	case "help", "-h", "--help":
		usage()
		return nil
	case "version", "--version":
		fmt.Println(versionString())
		return nil
	case "auth":
		return runAuth(ctx, args[1:])
	case "init":
		return runInit(ctx, args[1:])
	case "pull":
		return runPull(ctx, args[1:])
	case "push":
		return runPush(ctx, args[1:])
	case "check":
		return runCheck(args[1:])
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

func usage() {
	fmt.Print(`ascdir manages App Store Connect metadata as local files.

Usage:
  ascdir init  --bundle-id ID --version VERSION [--platform IOS] [--locale en-US]
  ascdir auth check
  ascdir pull  [--config ascdir.yaml] [--dry-run]
  ascdir push  [--config ascdir.yaml] [--dry-run] [--allow-empty]
  ascdir check [--config ascdir.yaml]
  ascdir version

Authentication:
  ASC_ISSUER_ID        App Store Connect API issuer ID
  ASC_KEY_ID           App Store Connect API key ID
  ASC_PRIVATE_KEY_PATH Path to the AuthKey_*.p8 private key
  ASCDIR_TIMEOUT       HTTP timeout (default: 30s)
`)
}

func runAuth(ctx context.Context, args []string) error {
	if len(args) != 1 || args[0] != "check" {
		return errors.New("usage: ascdir auth check")
	}
	client, err := newClient()
	if err != nil {
		return err
	}
	if err := client.CheckAuth(ctx); err != nil {
		return err
	}
	fmt.Println("Authentication succeeded.")
	return nil
}

func runInit(ctx context.Context, args []string) error {
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
	if !*force {
		if _, err := os.Stat(*configPath); err == nil {
			return fmt.Errorf("%s already exists; use --force to replace it", *configPath)
		}
	}

	client, err := newClient()
	if err != nil {
		return err
	}
	remote, err := client.FetchMetadata(ctx, "", *bundleID, strings.ToUpper(*platform), *appVersion)
	if err != nil {
		return err
	}
	locales := remote.Locales()
	if len(locales) == 0 {
		locales = []string{*locale}
	}
	cfg := config.New(remote.AppID, *bundleID, strings.ToUpper(*platform), *appVersion, locales)
	if err := config.Save(*configPath, cfg); err != nil {
		return err
	}
	if err := metadata.WriteLocal(cfg, *configPath, remote); err != nil {
		return err
	}
	fmt.Printf("Created %s with %d localization(s).\n", *configPath, len(locales))
	return nil
}

func runPull(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("pull", flag.ContinueOnError)
	configPath := fs.String("config", "ascdir.yaml", "configuration file")
	dryRun := fs.Bool("dry-run", false, "show changes without overwriting local files")
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
	client, err := newClient()
	if err != nil {
		return err
	}
	remote, err := client.FetchMetadata(ctx, cfg.App.ID, cfg.App.BundleID, cfg.App.Platform, cfg.App.Version)
	if err != nil {
		return err
	}
	if *dryRun {
		local, err := metadata.ReadLocal(cfg, *configPath)
		if err != nil {
			return err
		}
		changes := metadata.Diff(metadata.Select(cfg, remote), local)
		metadata.PrintChanges(os.Stdout, changes)
		if len(changes) == 0 {
			fmt.Println("No changes.")
			return nil
		}
		fmt.Printf("Dry run: %d local change(s) not written.\n", len(changes))
		return nil
	}
	if err := metadata.WriteLocal(cfg, *configPath, remote); err != nil {
		return err
	}
	fmt.Printf("Pulled %d localization(s).\n", len(cfg.Localizations))
	return nil
}

func runPush(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("push", flag.ContinueOnError)
	configPath := fs.String("config", "ascdir.yaml", "configuration file")
	dryRun := fs.Bool("dry-run", false, "show changes without updating App Store Connect")
	allowEmpty := fs.Bool("allow-empty", false, "allow clearing non-empty remote fields")
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
			fmt.Fprintln(os.Stderr, "-", problem)
		}
		return fmt.Errorf("validation failed with %d problem(s)", len(problems))
	}
	client, err := newClient()
	if err != nil {
		return err
	}
	remote, err := client.FetchMetadata(ctx, cfg.App.ID, cfg.App.BundleID, cfg.App.Platform, cfg.App.Version)
	if err != nil {
		return err
	}
	changes := metadata.Diff(desired, remote)
	metadata.PrintChanges(os.Stdout, changes)
	if len(changes) == 0 {
		fmt.Println("No changes.")
		return nil
	}
	if *dryRun {
		fmt.Printf("Dry run: %d change(s) not applied.\n", len(changes))
		return nil
	}
	clears := metadata.ClearingChanges(changes)
	if len(clears) > 0 && !*allowEmpty {
		return fmt.Errorf("%d change(s) would clear non-empty remote fields; review with --dry-run and rerun with --allow-empty", len(clears))
	}
	if err := client.ApplyMetadata(ctx, remote, changes); err != nil {
		return err
	}
	fmt.Printf("Applied %d change(s).\n", len(changes))
	return nil
}

func runCheck(args []string) error {
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
		fmt.Println("-", problem)
	}
	if len(problems) > 0 {
		return fmt.Errorf("validation failed with %d problem(s)", len(problems))
	}
	fmt.Printf("Configuration and %d localization(s) are valid.\n", len(values.Localizations))
	return nil
}

func requireNoArgs(fs *flag.FlagSet) error {
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	return nil
}

func newClient() (*appstore.Client, error) {
	credentials, err := appstore.CredentialsFromEnv()
	if err != nil {
		return nil, err
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
