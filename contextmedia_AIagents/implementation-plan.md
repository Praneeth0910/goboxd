# implementation-plan.md

**Team:** Sudo — Baratam Praneeth Gupta  
**Repo:** `github.com/Praneeth0910/goboxd` | Branch: `team/team-sudo`  
**Framework:** `github.com/go-chi/chi/v5`  
**Deadline:** June 1, 2026 at 23:59 (Stage 1) → June 4–5 (Stage 2) → Paradox (Stage 3)  
**Target:** 50–60 clean, structured commits across all three stages

---

## Guiding Philosophy

> Build the simplest thing that is **correct first**, then make it **fast**, then make it **hardened**.  
> Every commit must leave the repo in a buildable, non-broken state.  
> Commit prefixes: `feat:` `fix:` `docs:` `test:` `chore:` `refactor:` `security:` `perf:`

---

## Project Structure (Target Layout)

```
goboxd/
├── cmd/
│   └── goboxd/
│       └── main.go                  # entrypoint
├── internal/
│   ├── config/
│   │   ├── config.go                # YAML loader + validator
│   │   └── config_test.go
│   ├── handler/
│   │   ├── health.go                # GET /healthz, /readyz, /info
│   │   ├── run.go                   # POST /run
│   │   └── run_test.go
│   ├── runner/
│   │   ├── runner.go                # nsjail invocation + sandbox lifecycle
│   │   ├── probe.go                 # nsjail + language smoke probes
│   │   ├── sweep.go                 # orphan directory cleanup
│   │   └── runner_test.go
│   ├── sandbox/
│   │   ├── dir.go                   # per-request jail dir management
│   │   └── limits.go                # resource limit merging
│   ├── validate/
│   │   ├── filename.go              # path traversal prevention
│   │   ├── flags.go                 # per-language flag allowlist
│   │   └── validate_test.go
│   ├── status/
│   │   └── status.go                # status vocabulary + top-level rule
│   ├── stats/
│   │   └── stats.go                 # atomic job counters
│   └── middleware/
│       └── logger.go                # structured JSON request logging
├── external/
│   └── nsjail/                      # git submodule, tag 3.4
├── docs/
│   ├── api.md
│   ├── languages.md
│   ├── security.md
│   ├── benchmarks.md
│   ├── architecture.md
│   └── ai/
│       ├── prompts.md               # REQUIRED
│       ├── plan-evolution.md
│       ├── adrs.md
│       ├── patterns.md
│       ├── issues.md
│       └── postmortem.md
├── languages.yaml                   # language registry
├── scripts/
│   └── load-test.sh                 # hey/vegeta load test script
├── Dockerfile
├── docker-compose.yml
├── Makefile
├── README.md
├── go.mod
└── go.sum
```

---

## Stage 1 — Prototype (May 22 – June 1)

**Goal:** Working sandbox, two languages end-to-end, correct API, clean repo.  
**Commit budget:** ~28 commits

---

### Phase 0 — Repo Bootstrap (Commits 1–5)

#### Commit 1 — `chore: init module and project skeleton`
- `go mod init github.com/thesouldev/goboxd`
- Create all empty package directories listed in the structure above
- Add `.gitignore` (binaries, temp dirs, `*.test`)
- **Verify:** `go build ./...` passes (empty packages)

#### Commit 2 — `chore: add nsjail as git submodule at tag 3.4`
```bash
git submodule add https://github.com/google/nsjail external/nsjail
cd external/nsjail && git checkout 3.4
```
- **Verify:** `git submodule status` shows correct tag hash

#### Commit 3 — `chore: add Dockerfile with nsjail build and Go binary`
- Multi-stage Dockerfile:
  - Stage 1 (`builder`): build nsjail from `external/nsjail` source
  - Stage 2 (`toolchains`): install Python 3, g++, gcc, Node.js (for Stage 1 two-language requirement)
  - Stage 3 (`final`): copy nsjail binary, language toolchains, Go binary
- Set `NSJAIL_PATH=/usr/bin/nsjail` env var
- **Verify:** `docker build -t goboxd .` succeeds

#### Commit 4 — `chore: add docker-compose.yml and Makefile`

**Makefile targets (minimum):**
```makefile
build:      # docker build
run:        # docker-compose up
test:       # go test ./...
integration:# docker run + curl /healthz
load:       # scripts/load-test.sh
lint:       # golangci-lint run
vet:        # go vet ./...
```

**docker-compose.yml:** single service, port 8080, mounts `languages.yaml`

#### Commit 5 — `docs: add README.md skeleton and docs/ structure`
- `README.md`: what it is, how to run (points to Makefile), where the docs are. No AI filler.
- Create empty `docs/api.md`, `docs/languages.md`, `docs/security.md`, `docs/benchmarks.md`, `docs/architecture.md`
- Create `docs/ai/prompts.md` with template header
- **Rule:** If a sentence in README wouldn't survive a code review, cut it.

---

### Phase 1 — Config & Validation (Commits 6–10)

#### Commit 6 — `feat(config): YAML language registry loader`

`internal/config/config.go`:
```go
type RunConfig struct {
    Cmd    string            `yaml:"cmd"`
    Args   []string          `yaml:"args"`
    Limits ResourceLimits    `yaml:"limits"`
}

type BuildConfig struct {
    Cmd           string         `yaml:"cmd"`
    Args          []string       `yaml:"args"`
    Limits        ResourceLimits `yaml:"limits"`
    FlagAllowlist []string       `yaml:"flag_allowlist"`
}

type Language struct {
    ID                     string      `yaml:"id"`
    Name                   string      `yaml:"name"`
    SourceFilename         string      `yaml:"source_filename"`
    SourceFilenameStrategy string      `yaml:"source_filename_strategy"` // "from_request"
    ArtifactFilename       string      `yaml:"artifact"`
    ArtifactFilenameStrategy string    `yaml:"artifact_filename_strategy"`
    Build                  *BuildConfig `yaml:"build,omitempty"`
    Run                    RunConfig    `yaml:"run"`
}

type ResourceLimits struct {
    WallTimeS    int `yaml:"wall_time_s"`
    MemoryKB     int `yaml:"memory_kb"`
    MaxProcesses int `yaml:"max_processes"`
}

type Config struct {
    Languages map[string]Language // keyed by id after load
    // global limits
    MaxSourceBytes int
    MaxTests       int
    MaxConcurrent  int
}

func Load(path string) (*Config, error)
func Validate(cfg *Config) error  // fail loudly at startup
```

- Template placeholder constants: `{{source}}`, `{{artifact}}`, `{{flags}}`
- **Verify:** loads `languages.yaml`, returns error on invalid config

#### Commit 7 — `feat(config): add languages.yaml with python3 and cpp`

```yaml
languages:
  - id: py3
    name: Python 3
    source_filename: solution.py
    run:
      cmd: /usr/bin/python3
      args: ["{{source}}"]
      limits: { wall_time_s: 9, memory_kb: 102400, max_processes: 100 }

  - id: cpp
    name: C++
    source_filename: solution.cpp
    artifact: solution
    build:
      cmd: /usr/bin/g++
      args: ["{{flags}}", "-o", "{{artifact}}", "{{source}}"]
      limits: { wall_time_s: 3, memory_kb: 1048576, max_processes: 100 }
      flag_allowlist: ["-O0","-O1","-O2","-O3","-Wall","-Wextra","-std=*"]
    run:
      cmd: ./{{artifact}}
      limits: { wall_time_s: 3, memory_kb: 524288, max_processes: 64 }
```

#### Commit 8 — `feat(validate): filename path-traversal prevention` *(Security Hole #1)*

`internal/validate/filename.go`:
```go
// ValidateFilename rejects:
//   - empty string
//   - any path separator (/ or \)
//   - leading dot (.hidden)
//   - absolute paths
//   - length > 128 characters
//   - directory traversal components (..)
func ValidateFilename(name string) error
```

#### Commit 9 — `feat(validate): per-language flag allowlist` *(Security Hole #3)*

`internal/validate/flags.go`:
```go
// ValidateFlags checks each flag against the language's flag_allowlist.
// Supports glob-style patterns (e.g. "-std=*" matches "-std=c++17").
// Returns 400-worthy error listing the rejected flags.
func ValidateFlags(flags []string, allowlist []string) error
```

#### Commit 10 — `test(validate): unit tests for filename and flag validation`

- Table-driven tests for `ValidateFilename`:
  - `../../etc/passwd` → error
  - `/absolute/path` → error
  - `.hidden` → error
  - `solution.cpp` → ok
  - `MyClass.java` → ok
- Table-driven tests for `ValidateFlags`:
  - `-fplugin=evil` → error
  - `@response_file` → error
  - `-O2` → ok for cpp
  - `-std=c++17` matches `-std=*` → ok

---

### Phase 2 — HTTP Layer (Commits 11–14)

#### Commit 11 — `feat(handler): GET /healthz with chi router`

`cmd/goboxd/main.go` + `internal/handler/health.go`:
```go
r := chi.NewRouter()
r.Use(middleware.Logger)          // chi's built-in logger
r.Use(middleware.Recoverer)       // don't crash on panic

r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
})
```

- **Verify:** `curl localhost:8080/healthz` returns `{"status":"ok"}`

#### Commit 12 — `feat(runner): nsjail probe + language smoke probes`

`internal/runner/probe.go`:
```go
type ProbeResult struct {
    OK      bool
    Version string
    Error   string
}

func ProbeNsjail() ProbeResult         // runs nsjail --version
func ProbeLanguage(lang Language) ProbeResult  // runs cmd --version or override
```

#### Commit 13 — `feat(handler): GET /readyz`

```go
// Returns 200 only if nsjail OK + all language probes OK.
// Returns 503 with breakdown on any failure.
// JSON shape exactly as spec §03.
func (h *HealthHandler) Readyz(w http.ResponseWriter, r *http.Request)
```

#### Commit 14 — `feat(handler): GET /info`

```go
// Always 200.
// Returns build_info (version, commit, go_version),
//         nsjail (path, version),
//         languages[] with id/name/version/default_run_limits,
//         limits (max_source_bytes, max_tests, max_concurrent_jobs),
//         stats (in_flight, jobs_total, jobs_failed_internal, last_error, disk_free)
func (h *HealthHandler) Info(w http.ResponseWriter, r *http.Request)
```

---

### Phase 3 — Sandbox Core (Commits 15–20)

#### Commit 15 — `feat(sandbox): per-request jail directory with unique ID` *(Security Hole #5)*

`internal/sandbox/dir.go`:
```go
var jailCounter atomic.Int64

// NewJailDir creates a unique directory under baseDir.
// Name format: goboxd-{PID}-{counter}-{randomHex}
// Never reuses a directory. Returns cleanup func.
func NewJailDir(baseDir string) (path string, cleanup func(), err error)
```

- Use `os.MkdirTemp` as the foundation
- Cleanup func: `os.RemoveAll(path)` — called via `defer cleanup()` in caller

#### Commit 16 — `feat(runner): startup orphan sweep` *(Security Hole #7)*

`internal/runner/sweep.go`:
```go
// SweepOrphanedDirectories removes goboxd-* dirs in baseDir
// that are older than maxAge. Called once at startup.
func SweepOrphanedDirectories(baseDir string, maxAge time.Duration)
```

- Pattern match: `goboxd-*` prefix
- Check `os.Stat` mtime; remove if older than maxAge (default 10 min)

#### Commit 17 — `feat(runner): request size limits + output cap` *(Security Holes #4 and #6)*

`internal/sandbox/limits.go`:
```go
const (
    DefaultMaxSourceBytes   = 256 * 1024       // 256 KiB
    DefaultMaxStdinBytes    = 64 * 1024        // 64 KiB
    DefaultMaxOutputBytes   = 1 * 1024 * 1024  // 1 MiB
    TruncationMarker        = "\n[output truncated]"
)

// CapReader wraps an io.Reader, enforcing a byte limit.
// On limit exceeded, appends TruncationMarker.
func CapReader(r io.Reader, limit int64) io.Reader
```

- HTTP body size: `http.MaxBytesReader(w, r.Body, maxSourceBytes)`
- Child stdout/stderr: wrapped with `CapReader`

#### Commit 18 — `feat(runner): nsjail invocation and process execution`

`internal/runner/runner.go` core logic:
```go
type RunRequest struct {
    Language         string
    Source           string
    SourceFilename   string
    ArtifactFilename string
    Build            *PhaseConfig
    Run              PhaseConfig
    Tests            []TestCase
}

type RunResult struct {
    Status string
    Build  BuildResult
    Tests  []TestResult
}

// RunSandbox: full lifecycle for one POST /run request.
// 1. Create jail dir (defer cleanup)
// 2. Write source file
// 3. If lang has build step: invoke nsjail for build, capture output
// 4. For each test: invoke nsjail for run, compare stdout
// 5. Return result with correct status vocabulary
func RunSandbox(cfg *config.Config, req RunRequest) (RunResult, error)
```

**nsjail invocation pattern:**
```go
args := []string{
    "--mode", "o",                          // one-shot mode
    "--time_limit", strconv.Itoa(limits.WallTimeS),
    "--rlimit_as", strconv.Itoa(limits.MemoryKB),
    "--max_cpus", "1",
    "--log_fd", "3",                        // nsjail logs to fd 3
    "--bindmount_ro", "/usr:/usr",
    "--bindmount_ro", "/lib:/lib",
    "--bindmount_ro", "/lib64:/lib64",
    "--chroot", jailDir,
    "--",
    cmdPath,
}
args = append(args, resolvedArgs...)
```

#### Commit 19 — `feat(status): status vocabulary and top-level rule`

`internal/status/status.go`:
```go
const (
    StatusAccepted               = "accepted"
    StatusBuildFailed            = "build_failed"
    StatusWrongOutput            = "wrong_output"
    StatusOutputWhitespaceMismatch = "output_whitespace_mismatch"
    StatusTimeExceeded           = "time_exceeded"
    StatusMemoryExceeded         = "memory_exceeded"
    StatusRuntimeError           = "runtime_error"
    StatusNotExecuted            = "not_executed"
    StatusInternalError          = "internal_error"
)

// TopLevelStatus computes top-level status from build + test results.
// Rule: accepted only if build ok AND all tests accepted.
// If build fails: build_failed, all tests not_executed.
// Otherwise: first non-accepted test status.
func TopLevelStatus(buildStatus string, testStatuses []string) string

// CompareOutput compares actual vs expected stdout.
// Returns accepted, wrong_output, or output_whitespace_mismatch.
func CompareOutput(actual, expected string) string
```

#### Commit 20 — `feat(handler): POST /run handler with full validation`

`internal/handler/run.go`:
```go
func NewRunHandler(cfg *config.Config, s *stats.Stats) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // 1. Body size cap (http.MaxBytesReader)
        // 2. Decode JSON
        // 3. Validate language exists → 400
        // 4. Validate source_filename (if provided) → 400
        // 5. Validate flags against allowlist → 400
        // 6. Validate test count ≤ max_tests → 400
        // 7. Acquire concurrency slot (blocking queue)
        // 8. runner.RunSandbox(cfg, req)
        // 9. Release slot
        // 10. Respond 200 with result (never 5xx for user-code failure)
    }
}
```

---

### Phase 4 — Concurrency (Commit 21)

#### Commit 21 — `feat(handler): bounded concurrency queue`

```go
// In main.go or handler init:
maxConcurrent := runtime.NumCPU()
if v := os.Getenv("GOBOXD_MAX_CONCURRENT"); v != "" {
    maxConcurrent, _ = strconv.Atoi(v)
}
sem := make(chan struct{}, maxConcurrent)

// In run handler:
sem <- struct{}{}           // acquire (blocks if full — queue behaviour)
defer func() { <-sem }()   // release on exit
```

- Requests **queue**, not fail, when limit reached
- `maxConcurrent` also exposed in `/info` response as `max_concurrent_jobs`

---

### Phase 5 — Security Hardening (Commits 22–24)

#### Commit 22 — `security: use Go filesystem APIs for jail dirs` *(Security Hole #2)*

- Audit every path operation in `runner.go` and `sandbox/dir.go`
- Replace any `exec.Command("rm", "-rf", ...)` patterns with `os.RemoveAll()`
- Replace any `exec.Command("mkdir", ...)` patterns with `os.MkdirAll()`
- No shell string formatting of paths — ever

#### Commit 23 — `security: cap child output with truncation marker` *(Security Hole #6 — explicit wiring)*

- Wire `CapReader` into the nsjail stdout/stderr pipe readers in `runner.go`
- Add unit test: a program that writes 2 MiB should return capped output with `[output truncated]`

#### Commit 24 — `security: document all 7 holes with file:line links`

`docs/security.md`:
- For each hole: name, description, how it manifests in the Python reference, and the exact fix with `file:line` link
- Format ready to paste directly into PR description

---

### Phase 6 — Testing & Polish (Commits 25–28)

#### Commit 25 — `test(runner): unit tests for status mapping and output comparison`

- `TopLevelStatus` table-driven tests: build fail → build_failed, all pass → accepted, mixed → first failure
- `CompareOutput` tests: exact match, whitespace-only difference, wrong output

#### Commit 26 — `test(config): unit tests for config loading and validation`

- Valid YAML loads correctly
- Missing required field → error
- Unknown language id in request → 400
- Config with duplicate language id → error at startup

#### Commit 27 — `feat(middleware): structured JSON request logging`

`internal/middleware/logger.go`:
```go
// One JSON line per request:
// { "ts": "...", "method": "POST", "path": "/run",
//   "language": "py3", "status": 200,
//   "duration_ms": 145, "job_status": "accepted",
//   "request_id": "uuid" }
```

- Uses `log/slog` with JSON handler
- Attach `request_id` via chi middleware context
- This satisfies the **structured logs bonus criterion**

#### Commit 28 — `chore: go vet + golangci-lint clean, final Stage 1 check`

- `golangci-lint run` must be clean
- `docker build && docker run` → `/healthz` returns 200
- `curl -X POST /run` with Python hello-world returns `{"status":"accepted",...}`
- `curl -X POST /run` with C++ hello-world returns `{"status":"accepted",...}`
- **Open PR for Stage 1**

---

## Stage 2 — Polyglot (June 4–5, ~24–36 hours)

**Goal:** All 7 in-scope languages via YAML. Demo-day language in <30 min.  
**Commit budget:** ~12 commits (Commits 29–40)

---

#### Commit 29 — `feat(languages): add C and Java to languages.yaml`
```yaml
  - id: c
    name: C
    source_filename: solution.c
    artifact: solution
    build:
      cmd: /usr/bin/gcc
      args: ["{{flags}}", "-o", "{{artifact}}", "{{source}}"]
      limits: { wall_time_s: 3, memory_kb: 1048576, max_processes: 100 }
      flag_allowlist: ["-O0","-O1","-O2","-O3","-Wall","-Wextra","-std=*"]
    run:
      cmd: ./{{artifact}}
      limits: { wall_time_s: 3, memory_kb: 524288, max_processes: 64 }

  - id: java
    name: Java
    source_filename_strategy: from_request
    artifact_filename_strategy: from_request
    build:
      cmd: /usr/bin/javac
      args: ["{{flags}}", "{{source}}"]
      limits: { wall_time_s: 6, memory_kb: 102400, max_processes: 100 }
    run:
      cmd: /usr/bin/java
      args: ["{{artifact}}"]
      limits: { wall_time_s: 6, memory_kb: 102400, max_processes: 100 }
```

#### Commit 30 — `feat(languages): add Bash and Node.js to languages.yaml`
```yaml
  - id: bash
    name: Bash
    source_filename: solution.sh
    run:
      cmd: /bin/bash
      args: ["{{source}}"]
      limits: { wall_time_s: 5, memory_kb: 51200, max_processes: 50 }

  - id: node
    name: JavaScript (Node)
    source_filename: solution.js
    run:
      cmd: /usr/bin/node
      args: ["{{source}}"]
      limits: { wall_time_s: 5, memory_kb: 204800, max_processes: 100 }
```

#### Commit 31 — `feat(languages): add Verilog to languages.yaml`
```yaml
  - id: verilog
    name: Verilog
    source_filename: solution.v
    artifact: solution
    build:
      cmd: /usr/bin/iverilog
      args: ["-o", "{{artifact}}", "{{source}}"]
      limits: { wall_time_s: 5, memory_kb: 524288, max_processes: 100 }
    run:
      cmd: /usr/bin/vvp
      args: ["{{artifact}}"]
      limits: { wall_time_s: 3, memory_kb: 262144, max_processes: 64 }
```

#### Commit 32 — `chore(docker): install all 7 language toolchains in Dockerfile`
- `apt-get install`: `gcc g++ default-jdk nodejs npm bash iverilog`
- Python 3 already present; verify all `--version` probes pass
- `make build` must succeed

#### Commit 33 — `feat(config): resolve {{flags}} placeholder correctly for interpreted langs`
- Interpreted languages have no build step and no flags in run
- Ensure `{{flags}}` in args is expanded to empty slice, not literal string
- Add config-level validation: if `flag_allowlist` is empty, client-supplied flags still reject with 400

#### Commit 34 — `feat(handler): /readyz and /info reflect full registered language set`
- `/readyz`: probe all 7 languages at startup, include all in response
- `/info`: `languages[]` array includes all 7 with correct default limits
- No hardcoded language list in handler — driven entirely from loaded config

#### Commit 35 — `test(integration): one end-to-end test per language`
- Each language: hello-world source → `status: accepted`
- C++: compilation error source → `status: build_failed`, all tests `not_executed`
- Python: runtime error (division by zero) → `status: runtime_error`
- All tests run against local Docker container via `make integration`

#### Commit 36 — `feat(sandbox): per-request resource limit override merging`
- Client-supplied `build.limits` and `run.limits` partially override language defaults
- `MergeLimits(base ResourceLimits, override *ResourceLimits) ResourceLimits`
- Zero values in override = use base default
- Add unit test

#### Commit 37 — `feat(validate): validate per-request limits are within server maximums`
- Client cannot request `wall_time_s > 60` or `memory_kb > 2097152`
- Returns 400 with descriptive error if exceeded

#### Commit 38 — `docs: complete docs/languages.md and docs/api.md`
- `docs/languages.md`: table of all 7 languages, their IDs, toolchain, default limits, source filename strategy
- `docs/api.md`: full API reference with request/response examples for each endpoint

#### Commit 39 — `feat(bonus): add Rust to languages.yaml` *(+1 bonus point)*
```yaml
  - id: rust
    name: Rust
    source_filename: solution.rs
    artifact: solution
    build:
      cmd: /usr/bin/rustc
      args: ["-o", "{{artifact}}", "{{source}}"]
      limits: { wall_time_s: 10, memory_kb: 2097152, max_processes: 100 }
    run:
      cmd: ./{{artifact}}
      limits: { wall_time_s: 3, memory_kb: 524288, max_processes: 64 }
```
- Add `rustc` install to Dockerfile

#### Commit 40 — `chore: Stage 2 readiness check`
- All 7 in-scope languages pass `make integration`
- `/readyz` returns 200 with all languages `ok: true`
- `/info` lists all 7 (+ Rust)
- Adding a new language = only YAML edit + Dockerfile install line (no Go change)
- **Open PR update for Stage 2**

---

## Stage 3 — Harden & Load (Paradox, in-person)

**Goal:** All 7 security holes closed, real benchmark numbers, holds under sustained load.  
**Commit budget:** ~12 commits (Commits 41–52)

---

#### Commit 41 — `security: audit and lock down all 7 security holes`
- Re-read `docs/security.md` entry for each hole
- Add integration test that proves each fix works:
  - Hole #1: `POST /run` with `source_filename: "../../etc/passwd"` → 400
  - Hole #3: `POST /run` with `flags: ["-fplugin=evil.so"]` → 400
  - Hole #4: `POST /run` with 300 KiB source → 400
  - Hole #6: program that writes 2 MiB stdout → 1 MiB truncated result (not OOM)

#### Commit 42 — `perf: tune concurrency queue with configurable worker pool`
- Replace simple semaphore channel with a proper worker pool if needed for sustained load
- Ensure no goroutine leaks under high concurrency
- `GOBOXD_MAX_CONCURRENT` env var controls pool size

#### Commit 43 — `perf: add per-request timing and nsjail log parsing`
- Capture `duration_ms` accurately for both build and run phases
- Parse `memory_peak_kb` from nsjail's output or cgroup stats
- Expose in test result objects

#### Commit 44 — `feat: load test script and initial benchmark run`
`scripts/load-test.sh`:
```bash
#!/bin/bash
# Usage: ./scripts/load-test.sh [concurrency]
# Requires: hey (https://github.com/rakyll/hey)
PAYLOAD='{"language":"py3","source":"print(\"hi\")","tests":[{"stdin":"","expected_stdout":"hi"}]}'
for C in 1 10 50 100; do
    echo "=== Concurrency: $C ==="
    echo "$PAYLOAD" | hey -n 200 -c $C -m POST \
        -H "Content-Type: application/json" \
        -D /dev/stdin \
        http://localhost:8080/run
done
```

#### Commit 45 — `docs: complete docs/benchmarks.md with real numbers`
`docs/benchmarks.md` format:
```markdown
## Benchmark: Hello World (py3)
Host: [describe the box]
Docker run, clean state, no debugger.

| Clients | Req/sec | p50 (ms) | p95 (ms) | p99 (ms) |
|---------|---------|----------|----------|----------|
| 1       |         |          |          |          |
| 10      |         |          |          |          |
| 50      |         |          |          |          |
| 100     |         |          |          |          |
```
- Fill with actual numbers from `make load`

#### Commit 46 — `docs: complete docs/architecture.md` *(bonus criterion)*
Content:
- Request lifecycle diagram (text-based)
- Package dependency graph
- Concurrency model: how the semaphore/queue works
- Sandbox lifecycle: jail dir creation → write source → build → run tests → cleanup
- Security model: what nsjail provides vs what the Go layer adds
- YAML registry: how languages are loaded and how placeholders are resolved

#### Commit 47 — `feat(bonus): add Zig to languages.yaml` *(+1 bonus point)*

#### Commit 48 — `feat(bonus): add Go to languages.yaml` *(+1 bonus point)*
```yaml
  - id: golang
    name: Go
    source_filename: main.go
    artifact: solution
    build:
      cmd: /usr/local/go/bin/go
      args: ["build", "-o", "{{artifact}}", "{{source}}"]
      limits: { wall_time_s: 10, memory_kb: 2097152, max_processes: 200 }
    run:
      cmd: ./{{artifact}}
      limits: { wall_time_s: 3, memory_kb: 524288, max_processes: 64 }
```

#### Commit 49 — `docs(ai): complete prompts.md and adrs.md`
- Ensure all AI interactions during development are logged in `docs/ai/prompts.md`
- Write `docs/ai/adrs.md` with at least 3 ADRs:
  - Why chi over net/http/gin/echo
  - Why channel semaphore over sync.Mutex for concurrency
  - Why os.MkdirTemp over manual UID scheme for jail dirs

#### Commit 50 — `docs(ai): write plan-evolution.md and postmortem.md`
- `plan-evolution.md`: document at least 2 significant design pivots with dates and reasoning
- `postmortem.md`: what was harder than expected, where AI was confidently wrong, what you'd do on Day 1 differently

#### Commit 51 — `chore: final lint, vet, and security checklist`

**Pre-submission checklist:**
- [ ] `go vet ./...` — clean
- [ ] `golangci-lint run` — clean
- [ ] `docker build -t goboxd .` — succeeds
- [ ] `docker run goboxd` → `curl /healthz` → `{"status":"ok"}`
- [ ] `curl /readyz` → 200 with all languages ok
- [ ] `curl /info` → all languages listed
- [ ] `POST /run` py3 hello-world → `{"status":"accepted"}`
- [ ] `POST /run` cpp hello-world → `{"status":"accepted"}`
- [ ] `POST /run` with `source_filename: "../../etc"` → 400
- [ ] `POST /run` with disallowed flag → 400
- [ ] `make test` → all pass
- [ ] `make integration` → all pass
- [ ] `make load` → numbers in `docs/benchmarks.md`
- [ ] All 7 security holes have `file:line` references in `docs/security.md`
- [ ] README has no "elegant", "robust", "seamlessly", "leverage", no emoji

#### Commit 52 — `docs: final PR description draft`

`PR_DESCRIPTION.md` (keep in docs/, use as PR body):
```markdown
## Team Sudo — Baratam Praneeth Gupta

**Framework:** chi/v5 — composable, idiomatic, zero-dependency HTTP mux.
Avoids gin's magic reflection overhead and echo's global state.

**How to run:**
Clone, then `make build && make run`. Server starts on :8080.
Full target list in the Makefile. See docs/api.md for request examples.

**Security holes closed:**
1. Path traversal — validate/filename.go:L22
2. Shell-style dir commands — sandbox/dir.go:L18
3. Compiler-flag injection — validate/flags.go:L41
4. No request size limits — handler/run.go:L34
5. UID collisions — sandbox/dir.go:L29
6. Unbounded child output — runner/runner.go:L88
7. Stale jail dirs — runner/sweep.go:L15

**Languages:** C, C++, Java, Python 3, Bash, Node.js, Verilog, Rust, Go, Zig

**Benchmarks:** docs/benchmarks.md
```

---

## Commit Summary

| Range | Stage | Phase | Count |
|---|---|---|---|
| 1–5 | Stage 1 | Repo bootstrap | 5 |
| 6–10 | Stage 1 | Config & validation | 5 |
| 11–14 | Stage 1 | HTTP layer | 4 |
| 15–20 | Stage 1 | Sandbox core | 6 |
| 21 | Stage 1 | Concurrency | 1 |
| 22–24 | Stage 1 | Security hardening | 3 |
| 25–28 | Stage 1 | Tests & polish | 4 |
| 29–40 | Stage 2 | Polyglot | 12 |
| 41–52 | Stage 3 | Harden & load | 12 |
| **Total** | | | **52 commits** |

---

## Why This Beats the Competitor (Team silverex)

| Dimension | Team silverex (observed) | Team Sudo (this plan) |
|---|---|---|
| Security holes | Confirmed hole #7 only | All 7 closed + documented |
| Bonus languages | Unknown | Rust, Go, Zig (+3 bonus pts) |
| Benchmarks | Unknown | Real p50/p95/p99 at 4 concurrency levels |
| Architecture doc | Unknown | docs/architecture.md (bonus criterion) |
| AI logging | Unknown | Full docs/ai/ folder (scored) |
| Structured logs | Unknown | JSON request logs (bonus criterion) |
| Stage 3 bonus | Unknown | postmortem.md + plan-evolution.md |

---

## Key ADR — Why chi

```
Context: Need a Go HTTP framework. Options: net/http, chi, gin, echo.
Decision: chi/v5
Rationale: chi is a lightweight, composable router that uses only
standard library interfaces (http.Handler, http.ResponseWriter).
No magic reflection (gin), no global state (echo), no framework
lock-in. Middleware is just http.Handler. Easy to test. The spec
says justify in two sentences — this is it.
```

---

*Last updated: May 24, 2026 — Team Sudo*
