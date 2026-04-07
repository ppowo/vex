package bin

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
)

const shellcheckLatestReleaseURL = "https://api.github.com/repos/koalaman/shellcheck/releases/latest"

func resolveShellcheckLatest(ctx context.Context, spec ToolSpec) (*ResolvedArtifact, error) {
	data, err := fetchBytes(ctx, shellcheckLatestReleaseURL)
	if err != nil {
		return nil, err
	}

	var release githubRelease
	if err := json.Unmarshal(data, &release); err != nil {
		return nil, fmt.Errorf("failed to parse shellcheck release metadata: %w", err)
	}

	tag := strings.TrimSpace(release.TagName)
	assetName, err := shellcheckAssetNameForCurrentPlatform(tag)
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
		return nil, fmt.Errorf("no compatible shellcheck asset found for %s/%s (expected %s)", runtime.GOOS, runtime.GOARCH, assetName)
	}
	if strings.TrimSpace(selected.BrowserDownloadURL) == "" {
		return nil, fmt.Errorf("release metadata for shellcheck asset %s is missing a download URL", selected.Name)
	}

	version := strings.TrimPrefix(strings.TrimSpace(release.TagName), "v")
	if version == "" {
		version = strings.TrimPrefix(strings.TrimSpace(release.Name), "v")
	}
	if version == "" {
		return nil, fmt.Errorf("release metadata for shellcheck is missing a version tag")
	}

	return &ResolvedArtifact{
		SourceType:     "github-release",
		Version:        version,
		ReleaseTag:     release.TagName,
		ManifestURL:    shellcheckLatestReleaseURL,
		AssetName:      selected.Name,
		DownloadURL:    selected.BrowserDownloadURL,
		ArchiveType:    DetectArchiveType(selected.Name),
		BinaryPath:     tag + "/" + spec.InstalledFilename(),
		ChecksumSHA256: normalizeSHA256Digest(selected.Digest),
	}, nil
}

func shellcheckAssetNameForCurrentPlatform(tag string) (string, error) {
	var osName string
	switch runtime.GOOS {
	case "darwin":
		osName = "darwin"
	case "linux":
		osName = "linux"
	default:
		return "", fmt.Errorf("unsupported platform for shellcheck: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	var archName string
	switch runtime.GOARCH {
	case "arm64":
		archName = "aarch64"
	case "amd64":
		archName = "x86_64"
	default:
		return "", fmt.Errorf("unsupported architecture for shellcheck: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	return fmt.Sprintf("shellcheck-%s.%s.%s.tar.xz", tag, osName, archName), nil
}
