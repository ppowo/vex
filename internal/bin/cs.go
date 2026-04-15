package bin

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"runtime"
	"strings"
)

const csLatestReleaseURL = "https://api.github.com/repos/boyter/cs/releases/latest"

var sha256HexPattern = regexp.MustCompile(`(?i)\b[a-f0-9]{64}\b`)

type githubRelease struct {
	TagName    string               `json:"tag_name"`
	Name       string               `json:"name"`
	Draft      bool                 `json:"draft"`
	Prerelease bool                 `json:"prerelease"`
	Assets     []githubReleaseAsset `json:"assets"`
}

type githubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest"`
}

func resolveCSLatest(ctx context.Context, spec ToolSpec) (*ResolvedArtifact, error) {
	data, err := fetchBytes(ctx, csLatestReleaseURL)
	if err != nil {
		return nil, err
	}

	var release githubRelease
	if err := json.Unmarshal(data, &release); err != nil {
		return nil, fmt.Errorf("failed to parse cs release metadata: %w", err)
	}

	assetName, err := csAssetNameForCurrentPlatform()
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
		return nil, fmt.Errorf("no compatible cs asset found for %s/%s (expected %s)", runtime.GOOS, runtime.GOARCH, assetName)
	}
	if strings.TrimSpace(selected.BrowserDownloadURL) == "" {
		return nil, fmt.Errorf("release metadata for cs asset %s is missing a download URL", selected.Name)
	}

	version := strings.TrimPrefix(strings.TrimSpace(release.TagName), "v")
	if version == "" {
		version = strings.TrimPrefix(strings.TrimSpace(release.Name), "v")
	}
	if version == "" {
		return nil, fmt.Errorf("release metadata for cs is missing a version tag")
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
		ManifestURL:    csLatestReleaseURL,
		AssetName:      selected.Name,
		DownloadURL:    selected.BrowserDownloadURL,
		ArchiveType:    DetectArchiveType(selected.Name),
		BinaryPath:     spec.InstalledFilename(),
		ChecksumSHA256: checksum,
	}, nil
}

func csAssetNameForCurrentPlatform() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		switch runtime.GOARCH {
		case "arm64":
			return "cs_Darwin_arm64.tar.gz", nil
		case "amd64":
			return "cs_Darwin_x86_64.tar.gz", nil
		}
	case "linux":
		switch runtime.GOARCH {
		case "arm64":
			return "cs_Linux_arm64.tar.gz", nil
		case "amd64":
			return "cs_Linux_x86_64.tar.gz", nil
		case "386":
			return "cs_Linux_i386.tar.gz", nil
		}
	case "windows":
		switch runtime.GOARCH {
		case "arm64":
			return "cs_Windows_arm64.zip", nil
		case "amd64":
			return "cs_Windows_x86_64.zip", nil
		case "386":
			return "cs_Windows_i386.zip", nil
		}
	}

	return "", fmt.Errorf("unsupported platform for cs: %s/%s", runtime.GOOS, runtime.GOARCH)
}

func normalizeSHA256Digest(raw string) string {
	raw = strings.TrimSpace(raw)
	if idx := strings.Index(raw, ":"); idx >= 0 && strings.EqualFold(raw[:idx], "sha256") {
		raw = raw[idx+1:]
	}
	return strings.TrimSpace(raw)
}

func checksumForAsset(data []byte, assetName string) string {
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, assetName) {
			continue
		}
		if checksum := sha256HexPattern.FindString(line); checksum != "" {
			return strings.ToLower(checksum)
		}
	}
	return ""
}
