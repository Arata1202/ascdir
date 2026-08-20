package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/Arata1202/ascdir/internal/appstore"
	"github.com/Arata1202/ascdir/internal/config"
)

func runAppStore(ctx context.Context, args []string, environment commandEnvironment) error {
	if len(args) == 0 {
		return errors.New("usage: ascdir app-store <status|submit|release>")
	}
	switch args[0] {
	case "status":
		return runAppStoreStatus(ctx, args[1:], environment)
	case "submit":
		return runAppStoreSubmit(ctx, args[1:], environment)
	case "release":
		return runAppStoreRelease(ctx, args[1:], environment)
	default:
		return errors.New("usage: ascdir app-store <status|submit|release>")
	}
}

func runAppStoreStatus(ctx context.Context, args []string, environment commandEnvironment) error {
	fs := flag.NewFlagSet("app-store status", flag.ContinueOnError)
	configPath := fs.String("config", "ascdir.yaml", "configuration file")
	jsonOutput := fs.Bool("json", false, "write machine-readable JSON")
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

func runAppStoreSubmit(ctx context.Context, args []string, environment commandEnvironment) error {
	fs := flag.NewFlagSet("app-store submit", flag.ContinueOnError)
	configPath := fs.String("config", "ascdir.yaml", "configuration file")
	build := fs.String("build", "", "build number (defaults to the newest eligible build)")
	releaseType := fs.String("release-type", "", "release type (MANUAL, AFTER_APPROVAL, or SCHEDULED; preserves an existing version setting)")
	earliestReleaseDate := fs.String("earliest-release-date", "", "earliest scheduled release date in RFC3339 format")
	dryRun := fs.Bool("dry-run", false, "validate and show the submission plan without changing App Store Connect")
	confirm := fs.String("confirm", "", "confirm submission by repeating the configured version")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireNoArgs(fs); err != nil {
		return err
	}
	if *dryRun && *confirm != "" {
		return errors.New("--dry-run and --confirm cannot be used together")
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	client, err := environment.newClient()
	if err != nil {
		return err
	}
	options := appstore.SubmitOptions{BuildVersion: strings.TrimSpace(*build), ReleaseType: strings.TrimSpace(*releaseType), EarliestReleaseDate: strings.TrimSpace(*earliestReleaseDate)}
	plan, err := client.PlanAppStoreSubmission(ctx, cfg.App.ID, cfg.App.BundleID, cfg.App.Platform, cfg.App.Version, options)
	if err != nil {
		return err
	}
	printReleasePlan(environment.stdout, plan)
	if len(plan.Operations) == 0 {
		fmt.Fprintln(environment.stdout, "No changes: this version is already submitted or released.")
		return nil
	}
	if *dryRun {
		fmt.Fprintf(environment.stdout, "Dry run: %d operation(s) validated and not applied.\n", len(plan.Operations))
		return nil
	}
	if *confirm != cfg.App.Version {
		return fmt.Errorf("submission changes App Store Connect; review with --dry-run and rerun with --confirm %s", cfg.App.Version)
	}
	if err := client.ApplyAppStoreSubmission(ctx, plan); err != nil {
		return err
	}
	if plan.Build == "" {
		fmt.Fprintf(environment.stdout, "Created %s %s. Sync metadata with ascdir push, then rerun app-store submit.\n", cfg.App.Platform, cfg.App.Version)
		return nil
	}
	fmt.Fprintf(environment.stdout, "Submitted %s %s (build %s) for App Review.\n", cfg.App.Platform, cfg.App.Version, plan.Build)
	return nil
}

func runAppStoreRelease(ctx context.Context, args []string, environment commandEnvironment) error {
	fs := flag.NewFlagSet("app-store release", flag.ContinueOnError)
	configPath := fs.String("config", "ascdir.yaml", "configuration file")
	dryRun := fs.Bool("dry-run", false, "validate and show the release plan without changing App Store Connect")
	confirm := fs.String("confirm", "", "confirm release by repeating the configured version")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireNoArgs(fs); err != nil {
		return err
	}
	if *dryRun && *confirm != "" {
		return errors.New("--dry-run and --confirm cannot be used together")
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	client, err := environment.newClient()
	if err != nil {
		return err
	}
	plan, err := client.PlanAppStoreRelease(ctx, cfg.App.ID, cfg.App.BundleID, cfg.App.Platform, cfg.App.Version)
	if err != nil {
		return err
	}
	printReleasePlan(environment.stdout, plan)
	if len(plan.Operations) == 0 {
		fmt.Fprintln(environment.stdout, "No changes: this version is already released or being processed.")
		return nil
	}
	if *dryRun {
		fmt.Fprintf(environment.stdout, "Dry run: %d operation(s) validated and not applied.\n", len(plan.Operations))
		return nil
	}
	if *confirm != cfg.App.Version {
		return fmt.Errorf("release makes the approved version available to customers; review with --dry-run and rerun with --confirm %s", cfg.App.Version)
	}
	if err := client.ApplyAppStoreRelease(ctx, plan); err != nil {
		return err
	}
	fmt.Fprintf(environment.stdout, "Requested release of %s %s.\n", cfg.App.Platform, cfg.App.Version)
	return nil
}

func printReleasePlan(writer io.Writer, plan appstore.AppStoreReleasePlan) {
	fmt.Fprintf(writer, "%s %s %s", plan.Kind, plan.Platform, plan.Version)
	if plan.Build != "" {
		fmt.Fprintf(writer, " (build %s)", plan.Build)
	}
	fmt.Fprintln(writer)
	for index, operation := range plan.Operations {
		fmt.Fprintf(writer, "%d. %s\n", index+1, operation.Description)
	}
}

func runTestFlight(ctx context.Context, args []string, environment commandEnvironment) error {
	if len(args) == 0 {
		return errors.New("usage: ascdir testflight <status|distribute>")
	}
	switch args[0] {
	case "status":
		return runTestFlightStatus(ctx, args[1:], environment)
	case "distribute":
		return runTestFlightDistribute(ctx, args[1:], environment)
	default:
		return errors.New("usage: ascdir testflight <status|distribute>")
	}
}

func runTestFlightStatus(ctx context.Context, args []string, environment commandEnvironment) error {
	fs := flag.NewFlagSet("testflight status", flag.ContinueOnError)
	configPath := fs.String("config", "ascdir.yaml", "configuration file")
	jsonOutput := fs.Bool("json", false, "write machine-readable JSON")
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

type repeatedStringFlag []string

func (values *repeatedStringFlag) String() string { return strings.Join(*values, ", ") }

func (values *repeatedStringFlag) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func runTestFlightDistribute(ctx context.Context, args []string, environment commandEnvironment) error {
	fs := flag.NewFlagSet("testflight distribute", flag.ContinueOnError)
	configPath := fs.String("config", "ascdir.yaml", "configuration file")
	build := fs.String("build", "", "build number (defaults to the newest eligible build)")
	var groups repeatedStringFlag
	fs.Var(&groups, "group", "existing TestFlight beta group name (repeat for multiple groups)")
	dryRun := fs.Bool("dry-run", false, "validate and show the distribution plan without changing App Store Connect")
	confirm := fs.String("confirm", "", "confirm distribution by repeating the configured version")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireNoArgs(fs); err != nil {
		return err
	}
	if *dryRun && *confirm != "" {
		return errors.New("--dry-run and --confirm cannot be used together")
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	client, err := environment.newClient()
	if err != nil {
		return err
	}
	plan, err := client.PlanTestFlightDistribution(ctx, cfg.App.ID, cfg.App.BundleID, cfg.App.Platform, cfg.App.Version, appstore.TestFlightDistributionOptions{BuildVersion: strings.TrimSpace(*build), GroupNames: groups})
	if err != nil {
		return err
	}
	printTestFlightDistributionPlan(environment.stdout, plan)
	if len(plan.Operations) == 0 {
		fmt.Fprintln(environment.stdout, "No remaining operations: the build is attached to the requested groups.")
		if plan.BetaReviewState != "" && plan.BetaReviewState != "APPROVED" {
			fmt.Fprintf(environment.stdout, "External testing remains subject to Beta App Review (%s).\n", plan.BetaReviewState)
		}
		return nil
	}
	if *dryRun {
		fmt.Fprintf(environment.stdout, "Dry run: %d operation(s) validated and not applied.\n", len(plan.Operations))
		return nil
	}
	if *confirm != cfg.App.Version {
		return fmt.Errorf("distribution grants testers access to a build; review with --dry-run and rerun with --confirm %s", cfg.App.Version)
	}
	if err := client.ApplyTestFlightDistribution(ctx, plan); err != nil {
		return err
	}
	fmt.Fprintf(environment.stdout, "Distributed TestFlight %s %s (build %s) to %d group(s).\n", cfg.App.Platform, cfg.App.Version, plan.Build, len(plan.Groups))
	return nil
}

func printTestFlightDistributionPlan(writer io.Writer, plan appstore.TestFlightDistributionPlan) {
	fmt.Fprintf(writer, "testflight-distribute %s %s (build %s)\n", plan.Platform, plan.Version, plan.Build)
	for _, group := range plan.Groups {
		typeName := "external"
		if group.Internal {
			typeName = "internal"
		}
		state := "pending"
		if group.Attached {
			state = "attached"
		}
		fmt.Fprintf(writer, "- %s (%s, %s)\n", group.Name, typeName, state)
	}
	if plan.BetaReviewState != "" {
		fmt.Fprintf(writer, "Beta App Review: %s\n", plan.BetaReviewState)
	}
	for index, operation := range plan.Operations {
		fmt.Fprintf(writer, "%d. %s\n", index+1, operation.Description)
	}
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
