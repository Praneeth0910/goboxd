#!/usr/bin/env python3
"""
Attack test script for filename validation
Tests path traversal and other injection attacks
"""

import subprocess
import json
import sys

# Test cases: (filename, should_fail, description)
ATTACK_VECTORS = [
    # Path traversal attacks
    ("../../etc/passwd", True, "Classic path traversal"),
    ("../../../etc/shadow", True, "Deep path traversal"),
    ("..\\..\\windows\\system32\\config\\sam", True, "Windows path traversal"),
    ("..\\..\\..\\root\\.ssh\\id_rsa", True, "Windows SSH key traversal"),
    
    # Absolute paths
    ("/etc/passwd", True, "Absolute path to passwd"),
    ("/root/.ssh/id_rsa", True, "Absolute path to SSH key"),
    ("C:\\Windows\\System32\\config\\SAM", True, "Windows absolute path"),
    
    # Hidden files and dot traversal
    (".bashrc", True, "Hidden bash config"),
    (".ssh", True, "Hidden SSH directory"),
    ("..", True, "Parent directory reference"),
    ("..passwd", True, "Dot prefix with traversal"),
    ("....", True, "Multiple dots"),
    
    # Null byte injection
    ("solution.py\x00.txt", True, "Null byte injection"),
    ("file\x00etc\x00passwd", True, "Multiple null bytes"),
    
    # Control character injection
    ("solution.py\r\nAuthority: admin", True, "CRLF injection"),
    ("file.cpp\t../../etc\tpasswd", True, "Tab injection"),
    ("solution\x1bpy", True, "ESC character"),
    
    # Length overflow
    ("a" * 129, True, "Length exceeds 128 chars"),
    ("a" * 256, True, "Very long filename"),
    
    # Mixed attacks
    ("../../.bashrc", True, "Traversal + hidden"),
    ("./solution.cpp", True, "Relative path"),
    ("/./etc/passwd", True, "Absolute + relative combo"),
    ("solution/../../../etc/passwd", True, "Traversal in middle"),
    
    # Valid filenames
    ("solution.cpp", False, "Valid C++ file"),
    ("solution.py", False, "Valid Python file"),
    ("Solution.java", False, "Valid Java file"),
    ("main.go", False, "Valid Go file"),
    ("solution_v2.py", False, "Valid with underscore and number"),
    ("test-file.cpp", False, "Valid with dash"),
    ("README.md", False, "Valid README"),
    ("file123.txt", False, "Valid with numbers"),
    ("something..", False, "Valid trailing dots"),
]

def test_validation():
    """Test the ValidateFilename function"""
    
    # Create a Go test program
    test_code = '''
package main

import (
    "fmt"
    "encoding/json"
    "github.com/thesouldev/goboxd/internal/validate"
)

func main() {
    filename := ` + "`" + `%s` + "`" + `
    err := validate.ValidateFilename(filename)
    
    if err != nil {
        var errJSON validate.ValidationError
        json.Unmarshal([]byte(err.Error()), &errJSON)
        fmt.Printf(`+"`"+"REJECT|%s|%s|%s"+"`"+`\n`, filename, errJSON.Code, errJSON.Message)
    } else {
        fmt.Printf(`+"`"+"ACCEPT|%s"+"`"+`\n`, filename)
    }
}
'''
    
    passed = 0
    failed = 0
    
    print("=" * 80)
    print("FILENAME VALIDATION ATTACK TEST SUITE")
    print("=" * 80)
    print()
    
    for filename, should_fail, description in ATTACK_VECTORS:
        # Create temp Go file
        import tempfile
        import os
        
        with tempfile.NamedTemporaryFile(mode='w', suffix='.go', delete=False) as f:
            try:
                # Escape filename for Go string
                escaped = filename.replace('\\', '\\\\').replace('`', '\\`').replace('\n', '\\n').replace('\r', '\\r').replace('\t', '\\t')
                escaped = repr(escaped)[1:-1]  # Use Python repr but remove quotes
                
                f.write(test_code % escaped)
                f.flush()
                temp_file = f.name
            except Exception as e:
                print(f"SKIP: {description:40} - {repr(filename)[:50]}")
                print(f"       Error creating test: {e}")
                print()
                continue
        
        try:
            # Run the test
            result = subprocess.run(
                ['go', 'run', temp_file],
                cwd='/home/praneeth_0910/goboxd',
                capture_output=True,
                text=True,
                timeout=5
            )
            
            output = result.stdout.strip()
            
            if output.startswith('REJECT|'):
                parts = output.split('|', 3)
                code = parts[2]
                message = parts[3] if len(parts) > 3 else "Unknown"
                
                if should_fail:
                    status = "✓ PASS"
                    passed += 1
                else:
                    status = "✗ FAIL"
                    failed += 1
                    
                print(f"{status}: {description:40} - REJECTED")
                print(f"       Input: {repr(filename)[:60]}")
                print(f"       Code: {code}")
                print(f"       Msg: {message[:60]}")
                
            elif output.startswith('ACCEPT|'):
                if not should_fail:
                    status = "✓ PASS"
                    passed += 1
                else:
                    status = "✗ FAIL"
                    failed += 1
                    
                print(f"{status}: {description:40} - ACCEPTED")
                print(f"       Input: {repr(filename)[:60]}")
                
            else:
                print(f"? ERROR: {description:40}")
                print(f"       Output: {output}")
                failed += 1
                
        except subprocess.TimeoutExpired:
            print(f"✗ TIMEOUT: {description:40}")
            failed += 1
        except Exception as e:
            print(f"✗ ERROR: {description:40} - {e}")
            failed += 1
        finally:
            try:
                os.unlink(temp_file)
            except:
                pass
        
        print()
    
    print("=" * 80)
    print(f"RESULTS: {passed} passed, {failed} failed out of {len(ATTACK_VECTORS)} tests")
    print("=" * 80)
    
    return 0 if failed == 0 else 1

if __name__ == '__main__':
    sys.exit(test_validation())
