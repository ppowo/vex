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
	data, err := fetchBytes(ctx, nushellLatestReleaseURL)
	if err != nil {
		return nil, err
	}

	var release githubRelease
	if err := json.Unmarshal(data, &release); err != nil {
		return nil, fmt.Errorf("failed to parse nushell release metadata: %w", err)
	}

	version := strings.TrimPrefix(strings.TrimSpace(release.TagName), "v")
	if version == "" {
		version = strings.TrimPrefix(strings.TrimSpace(release.Name), "v")
	}
	if version == "" {
		return nil, fmt.Errorf("release metadata for nushell is missing a version tag")
	}

	triple, err := nushellTargetTriple()
	if err != nil {
		return nil, err
	}

	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}
	assetName := fmt.Sprintf("nu-%s-%s%s", version, triple, ext)

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
		return nil, fmt.Errorf("no compatible nushell asset found for %s/%s (expected %s)", runtime.GOOS, runtime.GOARCH, assetName)
	}
	if strings.TrimSpace(selected.BrowserDownloadURL) == "" {
		return nil, fmt.Errorf("release metadata for nushell asset %s is missing a download URL", selected.Name)
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
		ManifestURL:    nushellLatestReleaseURL,
		AssetName:      selected.Name,
		DownloadURL:    selected.BrowserDownloadURL,
		ArchiveType:    DetectArchiveType(selected.Name),
		BinaryPath:     spec.InstalledFilename(),
		ChecksumSHA256: checksum,
	}, nil
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
