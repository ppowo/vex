package bin

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
)

const sccLatestReleaseURL = "https://api.github.com/repos/boyter/scc/releases/latest"

func resolveSCCLatest(ctx context.Context, spec ToolSpec) (*ResolvedArtifact, error) {
	data, err := fetchBytes(ctx, sccLatestReleaseURL)
	if err != nil {
		return nil, err
	}

	var release githubRelease
	if err := json.Unmarshal(data, &release); err != nil {
		return nil, fmt.Errorf("failed to parse scc release metadata: %w", err)
	}

	assetName, err := sccAssetNameForCurrentPlatform()
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
		if strings.EqualFold(asset.Name, "checksums.txt") {
			checksumsURL = asset.BrowserDownloadURL
		}
	}
	if selected == nil {
		return nil, fmt.Errorf("no compatible scc asset found for %s/%s (expected %s)", runtime.GOOS, runtime.GOARCH, assetName)
	}
	if strings.TrimSpace(selected.BrowserDownloadURL) == "" {
		return nil, fmt.Errorf("release metadata for scc asset %s is missing a download URL", selected.Name)
	}

	version := strings.TrimPrefix(strings.TrimSpace(release.TagName), "v")
	if version == "" {
		version = strings.TrimPrefix(strings.TrimSpace(release.Name), "v")
	}
	if version == "" {
		return nil, fmt.Errorf("release metadata for scc is missing a version tag")
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
		ManifestURL:    sccLatestReleaseURL,
		AssetName:      selected.Name,
		DownloadURL:    selected.BrowserDownloadURL,
		ArchiveType:    DetectArchiveType(selected.Name),
		BinaryPath:     spec.InstalledFilename(),
		ChecksumSHA256: checksum,
	}, nil
}

func sccAssetNameForCurrentPlatform() (string, error) {
	var osName string
	switch runtime.GOOS {
	case "darwin":
		osName = "Darwin"
	case "linux":
		osName = "Linux"
	case "windows":
		osName = "Windows"
	default:
		return "", fmt.Errorf("unsupported platform for scc: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	var archName string
	switch runtime.GOARCH {
	case "arm64":
		archName = "arm64"
	case "amd64":
		archName = "x86_64"
	case "386":
		archName = "i386"
	default:
		return "", fmt.Errorf("unsupported architecture for scc: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}

	return fmt.Sprintf("scc_%s_%s%s", osName, archName, ext), nil
}
