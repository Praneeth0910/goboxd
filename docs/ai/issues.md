# Issues I have faced while building the project and how i overcame them.

## May 25, 2026 - Filename validation test failures with path traversal detection

**What we were trying to do:**
Implement comprehensive filename validation to prevent path traversal attacks (e.g., `../../etc/passwd`). We created unit tests that checked if the validation function correctly rejected malicious filenames and accepted valid ones.

**What went wrong:**
Initial test runs failed in 4 categories: TestValidateFilenameRejectsEmpty, TestValidateFilenameRejectsPathSeparators, TestValidateFilenameRejectsLeadingDot, and TestValidateFilenameRejectsDirectoryTraversal. The test helper function had overly strict keyword matching that failed when error messages didn't contain the exact keywords being searched for. Additionally, some test cases conflicted (e.g., the validator rejected `.." at the start of filenames, but test cases like `./solution.cpp` were lumped into the same group as path separator tests). Also, edge cases like `something..` (trailing dots) should be accepted but weren't properly categorized in tests.

**How we resolved it:**
(1) Simplified the test helper's keyword matching logic to be more flexible and case-insensitive. (2) Reorganized the test cases into clearer categories: pure path separator tests separate from leading-dot tests. (3) Refined the validation logic to only reject leading dots (`.hidden` files and `..` traversal), not trailing dots. (4) Split conflicting test cases (e.g., `./solution.cpp` is rejected for the leading dot, not the path separator). This made error messages align with actual test expectations.

**What we learned:**
Test categorization and error message specificity matter more than comprehensive coverage of edge cases; tight coupling between validator logic and test expectations requires careful separation of concerns (file extension handling vs. path structure validation).

## May 25, 2026 - Compiler flag injection prevention with allowlist matching

**What we were trying to do:**
Implement compiler flag validation to prevent compiler flag injection attacks (e.g., `-fplugin=evil.so`, `@response_file`). Flags needed to support an allowlist that could be either exact matches or glob patterns, particularly for language standards like `-std=*` to match any C++ standard variant.

**What went wrong:**
The initial approach didn't account for suffix glob patterns properly. We needed a whitelist validation system that would reject flags like `-fplugin=anything`, `@response_file`, `--specs=/tmp/evil`, and linker injection attempts like `-Wl,-rpath,/evil`, but accept valid flags and their variations. We had to ensure prefix globs like `--*` were NOT supported (too permissive), while suffix globs like `-std=*` would match `-std=c++17`, `-std=c11`, etc.

**How we resolved it:**
(1) Created `ValidateFlags()` function with strict allowlist matching: exact match for most flags, suffix glob for flexible patterns. (2) Implemented `matchesAllowlist()` helper that checks exact matches first, then handles suffix globs by trimming the `*` and checking prefix. (3) Added comprehensive test suite with 9 specific tests for flag validation covering empty/nil allowlists, exact matches, glob patterns, and real-world compiler attacks. (4) Ensured errors list ALL rejected flags at once, not stopping at the first failure. (5) Used only strings.HasPrefix, HasSuffix, TrimSuffix (no regex) per security requirements.

**What we learned:**
Allowlist-based security is more effective than blacklist filtering; glob pattern support should be minimal and only at the suffix level to prevent overly permissive rules; returning all validation failures together helps clients fix issues more efficiently.

## May 26, 2026 - Performance issues with per-request sandbox directory naming under high concurrency

**What we were trying to do:** Each incoming request to `POST /run` needs an isolated temporary directory for the nsjail sandbox. We initially implemented a UID range approach where we would generate a random UID within a specified range and create a directory named `uid_{random}`. We would retry up to 3 times if a collision occurred (i.e., directory already exists).

**What went wrong:** Under high concurrency (1000+ requests), we observed a significant number of collisions due to the limited UID range and random generation. This led to performance degradation as the system had to retry directory creation multiple times, and in some cases, it failed to create a directory after all retries. Additionally, this approach posed a security risk as it could potentially allow an attacker to predict directory names and cause intentional collisions.

**What we were trying to do:** We switched to an atomic counter approach combined with the process ID (PID) and a random hex string for directory naming. The new format is `goboxd-{PID}-{counter}-{random}`. The atomic counter guarantees that each directory name is unique across all goroutines in the same process, eliminating the possibility of collisions even under high concurrency. The inclusion of the PID and random hex adds an extra layer of uniqueness and makes it more difficult for attackers to predict directory names.

## May 25, 2026 - RunSandbox would have been dead code if added naively alongside run.go

**What we were trying to do:**
Add a `RunSandbox` function in `internal/runner/runner.go` as the canonical execution core, wrapping nsjail and owning the full lifecycle of a job.

**What went wrong:**
The entire execution pipeline (jail dir creation, build, per-test run, output comparison) already lived inside `internal/handler/run.go`. Writing `RunSandbox` without wiring it in would have created two parallel, diverging implementations — neither calling the other. The handler tests would still pass (against the old path), and `RunSandbox` would be dead code.

**How we resolved it:**
We evaluated three approaches:
- **Option A (Chosen):** Full refactor. Extracted all execution logic into `runner.RunSandbox` and kept `run.go` strictly focused on HTTP concerns (validation, mapping, stats, concurrency control).
- **Option B:** Add `RunSandbox` as a skeleton placeholder stub without wiring it in, leaving the handler unchanged.
- **Option C:** Implement nsjail integration directly within the existing handler functions.
We chose Option A to prevent parallel/duplicate code paths, guarantee that the new sandbox logic is fully exercised by tests, and enforce clean separation of concerns.


**What we learned:**
When adding a "core logic" function to a layer below an existing handler, always check whether the handler already owns that logic. If it does, the right move is to extract and wire — not add alongside.