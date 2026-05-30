# goboxd Security Audit

This document outlines the specific security holes that were identified and successfully closed during the implementation of the `goboxd` sandboxing service.

## 1. Path traversal via filename

**Risk**: The Python reference accepts user-supplied filenames without strict validation, allowing `../../etc/passwd` or similar traversal payloads to escape the jail and overwrite or read arbitrary files on the host.
**Location**: `internal/validate/filename.go:42` (also `internal/sandbox/dir.go:49`)
**Fix**: `internal/validate/filename.go` — `ValidateFilename()` and `SafeJoin()`:
- Validates all filenames to reject path separators, leading dots, and directory traversal patterns. Only safe, flat filenames are accepted.
- Employs a defense-in-depth double-check via `SafeJoin` to ensure `filepath.Abs` doesn't evaluate outside the target jail base directory.

```go
func ValidateFilename(name string) error {
	// Check empty string
	if name == "" {
		return ValidationError{
			Code:    "invalid_filename",
			Message: "filename cannot be empty",
		}
	}
// ... 
```
```go
func SafeJoin(base, name string) (string, error) {
	joined := filepath.Join(base, filepath.Base(name))
	absBase, _ := filepath.Abs(base)
	absJoined, _ := filepath.Abs(joined)
	if !strings.HasPrefix(absJoined, absBase+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes sandbox", name)
	}
	return absJoined, nil
}
```

---

## 2. Shell-style directory commands

**Risk**: Creating and deleting directories by formatting shell commands (e.g., `rm -rf <path>`) allows path injection and arbitrary command execution.
**Location**: `internal/sandbox/dir.go:37`, `internal/runner/sweep.go:47`
**Fix**: `internal/sandbox/dir.go` — `NewJailDir()`:
- All directory creation uses Go's `os.MkdirTemp` which safely creates a new directory instead of shelling out.
- Removals invoke pure `os.RemoveAll` to eliminate process shell execution logic.

```go
	// Using os.MkdirTemp as the foundation, under baseDir, using the unique pattern
	targetPath, err := os.MkdirTemp(baseDir, dirName)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp jail dir: %w", err)
	}
```

---

## 3. Compiler-flag injection

**Risk**: Passing user-supplied compiler flags directly to the compiler allows injection of dangerous flags like `-fplugin=evil.so` or `@response_file`.
**Location**: `internal/validate/flags.go:12`
**Fix**: `internal/validate/flags.go` — `ValidateFlags()`:
- Flags are checked rigorously. If no allowlist is configured but flags are provided, all flags are blocked.
- Checks each input flag against a strict allowlist configured via YAML, completely rejecting the job if *any* unallowed flag is injected.

```go
func ValidateFlags(flags, allowlist []string) error {
	if len(flags) == 0 {
		return nil
	}
	if len(allowlist) == 0 {
		return fmt.Errorf("flags are not allowed for this language: %q", flags[0])
	}
	// ... 
```

---

## 4. No request size limits

**Risk**: Reading the entire request body into memory without a size limit enables attackers to send huge payloads, rapidly leading to OOM (Out Of Memory) conditions.
**Location**: `internal/handler/run.go:129`
**Fix**: `internal/handler/run.go` — `runHandler()`:
- Enforces an `http.MaxBytesReader` wrap around the request `r.Body`.
- Does this *before* decoding the JSON, immediately dropping large payloads without loading them.

```go
	// 1. Wrap r.Body with http.MaxBytesReader BEFORE decoding
	maxSourceBytes := int64(h.cfg.MaxSourceBytes)
	r.Body = http.MaxBytesReader(w, r.Body, maxSourceBytes)
```

---

## 5. UID collisions under load

**Risk**: Generating random UIDs in a limited range for per-request directories causes collisions, enabling sandbox path reuse and denial of service under high concurrency.
**Location**: `internal/sandbox/dir.go:27`
**Fix**: `internal/sandbox/dir.go` — `NewJailDir()`:
- Appends process ID (`os.Getpid()`), an `atomic.Int64` counter, and cryptographic hex randomness logic to the prefix.
- Ensures absolute uniqueness without blocking state.

```go
func NewJailDir(baseDir string) (path string, cleanup func(), err error) {
	// Generate 3 bytes of secure random data -> 6 hex characters
	randomBytes := make([]byte, 3)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}
	randomHex := hex.EncodeToString(randomBytes)

	// Unique name pattern incorporating PID, thread-safe counter, and random hex
	dirName := fmt.Sprintf("goboxd-%d-%d-%s", os.Getpid(), jailCounter.Add(1), randomHex)
```

---

## 6. Unbounded child output

**Risk**: Reading the full stdout/stderr of child processes into memory allows a runaway or malicious process to easily write gigabytes and crash the server.
**Location**: `internal/sandbox/limits.go:36`
**Fix**: `internal/sandbox/limits.go` — `CapReader()`:
- Decorates the subprocess io stream pipes with a strict character limit.
- Efficiently terminates output read buffers directly in Go code upon exceeding caps.

```go
func CapReader(r io.Reader, limit int64) io.Reader {
	return &capReader{
		r:     r,
		limit: limit,
	}
}
```

---

## 7. Stale jail directories

**Risk**: Abandoning old jail directories indefinitely on disk creates exhaustion bugs (`no space left on device`) and leaves remnants of execution.
**Location**: `internal/runner/sweep.go:25`
**Fix**: `internal/runner/sweep.go` — `SweepOrphanedDirectories()`:
- Periodically background scans the designated base directory.
- Drops directories ending with the targeted prefix once they expire past a set `maxAge`.

```go
func SweepOrphanedDirectories(baseDir string, maxAge time.Duration) {
	entries, err := os.ReadDir(baseDir)
	// ... 
```

---

## 8. Slowloris / slow-body HTTP attack

**Risk**: Sending data extremely slowly holds server connections indefinitely, sequentially draining all descriptors limit.
**Location**: `cmd/goboxd/main.go:118`
**Fix**: `cmd/goboxd/main.go` — `main()`:
- Eliminates standard bare `http.ListenAndServe()` calls that default to indefinite holds.
- Employs strict, explicit timeout budgets assigned to native Go `http.Server`.

```go
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
		// Prevent slowloris by limiting time to read the entire request body
		ReadTimeout: 15 * time.Second,
		// Prevent slowloris by limiting time to read headers
		ReadHeaderTimeout: 5 * time.Second,
		// WriteTimeout must be > max nsjail wall time of ~10s + build time ~30s (at least 2x max possible job duration)
		WriteTimeout: 120 * time.Second,
		// Prevent idle connections from lingering indefinitely
		IdleTimeout: 60 * time.Second,
	}
```

---

## 9. Symlink attack on jail source file (TOCTOU)

**Risk**: Placing a symlink just in time before writing permits standard I/O writers (`os.WriteFile`) to overwrite arbitrary external nodes.
**Location**: `internal/runner/runner.go:445`
**Fix**: `internal/runner/runner.go` — `Run()`:
- Pre-opens files securely restricting against link resolution via `syscall.O_NOFOLLOW`.
- Fails rapidly out of boundary when encountering existing filesystem anomalies.

```go
	// Safely open the file to prevent TOCTOU symlink attacks
	f, err := os.OpenFile(sourcePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0o644)
	if err != nil {
		return RunResult{Status: status.StatusInternalError}, fmt.Errorf("failed to safely open source file: %w", err)
	}
```

---

## 10. Max Limit Overrides (`MergeWithCap`)

**Risk**: Processing client max parameter allocations directly without caps could permit resource exhaustion on compute nodes.
**Location**: `internal/config/config.go`
**Fix**: `internal/config/config.go` — `MergeWithCap()`:
- Establishes a floor restriction taking language defaults configuration as strict bounds over incoming runner constraints.
- Employs this within runner pipeline structures dynamically validating client capabilities prior to job setup.

```go
func (l ResourceLimits) MergeWithCap(override ResourceLimits) ResourceLimits {
	out := l // start from language defaults
	if override.WallTimeS > 0 && override.WallTimeS < l.WallTimeS {
		out.WallTimeS = override.WallTimeS
	}
	if override.MemoryKB > 0 && override.MemoryKB < l.MemoryKB {
		out.MemoryKB = override.MemoryKB
	}
// ...
```

---

## 11. Seccomp syscall filtering

**Risk**: The default nsjail sandbox provides isolation via namespaces, but the Linux kernel still exposes hundreds of syscalls. Some of these, like `ptrace`, `bpf`, `mount`, or `unshare`, can be exploited by advanced sandbox escapes if left unblocked.
**Location**: `internal/runner/runner.go`
**Fix**: `internal/runner/runner.go` — `buildNsjailRunArgs` and `buildNsjailBuildArgs`:
- Implements a strict `seccomp` Kafel policy blocking 28 dangerous syscalls.
- This applies a kernel-level filter using BPF to kill any sandbox process that attempts an unauthorized syscall.

```go
const seccompPolicy = `POLICY goboxd_safe {
    KILL_PROCESS {
        ptrace, process_vm_readv, process_vm_writev,
        init_module, finit_module, delete_module,
        kexec_load, reboot, settimeofday, adjtimex, clock_adjtime,
        mknodat, chroot, pivot_root, unshare, setns,
        userfaultfd, name_to_handle_at, open_by_handle_at,
        acct, bpf, syslog, add_key, request_key, keyctl,
        fanotify_init, capset, mount
    }
}
USE goboxd_safe DEFAULT ALLOW`
```

---

## Further Reading

- [Getting Started](getting-started.md) — Setup guide for beginners
- [Development Guide](development.md) — Contributing, testing, and CI/CD
- [Architecture](architecture.md) — System design and request lifecycle
- [Testing](testing.md) — Validation test suite documentation

