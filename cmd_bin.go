package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	binpkg "github.com/pun/vex/internal/bin"
)

// --- helpers ---

func lookupManagedTool(name string) (binpkg.ToolSpec, error) {
	spec, ok := binpkg.GetTool(name)
	if !ok {
		return binpkg.ToolSpec{}, fmt.Errorf("unknown managed tool %q (available: %s)", name, supportedToolsString())
	}
	return spec, nil
}

func supportedToolsString() string {
	tools := binpkg.AllTools()
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	return strings.Join(names, ", ")
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func toolStatusSummary(status *binpkg.ToolStatus, installedVersion, latestVersion string) string {
	if !status.Available {
		if status.UnavailableReason != "" {
			return "unavailable: " + status.UnavailableReason
		}
		return "unavailable"
	}
	if status.ResolutionExactVersion && status.VersionChangeRequired && installedVersion != "unknown" && latestVersion != "unknown" {
		switch comparison := binpkg.CompareVersions(installedVersion, latestVersion); {
		case comparison > 0:
			return "compatibility downgrade required"
		case comparison < 0:
			return "update required for compat stack"
		default:
			return "version alignment required"
		}
	}
	if status.LatestVersion != "" && installedVersion != "unknown" {
		if status.UpdateAvailable {
			return "update available"
		}
		return "up to date"
	}
	if status.Exists {
		return "installed"
	}
	return "not installed"
}

// --- install ---

func cmdBinInstall(toolName string, force bool) {
	spec, err := lookupManagedTool(toolName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	result, err := binpkg.InstallTool(context.Background(), spec, force)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Installed %s %s to %s\n", spec.Name, result.State.InstalledVersion, result.State.Path)
}

// --- ls ---

func cmdBinLs() {
	tools := binpkg.AllTools()
	if len(tools) == 0 {
		fmt.Println("No managed binaries are configured")
		return
	}

	fmt.Printf("%-12s %-12s %-12s %s\n", "TOOL", "STATE", "VERSION", "DETAIL")
	for _, spec := range tools {
		status, err := binpkg.LocalToolStatus(spec)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		state := "available"
		detail := status.Path
		switch {
		case !status.Available:
			state = "unavailable"
			detail = status.UnavailableReason
		case status.Managed && status.Exists:
			state = "installed"
		case status.Managed && !status.Exists:
			state = "missing"
		case !status.Managed && status.Exists:
			state = "unmanaged"
		}
		if detail == "" {
			detail = "-"
		}

		version := status.EffectiveInstalledVersion()
		if version == "" {
			version = "-"
		}

		fmt.Printf("%-12s %-12s %-12s %s\n",
			spec.Name,
			state,
			version,
			truncate(detail, 60),
		)
	}
}

// --- status ---

func cmdBinStatus(toolName string) {
	spec, err := lookupManagedTool(toolName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	status, err := binpkg.InspectTool(context.Background(), spec)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if status.LatestError != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to check latest version for %s: %v\n", spec.Name, status.LatestError)
	}
	installedVersion := status.EffectiveInstalledVersion()
	if installedVersion == "" {
		installedVersion = "unknown"
	}
	latestVersion := status.LatestVersion
	if latestVersion == "" {
		latestVersion = "unknown"
	}
	fmt.Printf("Tool:              %s\n", spec.Name)
	fmt.Printf("Managed:           %s\n", yesNo(status.Managed))
	fmt.Printf("Path:              %s\n", status.Path)
	fmt.Printf("Exists:            %s\n", yesNo(status.Exists))
	fmt.Printf("Executable:        %s\n", yesNo(status.Executable))
	fmt.Printf("Available:         %s\n", yesNo(status.Available))
	if !status.Available && status.UnavailableReason != "" {
		fmt.Printf("Unavailable:       %s\n", status.UnavailableReason)
	}
	fmt.Printf("Installed version: %s\n", installedVersion)
	if status.RuntimeVersion != "" && status.StoredVersion != "" && status.RuntimeVersion != status.StoredVersion {
		fmt.Printf("Stored version:    %s\n", status.StoredVersion)
	}
	fmt.Printf("Latest version:    %s\n", latestVersion)
	if status.LatestReleaseTag != "" {
		fmt.Printf("Latest tag:        %s\n", status.LatestReleaseTag)
	}
	if status.ResolutionStrategy != "" {
		fmt.Printf("Resolution:        %s\n", status.ResolutionStrategy)
	}
	if status.ResolutionReason != "" {
		fmt.Printf("Resolution note:   %s\n", status.ResolutionReason)
	}
	if status.UpstreamLatestVersion != "" && status.UpstreamLatestVersion != latestVersion {
		fmt.Printf("Upstream latest:   %s\n", status.UpstreamLatestVersion)
		if status.UpstreamLatestReleaseTag != "" {
			fmt.Printf("Upstream tag:      %s\n", status.UpstreamLatestReleaseTag)
		}
		if len(status.CompatibleTools) > 0 {
			fmt.Printf("Capped by:         %s\n", strings.Join(status.CompatibleTools, ", "))
		}
	}
	if status.SelectedNushellMinor != "" {
		fmt.Printf("Compat minor:      %s.x\n", status.SelectedNushellMinor)
	}
	fmt.Printf("Status:            %s\n", toolStatusSummary(status, installedVersion, latestVersion))
	if status.Available && status.LatestVersion != "" && installedVersion != "unknown" {
		if status.ResolutionExactVersion {
			fmt.Printf("Change required:   %s\n", yesNo(status.VersionChangeRequired))
		} else {
			fmt.Printf("Update available:  %s\n", yesNo(status.UpdateAvailable))
		}
	}
}

// --- sync ---

func cmdBinSync(dryRun bool) {
	ctx := context.Background()
	tools := binpkg.AllTools()
	var installed, updated, skipped, failed int
	rateLimitFailures := githubRateLimitFailureCollector{}
	for _, spec := range tools {
		wasUpdate, wasSkipped, err := syncTool(ctx, spec, dryRun)
		if err != nil {
			failed++
			if rateLimitFailures.Record(spec.Name, err) {
				continue
			}
			fmt.Fprintf(os.Stderr, "✗ %s: %v\n", spec.Name, err)
			continue
		}
		if wasSkipped {
			skipped++
			continue
		}
		if wasUpdate {
			updated++
		} else {
			installed++
		}
	}

	rateLimitFailures.PrintSummary()
	fmt.Printf("\nSummary: %d installed, %d updated, %d skipped, %d failed%s\n", installed, updated, skipped, failed, rateLimitFailures.SummarySuffix())
	if failed > 0 {
		os.Exit(1)
	}
}

func syncTool(ctx context.Context, spec binpkg.ToolSpec, dryRun bool) (bool, bool, error) {
	status, err := binpkg.InspectTool(ctx, spec)
	if err != nil {
		return false, false, fmt.Errorf("failed to inspect %s: %w", spec.Name, err)
	}
	if !status.Available {
		if dryRun {
			fmt.Printf("• %s: would skip (%s)\n", spec.Name, status.UnavailableReason)
		} else {
			fmt.Printf("• %s: skipped (%s)\n", spec.Name, status.UnavailableReason)
		}
		return false, true, nil
	}
	if status.LatestError != nil {
		return false, false, fmt.Errorf("failed to check latest version: %w", status.LatestError)
	}

	latestVersion := status.LatestVersion
	if latestVersion == "" {
		latestVersion = "unknown"
	}
	// Case 1: Not installed
	if !status.Exists {
		if dryRun {
			fmt.Printf("• %s: would install %s (not present)\n", spec.Name, latestVersion)
			return false, false, nil
		}
		installResult, err := binpkg.InstallTool(ctx, spec, true)
		if err != nil {
			return false, false, err
		}
		fmt.Printf("✓ %s: installed %s\n", spec.Name, installResult.State.InstalledVersion)
		return false, false, nil
	}
	// Case 2: Unmanaged binary exists - take it over
	if !status.Managed {
		if dryRun {
			fmt.Printf("• %s: would take over and install %s\n", spec.Name, latestVersion)
			return true, false, nil
		}
		installResult, err := binpkg.InstallTool(ctx, spec, true)
		if err != nil {
			return false, false, err
		}
		fmt.Printf("✓ %s: took over and installed %s\n", spec.Name, installResult.State.InstalledVersion)
		return true, false, nil
	}
	// Case 3: Managed and installed - update/re-install with force
	installedVersion := status.EffectiveInstalledVersion()
	if installedVersion == "" {
		installedVersion = "unknown"
	}

	if dryRun {
		switch {
		case status.ResolutionExactVersion && status.VersionChangeRequired:
			fmt.Printf("• %s: would align %s → %s (compat stack)\n", spec.Name, installedVersion, latestVersion)
		case status.UpdateAvailable:
			fmt.Printf("• %s: would update %s → %s\n", spec.Name, installedVersion, latestVersion)
		default:
			fmt.Printf("• %s: would re-install %s\n", spec.Name, latestVersion)
		}
		return true, false, nil
	}
	updateResult, err := binpkg.UpdateTool(ctx, spec, true)
	if err != nil {
		return false, false, err
	}
	if updateResult.Updated {
		if updateResult.PreviousVersion != "" && updateResult.PreviousVersion != updateResult.State.InstalledVersion {
			fmt.Printf("↑ %s: %s → %s\n", spec.Name, updateResult.PreviousVersion, updateResult.State.InstalledVersion)
		} else {
			fmt.Printf("↑ %s: re-installed %s\n", spec.Name, updateResult.State.InstalledVersion)
		}
	} else {
		fmt.Printf("↑ %s: re-installed %s\n", spec.Name, updateResult.State.InstalledVersion)
	}
	return true, false, nil
}

// --- update ---

func cmdBinUpdate(toolName string, updateAll, force bool) {
	if updateAll && toolName != "" {
		fmt.Fprintln(os.Stderr, "Error: cannot specify a tool name together with --all")
		os.Exit(1)
	}
	if !updateAll && toolName == "" {
		fmt.Fprintln(os.Stderr, "Error: specify either a tool name or --all")
		os.Exit(1)
	}

	ctx := context.Background()
	if updateAll {
		updateAllManagedTools(ctx, force)
		return
	}

	spec, err := lookupManagedTool(toolName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	result, err := binpkg.UpdateTool(ctx, spec, force)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if !result.Updated {
		fmt.Printf("%s is already up to date (%s)\n", spec.Name, result.State.InstalledVersion)
		return
	}
	if result.PreviousVersion != "" {
		fmt.Printf("✓ Updated %s from %s to %s\n", spec.Name, result.PreviousVersion, result.State.InstalledVersion)
	} else {
		fmt.Printf("✓ Installed latest %s (%s)\n", spec.Name, result.State.InstalledVersion)
	}
}

func updateAllManagedTools(ctx context.Context, force bool) {
	tools := binpkg.AllTools()
	updated := 0
	current := 0
	skipped := 0
	failed := 0
	rateLimitFailures := githubRateLimitFailureCollector{}
	for _, spec := range tools {
		status, err := binpkg.LocalToolStatus(spec)
		if err != nil {
			failed++
			fmt.Fprintf(os.Stderr, "✗ %s: %v\n", spec.Name, err)
			continue
		}
		if !status.Managed {
			continue
		}
		if !status.Available {
			skipped++
			fmt.Printf("• %s: skipped (%s)\n", spec.Name, status.UnavailableReason)
			continue
		}
		result, err := binpkg.UpdateTool(ctx, spec, force)
		if err != nil {
			failed++
			if rateLimitFailures.Record(spec.Name, err) {
				continue
			}
			fmt.Fprintf(os.Stderr, "✗ %s: %v\n", spec.Name, err)
			continue
		}
		if result.Updated {
			updated++
			if result.PreviousVersion != "" {
				fmt.Printf("✓ %s: %s -> %s\n", spec.Name, result.PreviousVersion, result.State.InstalledVersion)
			} else {
				fmt.Printf("✓ %s: installed %s\n", spec.Name, result.State.InstalledVersion)
			}
			continue
		}
		current++
		fmt.Printf("• %s: already up to date (%s)\n", spec.Name, result.State.InstalledVersion)
	}

	rateLimitFailures.PrintSummary()
	fmt.Printf("Summary: %d updated, %d already current, %d skipped, %d failed%s\n", updated, current, skipped, failed, rateLimitFailures.SummarySuffix())
	if failed > 0 {
		os.Exit(1)
	}
}

// --- version ---

func cmdBinVersion(toolName string) {
	spec, err := lookupManagedTool(toolName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	status, err := binpkg.InspectTool(context.Background(), spec)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if status.LatestError != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to check latest version for %s: %v\n", spec.Name, status.LatestError)
	}
	installedVersion := status.EffectiveInstalledVersion()
	if installedVersion == "" {
		installedVersion = "unknown"
	}
	latestVersion := status.LatestVersion
	if latestVersion == "" {
		latestVersion = "unknown"
	}
	fmt.Printf("%s\n", spec.Name)
	fmt.Printf("  installed: %s\n", installedVersion)
	fmt.Printf("  latest:    %s\n", latestVersion)
	if !status.Available && status.UnavailableReason != "" {
		fmt.Printf("  note:      %s\n", status.UnavailableReason)
	}
	if status.ResolutionStrategy != "" {
		fmt.Printf("  resolve:   %s\n", status.ResolutionStrategy)
	}
	if status.UpstreamLatestVersion != "" && status.UpstreamLatestVersion != latestVersion {
		fmt.Printf("  upstream:  %s\n", status.UpstreamLatestVersion)
	}
	fmt.Printf("  status:    %s\n", toolStatusSummary(status, installedVersion, latestVersion))
}
