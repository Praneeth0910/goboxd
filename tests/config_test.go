package goboxd_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/thesouldev/goboxd/internal/config"
)

// TestLoadValidInterpretedLanguage tests loading a valid YAML with one interpreted language.
func TestLoadValidInterpretedLanguage(t *testing.T) {
	yaml := `
languages:
  python3:
    name: Python 3
    source_filename: solution.py
    run:
      cmd: python3
      args:
        - "{{source}}"
      limits:
        wall_time_s: 10
        memory_kb: 262144
        max_processes: 64
max_source_bytes: 262144
max_tests: 50
`

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}

	if len(cfg.Languages) != 1 {
		t.Errorf("expected 1 language, got %d", len(cfg.Languages))
	}

	lang, ok := cfg.Languages["python3"]
	if !ok {
		t.Fatal("python3 language not found in config")
	}

	if lang.Name != "Python 3" {
		t.Errorf("expected name 'Python 3', got %q", lang.Name)
	}

	if lang.Run.Cmd != "python3" {
		t.Errorf("expected run.cmd 'python3', got %q", lang.Run.Cmd)
	}
}

// TestLoadValidCompiledLanguage tests loading a valid YAML with a compiled language (has build step).
func TestLoadValidCompiledLanguage(t *testing.T) {
	yaml := `
languages:
  cpp:
    name: C++
    source_filename: solution.cpp
    artifact: solution
    build:
      cmd: g++
      args:
        - "-o"
        - "{{artifact}}"
        - "{{source}}"
      limits:
        wall_time_s: 30
        memory_kb: 1048576
        max_processes: 100
      flag_allowlist:
        - "-O2"
        - "-std=*"
    run:
      cmd: ./{{artifact}}
      args: []
      limits:
        wall_time_s: 10
        memory_kb: 262144
        max_processes: 64
max_source_bytes: 262144
max_tests: 50
`

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	lang, ok := cfg.Languages["cpp"]
	if !ok {
		t.Fatal("cpp language not found in config")
	}

	if lang.Build == nil {
		t.Fatal("expected Build config to be present")
	}

	if lang.Build.Cmd != "g++" {
		t.Errorf("expected build.cmd 'g++', got %q", lang.Build.Cmd)
	}

	if len(lang.Build.FlagAllowlist) != 2 {
		t.Errorf("expected 2 allowed flags, got %d", len(lang.Build.FlagAllowlist))
	}
}

// TestLoadDuplicateLanguageIDs tests that duplicate language IDs return an error.
func TestLoadDuplicateLanguageIDs(t *testing.T) {
	yaml := `
languages:
  python:
    name: Python 3
    source_filename: solution.py
    run:
      cmd: python3
      args:
        - "{{source}}"
      limits:
        wall_time_s: 10
        memory_kb: 262144
        max_processes: 64
  python:
    name: Python 2
    source_filename: solution.py
    run:
      cmd: python2
      args:
        - "{{source}}"
      limits:
        wall_time_s: 10
        memory_kb: 262144
        max_processes: 64
`

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	_, err := config.Load(path)
	if err == nil {
		t.Fatal("Load() should return error for duplicate language IDs")
	}
}

// TestLoadNonexistentFile tests that loading a non-existent file returns an error.
func TestLoadNonexistentFile(t *testing.T) {
	_, err := config.Load("/nonexistent/path/to/config.yaml")
	if err == nil {
		t.Fatal("Load() should return error for non-existent file")
	}
}

// TestLoadEmptyYAML tests that loading an empty YAML (no languages) returns an error.
func TestLoadEmptyYAML(t *testing.T) {
	yaml := `
max_source_bytes: 262144
max_tests: 50
`

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	_, err := config.Load(path)
	if err == nil {
		t.Fatal("Load() should return error when no languages are configured")
	}
}

// TestLoadMissingRunCmd tests that a language missing run.cmd returns an error.
func TestLoadMissingRunCmd(t *testing.T) {
	yaml := `
languages:
  python:
    name: Python 3
    source_filename: solution.py
    run:
      args:
        - "{{source}}"
      limits:
        wall_time_s: 10
        memory_kb: 262144
        max_processes: 64
max_source_bytes: 262144
max_tests: 50
`

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	_, err := config.Load(path)
	if err == nil {
		t.Fatal("Load() should return error when run.cmd is empty")
	}
}

// TestLoadInvalidYAML tests that loading invalid YAML returns an error.
func TestLoadInvalidYAML(t *testing.T) {
	yaml := `
languages:
  python:
    name: Python 3
    [invalid yaml syntax
`

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	_, err := config.Load(path)
	if err == nil {
		t.Fatal("Load() should return error for invalid YAML")
	}
}

// TestValidateZeroLanguages tests that a config with zero languages returns error.
func TestValidateZeroLanguages(t *testing.T) {
	cfg := &config.Config{
		Languages:      make(map[string]config.Language),
		MaxSourceBytes: 262144,
		MaxTests:       50,
		MaxConcurrent:  4,
	}

	err := config.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() should return error for zero languages")
	}
}

// TestValidateEmptyLanguageID tests that a language with empty ID returns error.
func TestValidateEmptyLanguageID(t *testing.T) {
	cfg := &config.Config{
		Languages: map[string]config.Language{
			"python": {
				ID:   "",
				Name: "Python 3",
				Run: config.RunConfig{
					Cmd: "python3",
				},
			},
		},
		MaxSourceBytes: 262144,
		MaxTests:       50,
		MaxConcurrent:  4,
	}

	err := config.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() should return error for empty language ID")
	}
}

// TestValidateMissingRunCmd tests that a language with empty run.cmd returns error.
func TestValidateMissingRunCmd(t *testing.T) {
	cfg := &config.Config{
		Languages: map[string]config.Language{
			"python": {
				ID:   "python",
				Name: "Python 3",
				Run: config.RunConfig{
					Cmd: "",
				},
			},
		},
		MaxSourceBytes: 262144,
		MaxTests:       50,
		MaxConcurrent:  4,
	}

	err := config.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() should return error for empty run.cmd")
	}
}

// TestValidateBuildCmdEmpty tests that a language with build but empty build.cmd returns error.
func TestValidateBuildCmdEmpty(t *testing.T) {
	cfg := &config.Config{
		Languages: map[string]config.Language{
			"cpp": {
				ID:   "cpp",
				Name: "C++",
				Build: &config.BuildConfig{
					Cmd: "",
				},
				Run: config.RunConfig{
					Cmd: "./solution",
				},
			},
		},
		MaxSourceBytes: 262144,
		MaxTests:       50,
		MaxConcurrent:  4,
	}

	err := config.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() should return error when build.cmd is empty")
	}
}

// TestValidateMaxSourceBytesInvalid tests that invalid max_source_bytes returns error.
func TestValidateMaxSourceBytesInvalid(t *testing.T) {
	cfg := &config.Config{
		Languages: map[string]config.Language{
			"python": {
				ID:   "python",
				Name: "Python 3",
				Run: config.RunConfig{
					Cmd: "python3",
				},
			},
		},
		MaxSourceBytes: 0, // Invalid
		MaxTests:       50,
		MaxConcurrent:  4,
	}

	err := config.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() should return error for invalid max_source_bytes")
	}
}

// TestValidateMaxTestsInvalid tests that invalid max_tests returns error.
func TestValidateMaxTestsInvalid(t *testing.T) {
	cfg := &config.Config{
		Languages: map[string]config.Language{
			"python": {
				ID:   "python",
				Name: "Python 3",
				Run: config.RunConfig{
					Cmd: "python3",
				},
			},
		},
		MaxSourceBytes: 262144,
		MaxTests:       -5, // Invalid
		MaxConcurrent:  4,
	}

	err := config.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() should return error for invalid max_tests")
	}
}

// TestLoadDefaultLimits tests that Load() applies sensible defaults for limits.
func TestLoadDefaultLimits(t *testing.T) {
	yaml := `
languages:
  python3:
    name: Python 3
    source_filename: solution.py
    run:
      cmd: python3
      args:
        - "{{source}}"
max_source_bytes: 262144
max_tests: 50
`

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.MaxConcurrent <= 0 {
		t.Errorf("expected MaxConcurrent to be positive, got %d", cfg.MaxConcurrent)
	}
}

// TestLoadMultipleLanguages tests loading a config with multiple languages.
func TestLoadMultipleLanguages(t *testing.T) {
	yaml := `
languages:
  python3:
    name: Python 3
    source_filename: solution.py
    run:
      cmd: python3
      args:
        - "{{source}}"
      limits:
        wall_time_s: 10
        memory_kb: 262144
        max_processes: 64
  javascript:
    name: JavaScript
    source_filename: solution.js
    run:
      cmd: node
      args:
        - "{{source}}"
      limits:
        wall_time_s: 10
        memory_kb: 262144
        max_processes: 64
max_source_bytes: 262144
max_tests: 50
`

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if len(cfg.Languages) != 2 {
		t.Errorf("expected 2 languages, got %d", len(cfg.Languages))
	}

	_, pythonOk := cfg.Languages["python3"]
	_, jsOk := cfg.Languages["javascript"]

	if !pythonOk {
		t.Fatal("python3 language not found")
	}
	if !jsOk {
		t.Fatal("javascript language not found")
	}
}
