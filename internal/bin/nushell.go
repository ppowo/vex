package bin

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
)

const nushellLatestReleaseURL = "https://api.github.com/repos/nushell/nushell/releases/latest"

func resolveNushellLatest(ctx context.Context, spec ToolSpec) (*ResolvedArtifact, error) {
	if artifact, handled, err := resolveNushellCompatArtifact(ctx, spec); handled || err != nil {
		return artifact, err
	}

	data, err := fetchBytes(ctx, nushellLatestReleaseURL)
	if err != nil {
		return nil, err
	}

	var release githubRelease
	if err := json.Unmarshal(data, &release); err != nil {
		return nil, fmt.Errorf("failed to parse nushell release metadata: %w", err)
	}

	artifact, err := nushellArtifactFromRelease(ctx, spec, release)
	if err != nil {
		return nil, err
	}
	artifact.ManifestURL = nushellLatestReleaseURL
	return artifact, nil
}

func nushellArtifactFromRelease(ctx context.Context, spec ToolSpec, release githubRelease) (*ResolvedArtifact, error) {
	version, err := githubReleaseVersion(release)
	if err != nil {
		return nil, fmt.Errorf("release metadata for nushell is missing a version tag: %w", err)
	}

	selected, checksumsURL, err := selectNushellAsset(release, version)
	if err != nil {
		return nil, err
	}

	checksum := normalizeSHA256Digest(selected.Digest)
	if checksum == "" && checksumsURL != "" {
		if checksumsData, err := fetchBytes(ctx, checksumsURL); err == nil {
			checksum = checksumForAsset(checksumsData, selected.Name)
		}
	}

	return &ResolvedArtifact{
		SourceType:     "github-release",
		Version:        version,
		ReleaseTag:     release.TagName,
		ManifestURL:    nushellReleaseManifestURL(release),
		AssetName:      selected.Name,
		DownloadURL:    selected.BrowserDownloadURL,
		ArchiveType:    DetectArchiveType(selected.Name),
		BinaryPath:     spec.InstalledFilename(),
		ChecksumSHA256: checksum,
	}, nil
}

func selectNushellAsset(release githubRelease, version string) (*githubReleaseAsset, string, error) {
	assetName, err := nushellAssetName(version)
	if err != nil {
		return nil, "", err
	}
	var selected *githubReleaseAsset
	checksumsURL := ""
	for i := range release.Assets {
		asset := &release.Assets[i]
		if asset.Name == assetName {
			selected = asset
		}
		if strings.EqualFold(asset.Name, "SHA256SUMS") {
			checksumsURL = asset.BrowserDownloadURL
		}
	}
	if selected == nil {
		return nil, "", fmt.Errorf("no compatible nushell asset found for %s/%s (expected %s)", runtime.GOOS, runtime.GOARCH, assetName)
	}
	if strings.TrimSpace(selected.BrowserDownloadURL) == "" {
		return nil, "", fmt.Errorf("release metadata for nushell asset %s is missing a download URL", selected.Name)
	}
	return selected, checksumsURL, nil
}
func releaseHasNushellAsset(release githubRelease) bool {
	version, err := githubReleaseVersion(release)
	if err != nil {
		return false
	}
	assetName, err := nushellAssetName(version)
	if err != nil {
		return false
	}
	for _, asset := range release.Assets {
		if asset.Name == assetName && strings.TrimSpace(asset.BrowserDownloadURL) != "" {
			return true
		}
	}
	return false
}
func nushellReleaseManifestURL(release githubRelease) string {
	tag := strings.TrimSpace(release.TagName)
	if tag == "" {
		return nushellLatestReleaseURL
	}
	return fmt.Sprintf("https://api.github.com/repos/nushell/nushell/releases/tags/%s", tag)
}

func nushellAssetName(version string) (string, error) {
	triple, err := nushellTargetTriple()
	if err != nil {
		return "", err
	}
	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("nu-%s-%s%s", version, triple, ext), nil
}

func nushellTargetTriple() (string, error) {
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
		case "arm64":
			return "aarch64-unknown-linux-gnu", nil
		case "amd64":
			return "x86_64-unknown-linux-gnu", nil
		}
	case "windows":
		switch runtime.GOARCH {
		case "arm64":
			return "aarch64-pc-windows-msvc", nil
		case "amd64":
			return "x86_64-pc-windows-msvc", nil
		}
	}

	return "", fmt.Errorf("unsupported platform for nushell: %s/%s", runtime.GOOS, runtime.GOARCH)
}
