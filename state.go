package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func stateDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".vex")
}

func statePath() string {
	return filepath.Join(stateDir(), "state.json")
}

func ensureStateDir() error {
	return os.MkdirAll(stateDir(), 0755)
}

// readState returns the current alias→value map from disk.
// Returns an empty map if the file doesn't exist.
func readState() (map[string]string, error) {
	data, err := os.ReadFile(statePath())
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]string), nil
		}
		return nil, err
	}
	var state map[string]string
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return state, nil
}

// writeState persists the alias→value map to disk.
func writeState(state map[string]string) error {
	if err := ensureStateDir(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(statePath(), data, 0644)
}
