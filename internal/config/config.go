package config

import (
	"fmt"
	"os"
	"runtime"

	"gopkg.in/yaml.v3"
)

// Template placeholder constants used in command/argument expansion at runtime
const (
	TemplateSource   = "{{source}}"
	TemplateArtifact = "{{artifact}}"
	TemplateClass    = "{{class}}"
	TemplateFlags    = "{{flags}}"
)

// ResourceLimits defines resource constraints for build/run
type ResourceLimits struct {
	WallTimeS    int `yaml:"wall_time_s"`
	MemoryKB     int `yaml:"memory_kb"`
	MaxProcesses int `yaml:"max_processes"`
}

func (l ResourceLimits) MergeWithCap(override ResourceLimits) ResourceLimits {
	out := l // start from language defaults
	if override.WallTimeS > 0 && override.WallTimeS < l.WallTimeS {
		out.WallTimeS = override.WallTimeS
	}
	if override.MemoryKB > 0 && override.MemoryKB < l.MemoryKB {
		out.MemoryKB = override.MemoryKB
	}
	if override.MaxProcesses > 0 && override.MaxProcesses < l.MaxProcesses {
		out.MaxProcesses = override.MaxProcesses
	}
	return out
}

// BuildConfig defines how to compile source code
type BuildConfig struct {
	Cmd           string         `yaml:"cmd"`
	Args          []string       `yaml:"args"`
	Limits        ResourceLimits `yaml:"limits"`
	FlagAllowlist []string       `yaml:"flag_allowlist"`
}

// RunConfig defines how to execute code
type RunConfig struct {
	Cmd           string         `yaml:"cmd"`
	Args          []string       `yaml:"args"`
	Limits        ResourceLimits `yaml:"limits"`
	FlagAllowlist []string       `yaml:"flag_allowlist"`
}

// Language represents configuration for a supported language
type Language struct {
	ID                       string       `yaml:"id"`
	Name                     string       `yaml:"name"`
	SourceFilename           string       `yaml:"source_filename"`
	SourceFilenameStrategy   string       `yaml:"source_filename_strategy"` // "" or "from_request"
	ArtifactFilename         string       `yaml:"artifact"`
	ArtifactFilenameStrategy string       `yaml:"artifact_filename_strategy"` // "" or "from_request"
	Build                    *BuildConfig `yaml:"build,omitempty"`
	Run                      RunConfig    `yaml:"run"`
}

// Config holds the application configuration
type Config struct {
	Languages      map[string]Language `yaml:"languages"` // keyed by language id
	MaxSourceBytes int                 `yaml:"max_source_bytes"`
	MaxTests       int                 `yaml:"max_tests"`
	MaxConcurrent  int                 `yaml:"max_concurrent"`
	QueueTimeoutS  int                 `yaml:"queue_timeout_s"`
}

// Load reads and parses a YAML configuration file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse raw YAML first
	var rawConfig map[string]interface{}
	if err := yaml.Unmarshal(data, &rawConfig); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Initialize Config with defaults
	cfg := &Config{
		Languages:      make(map[string]Language),
		MaxSourceBytes: 262144, // 256 KiB
		MaxTests:       50,
		MaxConcurrent:  0,  // use runtime.NumCPU()
		QueueTimeoutS:  30, // 30 seconds default
	}

	// Parse languages separately to build map keyed by id
	if languagesRaw, ok := rawConfig["languages"].(map[string]interface{}); ok {
		for langID, langData := range languagesRaw {
			langBytes, err := yaml.Marshal(langData)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal language %s: %w", langID, err)
			}

			var lang Language
			if err := yaml.Unmarshal(langBytes, &lang); err != nil {
				return nil, fmt.Errorf("failed to parse language %s: %w", langID, err)
			}

			lang.ID = langID // Ensure ID is set from key
			cfg.Languages[langID] = lang
		}
	}

	// Parse global settings
	if maxSourceBytes, ok := rawConfig["max_source_bytes"].(int); ok {
		cfg.MaxSourceBytes = maxSourceBytes
	}
	if maxTests, ok := rawConfig["max_tests"].(int); ok {
		cfg.MaxTests = maxTests
	}
	if maxConcurrent, ok := rawConfig["max_concurrent"].(int); ok {
		cfg.MaxConcurrent = maxConcurrent
	}
	if queueTimeoutS, ok := rawConfig["queue_timeout_s"].(int); ok {
		cfg.QueueTimeoutS = queueTimeoutS
	}

	// Set MaxConcurrent to NumCPU if not specified or 0
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = runtime.NumCPU()
	}

	if cfg.QueueTimeoutS <= 0 {
		cfg.QueueTimeoutS = 30
	}

	// Validate before returning
	if err := Validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks configuration validity and fails loudly at startup
func Validate(cfg *Config) error {
	if len(cfg.Languages) == 0 {
		return fmt.Errorf("validation failed: no languages configured")
	}

	seenIDs := make(map[string]bool)

	for id := range cfg.Languages {
		lang := cfg.Languages[id]
		// Check required fields
		if lang.ID == "" {
			return fmt.Errorf("validation failed: language has empty id")
		}
		if lang.Name == "" {
			return fmt.Errorf("validation failed: language %q has empty name", lang.ID)
		}
		if lang.Run.Cmd == "" {
			return fmt.Errorf("validation failed: language %q has empty run.cmd", lang.ID)
		}

		// Check for duplicate ids
		if seenIDs[lang.ID] {
			return fmt.Errorf("validation failed: duplicate language id %q", lang.ID)
		}
		seenIDs[lang.ID] = true

		// Check build config if present
		if lang.Build != nil && lang.Build.Cmd == "" {
			return fmt.Errorf("validation failed: language %q has build but empty build.cmd", lang.ID)
		}
	}

	// Validate global limits
	if cfg.MaxSourceBytes <= 0 {
		return fmt.Errorf("validation failed: max_source_bytes must be positive")
	}
	if cfg.MaxTests <= 0 {
		return fmt.Errorf("validation failed: max_tests must be positive")
	}
	if cfg.MaxConcurrent <= 0 {
		return fmt.Errorf("validation failed: max_concurrent must be positive (got %d)", cfg.MaxConcurrent)
	}

	return nil
}
