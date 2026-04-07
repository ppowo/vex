package main

import (
	"os"
	"path/filepath"
	"runtime"
)

// vexBinDir returns the OS-appropriate directory for vex-managed binaries.
//
//   - macOS: ~/.local/share/vex/bin
//   - Linux: $XDG_DATA_HOME/vex/bin (defaults to ~/.local/share/vex/bin)
func vexBinDir() string {
	if runtime.GOOS == "linux" {
		base := os.Getenv("XDG_DATA_HOME")
		if base == "" {
			home, _ := os.UserHomeDir()
			base = filepath.Join(home, ".local", "share")
		}
		return filepath.Join(base, "vex", "bin")
	}

	// darwin and other unix
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "vex", "bin")
}

// ensureBinDir creates the vex bin directory if it doesn't exist.
func ensureBinDir() error {
	return os.MkdirAll(vexBinDir(), 0755)
}
