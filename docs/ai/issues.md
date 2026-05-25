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