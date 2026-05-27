package validate

import (
	"fmt"
	"strings"
)

// AllowedFlags defines per-language flag allowlists (legacy, deprecated)
var AllowedFlags = map[string][]string{
	"go":     {"-run", "-count", "-timeout", "-v", "-race", "-vet"},
	"python": {"-c", "-m", "-u", "-W"},
	"node":   {"-e", "-r", "--eval", "--require"},
}

// ValidateFlags checks each flag against the language's flag_allowlist.
// Supports glob-style patterns (e.g. "-std=*" matches "-std=c++17").
// If allowlist is nil or empty, all flags are rejected with an error.
// Returns error listing all rejected flags, or nil if all flags are valid.
func ValidateFlags(flags, allowlist []string) error {
	// If no allowlist, reject all flags
	if len(allowlist) == 0 {
		if len(flags) == 0 {
			return nil // No flags to reject
		}
		return ValidationError{
			Code:    "invalid_flags",
			Message: fmt.Sprintf("no flags allowed, but received %d flag(s)", len(flags)),
		}
	}

	// Check each flag against the allowlist
	var rejected []string
	for _, flag := range flags {
		if !matchesAllowlist(flag, allowlist) {
			rejected = append(rejected, flag)
		}
	}

	// If any flags were rejected, return error listing them all
	if len(rejected) > 0 {
		rejectedStr := strings.Join(rejected, ", ")
		return ValidationError{
			Code:    "invalid_flags",
			Message: fmt.Sprintf("flags not in allowlist: %s", rejectedStr),
		}
	}

	return nil
}

// matchesAllowlist checks if a flag matches at least one entry in the allowlist.
// Supports:
//   - Exact match: "-O2" matches "-O2"
//   - Suffix glob: "-std=*" matches "-std=c++17", "-std=c11", etc.
func matchesAllowlist(flag string, allowlist []string) bool {
	for _, entry := range allowlist {
		// Exact match
		if entry == flag {
			return true
		}

		// Suffix glob: "-std=*" matches "-std=c++17"
		if strings.HasSuffix(entry, "*") {
			prefix := strings.TrimSuffix(entry, "*")
			if strings.HasPrefix(flag, prefix) {
				return true
			}
		}
	}

	return false
}

// Flags validates that provided flags are allowed for a language (legacy, prefer ValidateFlags)
func Flags(lang, flagsStr string) error {
	if flagsStr == "" {
		return nil
	}

	allowed, ok := AllowedFlags[lang]
	if !ok {
		return fmt.Errorf("unknown language: %s", lang)
	}

	flags := strings.Fields(flagsStr)
	for _, flag := range flags {
		if !contains(allowed, flag) {
			return fmt.Errorf("disallowed flag: %s", flag)
		}
	}

	return nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
