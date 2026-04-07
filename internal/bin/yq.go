package bin

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
)

const yqLatestReleaseURL = "https://api.github.com/repos/mikefarah/yq/releases/latest"

func resolveYqLatest(ctx context.Context, spec ToolSpec) (*ResolvedArtifact, error) {
	data, err := fetchBytes(ctx, yqLatestReleaseURL)
	if err != nil {
		return nil, err
	}

	var release githubRelease
	if err := json.Unmarshal(data, &release); err != nil {
		return nil, fmt.Errorf("failed to parse yq release metadata: %w", err)
	}

	assetName, err := yqAssetNameForCurrentPlatform()
	if err != nil {
		return nil, err
	}

	var selected *githubReleaseAsset
	checksumsURL := ""
	for i := range release.Assets {
		asset := &release.Assets[i]
		if asset.Name == assetName {
			selected = asset
		}
		if strings.EqualFold(asset.Name, "checksums") {
			checksumsURL = asset.BrowserDownloadURL
		}
	}
	if selected == nil {
		return nil, fmt.Errorf("no compatible yq asset found for %s/%s (expected %s)", runtime.GOOS, runtime.GOARCH, assetName)
	}
	if strings.TrimSpace(selected.BrowserDownloadURL) == "" {
		return nil, fmt.Errorf("release metadata for yq asset %s is missing a download URL", selected.Name)
	}

	version := strings.TrimPrefix(strings.TrimSpace(release.TagName), "v")
	if version == "" {
		version = strings.TrimPrefix(strings.TrimSpace(release.Name), "v")
	}
	if version == "" {
		return nil, fmt.Errorf("release metadata for yq is missing a version tag")
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
		ManifestURL:    yqLatestReleaseURL,
		AssetName:      selected.Name,
		DownloadURL:    selected.BrowserDownloadURL,
		ArchiveType:    ArchiveTypeBinary,
		BinaryPath:     spec.InstalledFilename(),
		ChecksumSHA256: checksum,
	}, nil
}

func yqAssetNameForCurrentPlatform() (string, error) {
	var osName string
	switch runtime.GOOS {
	case "darwin":
		osName = "darwin"
	case "linux":
		osName = "linux"
	case "windows":
		osName = "windows"
	case "freebsd":
		osName = "freebsd"
	default:
		return "", fmt.Errorf("unsupported platform for yq: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	var archName string
	switch runtime.GOARCH {
	case "amd64":
		archName = "amd64"
	case "arm64":
		archName = "arm64"
	case "386":
		archName = "386"
	case "arm":
		archName = "arm"
	default:
		return "", fmt.Errorf("unsupported architecture for yq: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	return fmt.Sprintf("yq_%s_%s", osName, archName), nil
}
