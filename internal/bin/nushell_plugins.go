package bin

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
)

// resolveThirdPartyNuPlugin resolves a nushell plugin released from a
// third-party GitHub repository (e.g. abusch/nu_plugin_semver,
// fdncred/nu_plugin_file).  These repos use cargo-dist style releases
// with per-platform tar.xz/zip assets and a sha256.sum file.
func resolveThirdPartyNuPlugin(owner, repo string) ResolverFunc {
	releaseURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)
	return func(ctx context.Context, spec ToolSpec) (*ResolvedArtifact, error) {
		data, err := fetchBytes(ctx, releaseURL)
		if err != nil {
			return nil, err
		}

		var release githubRelease
		if err := json.Unmarshal(data, &release); err != nil {
			return nil, fmt.Errorf("failed to parse %s release metadata: %w", spec.Name, err)
		}

		version := strings.TrimPrefix(strings.TrimSpace(release.TagName), "v")
		if version == "" {
			version = strings.TrimPrefix(strings.TrimSpace(release.Name), "v")
		}
		if version == "" {
			return nil, fmt.Errorf("release metadata for %s is missing a version tag", spec.Name)
		}

		triple, err := nuPluginTargetTriple(owner, repo)
		if err != nil {
			return nil, err
		}

		// cargo-dist style: .tar.xz on unix, .zip on windows
		ext := ".tar.xz"
		if runtime.GOOS == "windows" {
			ext = ".zip"
		}
		assetName := fmt.Sprintf("%s-%s%s", repo, triple, ext)

		var selected *githubReleaseAsset
		checksumSumURL := ""
		assetChecksumURL := ""
		for i := range release.Assets {
			asset := &release.Assets[i]
			if asset.Name == assetName {
				selected = asset
			}
			if asset.Name == "sha256.sum" {
				checksumSumURL = asset.BrowserDownloadURL
			}
			if asset.Name == assetName+".sha256" {
				assetChecksumURL = asset.BrowserDownloadURL
			}
		}
		if selected == nil {
			return nil, fmt.Errorf("no compatible %s asset found for %s/%s (expected %s)", spec.Name, runtime.GOOS, runtime.GOARCH, assetName)
		}
		if strings.TrimSpace(selected.BrowserDownloadURL) == "" {
			return nil, fmt.Errorf("release metadata for %s asset %s is missing a download URL", spec.Name, selected.Name)
		}

		checksum := normalizeSHA256Digest(selected.Digest)
		if checksum == "" && assetChecksumURL != "" {
			if csData, err := fetchBytes(ctx, assetChecksumURL); err == nil {
				// .sha256 files contain just the hash (or hash + filename)
				checksum = sha256HexPattern.FindString(string(csData))
				if checksum != "" {
					checksum = strings.ToLower(checksum)
				}
			}
		}
		if checksum == "" && checksumSumURL != "" {
			if csData, err := fetchBytes(ctx, checksumSumURL); err == nil {
				checksum = checksumForAsset(csData, selected.Name)
			}
		}

		pluginBinary := spec.BinaryName
		if runtime.GOOS == "windows" {
			pluginBinary += ".exe"
		}

		return &ResolvedArtifact{
			SourceType:     "github-release",
			Version:        version,
			ReleaseTag:     release.TagName,
			ManifestURL:    releaseURL,
			AssetName:      selected.Name,
			DownloadURL:    selected.BrowserDownloadURL,
			ArchiveType:    DetectArchiveType(selected.Name),
			BinaryPath:     pluginBinary,
			ChecksumSHA256: checksum,
		}, nil
	}
}

// nuPluginTargetTriple returns the Rust target triple used by third-party
// nushell plugin releases.  Some repos only publish a subset of targets
// and may use musl vs gnu for linux.
func nuPluginTargetTriple(owner, repo string) (string, error) {
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
			return linuxArm64Triple(owner, repo), nil
		case "amd64":
			return linuxAmd64Triple(owner, repo), nil
		}
	case "windows":
		switch runtime.GOARCH {
		case "arm64":
			return "aarch64-pc-windows-msvc", nil
		case "amd64":
			return "x86_64-pc-windows-msvc", nil
		}
	}

	return "", fmt.Errorf("unsupported platform for %s/%s: %s/%s", owner, repo, runtime.GOOS, runtime.GOARCH)
}

// linuxAmd64Triple returns the correct Linux x86_64 triple for a given repo.
// nu_plugin_semver uses musl, nu_plugin_file uses gnu.
func linuxAmd64Triple(owner, repo string) string {
	if owner == "abusch" && repo == "nu_plugin_semver" {
		return "x86_64-unknown-linux-musl"
	}
	return "x86_64-unknown-linux-gnu"
}

// linuxArm64Triple returns the correct Linux aarch64 triple for a given repo.
func linuxArm64Triple(owner, repo string) string {
	// Both repos publish aarch64-unknown-linux-gnu
	return "aarch64-unknown-linux-gnu"
}
