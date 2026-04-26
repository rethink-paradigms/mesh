// Package manifest reads and writes JSON manifest sidecar files for snapshots.
package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Manifest describes a snapshot's metadata. Written as a .json sidecar next to
// the .tar.zst tarball after snapshot creation completes.
type Manifest struct {
	AgentName     string    `json:"agent_name"`
	Timestamp     time.Time `json:"timestamp"`
	SourceMachine string    `json:"source_machine"`
	SourceWorkdir string    `json:"source_workdir"`
	StartCmd      string    `json:"start_cmd"`
	StopTimeout   string    `json:"stop_timeout"`
	Checksum      string    `json:"checksum"`
	Size          int64     `json:"size"`
}

// Write marshals the manifest to indented JSON and writes it to path.
// Parent directories are created if needed.
func Write(path string, m *Manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create manifest dir: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	return nil
}

// Read parses a JSON manifest file at path and returns the Manifest.
func Read(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	return &m, nil
}

// ManifestPath derives the manifest file path from a snapshot path.
// Replaces .tar.zst suffix with .json; appends .json otherwise.
func ManifestPath(snapshotPath string) string {
	if strings.HasSuffix(snapshotPath, ".tar.zst") {
		return strings.TrimSuffix(snapshotPath, ".tar.zst") + ".json"
	}
	return snapshotPath + ".json"
}
