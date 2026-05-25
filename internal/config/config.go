package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Languages map[string]LanguageConfig `yaml:"languages"`
	Limits    ResourceLimits            `yaml:"limits"`
	Sandbox   SandboxConfig             `yaml:"sandbox"`
}

// LanguageConfig represents configuration for a supported language
type LanguageConfig struct {
	Name     string   `yaml:"name"`
	Runtime  string   `yaml:"runtime"`
	Version  string   `yaml:"version"`
	Flags    []string `yaml:"flags"`
	Timeout  int      `yaml:"timeout"`
	MemLimit string   `yaml:"mem_limit"`
	CPULimit string   `yaml:"cpu_limit"`
}

// ResourceLimits defines resource limits for jobs
type ResourceLimits struct {
	Memory      int `yaml:"memory"`
	CPU         int `yaml:"cpu"`
	Processes   int `yaml:"processes"`
	OpenFiles   int `yaml:"open_files"`
	MaxDuration int `yaml:"max_duration"`
}

// SandboxConfig defines sandbox behavior
type SandboxConfig struct {
	TempDir    string `yaml:"temp_dir"`
	RootFS     string `yaml:"rootfs"`
	NetworkNAT bool   `yaml:"network_nat"`
}

// Load reads and parses a YAML configuration file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

// Validate checks configuration validity
func (c *Config) Validate() error {
	if len(c.Languages) == 0 {
		return fmt.Errorf("no languages configured")
	}

	if c.Limits.Memory <= 0 {
		return fmt.Errorf("memory limit must be positive")
	}

	if c.Sandbox.TempDir == "" {
		return fmt.Errorf("sandbox temp dir not configured")
	}

	return nil
}
