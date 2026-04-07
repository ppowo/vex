package bin

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	vexpaths "github.com/pun/vex/internal/paths"
)

type State struct {
	Version string                `json:"version"`
	Tools   map[string]*ToolState `json:"tools"`
}

type ToolState struct {
	Installed        bool          `json:"installed"`
	Path             string        `json:"path,omitempty"`
	InstalledVersion string        `json:"installedVersion,omitempty"`
	InstalledAt      time.Time     `json:"installedAt,omitempty"`
	UpdatedAt        time.Time     `json:"updatedAt,omitempty"`
	Artifact         ArtifactState `json:"artifact,omitempty"`
}

type ArtifactState struct {
	SourceType  string `json:"sourceType,omitempty"`
	ManifestURL string `json:"manifestURL,omitempty"`
	ReleaseTag  string `json:"releaseTag,omitempty"`
	AssetName   string `json:"assetName,omitempty"`
	DownloadURL string `json:"downloadURL,omitempty"`
	Checksum    string `json:"checksum,omitempty"`
}

func LoadState() (*State, error) {
	configDir, err := vexpaths.VexConfigDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	statePath, err := vexpaths.BinStatePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &State{
				Version: "1.0",
				Tools:   make(map[string]*ToolState),
			}, nil
		}
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}
	if state.Version == "" {
		state.Version = "1.0"
	}
	if state.Tools == nil {
		state.Tools = make(map[string]*ToolState)
	}
	return &state, nil
}

func (s *State) Save() error {
	configDir, err := vexpaths.VexConfigDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	statePath, err := vexpaths.BinStatePath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize state: %w", err)
	}
	if err := os.WriteFile(statePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}
	return nil
}
