package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/Arata1202/ascdir/internal/config"
)

func runAppStore(ctx context.Context, args []string, environment commandEnvironment) error {
	if len(args) == 0 || args[0] != "status" {
		return errors.New("usage: ascdir app-store status [--config ascdir.yaml] [--json]")
	}
	fs := flag.NewFlagSet("app-store status", flag.ContinueOnError)
	configPath := fs.String("config", "ascdir.yaml", "configuration file")
	jsonOutput := fs.Bool("json", false, "write machine-readable JSON")
	if err := fs.Parse(args[1:]); err != nil {
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
	status, err := client.FetchAppStoreStatus(ctx, cfg.App.ID, cfg.App.BundleID, cfg.App.Platform, cfg.App.Version)
	if err != nil {
		return err
	}
	if *jsonOutput {
		return writeJSON(environment.stdout, status)
	}
	fmt.Fprintf(environment.stdout, "App Store %s %s: %s\n", status.Platform, status.Version, displayValue(status.AppVersionState))
	if status.Build == nil {
		fmt.Fprintln(environment.stdout, "Build: not selected")
	} else {
		fmt.Fprintf(environment.stdout, "Build: %s (%s)\n", status.Build.Version, displayValue(status.Build.ProcessingState))
	}
	if status.ReviewSubmission == nil {
		fmt.Fprintln(environment.stdout, "Review submission: none")
	} else {
		fmt.Fprintf(environment.stdout, "Review submission: %s\n", displayValue(status.ReviewSubmission.State))
	}
	if status.ReleaseType != "" {
		fmt.Fprintf(environment.stdout, "Release type: %s\n", status.ReleaseType)
	}
	return nil
}

func runTestFlight(ctx context.Context, args []string, environment commandEnvironment) error {
	if len(args) == 0 || args[0] != "status" {
		return errors.New("usage: ascdir testflight status [--config ascdir.yaml] [--json]")
	}
	fs := flag.NewFlagSet("testflight status", flag.ContinueOnError)
	configPath := fs.String("config", "ascdir.yaml", "configuration file")
	jsonOutput := fs.Bool("json", false, "write machine-readable JSON")
	if err := fs.Parse(args[1:]); err != nil {
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
	status, err := client.FetchTestFlightStatus(ctx, cfg.App.ID, cfg.App.BundleID, cfg.App.Platform, cfg.App.Version)
	if err != nil {
		return err
	}
	if *jsonOutput {
		return writeJSON(environment.stdout, status)
	}
	fmt.Fprintf(environment.stdout, "TestFlight %s %s\n", cfg.App.Platform, cfg.App.Version)
	if len(status.Builds) == 0 {
		fmt.Fprintln(environment.stdout, "Builds: none")
		return nil
	}
	for _, build := range status.Builds {
		expired := ""
		if build.Expired {
			expired = ", expired"
		}
		fmt.Fprintf(environment.stdout, "- %s: %s%s\n", build.Version, displayValue(build.ProcessingState), expired)
	}
	return nil
}

func writeJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

func displayValue(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}
