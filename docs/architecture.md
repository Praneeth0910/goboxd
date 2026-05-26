# goboxd Architecture

## Overview

goboxd is a stateless HTTP service that receives untrusted source code, compiles it (if needed), runs it inside an nsjail sandbox, and returns structured results including stdout, stderr, wall time, and a pass/fail status per test case.

## Component Diagram

```
                           +-----------+
    HTTP Client  -------->  |  Handler  |  (internal/handler)
                           |  Layer    |
                           +-----+-----+
                                 |
                     +-----------+-----------+
                     |                       |
               +-----+-----+         +------+------+
               |  Validate |         |    Config    |
               |   Layer   |         |    Loader    |
               | (validate)|         |   (config)   |
               +-----------+         +-------------+
                     |
               +-----+-----+
               |   Runner   |  (internal/runner)
               |  Sandbox   |
               +-----+-----+
                     |
               +-----+-----+
               |   nsjail   |  (Linux namespaces + cgroups)
               +-----------+
```

## Package Responsibilities

### `cmd/goboxd` -- Entry Point

- Parses CLI flags and loads `languages.yaml`
- Runs startup probes for nsjail and each language
- Sweeps orphaned jail directories from prior runs
- Creates the HTTP server with chi router and structured logging middleware
- Starts the HTTP listener on `:8080`

### `internal/config` -- Configuration

- Loads and validates `languages.yaml`
- Defines the `Config`, `Language`, `BuildConfig`, `RunConfig`, and `ResourceLimits` types
- Validates required fields and enforces structural constraints

### `internal/handler` -- HTTP Handlers

- **`health.go`**: Implements `GET /healthz` (liveness), `GET /readyz` (readiness with probe data), and `GET /info` (build info, languages, limits, stats)
- **`run.go`**: Implements `POST /run`. Decodes the JSON request, validates all fields (language, filename, flags, test count), acquires a concurrency slot via buffered channel, delegates to `runner.RunSandbox`, and returns a structured 200 response. User code failures never cause 5xx.

### `internal/validate` -- Input Validation

- **`filename.go`**: Rejects filenames containing path separators (`/`, `\`), leading dots, or directory traversal patterns
- **`flags.go`**: Validates compiler flags against a per-language allowlist, supporting exact match and glob patterns (e.g., `-std=*`)

### `internal/runner` -- Sandbox Execution

- **`runner.go`**: Core execution engine. Creates a temporary jail directory, writes the source file, constructs nsjail arguments, runs build (if compiled language) then run phases, collects stdout/stderr/exit code, compares output, and assembles the `RunResult`
- **`probe.go`**: Startup probes that verify nsjail exists and each configured language binary is available
- **`sweep.go`**: Removes stale jail directories at startup (age-based cleanup)

### `internal/sandbox` -- Jail Directory Management

- **`dir.go`**: Creates unique per-request jail directories using atomic counter + PID + random hex to prevent UID collisions
- **`limits.go`**: Merges per-request limit overrides with language defaults and enforces caps via `CapReader` for stdout/stderr

### `internal/status` -- Status Vocabulary

- Defines all status constants (`accepted`, `wrong_output`, `runtime_error`, `time_exceeded`, `memory_exceeded`, `build_failed`, etc.)
- `TopLevelStatus`: Computes the overall job status from build + test results
- `CompareOutput`: Byte-exact comparison with whitespace-mismatch detection
- `StatusFromExitCode`: Maps exit code + timeout + OOM to a status string

### `internal/stats` -- Metrics

- Thread-safe atomic counters for `in_flight_jobs`, `jobs_total`, `jobs_failed_internal`
- Tracks `last_internal_error_at` timestamp

### `internal/middleware` -- HTTP Middleware

- **`logger.go`**: Structured JSON request logging with method, path, status code, duration, language, and job status fields

## Request Lifecycle

1. Client sends `POST /run` with JSON body
2. Handler decodes and validates the request (language, filenames, flags, test count)
3. Handler acquires a concurrency semaphore slot (blocks if full, providing natural queue behavior)
4. Runner creates a unique jail directory under `/tmp/goboxd/`
5. Runner writes the source code to the jail directory
6. **Build phase** (compiled languages only): nsjail executes the compiler with bind-mounted paths and `PATH` injection
7. **Run phase** (per test case): nsjail executes the program with stdin piped and stdout/stderr captured
8. Runner compares actual vs expected output and assigns per-test statuses
9. Runner computes the top-level status and returns the result
10. Handler serializes the response as JSON (always HTTP 200 for execution results)
11. Jail directory is cleaned up

## Concurrency Model

Concurrency is bounded by a buffered channel acting as a counting semaphore. The channel capacity is set by `max_concurrent` in `languages.yaml` (0 means auto-detect from `runtime.NumCPU`). Requests that exceed the limit block in the handler until a slot becomes available, providing natural FIFO queue behavior without any external dependencies.

## Sandbox Isolation

Each execution runs inside an nsjail sandbox with:
- Separate PID, mount, UTS, IPC, and network namespaces
- Read-only bind mounts for system libraries
- Writable tmpfs for the working directory
- Strict resource limits (wall time, memory, max processes)
- No network access (CLONE_NEWNET)
