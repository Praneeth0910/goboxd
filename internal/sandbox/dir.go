package sandbox

import (
	"fmt"
	"os"
)

// Dir manages a per-request sandbox directory
type Dir struct {
	path string
}

// NewDir creates a new sandbox directory manager
func NewDir(path string) *Dir {
	return &Dir{path: path}
}

// Create creates the sandbox directory
func (d *Dir) Create() error {
	if err := os.MkdirAll(d.path, 0755); err != nil {
		return fmt.Errorf("failed to create sandbox dir: %w", err)
	}
	return nil
}

// Path returns the sandbox directory path
func (d *Dir) Path() string {
	return d.path
}

// Cleanup removes the sandbox directory
func (d *Dir) Cleanup() error {
	if err := os.RemoveAll(d.path); err != nil {
		return fmt.Errorf("failed to cleanup sandbox: %w", err)
	}
	return nil
}

// Exists checks if the sandbox directory exists
func (d *Dir) Exists() bool {
	_, err := os.Stat(d.path)
	return err == nil
}
