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

	fmt.Printf("%-12s %-10s %-12s %s\n", "TOOL", "STATE", "VERSION", "PATH")
	for _, spec := range tools {
		status, err := binpkg.LocalToolStatus(spec)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		state := "available"
		switch {
		case status.Managed && status.Exists:
			state = "installed"
		case status.Managed && !status.Exists:
			state = "missing"
		case !status.Managed && status.Exists:
			state = "unmanaged"
		}

		version := status.EffectiveInstalledVersion()
		if version == "" {
			version = "-"
		}

		fmt.Printf("%-12s %-10s %-12s %s\n",
			spec.Name,
			state,
			version,
			truncate(status.Path, 60),
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
	fmt.Printf("Installed version: %s\n", installedVersion)
	if status.RuntimeVersion != "" && status.StoredVersion != "" && status.RuntimeVersion != status.StoredVersion {
		fmt.Printf("Stored version:    %s\n", status.StoredVersion)
	}
	fmt.Printf("Latest version:    %s\n", latestVersion)
	if status.LatestReleaseTag != "" {
		fmt.Printf("Latest tag:        %s\n", status.LatestReleaseTag)
	}
	if status.LatestVersion != "" && installedVersion != "unknown" {
		fmt.Printf("Update available:  %s\n", yesNo(status.UpdateAvailable))
	}
}

// --- sync ---

func cmdBinSync(dryRun bool) {
	ctx := context.Background()
	tools := binpkg.AllTools()
	var installed, updated, failed int

	for _, spec := range tools {
		wasUpdate, err := syncTool(ctx, spec, dryRun)
		if err != nil {
			failed++
			fmt.Fprintf(os.Stderr, "✗ %s: %v\n", spec.Name, err)
			continue
		}
		if wasUpdate {
			updated++
		} else {
			installed++
		}
	}
	fmt.Printf("\nSummary: %d installed, %d updated, %d failed\n", installed, updated, failed)
	if failed > 0 {
		os.Exit(1)
	}
}

func syncTool(ctx context.Context, spec binpkg.ToolSpec, dryRun bool) (bool, error) {
	status, err := binpkg.LocalToolStatus(spec)
	if err != nil {
		return false, fmt.Errorf("failed to check local status: %w", err)
	}

	inspect, err := binpkg.InspectTool(ctx, spec)
	if err != nil {
		return false, fmt.Errorf("failed to check latest version: %w", err)
	}

	latestVersion := inspect.LatestVersion
	if latestVersion == "" {
		latestVersion = "unknown"
	}

	// Case 1: Not installed
	if !status.Exists {
		if dryRun {
			fmt.Printf("• %s: would install %s (not present)\n", spec.Name, latestVersion)
			return false, nil
		}
		installResult, err := binpkg.InstallTool(ctx, spec, true)
		if err != nil {
			return false, err
		}
		fmt.Printf("✓ %s: installed %s\n", spec.Name, installResult.State.InstalledVersion)
		return false, nil
	}

	// Case 2: Unmanaged binary exists - take it over
	if !status.Managed {
		if dryRun {
			fmt.Printf("• %s: would take over and install %s\n", spec.Name, latestVersion)
			return true, nil
		}
		installResult, err := binpkg.InstallTool(ctx, spec, true)
		if err != nil {
			return false, err
		}
		fmt.Printf("✓ %s: took over and installed %s\n", spec.Name, installResult.State.InstalledVersion)
		return true, nil
	}

	// Case 3: Managed and installed - update/re-install with force
	installedVersion := status.EffectiveInstalledVersion()
	if installedVersion == "" {
		installedVersion = "unknown"
	}

	if dryRun {
		if inspect.UpdateAvailable {
			fmt.Printf("• %s: would update %s → %s\n", spec.Name, installedVersion, latestVersion)
		} else {
			fmt.Printf("• %s: would re-install %s\n", spec.Name, latestVersion)
		}
		return true, nil
	}

	updateResult, err := binpkg.UpdateTool(ctx, spec, true)
	if err != nil {
		return false, err
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
	return true, nil
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
	failed := 0

	for _, spec := range tools {
		status, err := binpkg.LocalToolStatus(spec)
		if err != nil {
			failed++
			fmt.Printf("✗ %s: %v\n", spec.Name, err)
			continue
		}
		if !status.Managed {
			continue
		}

		result, err := binpkg.UpdateTool(ctx, spec, force)
		if err != nil {
			failed++
			fmt.Printf("✗ %s: %v\n", spec.Name, err)
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

	fmt.Printf("Summary: %d updated, %d already current, %d failed\n", updated, current, failed)
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
	if status.LatestVersion != "" && installedVersion != "unknown" {
		if status.UpdateAvailable {
			fmt.Println("  status:    update available")
		} else {
			fmt.Println("  status:    up to date")
		}
	} else if status.Exists {
		fmt.Println("  status:    installed")
	} else {
		fmt.Println("  status:    not installed")
	}
}
