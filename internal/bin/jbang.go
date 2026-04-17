package bin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const jbangLatestReleaseURL = "https://api.github.com/repos/jbangdev/jbang/releases/latest"

func resolveJBangLatest(ctx context.Context, _ ToolSpec) (*ResolvedArtifact, error) {
	data, err := fetchBytes(ctx, jbangLatestReleaseURL)
	if err != nil {
		return nil, err
	}

	var release githubRelease
	if err := json.Unmarshal(data, &release); err != nil {
		return nil, fmt.Errorf("failed to parse jbang release metadata: %w", err)
	}

	version, err := githubReleaseVersion(release)
	if err != nil {
		return nil, fmt.Errorf("release metadata for jbang is missing a version tag: %w", err)
	}

	selected, checksumURL, err := selectJBangAsset(release, version)
	if err != nil {
		return nil, err
	}

	checksum := normalizeSHA256Digest(selected.Digest)
	if checksum == "" && checksumURL != "" {
		if checksumData, err := fetchBytes(ctx, checksumURL); err == nil {
			checksum = normalizeJBangChecksum(checksumData)
		}
	}

	return &ResolvedArtifact{
		SourceType:     "github-release",
		Version:        version,
		ReleaseTag:     release.TagName,
		ManifestURL:    jbangReleaseManifestURL(release.TagName),
		AssetName:      selected.Name,
		DownloadURL:    selected.BrowserDownloadURL,
		ArchiveType:    DetectArchiveType(selected.Name),
		BinaryPath:     jbangArchiveBinaryPath(version),
		ChecksumSHA256: checksum,
	}, nil
}

func selectJBangAsset(release githubRelease, version string) (*githubReleaseAsset, string, error) {
	assetName := fmt.Sprintf("jbang-%s.zip", version)
	checksumName := assetName + ".sha256"

	var selected *githubReleaseAsset
	checksumURL := ""
	for i := range release.Assets {
		asset := &release.Assets[i]
		if asset.Name == assetName {
			selected = asset
		}
		if asset.Name == checksumName {
			checksumURL = asset.BrowserDownloadURL
		}
	}
	if selected == nil {
		return nil, "", fmt.Errorf("no compatible jbang asset found (expected %s)", assetName)
	}
	if strings.TrimSpace(selected.BrowserDownloadURL) == "" {
		return nil, "", fmt.Errorf("release metadata for jbang asset %s is missing a download URL", selected.Name)
	}
	return selected, checksumURL, nil
}

func jbangReleaseManifestURL(tag string) string {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return jbangLatestReleaseURL
	}
	return fmt.Sprintf("https://api.github.com/repos/jbangdev/jbang/releases/tags/%s", tag)
}

func jbangArchiveBinaryPath(version string) string {
	return fmt.Sprintf("jbang-%s/bin/jbang", strings.TrimSpace(version))
}

func normalizeJBangChecksum(data []byte) string {
	checksum := strings.TrimSpace(string(data))
	if sha256HexPattern.MatchString(checksum) {
		return strings.ToLower(checksum)
	}
	return ""
}

func jbangAvailabilityCheck() error {
	if hasSystemJDK() {
		return nil
	}
	return fmt.Errorf("no system JDK (set JAVA_HOME or put javac on PATH)")
}

func hasSystemJDK() bool {
	return hasUsableJavaHome() || hasJavaAndJavacOnPath()
}

func hasUsableJavaHome() bool {
	javaHome := strings.TrimSpace(os.Getenv("JAVA_HOME"))
	if javaHome == "" {
		return false
	}
	return isExecutablePath(filepath.Join(javaHome, "bin", platformJavaBinary("java"))) &&
		isExecutablePath(filepath.Join(javaHome, "bin", platformJavaBinary("javac")))
}

func hasJavaAndJavacOnPath() bool {
	if _, err := exec.LookPath("java"); err != nil {
		return false
	}
	if _, err := exec.LookPath("javac"); err != nil {
		return false
	}
	return true
}

func platformJavaBinary(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func isExecutablePath(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir() && isExecutable(info, path)
}

func finalizeJBangInstall(layout InstallLayout) error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("managed jbang wrapper is not supported on windows")
	}
	return installGeneratedLauncher(layout.TargetPath, jbangLauncherScript())
}

func installGeneratedLauncher(targetPath, content string) error {
	tempFile, err := os.CreateTemp(filepath.Dir(targetPath), filepath.Base(targetPath)+".vex-*")
	if err != nil {
		return fmt.Errorf("failed to create staged launcher: %w", err)
	}
	tempPath := tempFile.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()

	if _, err := tempFile.WriteString(content); err != nil {
		tempFile.Close()
		return fmt.Errorf("failed to write staged launcher: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to finalize staged launcher: %w", err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(tempPath, 0755); err != nil {
			return fmt.Errorf("failed to mark launcher executable: %w", err)
		}
	}
	if runtime.GOOS == "windows" {
		_ = os.Remove(targetPath)
	}
	if err := os.Rename(tempPath, targetPath); err != nil {
		return fmt.Errorf("failed to install launcher: %w", err)
	}
	return nil
}

func jbangLauncherScript() string {
	return `#!/usr/bin/env bash
set -euo pipefail

# Generated by vex. Requires a system JDK and will not bootstrap one automatically.
script_dir() {
  local script dir
  script=${BASH_SOURCE[0]}
  while [ -L "$script" ]; do
    dir=$(cd -P "$(dirname "$script")" >/dev/null 2>&1 && pwd)
    script=$(readlink "$script")
    [[ $script != /* ]] && script=$dir/$script
  done
  cd -P "$(dirname "$script")" >/dev/null 2>&1 && pwd
}

find_java_exec() {
  if [[ -n "${JAVA_HOME:-}" ]] && [[ -x "$JAVA_HOME/bin/javac" ]] && [[ -x "$JAVA_HOME/bin/java" ]]; then
    printf '%s\n' "$JAVA_HOME/bin/java"
    return 0
  fi
  if command -v javac >/dev/null 2>&1 && command -v java >/dev/null 2>&1; then
    command -v java
    return 0
  fi
  return 1
}

main() {
  local dir jar_path java_exec
  dir=$(script_dir)
  jar_path="$dir/jbang.jar"

  if [[ ! -f "$jar_path" ]]; then
    echo "Error: jbang.jar not found next to $0" >&2
    exit 1
  fi
  if ! java_exec=$(find_java_exec); then
    echo "Error: jbang requires a system JDK (set JAVA_HOME or put javac on PATH)" >&2
    exit 1
  fi

  export JBANG_RUNTIME_SHELL=bash
  if [ -t 0 ]; then
    export JBANG_STDIN_NOTTY=false
  else
    export JBANG_STDIN_NOTTY=true
  fi
  export JBANG_LAUNCH_CMD="$0"

  if [[ -n "${JBANG_JAVA_OPTIONS:-}" ]]; then
    # shellcheck disable=SC2086
    exec "$java_exec" ${JBANG_JAVA_OPTIONS} -classpath "$jar_path" dev.jbang.Main "$@"
  fi
  exec "$java_exec" -classpath "$jar_path" dev.jbang.Main "$@"
}

main "$@"
`
}
