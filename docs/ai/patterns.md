# Patterns-of-implementation

## Deferred Resource Cleanup

**Context:** Sandbox directories must be cleaned up on every execution path, including early returns and errors. Without proper cleanup, orphaned /tmp/goboxd directories accumulate and exhaust disk space.

**Pattern:**
Create a sandbox manager that defers cleanup immediately after directory creation. The defer block runs regardless of success/failure, ensuring cleanup always happens. Use error variables to preserve original execution errors over cleanup errors.

**Where we used it:**
[internal/sandbox/dir.go](../../../internal/sandbox/dir.go) - Create() + Cleanup() pair, with deferred Cleanup in Execute flow. [internal/runner/runner.go](../../../internal/runner/runner.go) - Execute() method defers sbx.Cleanup() immediately after NewDir().
