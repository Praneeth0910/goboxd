# Patterns-of-implementation

## Deferred Resource Cleanup

**Context:** Sandbox directories must be cleaned up on every execution path, including early returns and errors. Without proper cleanup, orphaned /tmp/goboxd directories accumulate and exhaust disk space.

**Pattern:**
Create a sandbox manager that defers cleanup immediately after directory creation. The defer block runs regardless of success/failure, ensuring cleanup always happens. Use error variables to preserve original execution errors over cleanup errors.

**Where we used it:**
[internal/sandbox/dir.go](../../../internal/sandbox/dir.go) - Create() + Cleanup() pair, with deferred Cleanup in Execute flow. [internal/runner/runner.go](../../../internal/runner/runner.go) - Execute() method defers sbx.Cleanup() immediately after NewDir().

## Table-Driven Tests
**Context:** Flag validation has many edge cases and combinations. Writing individual test functions for each scenario leads to code duplication and maintenance overhead.
**Pattern:**
Use table-driven tests to define a slice of test cases with input and expected output. Iterate over the test cases in a single test function, using sub-tests for better reporting. This approach reduces boilerplate and makes it easy to add new test cases by simply appending to the table.
**Where we used it:**
[internal/validate/table_driven_test.go](../../../internal/validate/table_driven_test.go) - TestValidateFlagsTableDriven and TestValidateFilenameTableDriven functions use table-driven testing to cover various scenarios for flag and filename validation.