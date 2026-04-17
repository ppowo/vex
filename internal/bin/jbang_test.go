package bin

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSelectJBangAssetUsesVersionedZipAndChecksum(t *testing.T) {
	release := githubRelease{
		TagName: "v0.138.0",
		Assets: []githubReleaseAsset{
			{Name: "jbang.zip", BrowserDownloadURL: "https://example.com/jbang.zip"},
			{Name: "jbang-0.138.0.zip", BrowserDownloadURL: "https://example.com/jbang-0.138.0.zip"},
			{Name: "jbang-0.138.0.zip.sha256", BrowserDownloadURL: "https://example.com/jbang-0.138.0.zip.sha256"},
		},
	}

	selected, checksumURL, err := selectJBangAsset(release, "0.138.0")
	if err != nil {
		t.Fatalf("expected asset selection to succeed: %v", err)
	}
	if selected.Name != "jbang-0.138.0.zip" {
		t.Fatalf("expected versioned zip asset, got %q", selected.Name)
	}
	if checksumURL != "https://example.com/jbang-0.138.0.zip.sha256" {
		t.Fatalf("expected checksum URL to be selected, got %q", checksumURL)
	}
}

func TestHasSystemJDKWithJAVA_HOME(t *testing.T) {
	javaHome := t.TempDir()
	binDir := filepath.Join(javaHome, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("failed to create JAVA_HOME/bin: %v", err)
	}
	createFakeExecutable(t, binDir, platformJavaBinary("java"))
	createFakeExecutable(t, binDir, platformJavaBinary("javac"))

	t.Setenv("JAVA_HOME", javaHome)
	t.Setenv("PATH", t.TempDir())

	if !hasSystemJDK() {
		t.Fatal("expected JAVA_HOME to satisfy the system JDK check")
	}
}

func TestHasSystemJDKWithPath(t *testing.T) {
	pathDir := t.TempDir()
	createFakeExecutable(t, pathDir, platformJavaBinary("java"))
	createFakeExecutable(t, pathDir, platformJavaBinary("javac"))

	t.Setenv("JAVA_HOME", "")
	t.Setenv("PATH", pathDir)

	if !hasSystemJDK() {
		t.Fatal("expected PATH to satisfy the system JDK check")
	}
}

func TestHasSystemJDKFalseWithoutJavaAndJavac(t *testing.T) {
	t.Setenv("JAVA_HOME", "")
	t.Setenv("PATH", t.TempDir())

	if hasSystemJDK() {
		t.Fatal("expected system JDK check to fail when neither JAVA_HOME nor PATH contain a JDK")
	}
}

func createFakeExecutable(t *testing.T, dir, name string) {
	t.Helper()
	path := filepath.Join(dir, name)
	content := []byte("#!/bin/sh\nexit 0\n")
	mode := os.FileMode(0755)
	if runtime.GOOS == "windows" {
		content = []byte("")
		mode = 0644
	}
	if err := os.WriteFile(path, content, mode); err != nil {
		t.Fatalf("failed to create fake executable %s: %v", path, err)
	}
}
