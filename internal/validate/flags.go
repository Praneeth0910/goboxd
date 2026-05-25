package validate

import (
	"fmt"
	"strings"
)

// AllowedFlags defines per-language flag allowlists
var AllowedFlags = map[string][]string{
	"go":     {"-run", "-count", "-timeout", "-v", "-race", "-vet"},
	"python": {"-c", "-m", "-u", "-W"},
	"node":   {"-e", "-r", "--eval", "--require"},
}

// Flags validates that provided flags are allowed for a language
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
