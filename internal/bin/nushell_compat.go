package bin

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	toml "github.com/pelletier/go-toml/v2"
)

type ResolutionInfo struct {
	Strategy             string
	ExactVersionRequired bool
	Reason               string
	SelectedNushellMinor string
	CompatibleTools      []string
	UpstreamVersion      string
	UpstreamReleaseTag   string
}

type NushellCompatOptions struct {
	PluginTools []string
}
type normalizedNushellCompatOptions struct {
	PluginTools []string
}

type nushellCompatContextKey struct{}

type thirdPartyNuPluginSource struct {
	Owner string
	Repo  string
}

type versionedRelease struct {
	Release githubRelease
	Version string
}

type pluginCompatCandidate struct {
	Release        githubRelease
	Version        string
	SupportedMinor string
}

type pluginCompatInventory struct {
	ToolName string
	Source   thirdPartyNuPluginSource
	ByMinor  map[string][]pluginCompatCandidate
}

type nushellCompatPlan struct {
	SelectedMinor string
	Artifacts     map[string]*ResolvedArtifact
}

var thirdPartyNuPluginSources = map[string]thirdPartyNuPluginSource{
	"nu-plugin-file": {
		Owner: "fdncred",
		Repo:  "nu_plugin_file",
	},
	"nu-plugin-semver": {
		Owner: "abusch",
		Repo:  "nu_plugin_semver",
	},
}

var githubReleaseHistoryCache sync.Map
var nushellCargoMinorCache sync.Map
var pluginCompatInventoryCache sync.Map
var nushellCompatPlanCache sync.Map

func IsExternalNushellPlugin(name string) bool {
	_, ok := thirdPartyNuPluginSources[strings.ToLower(strings.TrimSpace(name))]
	return ok
}

func WithNushellCompat(ctx context.Context, opts NushellCompatOptions) (context.Context, error) {
	normalized, err := normalizeNushellCompatOptions(opts)
	if err != nil {
		return nil, err
	}
	if len(normalized.PluginTools) == 0 {
		return ctx, nil
	}
	return context.WithValue(ctx, nushellCompatContextKey{}, normalized), nil
}

func resolveNushellCompatArtifact(ctx context.Context, spec ToolSpec) (*ResolvedArtifact, bool, error) {
	opts, ok := ctx.Value(nushellCompatContextKey{}).(normalizedNushellCompatOptions)
	if !ok || len(opts.PluginTools) == 0 {
		var err error
		opts, err = defaultNushellCompatOptions()
		if err != nil {
			return nil, true, err
		}
	}
	if len(opts.PluginTools) == 0 {
		return nil, false, nil
	}
	if spec.Name != "nushell" && !containsString(opts.PluginTools, spec.Name) {
		return nil, false, nil
	}
	plan, err := loadNushellCompatPlan(ctx, opts)
	if err != nil {
		return nil, true, fmt.Errorf("failed to resolve shared Nushell compatibility plan for %s (plugins: %s): %w", spec.Name, strings.Join(opts.PluginTools, ", "), err)
	}
	artifact, ok := plan.Artifacts[spec.Name]
	if !ok {
		return nil, true, fmt.Errorf("compatibility plan did not resolve %s", spec.Name)
	}
	return cloneResolvedArtifact(artifact), true, nil
}

func normalizeNushellCompatOptions(opts NushellCompatOptions) (normalizedNushellCompatOptions, error) {
	selected := make(map[string]struct{})
	for _, toolName := range opts.PluginTools {
		toolName = strings.ToLower(strings.TrimSpace(toolName))
		if toolName == "" {
			continue
		}
		if !IsExternalNushellPlugin(toolName) {
			return normalizedNushellCompatOptions{}, fmt.Errorf("%s is not a supported external Nushell plugin for compatibility resolution", toolName)
		}
		selected[toolName] = struct{}{}
	}
	tools := make([]string, 0, len(selected))
	for toolName := range selected {
		tools = append(tools, toolName)
	}
	sort.Strings(tools)
	return normalizedNushellCompatOptions{PluginTools: tools}, nil
}

func defaultNushellCompatOptions() (normalizedNushellCompatOptions, error) {
	return normalizeNushellCompatOptions(NushellCompatOptions{PluginTools: allExternalNushellPlugins()})
}

func allExternalNushellPlugins() []string {
	tools := make([]string, 0, len(thirdPartyNuPluginSources))
	for toolName := range thirdPartyNuPluginSources {
		tools = append(tools, toolName)
	}
	sort.Strings(tools)
	return tools
}

func loadNushellCompatPlan(ctx context.Context, opts normalizedNushellCompatOptions) (*nushellCompatPlan, error) {
	cacheKey := strings.Join(opts.PluginTools, ",")
	if cached, ok := nushellCompatPlanCache.Load(cacheKey); ok {
		return cached.(*nushellCompatPlan), nil
	}

	plan, err := buildNushellCompatPlan(ctx, opts)
	if err != nil {
		return nil, err
	}
	actual, _ := nushellCompatPlanCache.LoadOrStore(cacheKey, plan)
	return actual.(*nushellCompatPlan), nil
}

func buildNushellCompatPlan(ctx context.Context, opts normalizedNushellCompatOptions) (*nushellCompatPlan, error) {
	if len(opts.PluginTools) == 0 {
		return nil, fmt.Errorf("no external Nushell plugins were selected for compatibility resolution")
	}

	inventories := make(map[string]*pluginCompatInventory, len(opts.PluginTools))
	commonMinors := make(map[string]struct{})
	for idx, toolName := range opts.PluginTools {
		inventory, err := loadPluginCompatInventory(ctx, toolName)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate Nushell compatibility for %s: %w", toolName, err)
		}
		inventories[toolName] = inventory

		minors := inventory.supportedMinorKeys()
		if len(minors) == 0 {
			return nil, fmt.Errorf("%s has no installable releases with Nushell compatibility metadata", toolName)
		}

		if idx == 0 {
			for _, minor := range minors {
				commonMinors[minor] = struct{}{}
			}
			continue
		}

		allowed := make(map[string]struct{}, len(minors))
		for _, minor := range minors {
			allowed[minor] = struct{}{}
		}
		for minor := range commonMinors {
			if _, ok := allowed[minor]; !ok {
				delete(commonMinors, minor)
			}
		}
	}

	if len(commonMinors) == 0 {
		parts := make([]string, 0, len(opts.PluginTools))
		for _, toolName := range opts.PluginTools {
			inventory := inventories[toolName]
			parts = append(parts, fmt.Sprintf("%s=[%s]", toolName, strings.Join(inventory.supportedMinorKeys(), ", ")))
		}
		return nil, fmt.Errorf("no common Nushell minor found across selected plugins (%s)", strings.Join(parts, "; "))
	}

	selectedMinor := highestMinorKey(commonMinors)
	nushellSpec, err := compatToolSpec("nushell")
	if err != nil {
		return nil, err
	}

	nushellReleases, err := fetchGitHubReleaseHistory(ctx, "nushell", "nushell")
	if err != nil {
		return nil, fmt.Errorf("failed to load Nushell release history for compatibility resolution: %w", err)
	}
	stableNushellReleases := stableVersionedReleases(nushellReleases)
	if len(stableNushellReleases) == 0 {
		return nil, fmt.Errorf("no stable Nushell releases found")
	}

	var upstreamLatest *versionedRelease
	var selectedNushell *versionedRelease
	for i := range stableNushellReleases {
		release := &stableNushellReleases[i]
		if !releaseHasNushellAsset(release.Release) {
			continue
		}
		if upstreamLatest == nil {
			upstreamLatest = release
		}
		minor, err := versionMinorKey(release.Version)
		if err != nil {
			continue
		}
		if minor == selectedMinor {
			selectedNushell = release
			break
		}
	}
	if upstreamLatest == nil {
		return nil, fmt.Errorf("no installable Nushell releases found for the current platform")
	}
	if selectedNushell == nil {
		return nil, fmt.Errorf("no Nushell release found for compatible minor %s.x", selectedMinor)
	}

	nushellArtifact, err := nushellArtifactFromRelease(ctx, nushellSpec, selectedNushell.Release)
	if err != nil {
		return nil, err
	}
	nushellArtifact.Resolution = &ResolutionInfo{
		Strategy:             "compat",
		ExactVersionRequired: true,
		Reason:               fmt.Sprintf("selected the latest Nushell %s.x release jointly supported by %s", selectedMinor, strings.Join(opts.PluginTools, ", ")),
		SelectedNushellMinor: selectedMinor,
		CompatibleTools:      cloneStringSlice(opts.PluginTools),
		UpstreamVersion:      upstreamLatest.Version,
		UpstreamReleaseTag:   upstreamLatest.Release.TagName,
	}

	artifacts := map[string]*ResolvedArtifact{
		"nushell": nushellArtifact,
	}
	for _, toolName := range opts.PluginTools {
		inventory := inventories[toolName]
		candidate, ok := inventory.latestForMinor(selectedMinor)
		if !ok {
			return nil, fmt.Errorf("no %s release found for compatible Nushell minor %s.x", toolName, selectedMinor)
		}
		spec, err := compatToolSpec(toolName)
		if err != nil {
			return nil, err
		}
		artifact, err := thirdPartyNuPluginArtifactFromRelease(ctx, spec, inventory.Source.Owner, inventory.Source.Repo, candidate.Release)
		if err != nil {
			return nil, err
		}
		artifact.Resolution = &ResolutionInfo{
			Strategy:             "compat",
			ExactVersionRequired: true,
			Reason:               fmt.Sprintf("selected the latest %s release compatible with Nushell %s.x", toolName, selectedMinor),
			SelectedNushellMinor: selectedMinor,
			CompatibleTools:      cloneStringSlice(opts.PluginTools),
		}
		artifacts[toolName] = artifact
	}

	return &nushellCompatPlan{
		SelectedMinor: selectedMinor,
		Artifacts:     artifacts,
	}, nil
}

func compatToolSpec(name string) (ToolSpec, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "nushell":
		return ToolSpec{
			Name:       "nushell",
			BinaryName: "nu",
		}, nil
	case "nu-plugin-semver":
		return ToolSpec{
			Name:       "nu-plugin-semver",
			BinaryName: "nu_plugin_semver",
		}, nil
	case "nu-plugin-file":
		return ToolSpec{
			Name:       "nu-plugin-file",
			BinaryName: "nu_plugin_file",
		}, nil
	default:
		return ToolSpec{}, fmt.Errorf("tool spec for %s is not available for compatibility resolution", name)
	}
}

func loadPluginCompatInventory(ctx context.Context, toolName string) (*pluginCompatInventory, error) {
	toolName = strings.ToLower(strings.TrimSpace(toolName))
	if cached, ok := pluginCompatInventoryCache.Load(toolName); ok {
		return cached.(*pluginCompatInventory), nil
	}

	inventory, err := buildPluginCompatInventory(ctx, toolName)
	if err != nil {
		return nil, err
	}
	actual, _ := pluginCompatInventoryCache.LoadOrStore(toolName, inventory)
	return actual.(*pluginCompatInventory), nil
}

func buildPluginCompatInventory(ctx context.Context, toolName string) (*pluginCompatInventory, error) {
	source, ok := thirdPartyNuPluginSources[toolName]
	if !ok {
		return nil, fmt.Errorf("%s is not a supported external Nushell plugin", toolName)
	}

	releases, err := fetchGitHubReleaseHistory(ctx, source.Owner, source.Repo)
	if err != nil {
		return nil, fmt.Errorf("failed to load release history for %s (%s/%s): %w", toolName, source.Owner, source.Repo, err)
	}

	inventory := &pluginCompatInventory{
		ToolName: toolName,
		Source:   source,
		ByMinor:  make(map[string][]pluginCompatCandidate),
	}

	var firstErr error
	for _, release := range stableVersionedReleases(releases) {
		if !releaseHasThirdPartyNuPluginAsset(source.Owner, source.Repo, release.Release) {
			continue
		}
		if strings.TrimSpace(release.Release.TagName) == "" {
			continue
		}

		supportedMinor, err := thirdPartyNuPluginSupportedMinorForTag(ctx, source.Owner, source.Repo, release.Release.TagName)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		inventory.ByMinor[supportedMinor] = append(inventory.ByMinor[supportedMinor], pluginCompatCandidate{
			Release:        release.Release,
			Version:        release.Version,
			SupportedMinor: supportedMinor,
		})
	}

	for minor := range inventory.ByMinor {
		sort.Slice(inventory.ByMinor[minor], func(i, j int) bool {
			return CompareVersions(inventory.ByMinor[minor][i].Version, inventory.ByMinor[minor][j].Version) > 0
		})
	}

	if len(inventory.ByMinor) == 0 {
		if firstErr != nil {
			return nil, fmt.Errorf("failed to resolve compatible releases for %s: %w", toolName, firstErr)
		}
		return nil, fmt.Errorf("no installable releases with Nushell compatibility metadata were found for %s", toolName)
	}

	return inventory, nil
}

func (i *pluginCompatInventory) supportedMinorKeys() []string {
	keys := make([]string, 0, len(i.ByMinor))
	for key := range i.ByMinor {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(a, b int) bool {
		return compareMinorKeys(keys[a], keys[b]) > 0
	})
	return keys
}

func (i *pluginCompatInventory) latestForMinor(minor string) (pluginCompatCandidate, bool) {
	candidates := i.ByMinor[minor]
	if len(candidates) == 0 {
		return pluginCompatCandidate{}, false
	}
	return candidates[0], true
}

func fetchGitHubReleaseHistory(ctx context.Context, owner, repo string) ([]githubRelease, error) {
	cacheKey := owner + "/" + repo
	if cached, ok := githubReleaseHistoryCache.Load(cacheKey); ok {
		return cloneGitHubReleases(cached.([]githubRelease)), nil
	}

	var all []githubRelease
	for page := 1; ; page++ {
		url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?per_page=100&page=%d", owner, repo, page)
		data, err := fetchBytes(ctx, url)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch release history page %d for %s/%s: %w", page, owner, repo, err)
		}

		var batch []githubRelease
		if err := json.Unmarshal(data, &batch); err != nil {
			return nil, fmt.Errorf("failed to parse release metadata for %s/%s: %w", owner, repo, err)
		}
		if len(batch) == 0 {
			break
		}

		all = append(all, batch...)
		if len(batch) < 100 {
			break
		}
	}

	githubReleaseHistoryCache.Store(cacheKey, cloneGitHubReleases(all))
	return cloneGitHubReleases(all), nil
}

func stableVersionedReleases(releases []githubRelease) []versionedRelease {
	filtered := make([]versionedRelease, 0, len(releases))
	for _, release := range releases {
		if release.Draft || release.Prerelease {
			continue
		}
		version, err := githubReleaseVersion(release)
		if err != nil {
			continue
		}
		filtered = append(filtered, versionedRelease{Release: release, Version: version})
	}
	sort.Slice(filtered, func(i, j int) bool {
		return CompareVersions(filtered[i].Version, filtered[j].Version) > 0
	})
	return filtered
}

func thirdPartyNuPluginSupportedMinorForTag(ctx context.Context, owner, repo, tag string) (string, error) {
	cacheKey := owner + "/" + repo + "@" + tag
	if cached, ok := nushellCargoMinorCache.Load(cacheKey); ok {
		return cached.(string), nil
	}

	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/Cargo.toml", owner, repo, tag)
	data, err := fetchBytes(ctx, url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch Cargo.toml for %s/%s@%s: %w", owner, repo, tag, err)
	}
	minor, err := parseNushellMinorFromCargoToml(data)
	if err != nil {
		return "", fmt.Errorf("failed to infer Nushell compatibility for %s/%s@%s: %w", owner, repo, tag, err)
	}
	nushellCargoMinorCache.Store(cacheKey, minor)
	return minor, nil
}

func parseNushellMinorFromCargoToml(data []byte) (string, error) {
	var manifest map[string]any
	if err := toml.Unmarshal(data, &manifest); err != nil {
		return "", fmt.Errorf("failed to parse Cargo.toml: %w", err)
	}

	deps, ok := manifest["dependencies"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("Cargo.toml is missing a [dependencies] table")
	}

	candidateKeys := []string{"nu-plugin", "nu-protocol", "nu-path", "nu-engine", "nu-parser", "nu-cmd-base"}
	minors := make(map[string]struct{})
	for _, depName := range candidateKeys {
		rawDep, ok := deps[depName]
		if !ok {
			continue
		}
		version, ok := cargoDependencyVersion(rawDep)
		if !ok || strings.TrimSpace(version) == "" {
			continue
		}
		minor, err := inferNushellMinorFromConstraint(version)
		if err != nil {
			return "", fmt.Errorf("dependency %s: %w", depName, err)
		}
		minors[minor] = struct{}{}
	}

	if len(minors) == 0 {
		return "", fmt.Errorf("no Nushell dependency versions were found in [dependencies]")
	}
	if len(minors) > 1 {
		keys := make([]string, 0, len(minors))
		for minor := range minors {
			keys = append(keys, minor)
		}
		sort.Slice(keys, func(i, j int) bool {
			return compareMinorKeys(keys[i], keys[j]) > 0
		})
		return "", fmt.Errorf("dependencies reference multiple Nushell minors: %s", strings.Join(keys, ", "))
	}
	for minor := range minors {
		return minor, nil
	}
	return "", fmt.Errorf("no Nushell compatibility information found")
}

func cargoDependencyVersion(raw any) (string, bool) {
	switch value := raw.(type) {
	case string:
		return value, true
	case map[string]any:
		version, ok := value["version"].(string)
		return version, ok
	default:
		return "", false
	}
}

func inferNushellMinorFromConstraint(raw string) (string, error) {
	matches := versionPattern.FindAllString(raw, -1)
	if len(matches) == 0 {
		return "", fmt.Errorf("could not find a semantic version in %q", raw)
	}

	minors := make(map[string]struct{})
	for _, match := range matches {
		minor, err := versionMinorKey(strings.TrimPrefix(match, "v"))
		if err != nil {
			return "", err
		}
		minors[minor] = struct{}{}
	}

	if len(minors) > 1 {
		keys := make([]string, 0, len(minors))
		for minor := range minors {
			keys = append(keys, minor)
		}
		sort.Slice(keys, func(i, j int) bool {
			return compareMinorKeys(keys[i], keys[j]) > 0
		})
		return "", fmt.Errorf("constraint %q spans multiple Nushell minors (%s)", raw, strings.Join(keys, ", "))
	}
	for minor := range minors {
		return minor, nil
	}
	return "", fmt.Errorf("could not infer a Nushell minor from %q", raw)
}

func githubReleaseVersion(release githubRelease) (string, error) {
	version := strings.TrimPrefix(strings.TrimSpace(release.TagName), "v")
	if version == "" {
		version = strings.TrimPrefix(strings.TrimSpace(release.Name), "v")
	}
	if version == "" {
		return "", fmt.Errorf("release metadata is missing a version tag")
	}
	return version, nil
}

func versionMinorKey(version string) (string, error) {
	parsed, ok := parseSemVersion(version)
	if !ok {
		return "", fmt.Errorf("invalid semantic version %q", version)
	}
	return fmt.Sprintf("%d.%d", parsed.Major(), parsed.Minor()), nil
}

func compareMinorKeys(a, b string) int {
	return CompareVersions(a+".0", b+".0")
}

func highestMinorKey(keys map[string]struct{}) string {
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Slice(ordered, func(i, j int) bool {
		return compareMinorKeys(ordered[i], ordered[j]) > 0
	})
	if len(ordered) == 0 {
		return ""
	}
	return ordered[0]
}

func cloneResolvedArtifact(artifact *ResolvedArtifact) *ResolvedArtifact {
	if artifact == nil {
		return nil
	}
	clone := *artifact
	if artifact.Resolution != nil {
		resolution := *artifact.Resolution
		resolution.CompatibleTools = cloneStringSlice(artifact.Resolution.CompatibleTools)
		clone.Resolution = &resolution
	}
	return &clone
}

func cloneStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	return append([]string(nil), values...)
}

func cloneGitHubReleases(releases []githubRelease) []githubRelease {
	if len(releases) == 0 {
		return nil
	}
	cloned := make([]githubRelease, len(releases))
	copy(cloned, releases)
	return cloned
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
