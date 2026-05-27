package validate

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// ValidationError represents a validation failure that can be returned to clients as JSON.
type ValidationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error implements the error interface for ValidationError.
func (e ValidationError) Error() string {
	data, err := json.Marshal(e)
	if err != nil {
		return fmt.Sprintf("internal: failed to marshal error: %v", err)
	}
	return string(data)
}

// ValidateFilename rejects:
//   - empty string
//   - any path separator (/ or \)
//   - leading dot (.hidden files)
//   - absolute paths (starting with /)
//   - the string ".." or any component that is ".."
//   - length > 128 characters
//   - null bytes or control characters
//
// Returns ValidationError with code "invalid_filename" and descriptive message if invalid.
// Returns nil if the filename is safe.
func ValidateFilename(name string) error {
	// Check empty string
	if name == "" {
		return ValidationError{
			Code:    "invalid_filename",
			Message: "filename cannot be empty",
		}
	}

	// Check length
	if len(name) > 128 {
		return ValidationError{
			Code:    "invalid_filename",
			Message: fmt.Sprintf("filename exceeds 128 character limit (got %d characters)", len(name)),
		}
	}

	// Scan each byte for invalid characters (null bytes, control chars, path separators)
	for i := 0; i < len(name); i++ {
		ch := name[i]

		// Check for null bytes and control characters (< 0x20 is control, 0x7F is DEL)
		if ch < 0x20 || ch == 0x7F {
			return ValidationError{
				Code:    "invalid_filename",
				Message: fmt.Sprintf("filename contains invalid control character (byte 0x%02x at position %d)", ch, i),
			}
		}

		// Check for path separators
		if ch == '/' || ch == '\\' {
			return ValidationError{
				Code:    "invalid_filename",
				Message: "filename cannot contain path separators (/ or \\)",
			}
		}
	}

	// Check first character for absolute path and leading dot
	first := name[0]

	// Check for absolute path (starts with /)
	if first == '/' {
		return ValidationError{
			Code:    "invalid_filename",
			Message: "filename cannot be an absolute path (cannot start with /)",
		}
	}

	// Check for leading dot (.hidden files)
	if first == '.' {
		// Reject any filename starting with dot
		return ValidationError{
			Code:    "invalid_filename",
			Message: "filename cannot start with a dot (.hidden files and directory traversal not allowed)",
		}
	}

	// All checks passed
	return nil
}

// Filename validates a filename for path traversal attacks (legacy, prefer ValidateFilename)
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
