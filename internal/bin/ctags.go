package bin

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
)

const ctagsNightlyLatestReleaseURL = "https://api.github.com/repos/universal-ctags/ctags-nightly-build/releases/latest"

func resolveCtagsLatest(ctx context.Context, spec ToolSpec) (*ResolvedArtifact, error) {
	data, err := fetchBytes(ctx, ctagsNightlyLatestReleaseURL)
	if err != nil {
		return nil, err
	}

	var release githubRelease
	if err := json.Unmarshal(data, &release); err != nil {
		return nil, fmt.Errorf("failed to parse universal-ctags release metadata: %w", err)
	}

	version := ctagsNightlyVersion(release.TagName, release.Name)
	if version == "" {
		return nil, fmt.Errorf("release metadata for universal-ctags is missing a version tag")
	}

	assetName, err := ctagsAssetNameForCurrentPlatform(version)
	if err != nil {
		return nil, err
	}

	var selected *githubReleaseAsset
	for i := range release.Assets {
		asset := &release.Assets[i]
		if asset.Name == assetName {
			selected = asset
			break
		}
	}
	if selected == nil {
		return nil, fmt.Errorf("no compatible universal-ctags asset found for %s/%s (expected %s)", runtime.GOOS, runtime.GOARCH, assetName)
	}
	if strings.TrimSpace(selected.BrowserDownloadURL) == "" {
		return nil, fmt.Errorf("release metadata for universal-ctags asset %s is missing a download URL", selected.Name)
	}

	return &ResolvedArtifact{
		SourceType:     "github-release",
		Version:        version,
		ReleaseTag:     release.TagName,
		ManifestURL:    ctagsNightlyLatestReleaseURL,
		AssetName:      selected.Name,
		DownloadURL:    selected.BrowserDownloadURL,
		ArchiveType:    DetectArchiveType(selected.Name),
		BinaryPath:     "bin/" + spec.InstalledFilename(),
		ChecksumSHA256: normalizeSHA256Digest(selected.Digest),
	}, nil
}

func ctagsNightlyVersion(tagName, releaseName string) string {
	for _, candidate := range []string{tagName, releaseName} {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if before, _, ok := strings.Cut(candidate, "+"); ok {
			candidate = before
		}
		candidate = strings.TrimPrefix(candidate, "v")
		if candidate != "" {
			return candidate
		}
	}
	return ""
}

func ctagsAssetNameForCurrentPlatform(version string) (string, error) {
	return ctagsAssetNameForGOOSArch(version, runtime.GOOS, runtime.GOARCH)
}

func ctagsAssetNameForGOOSArch(version, goos, goarch string) (string, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return "", fmt.Errorf("universal-ctags version is empty")
	}

	var osPart string
	switch goos {
	case "darwin":
		// Use the oldest published macOS target for broad runtime compatibility.
		osPart = "macos-10.15"
	case "linux":
		osPart = "linux"
	default:
		return "", fmt.Errorf("unsupported platform for universal-ctags: %s/%s", goos, goarch)
	}

	var archPart string
	switch goarch {
	case "amd64":
		archPart = "x86_64"
	case "arm64":
		archPart = "aarch64"
		if goos == "darwin" {
			archPart = "arm64"
		}
	default:
		return "", fmt.Errorf("unsupported architecture for universal-ctags: %s/%s", goos, goarch)
	}

	return fmt.Sprintf("uctags-%s-%s-%s.release.tar.xz", version, osPart, archPart), nil
}
