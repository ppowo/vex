package bin

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
)

const ripgrepLatestReleaseURL = "https://api.github.com/repos/BurntSushi/ripgrep/releases/latest"

func resolveRipgrepLatest(ctx context.Context, spec ToolSpec) (*ResolvedArtifact, error) {
	data, err := fetchBytes(ctx, ripgrepLatestReleaseURL)
	if err != nil {
		return nil, err
	}

	var release githubRelease
	if err := json.Unmarshal(data, &release); err != nil {
		return nil, fmt.Errorf("failed to parse ripgrep release metadata: %w", err)
	}

	releaseTag := strings.TrimSpace(release.TagName)
	if releaseTag == "" {
		releaseTag = strings.TrimSpace(release.Name)
	}
	if releaseTag == "" {
		return nil, fmt.Errorf("release metadata for ripgrep is missing a version tag")
	}

	assetName, binaryPath, err := ripgrepAssetForCurrentPlatform(releaseTag, spec.InstalledFilename())
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
		if asset.Name == assetName+".sha256" {
			checksumsURL = asset.BrowserDownloadURL
		}
	}

	if selected == nil {
		return nil, fmt.Errorf("no compatible ripgrep asset found for %s/%s (expected %s)", runtime.GOOS, runtime.GOARCH, assetName)
	}
	if strings.TrimSpace(selected.BrowserDownloadURL) == "" {
		return nil, fmt.Errorf("release metadata for ripgrep asset %s is missing a download URL", selected.Name)
	}

	checksum := normalizeSHA256Digest(selected.Digest)
	if checksum == "" && checksumsURL != "" {
		if checksumsData, err := fetchBytes(ctx, checksumsURL); err == nil {
			checksum = parseSHA256File(string(checksumsData))
		}
	}

	return &ResolvedArtifact{
		SourceType:     "github-release",
		Version:        strings.TrimPrefix(releaseTag, "v"),
		ReleaseTag:     releaseTag,
		ManifestURL:    ripgrepLatestReleaseURL,
		AssetName:      selected.Name,
		DownloadURL:    selected.BrowserDownloadURL,
		ArchiveType:    ArchiveTypeTarGz,
		BinaryPath:     binaryPath,
		ChecksumSHA256: checksum,
	}, nil
}

func ripgrepAssetForCurrentPlatform(releaseTag, installedFilename string) (string, string, error) {
	return ripgrepAssetForGOOSArch(releaseTag, runtime.GOOS, runtime.GOARCH, installedFilename)
}

func ripgrepAssetForGOOSArch(releaseTag, goos, goarch, installedFilename string) (string, string, error) {
	triple, err := ripgrepTargetTripleForGOOSArch(goos, goarch)
	if err != nil {
		return "", "", err
	}

	releaseTag = strings.TrimSpace(releaseTag)
	if releaseTag == "" {
		return "", "", fmt.Errorf("ripgrep release tag is empty")
	}

	installedFilename = strings.TrimSpace(installedFilename)
	if installedFilename == "" {
		installedFilename = "rg"
	}

	assetName := fmt.Sprintf("ripgrep-%s-%s.tar.gz", releaseTag, triple)
	binaryPath := fmt.Sprintf("ripgrep-%s-%s/%s", releaseTag, triple, installedFilename)
	return assetName, binaryPath, nil
}

func ripgrepTargetTripleForGOOSArch(goos, goarch string) (string, error) {
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
		case "arm64":
			return "aarch64-unknown-linux-musl", nil
		case "amd64":
			return "x86_64-unknown-linux-musl", nil
		}
	case "windows":
		switch goarch {
		case "arm64":
			return "aarch64-pc-windows-msvc", nil
		case "amd64":
			return "x86_64-pc-windows-msvc", nil
		}
	}

	return "", fmt.Errorf("unsupported platform for ripgrep: %s/%s", goos, goarch)
}

func parseSHA256File(data string) string {
	parts := strings.Fields(strings.TrimSpace(data))
	if len(parts) >= 1 {
		return normalizeSHA256Digest(parts[0])
	}
	return ""
}
