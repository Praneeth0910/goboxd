package validate

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Filename validates a filename for path traversal attacks
func Filename(name string) error {
	if name == "" {
		return fmt.Errorf("filename cannot be empty")
	}

	// Prevent path traversal
	if strings.Contains(name, "..") {
		return fmt.Errorf("path traversal detected")
	}

	if strings.Contains(name, "/") {
		return fmt.Errorf("filename cannot contain directory separators")
	}

	// Clean and check
	cleaned := filepath.Clean(name)
	if cleaned != name {
		return fmt.Errorf("invalid filename format")
	}

	return nil
}

// SafePath validates that a path is within a base directory
func SafePath(base, target string) error {
	absBase := filepath.Clean(base)
	absTarget := filepath.Clean(target)

	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	if strings.HasPrefix(rel, "..") {
		return fmt.Errorf("path escapes base directory")
	}

	return nil
}
