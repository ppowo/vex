package bin

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
)

const difftasticLatestReleaseURL = "https://api.github.com/repos/Wilfred/difftastic/releases/latest"

func resolveDifftasticLatest(ctx context.Context, spec ToolSpec) (*ResolvedArtifact, error) {
	data, err := fetchBytes(ctx, difftasticLatestReleaseURL)
	if err != nil {
		return nil, err
	}

	var release githubRelease
	if err := json.Unmarshal(data, &release); err != nil {
		return nil, fmt.Errorf("failed to parse difftastic release metadata: %w", err)
	}

	assetName, err := difftasticAssetNameForCurrentPlatform()
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
		if strings.EqualFold(asset.Name, "checksums_hashes_order") || strings.HasSuffix(strings.ToLower(asset.Name), "checksums") {
			checksumsURL = asset.BrowserDownloadURL
		}
	}
	if selected == nil {
		return nil, fmt.Errorf("no compatible difftastic asset found for %s/%s (expected %s)", runtime.GOOS, runtime.GOARCH, assetName)
	}
	if strings.TrimSpace(selected.BrowserDownloadURL) == "" {
		return nil, fmt.Errorf("release metadata for difftastic asset %s is missing a download URL", selected.Name)
	}

	version := strings.TrimPrefix(strings.TrimSpace(release.TagName), "v")
	if version == "" {
		version = strings.TrimPrefix(strings.TrimSpace(release.Name), "v")
	}
	if version == "" {
		return nil, fmt.Errorf("release metadata for difftastic is missing a version tag")
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
		ManifestURL:    difftasticLatestReleaseURL,
		AssetName:      selected.Name,
		DownloadURL:    selected.BrowserDownloadURL,
		ArchiveType:    DetectArchiveType(selected.Name),
		BinaryPath:     spec.InstalledFilename(),
		ChecksumSHA256: checksum,
	}, nil
}

func difftasticAssetNameForCurrentPlatform() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		switch runtime.GOARCH {
		case "arm64":
			return "difft-aarch64-apple-darwin.tar.gz", nil
		case "amd64":
			return "difft-x86_64-apple-darwin.tar.gz", nil
		}
	case "linux":
		switch runtime.GOARCH {
		case "arm64":
			return "difft-aarch64-unknown-linux-gnu.tar.gz", nil
		case "amd64":
			return "difft-x86_64-unknown-linux-gnu.tar.gz", nil
		}
	case "windows":
		switch runtime.GOARCH {
		case "arm64":
			return "difft-aarch64-pc-windows-msvc.zip", nil
		case "amd64":
			return "difft-x86_64-pc-windows-msvc.zip", nil
		}
	}

	return "", fmt.Errorf("unsupported platform for difftastic: %s/%s", runtime.GOOS, runtime.GOARCH)
}
