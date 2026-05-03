package bin

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	semver "github.com/Masterminds/semver/v3"
	vexpaths "github.com/pun/vex/internal/paths"
)

const downloadTimeout = 30 * time.Second

var versionPattern = regexp.MustCompile(`v?\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.\-]+)?`)

type InstallResult struct {
	State    *ToolState
	Artifact *ResolvedArtifact
}

type UpdateResult struct {
	State           *ToolState
	Artifact        *ResolvedArtifact
	Updated         bool
	PreviousVersion string
}

type ToolStatus struct {
	Spec                     ToolSpec
	Managed                  bool
	Path                     string
	Exists                   bool
	Executable               bool
	Available                bool
	UnavailableReason        string
	StoredVersion            string
	RuntimeVersion           string
	LatestVersion            string
	LatestReleaseTag         string
	UpdateAvailable          bool
	LatestError              error
	ResolutionStrategy       string
	ResolutionReason         string
	ResolutionExactVersion   bool
	VersionChangeRequired    bool
	UpstreamLatestVersion    string
	UpstreamLatestReleaseTag string
	SelectedNushellMinor     string
	CompatibleTools          []string
}

func (s *ToolStatus) EffectiveInstalledVersion() string {
	if s.RuntimeVersion != "" {
		return s.RuntimeVersion
	}
	return s.StoredVersion
}

func GetManagedToolPath(spec ToolSpec) (string, error) {
	binDir, err := vexpaths.ManagedBinDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(binDir, spec.InstalledFilename()), nil
}

func unavailableToolError(spec ToolSpec, status *ToolStatus) error {
	reason := "unavailable"
	if status != nil && strings.TrimSpace(status.UnavailableReason) != "" {
		reason = strings.TrimSpace(status.UnavailableReason)
	}
	return fmt.Errorf("%s is unavailable: %s", spec.Name, reason)
}

func LocalToolStatus(spec ToolSpec) (*ToolStatus, error) {
	state, err := LoadState()
	if err != nil {
		return nil, err
	}

	path, err := GetManagedToolPath(spec)
	if err != nil {
		return nil, err
	}

	status := &ToolStatus{
		Spec:      spec,
		Path:      path,
		Available: true,
	}
	if spec.AvailabilityCheck != nil {
		if err := spec.AvailabilityCheck(); err != nil {
			status.Available = false
			status.UnavailableReason = strings.TrimSpace(err.Error())
			if status.UnavailableReason == "" {
				status.UnavailableReason = "unavailable"
			}
		}
	}

	if toolState, ok := state.Tools[spec.Name]; ok && toolState != nil && toolState.Installed {
		status.Managed = true
		status.StoredVersion = toolState.InstalledVersion
		if toolState.Path != "" {
			status.Path = toolState.Path
		}
	}

	info, err := os.Stat(status.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return status, nil
		}
		return nil, fmt.Errorf("failed to inspect %s: %w", status.Path, err)
	}

	status.Exists = true
	status.Executable = isExecutable(info, status.Path)
	if version, err := ProbeVersion(spec, status.Path); err == nil {
		status.RuntimeVersion = version
	}

	return status, nil
}

func InspectTool(ctx context.Context, spec ToolSpec) (*ToolStatus, error) {
	status, err := LocalToolStatus(spec)
	if err != nil {
		return nil, err
	}
	artifact, err := spec.ResolveLatest(ctx, spec)
	if err != nil {
		status.LatestError = err
		return status, nil
	}
	status.LatestVersion = artifact.Version
	status.LatestReleaseTag = artifact.ReleaseTag
	if artifact.Resolution != nil {
		status.ResolutionStrategy = artifact.Resolution.Strategy
		status.ResolutionReason = artifact.Resolution.Reason
		status.ResolutionExactVersion = artifact.Resolution.ExactVersionRequired
		status.UpstreamLatestVersion = artifact.Resolution.UpstreamVersion
		status.UpstreamLatestReleaseTag = artifact.Resolution.UpstreamReleaseTag
		status.SelectedNushellMinor = artifact.Resolution.SelectedNushellMinor
		status.CompatibleTools = cloneStringSlice(artifact.Resolution.CompatibleTools)
	}
	installedVersion := status.EffectiveInstalledVersion()
	if installedVersion != "" && artifact.Version != "" {
		comparison := CompareVersions(installedVersion, artifact.Version)
		status.UpdateAvailable = comparison < 0
		if status.ResolutionExactVersion {
			status.VersionChangeRequired = comparison != 0
		}
	}

	return status, nil
}

func InstallTool(ctx context.Context, spec ToolSpec, force bool) (*InstallResult, error) {
	status, err := LocalToolStatus(spec)
	if err != nil {
		return nil, err
	}
	if !status.Available {
		return nil, unavailableToolError(spec, status)
	}

	if status.Exists && !status.Managed && !force {
		return nil, fmt.Errorf("%s already exists at %s but is not managed by vex; rerun with --force to overwrite it", spec.Name, status.Path)
	}
	if status.Exists && status.Managed && !force {
		return nil, fmt.Errorf("%s is already installed at %s; use 'vex bin update %s' or rerun with --force", spec.Name, status.Path, spec.Name)
	}

	artifact, err := spec.ResolveLatest(ctx, spec)
	if err != nil {
		return nil, err
	}

	toolState, err := installResolvedTool(ctx, spec, artifact)
	if err != nil {
		return nil, err
	}

	return &InstallResult{State: toolState, Artifact: artifact}, nil
}

func UpdateTool(ctx context.Context, spec ToolSpec, force bool) (*UpdateResult, error) {
	status, err := LocalToolStatus(spec)
	if err != nil {
		return nil, err
	}
	if !status.Available {
		return nil, unavailableToolError(spec, status)
	}

	if !status.Managed {
		if status.Exists {
			return nil, fmt.Errorf("%s exists at %s but is not managed by vex; use 'vex bin install %s --force' if you want vex to take it over", spec.Name, status.Path, spec.Name)
		}
		return nil, fmt.Errorf("%s is not installed; use 'vex bin install %s' first", spec.Name, spec.Name)
	}

	artifact, err := spec.ResolveLatest(ctx, spec)
	if err != nil {
		return nil, err
	}

	previousVersion := status.EffectiveInstalledVersion()
	strictVersion := artifact.Resolution != nil && artifact.Resolution.ExactVersionRequired
	if status.Exists && previousVersion != "" && artifact.Version != "" && !force {
		comparison := CompareVersions(previousVersion, artifact.Version)
		if (!strictVersion && comparison >= 0) || (strictVersion && comparison == 0) {
			return &UpdateResult{
				State: &ToolState{
					Installed:        true,
					Path:             status.Path,
					InstalledVersion: previousVersion,
				},
				Artifact:        artifact,
				Updated:         false,
				PreviousVersion: previousVersion,
			}, nil
		}
	}

	toolState, err := installResolvedTool(ctx, spec, artifact)
	if err != nil {
		return nil, err
	}

	return &UpdateResult{
		State:           toolState,
		Artifact:        artifact,
		Updated:         true,
		PreviousVersion: previousVersion,
	}, nil
}

func ProbeVersion(spec ToolSpec, binaryPath string) (string, error) {
	if len(spec.VersionArgs) == 0 {
		return "", fmt.Errorf("no version arguments configured for %s", spec.Name)
	}

	cmd := exec.Command(binaryPath, spec.VersionArgs...)
	output, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if text == "" {
		if err != nil {
			return "", fmt.Errorf("version command failed: %w", err)
		}
		return "", fmt.Errorf("version command produced no output")
	}

	if match := versionPattern.FindString(text); match != "" {
		return strings.TrimPrefix(match, "v"), nil
	}

	line := text
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		line = text[:idx]
	}
	return strings.TrimSpace(line), nil
}

func CompareVersions(a, b string) int {
	if a == b {
		return 0
	}

	va, okA := parseSemVersion(a)
	vb, okB := parseSemVersion(b)
	if !okA || !okB {
		return strings.Compare(strings.TrimSpace(a), strings.TrimSpace(b))
	}

	if va.Equal(vb) {
		return 0
	}
	if va.LessThan(vb) {
		return -1
	}
	return 1
}

func parseSemVersion(raw string) (*semver.Version, bool) {
	candidate := versionPattern.FindString(strings.TrimSpace(raw))
	if candidate == "" {
		candidate = strings.TrimSpace(raw)
	}
	if candidate == "" {
		return nil, false
	}

	version, err := semver.NewVersion(candidate)
	if err != nil {
		return nil, false
	}
	return version, true
}

func installResolvedTool(ctx context.Context, spec ToolSpec, artifact *ResolvedArtifact) (*ToolState, error) {
	binDir, err := vexpaths.ManagedBinDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create managed bin directory: %w", err)
	}

	targetPath, err := GetManagedToolPath(spec)
	if err != nil {
		return nil, err
	}
	tempDir, err := os.MkdirTemp("", "vex-bin-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir)
	assetName := artifact.AssetName
	if assetName == "" {
		assetName = spec.InstalledFilename()
	}
	downloadPath := filepath.Join(tempDir, assetName)
	if err := downloadFile(ctx, artifact.DownloadURL, downloadPath); err != nil {
		return nil, err
	}
	if err := verifySHA256(downloadPath, artifact.ChecksumSHA256); err != nil {
		return nil, err
	}
	if artifact.ArchiveType == ArchiveTypeBinary && (len(spec.BundledBinaries) > 0 || len(spec.BundledFiles) > 0) {
		return nil, fmt.Errorf("%s declares bundled files but latest artifact is a standalone binary", spec.Name)
	}
	binaryPath, err := materializeBinary(downloadPath, tempDir, *artifact)
	if err != nil {
		return nil, err
	}
	if err := installBinary(binaryPath, targetPath); err != nil {
		return nil, fmt.Errorf("failed to install %s: %w", spec.InstalledFilename(), err)
	}

	for _, bundledBinary := range spec.BundledBinaries {
		bundledArtifact := *artifact
		bundledArtifact.BinaryPath = binaryInstallFilename(bundledBinary)

		bundledPath, err := materializeBinary(downloadPath, tempDir, bundledArtifact)
		if err != nil {
			return nil, fmt.Errorf("failed to materialize bundled binary %s: %w", bundledBinary, err)
		}

		bundledTarget := filepath.Join(binDir, binaryInstallFilename(bundledBinary))
		if err := installBinary(bundledPath, bundledTarget); err != nil {
			return nil, fmt.Errorf("failed to install bundled binary %s: %w", bundledBinary, err)
		}
	}
	for _, bundledFile := range spec.BundledFiles {
		bundledArtifact := *artifact
		bundledArtifact.BinaryPath = bundledFile

		bundledPath, err := materializeBinary(downloadPath, tempDir, bundledArtifact)
		if err != nil {
			return nil, fmt.Errorf("failed to materialize bundled file %s: %w", bundledFile, err)
		}

		bundledTarget := filepath.Join(binDir, bundledFile)
		if err := installFile(bundledPath, bundledTarget, false); err != nil {
			return nil, fmt.Errorf("failed to install bundled file %s: %w", bundledFile, err)
		}
	}
	if spec.FinalizeInstall != nil {
		if err := spec.FinalizeInstall(InstallLayout{
			BinDir:     binDir,
			TargetPath: targetPath,
			Artifact:   cloneResolvedArtifact(artifact),
		}); err != nil {
			return nil, fmt.Errorf("failed to finalize install for %s: %w", spec.Name, err)
		}
	}

	state, err := LoadState()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	installedAt := now
	if existing, ok := state.Tools[spec.Name]; ok && existing != nil && !existing.InstalledAt.IsZero() {
		installedAt = existing.InstalledAt
	}

	toolState := &ToolState{
		Installed:        true,
		Path:             targetPath,
		InstalledVersion: artifact.Version,
		InstalledAt:      installedAt,
		UpdatedAt:        now,
		Artifact: ArtifactState{
			SourceType:  artifact.SourceType,
			ManifestURL: artifact.ManifestURL,
			ReleaseTag:  artifact.ReleaseTag,
			AssetName:   artifact.AssetName,
			DownloadURL: artifact.DownloadURL,
			Checksum:    artifact.ChecksumSHA256,
		},
	}
	state.Tools[spec.Name] = toolState
	if err := state.Save(); err != nil {
		return nil, err
	}

	return toolState, nil
}

func installBinary(binaryPath, targetPath string) error {
	return installFile(binaryPath, targetPath, true)
}

func installFile(sourcePath, targetPath string, executable bool) error {
	tempTarget := targetPath + ".vex-tmp"
	if err := copyFile(sourcePath, tempTarget); err != nil {
		return fmt.Errorf("failed to stage file: %w", err)
	}
	if runtime.GOOS != "windows" {
		mode := os.FileMode(0644)
		if executable {
			mode = 0755
		}
		if err := os.Chmod(tempTarget, mode); err != nil {
			_ = os.Remove(tempTarget)
			return fmt.Errorf("failed to set file mode: %w", err)
		}
	}

	if runtime.GOOS == "windows" {
		_ = os.Remove(targetPath)
	}
	if err := os.Rename(tempTarget, targetPath); err != nil {
		_ = os.Remove(tempTarget)
		return fmt.Errorf("failed to install file: %w", err)
	}
	return nil
}

func materializeBinary(downloadPath, tempDir string, artifact ResolvedArtifact) (string, error) {
	extractDir := filepath.Join(tempDir, "extract")
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create extract directory: %w", err)
	}

	binaryPath := artifact.BinaryPath
	if binaryPath == "" {
		binaryPath = filepath.Base(downloadPath)
	}

	switch artifact.ArchiveType {
	case ArchiveTypeBinary:
		return downloadPath, nil
	case ArchiveTypeZip:
		if err := extractZipArchive(downloadPath, extractDir); err != nil {
			return "", err
		}
	case ArchiveTypeTarGz, ArchiveTypeTarXz:
		if err := extractTarArchive(downloadPath, extractDir, artifact.ArchiveType); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unsupported archive type: %s", artifact.ArchiveType)
	}

	resolvedPath := filepath.Join(extractDir, filepath.FromSlash(binaryPath))
	if info, err := os.Stat(resolvedPath); err == nil && !info.IsDir() {
		return resolvedPath, nil
	}

	baseName := filepath.Base(binaryPath)
	matches := make([]string, 0, 1)
	err := filepath.WalkDir(extractDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Base(path) == baseName {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to locate extracted binary: %w", err)
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("could not find %s in extracted archive", binaryPath)
	}
	return matches[0], nil
}

func extractTarArchive(src, dest string, archiveType ArchiveType) error {
	args := []string{"-xf", src, "-C", dest}
	switch archiveType {
	case ArchiveTypeTarXz:
		args = []string{"-xJf", src, "-C", dest}
	case ArchiveTypeTarGz:
		args = []string{"-xzf", src, "-C", dest}
	}

	cmd := exec.Command("tar", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if len(output) > 0 {
			return fmt.Errorf("failed to extract archive with tar: %w: %s", err, strings.TrimSpace(string(output)))
		}
		return fmt.Errorf("failed to extract archive with tar: %w", err)
	}
	return nil
}

func extractZipArchive(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return fmt.Errorf("failed to open zip archive: %w", err)
	}
	defer r.Close()

	cleanDest := filepath.Clean(dest) + string(os.PathSeparator)
	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)
		cleanPath := filepath.Clean(fpath)
		if !strings.HasPrefix(cleanPath, cleanDest) && cleanPath != filepath.Clean(dest) {
			return fmt.Errorf("illegal file path in zip archive: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(cleanPath, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", cleanPath, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(cleanPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", cleanPath, err)
		}

		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("failed to open file %s in zip archive: %w", f.Name, err)
		}

		mode := f.Mode()
		if mode == 0 {
			mode = 0644
		}
		outFile, err := os.OpenFile(cleanPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
		if err != nil {
			rc.Close()
			return fmt.Errorf("failed to create extracted file %s: %w", cleanPath, err)
		}

		_, copyErr := io.Copy(outFile, rc)
		closeErr := outFile.Close()
		rc.Close()
		if copyErr != nil {
			return fmt.Errorf("failed to extract %s: %w", f.Name, copyErr)
		}
		if closeErr != nil {
			return fmt.Errorf("failed to finalize %s: %w", cleanPath, closeErr)
		}
	}

	return nil
}

func downloadFile(ctx context.Context, url, dest string) error {
	client := &http.Client{Timeout: downloadTimeout}
	ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create download request: %w", err)
	}
	req.Header.Set("User-Agent", "vex-bin/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download %s: unexpected status %s", url, resp.Status)
	}

	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", dest, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("failed to write %s: %w", dest, err)
	}
	return nil
}

func fetchBytes(ctx context.Context, url string) ([]byte, error) {
	if rateLimitErr := activeGitHubRateLimit(url); rateLimitErr != nil {
		return nil, rateLimitErr
	}
	client := &http.Client{Timeout: downloadTimeout}
	ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "vex-bin/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if rateLimitErr := githubRateLimitErrorFromResponse(resp, url, body); rateLimitErr != nil {
			rememberGitHubRateLimit(rateLimitErr)
			return nil, rateLimitErr
		}
		return nil, fmt.Errorf("failed to fetch %s: unexpected status %s", url, resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	return data, nil
}

func verifySHA256(path, expected string) error {
	if strings.TrimSpace(expected) == "" {
		return nil
	}

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open %s for checksum verification: %w", path, err)
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return fmt.Errorf("failed to calculate checksum for %s: %w", path, err)
	}

	actual := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(actual, strings.TrimSpace(expected)) {
		return fmt.Errorf("checksum mismatch for %s: expected %s, got %s", filepath.Base(path), expected, actual)
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}

	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(dst)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(dst)
		return closeErr
	}
	return nil
}

func isExecutable(info os.FileInfo, path string) bool {
	if runtime.GOOS == "windows" {
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".exe", ".bat", ".cmd", ".com":
			return true
		default:
			return false
		}
	}
	return info.Mode().IsRegular() && info.Mode()&0111 != 0
}

func CurrentTargetTriple() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		switch runtime.GOARCH {
		case "arm64":
			return "aarch64-apple-darwin", nil
		case "amd64":
			return "x86_64-apple-darwin", nil
		}
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			return "x86_64-unknown-linux-gnu", nil
		}
	case "windows":
		switch runtime.GOARCH {
		case "amd64":
			return "x86_64-pc-windows-msvc", nil
		}
	}

	return "", fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
}

func DetectArchiveType(name string) ArchiveType {
	switch {
	case strings.HasSuffix(name, ".tar.xz"):
		return ArchiveTypeTarXz
	case strings.HasSuffix(name, ".tar.gz"):
		return ArchiveTypeTarGz
	case strings.HasSuffix(name, ".zip"):
		return ArchiveTypeZip
	default:
		return ArchiveTypeBinary
	}
}

type githubRelease struct {
	TagName    string               `json:"tag_name"`
	Name       string               `json:"name"`
	Draft      bool                 `json:"draft"`
	Prerelease bool                 `json:"prerelease"`
	Assets     []githubReleaseAsset `json:"assets"`
}

type githubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest"`
}

var sha256HexPattern = regexp.MustCompile(`(?i)\b[a-f0-9]{64}\b`)

func normalizeSHA256Digest(raw string) string {
	raw = strings.TrimSpace(raw)
	if idx := strings.Index(raw, ":"); idx >= 0 && strings.EqualFold(raw[:idx], "sha256") {
		raw = raw[idx+1:]
	}
	return strings.TrimSpace(raw)
}

func checksumForAsset(data []byte, assetName string) string {
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, assetName) {
			continue
		}
		if checksum := sha256HexPattern.FindString(line); checksum != "" {
			return strings.ToLower(checksum)
		}
	}
	return ""
}