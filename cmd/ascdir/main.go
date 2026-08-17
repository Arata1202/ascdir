package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Arata1202/ascdir/internal/appstore"
	"github.com/Arata1202/ascdir/internal/config"
	"github.com/Arata1202/ascdir/internal/metadata"
)

const version = "0.1.0"

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
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
		fmt.Println("ascdir", version)
		return nil
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

func usage() {
	fmt.Print(`ascdir manages App Store Connect metadata as local files.

Usage:
  ascdir init  --bundle-id ID --version VERSION [--platform IOS] [--locale en-US]
  ascdir pull  [--config ascdir.yaml]
  ascdir push  [--config ascdir.yaml] [--dry-run]
  ascdir check [--config ascdir.yaml]
  ascdir version

Authentication:
  ASC_ISSUER_ID        App Store Connect API issuer ID
  ASC_KEY_ID           App Store Connect API key ID
  ASC_PRIVATE_KEY_PATH Path to the AuthKey_*.p8 private key
`)
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
	remote, err := client.FetchMetadata(ctx, *bundleID, strings.ToUpper(*platform), *appVersion)
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
	if err := fs.Parse(args); err != nil {
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
	remote, err := client.FetchMetadata(ctx, cfg.App.BundleID, cfg.App.Platform, cfg.App.Version)
	if err != nil {
		return err
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
	if err := fs.Parse(args); err != nil {
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
	client, err := newClient()
	if err != nil {
		return err
	}
	remote, err := client.FetchMetadata(ctx, cfg.App.BundleID, cfg.App.Platform, cfg.App.Version)
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
	if err := client.ApplyMetadata(ctx, remote, desired, changes); err != nil {
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

func newClient() (*appstore.Client, error) {
	credentials, err := appstore.CredentialsFromEnv()
	if err != nil {
		return nil, err
	}
	return appstore.NewClient(credentials, os.Getenv("ASCDIR_API_BASE_URL")), nil
}
