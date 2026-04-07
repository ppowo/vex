package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const VexConfigRelativeDir = ".vex"
const BinStateFilename = "bin-state.json"

func HomeDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return homeDir, nil
}

// ManagedBinDir returns the OS-appropriate directory for vex-managed binaries.
//
//   - macOS: ~/.local/share/vex/bin
//   - Linux: $XDG_DATA_HOME/vex/bin (defaults to ~/.local/share/vex/bin)
func ManagedBinDir() (string, error) {
	if runtime.GOOS == "linux" {
		base := os.Getenv("XDG_DATA_HOME")
		if base == "" {
			home, err := HomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, ".local", "share")
		}
		return filepath.Join(base, "vex", "bin"), nil
	}

	// darwin and other unix
	home, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "vex", "bin"), nil
}

func VexConfigDir() (string, error) {
	homeDir, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, filepath.FromSlash(VexConfigRelativeDir)), nil
}

func BinStatePath() (string, error) {
	configDir, err := VexConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, BinStateFilename), nil
}
