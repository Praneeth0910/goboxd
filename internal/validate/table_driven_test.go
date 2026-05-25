package validate

import (
	"strings"
	"testing"
)

// ============ Table-Driven Tests for ValidateFilename ============

func TestValidateFilenameTableDriven(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		// Attack vectors - must error
		{name: "path_traversal_unix", input: "../../etc/passwd", wantErr: true},
		{name: "path_traversal_windows", input: "..\\..\\windows\\system32", wantErr: true},
		{name: "absolute_path_unix", input: "/absolute/path.cpp", wantErr: true},
		{name: "absolute_path_windows", input: "C:\\Windows\\System32\\file.exe", wantErr: true},
		{name: "hidden_file_unix", input: ".hidden", wantErr: true},
		{name: "hidden_ssh", input: ".ssh", wantErr: true},
		{name: "dot_dot_exactly", input: "..", wantErr: true},
		{name: "dot_dot_prefix", input: "..passwd", wantErr: true},
		{name: "nested_path", input: "sol/nested.cpp", wantErr: true},
		{name: "nested_backslash", input: "dir\\file.cpp", wantErr: true},
		{name: "relative_dot_slash", input: "./solution.cpp", wantErr: true},
		{name: "relative_dot_dot_slash", input: "../solution.cpp", wantErr: true},
		{name: "empty_string", input: "", wantErr: true},
		{name: "null_byte", input: "solution\x00.cpp", wantErr: true},
		{name: "crlf_injection", input: "file.cpp\r\ninjected", wantErr: true},
		{name: "esc_character", input: "solution\x1bpy", wantErr: true},
		{name: "tab_character", input: "test\t.py", wantErr: true},
		{name: "del_character", input: "file\x7f.cpp", wantErr: true},
		{name: "too_long_129_chars", input: strings.Repeat("a", 129), wantErr: true},
		{name: "too_long_256_chars", input: strings.Repeat("x", 256), wantErr: true},

		// Valid filenames - must pass
		{name: "cpp_solution", input: "solution.cpp", wantErr: false},
		{name: "java_class", input: "MyClass.java", wantErr: false},
		{name: "go_main", input: "main.go", wantErr: false},
		{name: "python_solution", input: "solution.py", wantErr: false},
		{name: "with_underscore", input: "solution_v2.py", wantErr: false},
		{name: "with_dash", input: "test-file.cpp", wantErr: false},
		{name: "with_numbers", input: "file123.txt", wantErr: false},
		{name: "readme_markdown", input: "README.md", wantErr: false},
		{name: "trailing_dots", input: "something..", wantErr: false},
		{name: "multiple_dots", input: "file....py", wantErr: false},
		{name: "max_length_128", input: strings.Repeat("a", 128), wantErr: false},
		{name: "header_file", input: "header.h", wantErr: false},
		{name: "capital_letters", input: "Solution.Java", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFilename(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFilename(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

// ============ Table-Driven Tests for ValidateFlags ============

func TestValidateFlagsTableDriven(t *testing.T) {
	// Default allowlist for C++ compilation
	defaultAllowlist := []string{"-O0", "-O1", "-O2", "-O3", "-Wall", "-Wextra", "-std=*"}

	tests := []struct {
		name      string
		flags     []string
		allowlist []string
		wantErr   bool
	}{
		// ===== Valid cases with default allowlist =====
		{name: "single_valid_flag_O2", flags: []string{"-O2"}, allowlist: defaultAllowlist, wantErr: false},
		{name: "single_valid_flag_O3", flags: []string{"-O3"}, allowlist: defaultAllowlist, wantErr: false},
		{name: "valid_warning_flag", flags: []string{"-Wall"}, allowlist: defaultAllowlist, wantErr: false},
		{name: "valid_std_glob_cpp17", flags: []string{"-std=c++17"}, allowlist: defaultAllowlist, wantErr: false},
		{name: "valid_std_glob_cpp20", flags: []string{"-std=c++20"}, allowlist: defaultAllowlist, wantErr: false},
		{name: "valid_std_glob_c11", flags: []string{"-std=c11"}, allowlist: defaultAllowlist, wantErr: false},
		{name: "valid_std_glob_gnu", flags: []string{"-std=gnu++17"}, allowlist: defaultAllowlist, wantErr: false},
		{name: "multiple_valid_flags", flags: []string{"-O2", "-Wall"}, allowlist: defaultAllowlist, wantErr: false},
		{name: "complex_valid_combo", flags: []string{"-O3", "-Wall", "-Wextra", "-std=c++20"}, allowlist: defaultAllowlist, wantErr: false},
		{name: "no_flags_with_allowlist", flags: []string{}, allowlist: defaultAllowlist, wantErr: false},

		// ===== Invalid cases - compiler injection attacks =====
		{name: "plugin_injection", flags: []string{"-fplugin=evil.so"}, allowlist: defaultAllowlist, wantErr: true},
		{name: "response_file", flags: []string{"@response_file"}, allowlist: defaultAllowlist, wantErr: true},
		{name: "specs_injection", flags: []string{"--specs=/tmp/evil.spec"}, allowlist: defaultAllowlist, wantErr: true},
		{name: "linker_rpath", flags: []string{"-Wl,-rpath,/evil"}, allowlist: defaultAllowlist, wantErr: true},
		{name: "linker_dynamic", flags: []string{"-Wl,--dynamic-linker=/evil/ld.so"}, allowlist: defaultAllowlist, wantErr: true},
		{name: "search_path_B", flags: []string{"-B/tmp/evil"}, allowlist: defaultAllowlist, wantErr: true},
		{name: "language_override_c", flags: []string{"-x", "c"}, allowlist: defaultAllowlist, wantErr: true},
		{name: "language_override_cpp_output", flags: []string{"-x", "cpp-output"}, allowlist: defaultAllowlist, wantErr: true},
		{name: "include_system", flags: []string{"-isystem", "/etc"}, allowlist: defaultAllowlist, wantErr: true},
		{name: "include_quote", flags: []string{"-iquote", "/tmp"}, allowlist: defaultAllowlist, wantErr: true},
		{name: "plugin_arg", flags: []string{"-fplugin-arg=x"}, allowlist: defaultAllowlist, wantErr: true},

		// ===== Mixed valid + invalid =====
		{name: "valid_with_injection", flags: []string{"-O2", "-fplugin=evil", "-Wall"}, allowlist: defaultAllowlist, wantErr: true},
		{name: "valid_with_response_file", flags: []string{"-std=c++17", "@rsp.txt", "-O3"}, allowlist: defaultAllowlist, wantErr: true},

		// ===== Edge cases with nil/empty allowlist =====
		{name: "nil_allowlist_with_flags", flags: []string{"-O2"}, allowlist: nil, wantErr: true},
		{name: "empty_allowlist_with_flags", flags: []string{"-O2"}, allowlist: []string{}, wantErr: true},
		{name: "nil_allowlist_no_flags", flags: []string{}, allowlist: nil, wantErr: false},
		{name: "empty_allowlist_no_flags", flags: []string{}, allowlist: []string{}, wantErr: false},

		// ===== Single-flag allowlist tests =====
		{name: "single_exact_match", flags: []string{"-O2"}, allowlist: []string{"-O2"}, wantErr: false},
		{name: "single_no_match", flags: []string{"-O3"}, allowlist: []string{"-O2"}, wantErr: true},
		{name: "single_glob_match", flags: []string{"-std=c++17"}, allowlist: []string{"-std=*"}, wantErr: false},
		{name: "single_glob_no_match", flags: []string{"-O2"}, allowlist: []string{"-std=*"}, wantErr: true},

		// ===== Realistic scenarios =====
		{name: "python_flags_allowed", flags: []string{"-u", "-W"}, allowlist: []string{"-c", "-m", "-u", "-W"}, wantErr: false},
		{name: "java_invalid_flag", flags: []string{"-version"}, allowlist: []string{"-cp", "-jar"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFlags(tt.flags, tt.allowlist)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFlags(%v, %v) error = %v, wantErr %v",
					tt.flags, tt.allowlist, err, tt.wantErr)
			}
		})
	}
}

// ============ Benchmark Tests ============

func BenchmarkValidateFilename(b *testing.B) {
	filenames := []string{
		"solution.cpp",
		"../../etc/passwd",
		"MyClass.java",
		".hidden",
		"main.go",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, fn := range filenames {
			ValidateFilename(fn)
		}
	}
}

func BenchmarkValidateFlags(b *testing.B) {
	allowlist := []string{"-O0", "-O1", "-O2", "-O3", "-Wall", "-Wextra", "-std=*"}
	flagSets := [][]string{
		{"-O2"},
		{"-O2", "-Wall"},
		{"-std=c++17", "-O3", "-Wall"},
		{"-fplugin=evil"},
		{"-std=c++20", "-O2", "-Wall"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, flags := range flagSets {
			ValidateFlags(flags, allowlist)
		}
	}
}
