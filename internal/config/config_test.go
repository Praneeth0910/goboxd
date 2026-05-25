package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadLanguagesYAML(t *testing.T) {
	// Find languages.yaml in project root
	configPath := filepath.Join("..", "..", "languages.yaml")

	// Handle running from different directories
	if _, err := os.Stat(configPath); err != nil {
		// Try from project root if in subdirectory
		configPath = "languages.yaml"
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load languages.yaml: %v", err)
	}

	if cfg == nil {
		t.Fatal("Config is nil")
	}

	// Verify Python 3 language
	py3, ok := cfg.Languages["py3"]
	if !ok {
		t.Fatal("Python 3 (py3) language not found")
	}
	if py3.Name != "Python 3" {
		t.Errorf("Expected Python 3 name, got %s", py3.Name)
	}
	if py3.SourceFilename != "solution.py" {
		t.Errorf("Expected source_filename solution.py, got %s", py3.SourceFilename)
	}
	if py3.Run.Cmd != "/usr/bin/python3" {
		t.Errorf("Expected run.cmd /usr/bin/python3, got %s", py3.Run.Cmd)
	}
	if py3.Run.Limits.WallTimeS != 9 {
		t.Errorf("Expected py3 wall_time_s=9, got %d", py3.Run.Limits.WallTimeS)
	}
	if py3.Build != nil {
		t.Fatal("Python 3 should not have build config")
	}

	// Verify C++ language
	cpp, ok := cfg.Languages["cpp"]
	if !ok {
		t.Fatal("C++ (cpp) language not found")
	}
	if cpp.Name != "C++" {
		t.Errorf("Expected C++ name, got %s", cpp.Name)
	}
	if cpp.SourceFilename != "solution.cpp" {
		t.Errorf("Expected source_filename solution.cpp, got %s", cpp.SourceFilename)
	}
	if cpp.ArtifactFilename != "solution" {
		t.Errorf("Expected artifact solution, got %s", cpp.ArtifactFilename)
	}

	// Verify C++ build config
	if cpp.Build == nil {
		t.Fatal("C++ should have build config")
	}
	if cpp.Build.Cmd != "/usr/bin/g++" {
		t.Errorf("Expected build.cmd /usr/bin/g++, got %s", cpp.Build.Cmd)
	}
	if cpp.Build.Limits.WallTimeS != 3 {
		t.Errorf("Expected build wall_time_s=3, got %d", cpp.Build.Limits.WallTimeS)
	}
	if len(cpp.Build.FlagAllowlist) == 0 {
		t.Fatal("C++ build should have flag_allowlist")
	}

	// Verify C++ run config
	if cpp.Run.Cmd != "./{{artifact}}" {
		t.Errorf("Expected run.cmd ./{{artifact}}, got %s", cpp.Run.Cmd)
	}
	if cpp.Run.Limits.WallTimeS != 3 {
		t.Errorf("Expected run wall_time_s=3, got %d", cpp.Run.Limits.WallTimeS)
	}

	// Verify global settings
	if cfg.MaxSourceBytes != 262144 {
		t.Errorf("Expected MaxSourceBytes=262144, got %d", cfg.MaxSourceBytes)
	}
	if cfg.MaxTests != 50 {
		t.Errorf("Expected MaxTests=50, got %d", cfg.MaxTests)
	}
	if cfg.MaxConcurrent <= 0 {
		t.Errorf("Expected MaxConcurrent>0, got %d", cfg.MaxConcurrent)
	}
}

func TestValidateConfig(t *testing.T) {
	configPath := filepath.Join("..", "..", "languages.yaml")
	if _, err := os.Stat(configPath); err != nil {
		configPath = "languages.yaml"
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify no validation errors
	if err := Validate(cfg); err != nil {
		t.Fatalf("Config validation failed: %v", err)
	}

	// Verify we have exactly 2 languages for Stage 1
	if len(cfg.Languages) != 2 {
		t.Errorf("Expected 2 languages, got %d", len(cfg.Languages))
	}
}

func TestValidateEmptyLanguages(t *testing.T) {
	cfg := &Config{
		Languages:      make(map[string]Language),
		MaxSourceBytes: 262144,
		MaxTests:       50,
		MaxConcurrent:  1,
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for empty languages")
	}
}

func TestValidateMissingRunCmd(t *testing.T) {
	cfg := &Config{
		Languages: map[string]Language{
			"test": {
				ID:   "test",
				Name: "Test",
				Run:  RunConfig{Cmd: ""}, // Missing cmd
			},
		},
		MaxSourceBytes: 262144,
		MaxTests:       50,
		MaxConcurrent:  1,
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for missing run.cmd")
	}
}
