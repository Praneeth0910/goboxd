package config

import "testing"

func TestValidateEmptyLanguages(t *testing.T) {
	cfg := &Config{}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for empty languages")
	}
}
