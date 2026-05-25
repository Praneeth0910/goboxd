package runner

import "testing"

func TestGenerateJobID(t *testing.T) {
	a := GenerateJobID()
	b := GenerateJobID()
	if a == b {
		t.Fatal("GenerateJobID returned duplicate IDs")
	}
	if a == "" || b == "" {
		t.Fatal("GenerateJobID returned empty string")
	}
}
