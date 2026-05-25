package validate

import "testing"

func TestFilenameValid(t *testing.T) {
	if err := Filename("solution.cpp"); err != nil {
		t.Fatalf("expected valid filename, got: %v", err)
	}
}

func TestFilenameTraversal(t *testing.T) {
	if err := Filename("../../etc/passwd"); err == nil {
		t.Fatal("expected error for path traversal")
	}
}

func TestFlagsEmpty(t *testing.T) {
	if err := Flags("go", ""); err != nil {
		t.Fatalf("expected no error for empty flags, got: %v", err)
	}
}
