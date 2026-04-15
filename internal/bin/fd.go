package bin

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
)

const fdLatestReleaseURL = "https://api.github.com/repos/sharkdp/fd/releases/latest"

func resolveFDLatest(ctx context.Context, spec ToolSpec) (*ResolvedArtifact, error) {
	data, err := fetchBytes(ctx, fdLatestReleaseURL)
	if err != nil {
		return nil, err
	}

	var release githubRelease
	if err := json.Unmarshal(data, &release); err != nil {
		return nil, fmt.Errorf("failed to parse fd release metadata: %w", err)
	}

	releaseTag := strings.TrimSpace(release.TagName)
	if releaseTag == "" {
		releaseTag = strings.TrimSpace(release.Name)
	}
	if releaseTag == "" {
		return nil, fmt.Errorf("release metadata for fd is missing a version tag")
	}

	assetName, binaryPath, err := fdAssetForCurrentPlatform(releaseTag, spec.InstalledFilename())
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
		return nil, fmt.Errorf("no compatible fd asset found for %s/%s (expected %s)", runtime.GOOS, runtime.GOARCH, assetName)
	}
	if strings.TrimSpace(selected.BrowserDownloadURL) == "" {
		return nil, fmt.Errorf("release metadata for fd asset %s is missing a download URL", selected.Name)
	}

	return &ResolvedArtifact{
		SourceType:     "github-release",
		Version:        strings.TrimPrefix(releaseTag, "v"),
		ReleaseTag:     releaseTag,
		ManifestURL:    fdLatestReleaseURL,
		AssetName:      selected.Name,
		DownloadURL:    selected.BrowserDownloadURL,
		ArchiveType:    DetectArchiveType(selected.Name),
		BinaryPath:     binaryPath,
		ChecksumSHA256: normalizeSHA256Digest(selected.Digest),
	}, nil
}

func fdAssetForCurrentPlatform(releaseTag, installedFilename string) (string, string, error) {
	return fdAssetForGOOSArch(releaseTag, runtime.GOOS, runtime.GOARCH, installedFilename)
}

func fdAssetForGOOSArch(releaseTag, goos, goarch, installedFilename string) (string, string, error) {
	triple, err := fdTargetTripleForGOOSArch(goos, goarch)
	if err != nil {
		return "", "", err
	}

	releaseTag = strings.TrimSpace(releaseTag)
	if releaseTag == "" {
		return "", "", fmt.Errorf("fd release tag is empty")
	}
	installedFilename = strings.TrimSpace(installedFilename)
	if installedFilename == "" {
		installedFilename = "fd"
	}

	assetName := fmt.Sprintf("fd-%s-%s.tar.gz", releaseTag, triple)
	binaryPath := fmt.Sprintf("fd-%s-%s/%s", releaseTag, triple, installedFilename)
	return assetName, binaryPath, nil
}

func fdTargetTripleForGOOSArch(goos, goarch string) (string, error) {
	switch goos {
	case "darwin":
		switch goarch {
		case "arm64":
			return "aarch64-apple-darwin", nil
		case "amd64":
			return "x86_64-apple-darwin", nil
		}
	case "linux":
		switch goarch {
		case "amd64":
			return "x86_64-unknown-linux-gnu", nil
		}
	}

	return "", fmt.Errorf("unsupported platform for fd: %s/%s", goos, goarch)
}
