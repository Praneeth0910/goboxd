# goboxd Security Audit

This document outlines the specific security holes that were identified and successfully closed during the implementation of the `goboxd` sandboxing service.

## 1. Path traversal via filename
**Category:** Input Validation
**Severity:** Critical

### What the Python reference does wrong
Accepts user-supplied filenames without strict validation, allowing `../../etc/passwd` or similar traversal payloads to escape the jail and overwrite or read arbitrary files on the host.

### Our fix in Go
We validate all filenames to reject path separators, leading dots, and directory traversal patterns. Only safe, flat filenames are accepted.

### Location
`internal/validate/filename.go:111`

---

## 2. Shell-style directory commands
**Category:** Filesystem
**Severity:** Critical

### What the Python reference does wrong
Creates and deletes directories by formatting shell commands (e.g., `rm -rf <path>`) and passing them to a shell, allowing path injection and arbitrary command execution.

### Our fix in Go
All directory creation and deletion uses Go's `os.MkdirTemp`, `os.MkdirAll`, and `os.RemoveAll`, never invoking a shell or formatting paths into commands.

### Location
`internal/sandbox/dir.go:36`, `internal/runner/sweep.go:47`

---

## 3. Compiler-flag injection
**Category:** Input Validation
**Severity:** High

### What the Python reference does wrong
Passes user-supplied compiler flags directly to the compiler, allowing injection of dangerous flags like `-fplugin=evil.so` or `@response_file`.

### Our fix in Go
Flags are validated against a strict allowlist (exact match or safe suffix glob). Only explicitly allowed flags are passed to the compiler.

### Location
`internal/validate/flags.go:55`

---

## 4. No request size limits
**Category:** Resource Control
**Severity:** High

### What the Python reference does wrong
Reads the entire request body into memory without any size limit, allowing attackers to send huge payloads and exhaust server memory.

### Our fix in Go
We wrap the request body with `http.MaxBytesReader` and use CapReader to enforce strict size limits on all untrusted input.

### Location
`internal/handler/run.go:127`

---

## 5. UID collisions under load
**Category:** Filesystem
**Severity:** Medium

### What the Python reference does wrong
Uses random UIDs in a limited range for per-request directories, leading to collisions and possible denial of service or directory reuse under high concurrency.

### Our fix in Go
We use an atomic counter, process ID, and random hex string to generate unique, unpredictable directory names for every request.

### Location
`internal/sandbox/dir.go:33`

---

## 6. Unbounded child output
**Category:** Resource Control
**Severity:** High

### What the Python reference does wrong
Reads the full stdout/stderr of child processes into memory, so a runaway or malicious program can write gigabytes and OOM the server.

### Our fix in Go
We wrap all process output pipes with CapReader, which enforces a strict output cap and appends a truncation marker if the limit is exceeded.

### Location
`internal/sandbox/limits.go:36`, `internal/runner/runner.go:259`

---

## 7. Stale jail directories
**Category:** Filesystem
**Severity:** Medium

### What the Python reference does wrong
Leaves old jail directories on disk indefinitely, never cleaning them up, which can fill up disk space and leak sensitive data.

### Our fix in Go
We sweep and remove all orphaned jail directories at startup and on a schedule, using safe age-based logic.

### Location
`internal/runner/sweep.go:25`
