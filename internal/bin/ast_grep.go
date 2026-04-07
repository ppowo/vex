package bin

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
)

const astGrepLatestReleaseURL = "https://api.github.com/repos/ast-grep/ast-grep/releases/latest"

func resolveAstGrepLatest(ctx context.Context, spec ToolSpec) (*ResolvedArtifact, error) {
	data, err := fetchBytes(ctx, astGrepLatestReleaseURL)
	if err != nil {
		return nil, err
	}

	var release githubRelease
	if err := json.Unmarshal(data, &release); err != nil {
		return nil, fmt.Errorf("failed to parse ast-grep release metadata: %w", err)
	}

	assetName, err := astGrepAssetNameForCurrentPlatform()
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
		return nil, fmt.Errorf("no compatible ast-grep asset found for %s/%s (expected %s)", runtime.GOOS, runtime.GOARCH, assetName)
	}
	if strings.TrimSpace(selected.BrowserDownloadURL) == "" {
		return nil, fmt.Errorf("release metadata for ast-grep asset %s is missing a download URL", selected.Name)
	}

	version := strings.TrimPrefix(strings.TrimSpace(release.TagName), "v")
	if version == "" {
		version = strings.TrimPrefix(strings.TrimSpace(release.Name), "v")
	}
	if version == "" {
		return nil, fmt.Errorf("release metadata for ast-grep is missing a version tag")
	}

	return &ResolvedArtifact{
		SourceType:     "github-release",
		Version:        version,
		ReleaseTag:     release.TagName,
		ManifestURL:    astGrepLatestReleaseURL,
		AssetName:      selected.Name,
		DownloadURL:    selected.BrowserDownloadURL,
		ArchiveType:    DetectArchiveType(selected.Name),
		BinaryPath:     "ast-grep",
		ChecksumSHA256: normalizeSHA256Digest(selected.Digest),
	}, nil
}

func astGrepAssetNameForCurrentPlatform() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		switch runtime.GOARCH {
		case "arm64":
			return "app-aarch64-apple-darwin.zip", nil
		case "amd64":
			return "app-x86_64-apple-darwin.zip", nil
		}
	case "linux":
		switch runtime.GOARCH {
		case "arm64":
			return "app-aarch64-unknown-linux-gnu.zip", nil
		case "amd64":
			return "app-x86_64-unknown-linux-gnu.zip", nil
		}
	case "windows":
		switch runtime.GOARCH {
		case "arm64":
			return "app-aarch64-pc-windows-msvc.zip", nil
		case "amd64":
			return "app-x86_64-pc-windows-msvc.zip", nil
		case "386":
			return "app-i686-pc-windows-msvc.zip", nil
		}
	}

	return "", fmt.Errorf("unsupported platform for ast-grep: %s/%s", runtime.GOOS, runtime.GOARCH)
}
