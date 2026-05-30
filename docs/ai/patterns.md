# Patterns-of-implementation

## Deferred Resource Cleanup

**Context:** Sandbox directories must be cleaned up on every execution path, including early returns and errors. Without proper cleanup, orphaned /tmp/goboxd directories accumulate and exhaust disk space.

**Pattern:**
Create a sandbox manager that defers cleanup immediately after directory creation. The defer block runs regardless of success/failure, ensuring cleanup always happens. Use error variables to preserve original execution errors over cleanup errors.

**Where I used it:**
[internal/sandbox/dir.go](../../../internal/sandbox/dir.go) - Create() + Cleanup() pair, with deferred Cleanup in Execute flow. [internal/runner/runner.go](../../../internal/runner/runner.go) - Execute() method defers sbx.Cleanup() immediately after NewDir().

## Table-Driven Tests
**Context:** Flag validation has many edge cases and combinations. Writing individual test functions for each scenario leads to code duplication and maintenance overhead.
**Pattern:**
Use table-driven tests to define a slice of test cases with input and expected output. Iterate over the test cases in a single test function, using sub-tests for better reporting. This approach reduces boilerplate and makes it easy to add new test cases by simply appending to the table.
**Where I used it:**
[internal/validate/table_driven_test.go](../../../internal/validate/table_driven_test.go) - TestValidateFlagsTableDriven and TestValidateFilenameTableDriven functions use table-driven testing to cover various scenarios for flag and filename validation.

## Two-Coordinate Pathing for Chroot Sandboxes

**Context:** When working with a sandbox that utilizes `chroot` (like `nsjail`), there is often confusion between the host filesystem namespace and the jail filesystem namespace. Passing an absolute host path (e.g., `/tmp/goboxd-123/solution.py`) to a process isolated within a chroot of `/tmp/goboxd-123` results in the process looking for `/tmp/goboxd-123/tmp/goboxd-123/solution.py`, leading to file-not-found errors.

**Pattern:**
Explicitly separate path construction based on the sandbox phase:
-   **Host-centric execution (Build phase):** When compilers need access to the broader system (e.g., `/usr/lib`), set the chroot to `/` and explicitly bind-mount the working directory as writable. Use absolute host paths.
-   **Jail-centric execution (Run phase):** When running untrusted code, set the chroot to the isolated directory. Construct paths relative to the new chroot root (e.g., `/solution.py`). Never pass host paths into a tightly chrooted environment.

**Where I used it:**
[internal/runner/runner.go](../../../internal/runner/runner.go) - Separated `buildNsjailBuildArgs` (uses host root `/` with absolute paths) and `buildNsjailRunArgs` (uses jail dir root with relative paths like `"/"+sourceFilename`).

## Constant-Based Path Construction for Lint-Safe Filesystem Paths

**Context:** The `gocritic` linter flags any literal argument to `filepath.Join` that contains a path separator (`/`). When constructing paths under kernel pseudo-filesystems like `/sys/fs/cgroup/`, there's no way to use `filepath.Join` without triggering this diagnostic, because every possible decomposition still contains `/` in at least one segment.

**Pattern:**
Define the well-known base path as a package-level `const` (e.g., `const cgroupBase = "/sys/fs/cgroup/"`), then construct the full path via string concatenation: `path = cgroupBase + dynamicSuffix`. This satisfies the linter (no `filepath.Join` call with separators), avoids `filepath.Join` overhead for a fixed-prefix path, and makes the base path a single point of change.

**Where I used it:**
[internal/runner/runner.go](../../../internal/runner/runner.go) - `const cgroupBase = "/sys/fs/cgroup/"` used in `runCommand` to construct per-request cgroup paths as `cgroupBase + cgroupName`.

## Graceful Fallback for Optional Kernel Features

**Context:** Features like cgroupv2 `memory.peak` tracking depend on the host kernel and container runtime supporting cgroupv2 with the correct mount. In development/CI environments (e.g., GitHub Actions runners), these may not be available. The code should use the feature when available but degrade gracefully when it's not, without failing the request.

**Pattern:**
Attempt the privileged operation (e.g., `os.Mkdir` for the cgroup directory). If it succeeds, set up deferred cleanup and pass the feature name to downstream functions. If it fails, set the feature name to empty string and log a warning. Downstream code checks for empty string and skips the feature entirely. The API response field (e.g., `memory_peak_kb`) defaults to `0`, which callers already handle.

**Where I used it:**
[internal/runner/runner.go](../../../internal/runner/runner.go) - `runCommand` attempts cgroup creation; on failure, sets `cgroupName = ""` and `cgroupPath = ""`, causing `buildNsjailRunArgs` to skip cgroup flags and the post-execution `memory.peak` read to be skipped. The request completes successfully with `memory_peak_kb: 0`.