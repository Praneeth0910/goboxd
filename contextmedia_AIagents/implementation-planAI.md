# implementation-planAI.md

**Team:** Sudo — Baratam Praneeth Gupta  
**Repo:** `github.com/Praneeth0910/goboxd` | Branch: `team/team-sudo`  
**Framework:** `github.com/go-chi/chi/v5`  
**Deadline:** June 1, 2026 at 23:59 (Stage 1) → June 4–5 (Stage 2) → Paradox (Stage 3)  
**Purpose:** This file is the AI-assisted version of `implementation-plan.md`.  
Each commit has an exact prompt to paste into an AI tool. Copy, run, review, adapt.

> **Rule:** Never paste AI output directly into the repo without reading it.  
> Log every prompt you use in `docs/ai/prompts.md` with what you used and what you discarded.  
> Commit prefixes: `feat:` `fix:` `docs:` `test:` `chore:` `refactor:` `security:` `perf:`

---

## Stage 1 — Prototype (May 22 – June 1)
### Commit budget: ~28 commits

---

### Phase 0 — Repo Bootstrap (Commits 1–5)

---

#### Commit 1 — `chore: init module and project skeleton`

**Prompt:**
```
I'm starting a Go project called goboxd (Go sandbox daemon). It's an HTTP service 
that runs untrusted code inside nsjail sandboxes and returns per-test results.

The module path is: github.com/thesouldev/goboxd
Go version: 1.22

Create the go.mod file and the following empty package directory structure:
- cmd/goboxd/
- internal/config/
- internal/handler/
- internal/runner/
- internal/sandbox/
- internal/validate/
- internal/status/
- internal/stats/
- internal/middleware/
- external/   (for nsjail submodule later)
- docs/ai/
- scripts/

For each internal package, create a minimal .go file with just the package declaration 
so the module compiles.

Also create a .gitignore that ignores: Go binaries, *.test files, temp directories 
matching goboxd-*, and the built Docker image.

The project uses chi/v5 as the HTTP router. Add it to go.mod.
```

---

#### Commit 2 — `chore: add nsjail as git submodule at tag 3.4`

**Prompt:**
```
I need to add Google's nsjail as a git submodule to my Go project at tag 3.4.

The submodule should live at: external/nsjail
The upstream repo is: https://github.com/google/nsjail

Give me the exact sequence of git commands to:
1. Add the submodule
2. Check out tag 3.4 specifically
3. Verify the submodule is correctly pinned

Also explain what I need to add to my Dockerfile later so nsjail is built from 
source inside the Docker image (not installed from a package archive, and no 
prebuilt binary bundled).
```

---

#### Commit 3 — `chore: add Dockerfile with nsjail build and Go binary`

**Prompt:**
```
Write a multi-stage Dockerfile for a Go project called goboxd that:

Stage 1 (named "nsjail-builder"):
- Starts from debian:bookworm-slim
- Installs all build dependencies for nsjail (bison flex protobuf-compiler 
  libprotobuf-dev libnl-route-3-dev pkg-config g++ make git)
- Copies external/nsjail/ into the image
- Builds nsjail from source with make
- The resulting binary should be at /usr/sbin/nsjail

Stage 2 (named "go-builder"):
- Starts from golang:1.22-bookworm
- Copies go.mod, go.sum and downloads dependencies
- Copies the full source and builds the binary with:
  go build -ldflags "-X main.version=0.1.0 -X main.commit=$(git rev-parse --short HEAD)"
- Output binary at /usr/local/bin/goboxd

Stage 3 (named "toolchains"):
- Starts from debian:bookworm-slim
- Installs: python3 g++ gcc default-jdk nodejs npm bash
- Copies nsjail binary from stage 1
- Copies goboxd binary from stage 2
- Sets ENV NSJAIL_PATH=/usr/sbin/nsjail
- Sets ENV LANGUAGES_CONFIG=/etc/goboxd/languages.yaml
- EXPOSE 8080
- Entrypoint: goboxd binary

The container is the unit of "it works". Keep the final image as small as practical.
```

---

#### Commit 4 — `chore: add docker-compose.yml and Makefile`

**Prompt:**
```
Write a docker-compose.yml and Makefile for a Go project called goboxd.

docker-compose.yml requirements:
- Single service named "goboxd"
- Build from local Dockerfile
- Port mapping: 8080:8080
- Mount languages.yaml as a volume to /etc/goboxd/languages.yaml (read-only)
- Environment variables: GOBOXD_MAX_CONCURRENT (default empty, meaning use NumCPU)
- Restart policy: unless-stopped
- Privileged mode required (nsjail needs it)

Makefile requirements (these exact targets, no others required):
- make build       → docker build
- make run         → docker-compose up
- make test        → go test ./...
- make integration → docker run + curl /healthz, return non-zero on failure
- make load        → run scripts/load-test.sh
- make lint        → golangci-lint run
- make vet         → go vet ./...
- make clean       → remove built binaries and temp files

Use standard Makefile conventions. No bare "go run" instructions anywhere.
```

---

#### Commit 5 — `docs: add README.md skeleton and docs/ structure`

**Prompt:**
```
Write a README.md for a Go project called goboxd (Go sandbox daemon).

Rules for this README (these are scored, follow them exactly):
- Write it as a human wrote it. No AI voice.
- No words: "elegant", "robust", "seamlessly", "leverage", "powerful", "cutting-edge"
- No emoji anywhere
- No marketing copy
- Maximum 40 lines total
- Must contain: what it is (one sentence), how to run (points to Makefile only),
  where the docs are (list docs/ files)
- No "go run" instructions

Also create the following empty files with a single H1 heading placeholder each:
- docs/api.md
- docs/languages.md  
- docs/security.md
- docs/benchmarks.md
- docs/architecture.md

And create docs/ai/prompts.md with this template header:
# AI Interaction Log
Format: Date · Context · Prompt · Response summary · What we used / discarded
```

---

### Phase 1 — Config & Validation (Commits 6–10)

---

#### Commit 6 — `feat(config): YAML language registry loader`

**Prompt:**
```
I'm building goboxd, a Go HTTP service that runs untrusted code in nsjail sandboxes.

Write the file internal/config/config.go with:

1. A Config struct that holds:
   - Languages as map[string]Language (keyed by language id)
   - MaxSourceBytes int (default 262144 = 256 KiB)
   - MaxTests int (default 50)
   - MaxConcurrent int (default 0 = use runtime.NumCPU())

2. A Language struct with these YAML-tagged fields:
   - ID, Name string
   - SourceFilename string (yaml: source_filename)
   - SourceFilenameStrategy string (yaml: source_filename_strategy) 
     values: "" (use field) or "from_request"
   - ArtifactFilename string (yaml: artifact)
   - ArtifactFilenameStrategy string (yaml: artifact_filename_strategy)
   - Build *BuildConfig (optional, pointer, yaml: build)
   - Run RunConfig (yaml: run)

3. BuildConfig struct: Cmd, Args []string, Limits ResourceLimits, 
   FlagAllowlist []string (yaml: flag_allowlist)

4. RunConfig struct: Cmd, Args []string, Limits ResourceLimits

5. ResourceLimits struct: WallTimeS, MemoryKB, MaxProcesses int

6. func Load(path string) (*Config, error)
   - reads YAML file
   - builds map keyed by language id
   - calls Validate before returning

7. func Validate(cfg *Config) error
   - at least one language must exist
   - each language must have a non-empty id, name, and run.cmd
   - no duplicate ids
   - if build exists, cmd must be non-empty
   - fail loudly at startup, not silently

Template placeholders used in args: {{source}}, {{artifact}}, {{flags}}
These are expanded at runtime, not here. Just document the constants.

Use gopkg.in/yaml.v3 for YAML parsing.
```

---

#### Commit 7 — `feat(config): add languages.yaml with python3 and cpp`

**Prompt:**
```
Write a languages.yaml file for goboxd with exactly two language entries for Stage 1:

1. Python 3 (id: py3):
   - Interpreted, no build step
   - Source filename: solution.py
   - Run: /usr/bin/python3 with {{source}} as arg
   - Limits: wall_time_s: 9, memory_kb: 102400, max_processes: 100

2. C++ (id: cpp):
   - Compiled language, has build + run steps
   - Source filename: solution.cpp, artifact name: solution
   - Build: /usr/bin/g++ with {{flags}}, -o {{artifact}}, {{source}}
   - Build limits: wall_time_s: 3, memory_kb: 1048576, max_processes: 100
   - Build flag_allowlist: -O0 -O1 -O2 -O3 -Wall -Wextra -std=* (glob)
   - Run: ./{{artifact}}
   - Run limits: wall_time_s: 3, memory_kb: 524288, max_processes: 64

The YAML shape should match this struct (show me the YAML, not Go code):
- id, name, source_filename, artifact (optional), build (optional), run

Add a comment at the top explaining that adding a new language = one YAML block 
+ one Dockerfile install line, zero Go code changes.
```

---

#### Commit 8 — `feat(validate): filename path-traversal prevention`

**Prompt:**
```
I'm building a Go sandbox service. One of the critical security holes I need to fix 
is path traversal via client-supplied filenames.

The attack: a client sends source_filename: "../../etc/passwd" and the server writes 
the source file to that path, escaping the jail directory.

Write internal/validate/filename.go with a function:
  func ValidateFilename(name string) error

It must reject ALL of the following (return a descriptive error for each case):
- Empty string
- Any path separator (forward slash or backslash)
- Leading dot (.hidden files)
- Absolute paths (starting with /)
- The string ".." or any component that is ".."
- Length greater than 128 characters
- Null bytes or control characters

It must accept:
- "solution.cpp"
- "Solution.java"
- "main.go"
- "solution_v2.py"

Return errors in the format: {code: "invalid_filename", message: "...explanation..."}
that can be serialised to JSON for the API 400 response.

Do not use any external libraries. Standard library only.
```

---

#### Commit 9 — `feat(validate): per-language flag allowlist`

**Prompt:**
```
I'm building a Go sandbox service. I need to prevent compiler flag injection.

The attack: a client sends flags like ["-fplugin=evil.so", "@response_file", 
"--specs=/tmp/evil"] to gcc/g++/javac and gets compile-time code execution.

Write internal/validate/flags.go with a function:
  func ValidateFlags(flags []string, allowlist []string) error

Rules:
- If allowlist is nil or empty: reject ALL flags with a 400 error (no flags allowed)
- Each flag in the client request must match at least one entry in the allowlist
- Allowlist supports exact match: "-O2" matches only "-O2"
- Allowlist supports suffix glob: "-std=*" matches "-std=c++17", "-std=c11", etc.
- Allowlist supports prefix glob: "--*" is NOT supported (too permissive)
- If any flag is rejected, return a single error listing ALL rejected flags

Examples that must be rejected:
- "-fplugin=anything"
- "@response_file"  
- "--specs=anything"
- "-Wl,-rpath,/evil"
- "-B/tmp/evil"
- "-x c"

Examples that must pass against allowlist ["-O0","-O1","-O2","-O3","-Wall","-std=*"]:
- "-O2"
- "-Wall"
- "-std=c++17"
- "-std=c11"

Do not use regexp. Use strings.HasPrefix, strings.HasSuffix, strings.TrimSuffix only.
Standard library only.
```

---

#### Commit 10 — `test(validate): unit tests for filename and flag validation`

**Prompt:**
```
Write Go unit tests for two functions in a sandbox project:

1. internal/validate/filename.go — ValidateFilename(name string) error
2. internal/validate/flags.go — ValidateFlags(flags []string, allowlist []string) error

Use table-driven tests ([]struct{ name, input, wantErr }).

For ValidateFilename, test ALL of these cases:
- "../../etc/passwd" → must error
- "/absolute/path.cpp" → must error  
- ".hidden" → must error
- "solution.cpp" → must pass
- "MyClass.java" → must pass
- "main.go" → must pass
- "" → must error
- "sol/nested.cpp" → must error (contains separator)
- strings.Repeat("a", 129) + ".cpp" → must error (too long)
- "solution\x00.cpp" → must error (null byte)

For ValidateFlags, test ALL of these cases (allowlist: ["-O0","-O1","-O2","-O3","-Wall","-Wextra","-std=*"]):
- ["-O2"] → must pass
- ["-std=c++17"] → must pass (glob match)
- ["-fplugin=evil.so"] → must error
- ["@response_file"] → must error
- ["-O2", "-fplugin=evil"] → must error (second flag rejected)
- [] → must pass (no flags is always fine)
- nil allowlist, ["-O2"] → must error (no flags permitted)

Use testing package only. No testify.
```

---

### Phase 2 — HTTP Layer (Commits 11–14)

---

#### Commit 11 — `feat(handler): GET /healthz with chi router`

**Prompt:**
```
I'm building a Go HTTP service called goboxd using github.com/go-chi/chi/v5.

Write cmd/goboxd/main.go that:
1. Parses two flags: --port (default 8080), --config (default "languages.yaml")
2. Sets up structured JSON logging using log/slog with slog.NewJSONHandler
3. Calls config.Load(configPath) and exits if it fails
4. Builds a chi.NewRouter() with:
   - chi middleware.Logger (chi's built-in)
   - chi middleware.Recoverer (catches panics, returns 500)
5. Registers GET /healthz that returns 200 {"status":"ok"} with Content-Type: application/json
6. Starts http.ListenAndServe on the given port

Also write internal/stats/stats.go with a Stats struct using sync/atomic that tracks:
- InFlightJobs int64
- JobsTotal int64
- JobsFailedInternal int64
- LastInternalErrorAt time.Time (protected by sync.Mutex)

Methods: IncrInflight(), DecrInflight(), IncrTotal(), IncrFailed(), SetLastError(t time.Time)

The version and commit should be set via ldflags. Declare them as package-level vars 
in main.go: var version = "dev" and var commit = "none".
```

---

#### Commit 12 — `feat(runner): nsjail probe and language smoke probes`

**Prompt:**
```
Write internal/runner/probe.go for a Go sandbox service called goboxd.

I need two probe functions called at startup to check if the system is ready.

Define this type:
  type ProbeResult struct {
    OK      bool
    Version string
    Error   string
  }

Write these functions:

1. func ProbeNsjail() ProbeResult
   - Runs: nsjail --version (path from NSJAIL_PATH env var, default /usr/sbin/nsjail)
   - Captures stderr (nsjail writes version to stderr)
   - Extracts version string from output
   - Returns OK: false if binary not found or exits non-zero
   - Timeout: 5 seconds

2. func ProbeLanguage(lang config.Language) ProbeResult
   - Runs: lang.Run.Cmd --version
   - For Java: use lang.Build.Cmd --version (javac)
   - Captures combined stdout+stderr
   - Returns first line of output as Version
   - Returns OK: false if binary not found or exits non-zero
   - Timeout: 5 seconds

Use exec.CommandContext with a 5-second context for timeouts.
Use os/exec, context, time packages only.
```

---

#### Commit 13 — `feat(handler): GET /readyz`

**Prompt:**
```
Write the Readyz handler for a Go HTTP service (goboxd) using chi.

The handler is a method on a HealthHandler struct that holds:
- nsjailProbe runner.ProbeResult
- langProbes map[string]runner.ProbeResult  
- cfg *config.Config

GET /readyz must:
- Return 200 if nsjail probe OK AND all language probes OK
- Return 503 if any probe failed
- Always return Content-Type: application/json

The exact JSON shape for a degraded response:
{
  "status": "degraded",
  "nsjail": { "ok": false, "version": "", "error": "nsjail not found" },
  "languages": {
    "py3": { "ok": true, "version": "Python 3.11.2" },
    "java": { "ok": false, "error": "javac not found at /usr/bin/javac" }
  }
}

The exact JSON shape for a healthy response:
{
  "status": "ok",
  "nsjail": { "ok": true, "version": "3.4" },
  "languages": {
    "py3": { "ok": true, "version": "Python 3.11.2" }
  }
}

Write the HealthHandler struct, its constructor NewHealthHandler, and the Readyz method.
Do not hardcode language names. Derive the response from the stored probe results.
```

---

#### Commit 14 — `feat(handler): GET /info`

**Prompt:**
```
Write the Info handler for goboxd. GET /info must always return 200.

The JSON response must contain exactly these top-level keys:

1. "build_info": { "version": "0.1.0", "commit": "abc1234", "go_version": "go1.22.3" }
   - go_version from runtime.Version()

2. "nsjail": { "path": "/usr/sbin/nsjail", "version": "3.4" }
   - path from NSJAIL_PATH env var

3. "languages": array of objects, one per configured language:
   { "id": "py3", "name": "Python 3", "version": "Python 3.11.2",
     "default_run_limits": { "wall_time_s": 9, "memory_kb": 102400, "max_processes": 100 } }

4. "limits": { "max_source_bytes": 262144, "max_tests": 50, "max_concurrent_jobs": 16 }
   - values from config

5. "stats": {
     "in_flight_jobs": 3,
     "jobs_total": 41892,
     "jobs_failed_internal": 4,
     "last_internal_error_at": "2026-05-04T11:22:09Z",
     "disk_free_bytes_jail_dir": 53687091200
   }
   - in_flight and totals from stats.Stats struct (atomic reads)
   - disk_free from syscall.Statfs on os.TempDir()

Write the Info method on the existing HealthHandler struct.
All fields must be present even if zero/empty. Use omitempty only for last_internal_error_at 
when it is the zero time.
```

---

### Phase 3 — Sandbox Core (Commits 15–20)

---

#### Commit 15 — `feat(sandbox): per-request jail directory with unique ID`

**Prompt:**
```
I'm fixing security hole #5 in goboxd: UID collisions under load.

The bug in the Python reference: it picks a UID from a 30k-wide range and retries 
3 times on collision. Under concurrent load this causes races and directory reuse.

Write internal/sandbox/dir.go with:

1. A package-level atomic counter: var jailCounter atomic.Int64

2. func NewJailDir(baseDir string) (path string, cleanup func(), err error)
   - Creates a unique directory under baseDir
   - Name format: goboxd-{PID}-{counter}-{6-hex-random}
     where PID = os.Getpid(), counter = jailCounter.Add(1), random = crypto/rand hex
   - Never uses the same directory twice (counter + PID guarantee this)
   - Returns a cleanup func that calls os.RemoveAll(path)
   - The caller MUST call cleanup via defer immediately after calling NewJailDir

3. Document in a comment: why this scheme prevents collisions even under 
   concurrent load across multiple goroutines.

Use os, fmt, crypto/rand, sync/atomic packages only.
```

---

#### Commit 16 — `feat(runner): startup orphan sweep`

**Prompt:**
```
I'm fixing security hole #7 in goboxd: stale jail directories.

The bug: if the process panics between creating a jail directory and the cleanup defer 
running, the directory leaks. Over time this fills the disk.

Write internal/runner/sweep.go with:

  func SweepOrphanedDirectories(baseDir string, maxAge time.Duration)

Requirements:
- Scan baseDir for entries matching the prefix "goboxd-"
- For each matching entry: check if its mtime is older than maxAge
- If older: remove it with os.RemoveAll and log at slog.Info level with the path
- If removal fails: log at slog.Warn level (don't crash)
- If the entry is newer than maxAge: skip it (might be an active run)
- This function is called ONCE at startup in main.go before serving

Also explain in a comment: why we use mtime-based age rather than trying to 
match against currently running processes.

Use os, path/filepath, time, log/slog packages only.
```

---

#### Commit 17 — `feat(runner): request size limits and output cap`

**Prompt:**
```
I'm fixing security holes #4 and #6 in goboxd.

Hole #4: No request size limits — source, stdin, expected_stdout all unbounded.
Hole #6: Unbounded child output — a runaway program can OOM the host.

Write internal/sandbox/limits.go with:

1. Constants:
   - DefaultMaxSourceBytes = 256 * 1024  (256 KiB)
   - DefaultMaxStdinBytes = 64 * 1024    (64 KiB)
   - DefaultMaxOutputBytes = 1 * 1024 * 1024  (1 MiB)
   - TruncationMarker = "\n[output truncated]"

2. func CapReader(r io.Reader, limit int64) io.Reader
   - Wraps r with io.LimitReader
   - When the limit is hit, the resulting bytes are followed by TruncationMarker
   - Hint: read up to limit bytes, then check if there are more bytes; 
     if yes, append TruncationMarker to the captured output

3. func CapOutput(output string, limit int) string
   - If len(output) <= limit: return as-is
   - Otherwise: return output[:limit] + TruncationMarker

4. How to apply at the HTTP layer (comment, not code here):
   "In the run handler, wrap r.Body with http.MaxBytesReader before json.Decode"

5. How to apply at the child process layer (comment):
   "Wrap the nsjail stdout/stderr pipes with CapReader before reading"

Use io, strings packages only.
```

---

#### Commit 18 — `feat(runner): nsjail invocation and sandbox lifecycle`

**Prompt:**
```
Write the core sandbox execution logic for goboxd: internal/runner/runner.go

I need a function RunSandbox that handles one POST /run request end-to-end.

Context:
- goboxd wraps Google's nsjail to isolate untrusted code
- nsjail is at the path in env var NSJAIL_PATH (default /usr/sbin/nsjail)
- Each request gets a fresh temporary directory (the "jail dir")
- For compiled languages: build phase first, then run phase for each test
- For interpreted languages: run phase only for each test

Define these request/response types:

RunRequest:
  Language, Source, SourceFilename, ArtifactFilename string
  Build *PhaseConfig  (optional, for compiled langs)
  Run   PhaseConfig
  Tests []TestCase

PhaseConfig:
  Limits config.ResourceLimits
  Flags  []string

TestCase:
  Stdin          string
  ExpectedStdout string

RunResult:
  Status string
  Build  BuildResult
  Tests  []TestResult

BuildResult:
  Status     string  (ok / failed / internal_error)
  Stdout     string
  Stderr     string
  DurationMS int64

TestResult:
  Status       string
  Stdout       string
  Stderr       string
  DurationMS   int64
  MemoryPeakKB int64

Write func RunSandbox(nsjailPath string, lang config.Language, req RunRequest) (RunResult, error)

The function must:
1. Call sandbox.NewJailDir(os.TempDir()) — defer cleanup immediately
2. Write source file into the jail dir (using the validated filename)
3. If language has a build step: run nsjail for the build command
   - If build fails: return build_failed, all tests not_executed
4. For each test: run nsjail for the run command, pipe stdin, capture stdout/stderr
5. Compare each output using status.CompareOutput
6. Assemble the RunResult with correct top-level status

For the nsjail command, use these flags as a starting point:
--mode o (one-shot), --time_limit, --rlimit_as (memory), --log /dev/null,
--bindmount_ro for /usr /lib /lib64 /bin, --chroot jailDir, -- cmd args

Wrap stdout/stderr pipes with sandbox.CapReader before reading.
Use time.Now() to measure DurationMS for each phase.

Do not handle concurrency here — that's the handler's responsibility.
```

---

#### Commit 19 — `feat(status): status vocabulary and top-level rule`

**Prompt:**
```
Write internal/status/status.go for goboxd.

This file defines the exact status strings from the spec and the rules for computing them.

1. Define string constants for all valid statuses:
   Build: ok, failed, internal_error
   Test: accepted, wrong_output, output_whitespace_mismatch, time_exceeded,
         memory_exceeded, runtime_error, not_executed, internal_error
   Top-level: accepted, build_failed, wrong_output, output_whitespace_mismatch,
              time_exceeded, memory_exceeded, runtime_error, internal_error

2. func TopLevelStatus(buildStatus string, testStatuses []string) string
   Rules (implement exactly):
   - If buildStatus != "ok": return "build_failed"
   - If all testStatuses are "accepted": return "accepted"
   - Otherwise: return the FIRST non-accepted status in testStatuses order

3. func CompareOutput(actual, expected string) string
   Rules:
   - If actual == expected: return "accepted"
   - If strings.TrimSpace(actual) == strings.TrimSpace(expected): 
     return "output_whitespace_mismatch"
   - Otherwise: return "wrong_output"

4. func StatusFromExitCode(exitCode int, timedOut bool, memKilled bool) string
   Rules:
   - timedOut: return "time_exceeded"
   - memKilled: return "memory_exceeded"  
   - exitCode == 0: return "accepted" (caller should check output too)
   - exitCode != 0: return "runtime_error"

No external dependencies. Standard library only.
```

---

#### Commit 20 — `feat(handler): POST /run handler with full validation`

**Prompt:**
```
Write the POST /run handler for goboxd: internal/handler/run.go

This is the main endpoint. It must follow the spec exactly.

Request JSON shape:
{
  "language": "cpp",
  "source": "...",
  "source_filename": "solution.cpp",      // optional
  "artifact_filename": "solution",        // optional  
  "build": {
    "limits": { "wall_time_s": 5, "memory_kb": 1048576, "max_processes": 100 },
    "flags": ["-O2"]
  },
  "run": {
    "limits": { "wall_time_s": 3, "memory_kb": 524288, "max_processes": 64 },
    "flags": []
  },
  "tests": [{ "stdin": "1\n", "expected_stdout": "hi" }]
}

The handler must:
1. Wrap r.Body with http.MaxBytesReader(w, r.Body, maxSourceBytes) BEFORE decoding
2. Decode JSON; on error return 400 {"error": {"code": "invalid_json", "message": "..."}}
3. Validate language exists in config; if not return 400 {"error": {"code": "unknown_language", ...}}
4. If source_filename provided: validate with validate.ValidateFilename; 400 on error
5. If artifact_filename provided: validate with validate.ValidateFilename; 400 on error
6. If build.flags provided: validate with validate.ValidateFlags against lang allowlist; 400 on error
7. Validate len(tests) >= 1 and <= config.MaxTests; 400 on error
8. Acquire concurrency semaphore (blocking — callers queue, not fail)
9. Call runner.RunSandbox; on internal error return 500 {"error": {"code": "internal_error", ...}}
10. Release semaphore
11. Always return 200 with the run result (even if user code failed)

Key rule: NEVER return 5xx because user code crashed. 5xx is only for server failures.

Write NewRunHandler(cfg *config.Config, sem chan struct{}, s *stats.Stats) http.HandlerFunc
The sem channel is the bounded concurrency semaphore.
```

---

### Phase 4 — Concurrency (Commit 21)

---

#### Commit 21 — `feat(handler): bounded concurrency queue`

**Prompt:**
```
Explain and implement bounded concurrency for goboxd's POST /run handler.

Requirements from the spec:
- A global limit on concurrent sandbox executions
- When the limit is reached: incoming requests QUEUE (block), they do NOT fail
- The limit is configurable via env var GOBOXD_MAX_CONCURRENT
- Default: runtime.NumCPU()
- The current limit must be exposed in GET /info as max_concurrent_jobs

In main.go, show me:
1. How to read GOBOXD_MAX_CONCURRENT from env, fall back to runtime.NumCPU()
2. How to create the semaphore channel: sem := make(chan struct{}, maxConcurrent)
3. How to pass it to NewRunHandler

In the run handler (already written), confirm:
- How "sem <- struct{}{}" blocks when full (this IS the queue behaviour)
- Why defer func() { <-sem }() must be called immediately after acquiring
- Why this is correct and does NOT leak goroutines

Also write a short comment in main.go explaining the tradeoff:
channel semaphore vs sync.Mutex vs a proper worker pool — and why channel 
semaphore is correct for this use case.
```

---

### Phase 5 — Security Hardening (Commits 22–24)

---

#### Commit 22 — `security: Go filesystem APIs for all path operations`

**Prompt:**
```
I'm fixing security hole #2 in goboxd: shell-style directory commands.

The bug in the Python reference: it creates and deletes per-request directories 
by string-formatting shell commands like "rm -rf " + path and passing them to a 
shell. This is dangerous.

Audit the following functions in my Go code and tell me if any of them use:
- exec.Command("rm", ...) or exec.Command("sh", "-c", ...)
- string formatting of paths into shell commands
- any use of os/exec for filesystem operations

The functions to audit:
- sandbox.NewJailDir
- sandbox.CapReader
- runner.SweepOrphanedDirectories
- runner.RunSandbox (the jail dir creation and cleanup parts only)

For each function, either confirm it is safe or show the specific fix.

The correct replacements:
- "mkdir -p path" → os.MkdirAll(path, 0755)
- "rm -rf path" → os.RemoveAll(path)
- "chmod 755 path" → os.Chmod(path, 0755)

After the audit, write a docs/security.md entry for hole #2 with:
- Name: Shell-style directory commands
- Description: what the Python reference does wrong
- Fix: what we do in Go
- file:line reference (use placeholder FILE:LINE for now)
```

---

#### Commit 23 — `security: wire output cap into nsjail pipe readers`

**Prompt:**
```
I need to confirm that security hole #6 (unbounded child output) is correctly 
fixed in goboxd's runner.go.

The bug: reading the full child stdout into memory. A runaway program that 
writes gigabytes of output can OOM the host process.

The fix: sandbox.CapReader wraps io.LimitReader and appends a truncation marker.

Show me exactly how to wire this into the nsjail process execution in runner.go:

1. After cmd.StdoutPipe() and cmd.StderrPipe(): wrap each pipe with CapReader
2. Use io.ReadAll on the capped reader (safe because it's bounded)
3. The resulting output string may end with "\n[output truncated]" — this is correct

Also write a unit test in runner_test.go that proves the cap works:
- Create a mock that writes DefaultMaxOutputBytes + 1024 bytes
- Confirm the captured output is exactly DefaultMaxOutputBytes bytes 
  + the TruncationMarker
- This test must NOT use nsjail (it's a unit test)
```

---

#### Commit 24 — `security: complete docs/security.md with all 7 holes`

**Prompt:**
```
Write docs/security.md for goboxd documenting all 7 security holes from the spec.

For each hole, use this format:

## Hole N — [Name]
**Category:** [Input Validation / Filesystem / Resource Control / Process Management]
**Severity:** [Critical / High / Medium]

### What the Python reference does wrong
[One paragraph describing the vulnerable code pattern]

### What this enables
[What an attacker could do]

### Our fix in Go
[One paragraph describing the fix]

### Location
`file:line` — [placeholder for now, I'll fill real line numbers before PR]

---

The 7 holes:
1. Path traversal via filename — internal/validate/filename.go
2. Shell-style directory commands — internal/sandbox/dir.go + internal/runner/sweep.go
3. Compiler-flag injection — internal/validate/flags.go
4. No request size limits — internal/sandbox/limits.go + internal/handler/run.go
5. UID collisions under load — internal/sandbox/dir.go
6. Unbounded child output — internal/sandbox/limits.go + internal/runner/runner.go
7. Stale jail directories — internal/runner/sweep.go

This document will be linked from the PR description as proof of fixes.
```

---

### Phase 6 — Testing & Polish (Commits 25–28)

---

#### Commit 25 — `test(runner): unit tests for status mapping and output comparison`

**Prompt:**
```
Write unit tests for internal/status/status.go in goboxd.

Use table-driven tests. Test these functions:

1. TopLevelStatus(buildStatus string, testStatuses []string) string

Test cases:
- buildStatus "failed", any tests → "build_failed"
- buildStatus "ok", all tests "accepted" → "accepted"
- buildStatus "ok", tests ["accepted", "wrong_output", "time_exceeded"] → "wrong_output"
- buildStatus "ok", tests ["time_exceeded", "accepted"] → "time_exceeded"
- buildStatus "ok", tests ["accepted", "runtime_error"] → "runtime_error"
- buildStatus "ok", no tests → "accepted"
- buildStatus "internal_error", any → "build_failed"

2. CompareOutput(actual, expected string) string

Test cases:
- actual == expected → "accepted"
- actual "hi\n", expected "hi" → "output_whitespace_mismatch"
- actual "  hi  ", expected "hi" → "output_whitespace_mismatch"
- actual "HI", expected "hi" → "wrong_output"
- actual "", expected "hi" → "wrong_output"
- actual "hi", expected "hi" → "accepted"

No external dependencies. Use only the testing package.
```

---

#### Commit 26 — `test(config): unit tests for config loading and validation`

**Prompt:**
```
Write unit tests for internal/config/config.go in goboxd.

I need tests for Load() and Validate(). Use table-driven tests where applicable.

Test Load():
- Load a valid YAML string with one interpreted language → succeeds, lang is in map
- Load a valid YAML string with a compiled language (has build step) → succeeds
- Load a YAML string with duplicate language IDs → returns error
- Load a non-existent file path → returns error
- Load an empty YAML file → returns error (no languages)
- Load a YAML with a language missing run.cmd → returns error

For tests that need a YAML file, use os.WriteFile to a temp dir (t.TempDir()).
Do not use real filesystem paths or golden files.

Test Validate():
- Config with zero languages → error
- Config where a language has empty ID → error
- Config where a language has a build step but empty build.cmd → error

Use only testing and os packages. No testify.
```

---

#### Commit 27 — `feat(middleware): structured JSON request logging`

**Prompt:**
```
Write internal/middleware/logger.go for goboxd.

I need a chi-compatible middleware that logs one structured JSON line per request.

The log line must include:
- "ts": RFC3339 timestamp
- "request_id": a UUID generated per request (use crypto/rand, format as hex)
- "method": HTTP method
- "path": request path
- "status": HTTP response status code
- "duration_ms": request duration in milliseconds
- "language": the language field from POST /run body (empty for other endpoints)
- "job_status": the top-level status from the run result (empty for non-run endpoints)

Use log/slog with JSON output.

To capture the response status code, use a response writer wrapper:
type statusRecorder struct {
    http.ResponseWriter
    status int
}

To capture language and job_status from the /run handler, store them in the 
request context using a typed context key and retrieve them in the middleware.

Write:
1. The middleware function: func Logger(next http.Handler) http.Handler
2. Helper functions to set/get language and job_status in context
3. Show how to call the context setters from the run handler

The middleware should be added to the chi router in main.go BEFORE chi's built-in 
middleware.Logger (or replace it entirely with this one).
```

---

#### Commit 28 — `chore: go vet + golangci-lint clean, Stage 1 PR ready`

**Prompt:**
```
I'm about to submit Stage 1 of a Go hackathon project. Give me:

1. A golangci-lint configuration file (.golangci.yml) appropriate for a Go 1.22 
   project that enables these linters:
   - govet, errcheck, staticcheck, gosimple, ineffassign, unused
   - gofmt (format check)
   - gocritic (style suggestions)
   - gosec (security checks relevant to our use case)
   Disable linters that produce too many false positives for a systems project:
   - wrapcheck (too strict for internal errors)
   - exhaustive (not needed here)

2. A pre-submission checklist in markdown that covers:
   - docker build succeeds
   - docker run → /healthz returns 200
   - POST /run with py3 hello world returns {"status":"accepted"}
   - POST /run with cpp hello world returns {"status":"accepted"}  
   - POST /run with source_filename "../../etc/passwd" returns 400
   - POST /run with disallowed flag returns 400
   - make test passes
   - make integration passes
   - go vet ./... clean
   - golangci-lint run clean
   - README has no "elegant", "robust", "seamlessly", "leverage", no emoji
   - docs/ai/prompts.md has at least 3 entries

3. The exact curl commands to manually test each endpoint before opening the PR.
```

---

## Stage 2 — Polyglot (June 4–5)
### Commit budget: ~12 commits (29–40)

---

#### Commits 29–31 — Add remaining 5 languages to languages.yaml

**Prompt (use once for all 5):**
```
I'm extending goboxd's languages.yaml to support all 7 in-scope languages.
I already have py3 and cpp. I need to add:

1. C (id: c) — gcc, similar to cpp, flag_allowlist same as cpp
2. Java (id: java) — javac to build, java to run. Special: source_filename and 
   artifact_filename come from the request (source_filename_strategy: from_request)
   because Java requires the filename to match the public class name.
3. Bash (id: bash) — /bin/bash interpreter, no build step, lower limits
4. JavaScript/Node (id: node) — /usr/bin/node interpreter, no build step
5. Verilog (id: verilog) — iverilog to build, vvp to run

For each language provide:
- Appropriate wall_time_s, memory_kb, max_processes limits
- flag_allowlist for compiled languages (iverilog and gcc)
- The correct source filename convention

The YAML should be a drop-in addition to languages.yaml with no Go code changes.
After showing the YAML, also give me the apt-get install command to add to the 
Dockerfile for any new toolchain (iverilog, nodejs, default-jdk, etc.)
```

---

#### Commit 32 — `chore(docker): install all 7 language toolchains`

**Prompt:**
```
Update the Dockerfile for goboxd to install all 7 in-scope language toolchains.

Current toolchains installed: python3, g++, gcc
Need to add: default-jdk, nodejs, bash (already present), iverilog

Requirements:
- Single RUN layer to minimise image size
- Install in dependency order
- After install, verify each binary exists at its expected path:
  /usr/bin/python3, /usr/bin/g++, /usr/bin/gcc, /usr/bin/javac, /usr/bin/java,
  /usr/bin/node, /bin/bash, /usr/bin/iverilog, /usr/bin/vvp
- Add a smoke test in the Dockerfile: run each binary --version and fail the 
  build if any is missing

Show me the updated toolchains stage of the Dockerfile only.
```

---

#### Commit 33 — `feat(config): handle {{flags}} for interpreted languages`

**Prompt:**
```
In goboxd's runner, when we resolve YAML arg templates like {{flags}}, there is 
an edge case for interpreted languages (Python, Bash, Node) that have no build 
step and therefore no flag_allowlist.

I need to handle two scenarios correctly:

1. An interpreted language has {{flags}} in its run.args
   - If client sends flags: [] or omits flags: validate against empty allowlist → reject with 400
   - If client sends no flags field: expand {{flags}} to empty list (no arg injected)

2. A compiled language's build has {{flags}} in build.args
   - If client sends flags: ["-O2"]: validate against flag_allowlist, inject if valid
   - If client sends no flags field: expand {{flags}} to empty list

Write a function:
  func ResolveArgs(args []string, source, artifact string, flags []string) []string

That replaces {{source}}, {{artifact}}, {{flags}} in the args template.
Rules:
- {{flags}} expands to the flags slice items (spliced into position, not as a single string)
- {{source}} expands to the source filename
- {{artifact}} expands to the artifact filename
- If flags is nil/empty and {{flags}} appears: remove that arg position entirely

Write this in internal/config/config.go or a new internal/runner/resolve.go file.
Add unit tests covering the edge cases above.
```

---

#### Commit 34 — `feat(handler): /readyz and /info reflect full language set`

**Prompt:**
```
I've added 5 more languages to languages.yaml (total: 7 + Rust bonus).
I need to make sure /readyz and /info automatically reflect the full registered set.

Review my current HealthHandler implementation and tell me if any language names 
or IDs are hardcoded. If so, show the fix.

Requirements:
- /readyz: probe all languages in cfg.Languages at startup, not a hardcoded list
- /info: languages[] array is built from cfg.Languages, not hardcoded
- Adding a new language to languages.yaml must not require ANY Go code change

Also: the startup probe in main.go currently loops over cfg.Languages to build 
langProbes map. Confirm this is correct or show the fix.

Finally, write a quick integration test that:
- Starts the server with a config containing 3 languages
- Calls /info
- Asserts all 3 language IDs appear in the response
```

---

#### Commits 35–37 — Integration tests, limit merging, request limit validation

**Prompt:**
```
I need three things for goboxd Stage 2:

1. Integration test template (internal/handler/run_test.go):
Write one table-driven integration test per in-scope language. Each test:
- Sends a hello-world program appropriate for that language
- Expects {"status": "accepted"} in the response
- The test uses httptest.NewServer (no Docker needed for unit tests)
- Mock the runner so tests don't require nsjail installed

Also write one test for build failure (bad C++ code) that expects:
- top-level "status": "build_failed"
- all tests[].status: "not_executed"

2. Limit merging (internal/sandbox/limits.go):
Write func MergeLimits(base config.ResourceLimits, override *config.ResourceLimits) config.ResourceLimits
Rules: zero value in override means "use base". Non-zero overrides the base field.
Write table-driven unit tests.

3. Request limit validation (internal/handler/run.go addition):
In the run handler, after decoding JSON, validate:
- build.limits.wall_time_s cannot exceed 60 if provided
- run.limits.wall_time_s cannot exceed 30 if provided
- run.limits.memory_kb cannot exceed 2097152 (2 GiB) if provided
Return 400 with code "limit_exceeded" if violated.
```

---

#### Commit 38 — `docs: complete docs/languages.md and docs/api.md`

**Prompt:**
```
Write two documentation files for goboxd:

1. docs/languages.md
A table of all supported languages with columns:
ID | Name | Toolchain | Source Filename | Artifact | Build? | Default Wall Time | Default Memory

Then below the table, one paragraph per language explaining any quirks:
- Java: source_filename must match public class name, comes from request
- Verilog: iverilog + vvp toolchain, not widely known
- Bash: lower limits, no flag support
- Others: brief note

End with a section "Adding a new language" that describes the two-step process:
1. Add a YAML block to languages.yaml following the schema
2. Add the toolchain install to the Dockerfile

2. docs/api.md
Full API reference for all 4 endpoints. For each endpoint:
- Method and path
- Description
- Request body (if any) with field descriptions
- Response body with all possible field values
- Error cases with HTTP status codes and error codes
- One curl example

Write in plain technical prose. No marketing language.
```

---

#### Commit 39 — `feat(bonus): add Rust to languages.yaml`

**Prompt:**
```
Add Rust as a bonus language to goboxd's languages.yaml.

Requirements:
- id: rust
- Compiler: /usr/bin/rustc
- Source filename: solution.rs
- Artifact: solution
- Build limits: wall_time_s: 10 (Rust compiles slowly), memory_kb: 2097152
- Run limits: same as cpp
- No flag_allowlist needed for Stage 1 bonus (empty = no flags)

Also show me:
- The apt-get or curl command to install rustc in the Dockerfile
  (note: Rust is not in apt on Debian bookworm; use rustup)
- The Dockerfile RUN lines to install rustup non-interactively
- How to verify: rustc --version in the Dockerfile smoke test

After showing the YAML and Dockerfile changes, confirm: does adding this language 
require any Go code change? The answer must be no.
```

---

#### Commit 40 — `chore: Stage 2 readiness check`

**Prompt:**
```
Give me a Stage 2 completion checklist for goboxd with exact curl commands to verify:

1. /readyz returns 200 with all 7 in-scope languages showing ok: true
   (show the expected JSON shape)

2. /info lists all 7 + Rust (8 total) in the languages array
   (show the expected JSON shape for the languages array)

3. POST /run works for each language — show the exact curl command and expected 
   response shape for:
   - Python 3 (hello world: print("hi"), stdin: "", expected: "hi\n")
   - C++ (hello world, expected: "hello\n")
   - Java (HelloWorld class, stdin: "", expected: "Hello World\n")
   - Bash (echo "hi", expected: "hi\n")
   - Node (console.log("hi"), expected: "hi\n")
   - Verilog (basic module, expected: "1\n")

4. Live language addition test:
   - Show me the exact YAML block to add Kotlin
   - Confirm no Go file needs to change
   - Show the Dockerfile line to install the Kotlin compiler
   - After rebuild, /readyz should show kotlin: ok: true
```

---

## Stage 3 — Harden & Load (Paradox)
### Commit budget: ~12 commits (41–52)

---

#### Commit 41 — `security: integration tests proving all 7 holes are closed`

**Prompt:**
```
Write security integration tests for goboxd that prove all 7 security holes are fixed.

For each test: send a malicious/boundary request and assert the correct safe response.
Use httptest.NewServer. These tests must pass with the real handler code.

Test 1 — Path traversal (#1):
POST /run with source_filename: "../../etc/passwd"
Expected: 400, error code "invalid_filename"

Test 2 — Shell commands (#2):
This is a code audit, not a runtime test. Write a test that:
- Scans internal/sandbox/*.go and internal/runner/*.go
- Fails if any file contains the string "exec.Command" called with shell binary names 
  ("sh", "bash", "rm", "mkdir", "chmod")
- This proves we use Go filesystem APIs

Test 3 — Flag injection (#3):
POST /run with language "cpp", flags: ["-fplugin=evil.so"]
Expected: 400, error code "disallowed_flag"

Test 4 — Request size limit (#4):
POST /run with source that is 300 KiB (over the 256 KiB limit)
Expected: 400, error code "source_too_large"

Test 5 — Unique jail dirs (#5):
Send 50 concurrent POST /run requests (use goroutines in the test)
Assert: no two requests share a jail directory path
Hint: add an optional test hook to runner.RunSandbox that captures the jail dir path

Test 6 — Output cap (#6):
Run a Python program that writes 2 MiB to stdout
Assert: response test stdout is exactly DefaultMaxOutputBytes + TruncationMarker

Test 7 — Orphan cleanup (#7):
Manually create a goboxd-* directory in os.TempDir() with mtime 20 minutes ago
Call SweepOrphanedDirectories(os.TempDir(), 10*time.Minute)
Assert: the directory is gone
```

---

#### Commit 42 — `perf: tune concurrency for sustained load`

**Prompt:**
```
I need to tune goboxd's concurrency model to survive sustained load, not just 
micro-bursts. The judging will run a sustained load test.

Current implementation: channel semaphore with runtime.NumCPU() capacity.

Questions to answer:
1. What happens to requests that are queued when GOBOXD_MAX_CONCURRENT is reached?
   Do they time out? How long will they wait? Is there a queue length limit?

2. Should I add a maximum queue depth? If a request has been waiting more than X 
   seconds, should I return 503 instead of continuing to wait?
   Show me how to implement this with a select statement and time.After.

3. Is there a goroutine leak risk in my current implementation?
   Show me a test that verifies: after 100 concurrent requests complete, the 
   goroutine count returns to baseline.

4. For nsjail process management: after cmd.Wait() returns, are there any 
   lingering child processes? How do I ensure cleanup with cmd.Process.Kill() 
   in a defer?

After answering, show me the minimal changes to runner.go and the run handler 
to address any issues found.
```

---

#### Commit 43 — `perf: accurate per-request timing and memory reporting`

**Prompt:**
```
I need accurate per-request timing in goboxd.

Currently I use time.Now() before and after nsjail execution for DurationMS.
I need memory_peak_kb in each test result.

1. For DurationMS: confirm that my current approach (wall clock time) is correct.
   Should I use time.Since(start).Milliseconds()? Is there a more accurate method 
   available in Go for measuring execution duration of a child process?

2. For memory_peak_kb: nsjail writes a log file with resource usage information.
   Show me how to:
   - Tell nsjail to write logs to a temp file (--log flag)
   - Parse the nsjail log file after the process exits to extract peak memory usage
   - The relevant nsjail log line contains "MEM" or "memory" — show me a 
     simple string parsing approach (no regex)

3. As a fallback if nsjail log parsing is too complex for Stage 1:
   How can I read memory usage from /sys/fs/cgroup/goboxd/{job-id}/memory.peak 
   for cgroup v2?

Show me the code changes to RunSandbox to capture and return memory_peak_kb.
```

---

#### Commit 44 — `feat: load test script`

**Prompt:**
```
Write scripts/load-test.sh for goboxd.

Requirements:
- Uses "hey" (https://github.com/rakyll/hey) as the load testing tool
- Tests POST /run with a trivial Python hello-world payload
- Runs 4 concurrency levels in sequence: 1, 10, 50, 100
- For each level: sends 200 total requests
- The payload:
  {"language":"py3","source":"print('hi')","tests":[{"stdin":"","expected_stdout":"hi\n"}]}
- Outputs a summary for docs/benchmarks.md:
  | Clients | Req/sec | p50 (ms) | p95 (ms) | p99 (ms) |

Also:
- Check if hey is installed; if not, print install instructions and exit 1
- Accept optional --host flag (default: http://localhost:8080)
- Make the script safe to run multiple times

Also write docs/benchmarks.md with the table template pre-filled (I'll add numbers 
after running the script):
- Describe the test environment section
- Show the table structure
- Add a note about what "clean Docker run" means
```

---

#### Commit 45 — `docs: fill docs/benchmarks.md with real numbers`

**Prompt:**
```
I've run make load and got these benchmark results from hey for goboxd.
Help me interpret and document them in docs/benchmarks.md.

[PASTE YOUR ACTUAL HEY OUTPUT HERE]

I need you to:
1. Extract the relevant p50, p95, p99 numbers from the hey output format
2. Fill in the benchmarks table
3. Write one paragraph interpreting the results:
   - Does throughput scale linearly with concurrency?
   - At what concurrency level does latency degrade significantly?
   - Is the queue working as expected (latency rising, not errors)?
4. Note the host specs I measured on: [YOUR BOX SPECS HERE]
5. Note that numbers are from a clean Docker run, not from a debugger

The benchmarks.md must be honest — do not round numbers favorably.
```

---

#### Commit 46 — `docs: complete docs/architecture.md`

**Prompt:**
```
Write docs/architecture.md for goboxd. This is a bonus criterion worth extra points.
It must be good enough that a new engineer can understand the codebase on their first day.

Structure:

## Overview
One paragraph: what goboxd is, what problem it solves.

## Request Lifecycle
A text-based diagram showing the flow of one POST /run request:
  HTTP request → chi router → run handler
  → validation (filename, flags, size)
  → semaphore acquire (blocks if at capacity)
  → RunSandbox()
    → NewJailDir() → defer cleanup()
    → write source file
    → [build phase if compiled language]
    → for each test: nsjail invocation → compare output
  → semaphore release
  → JSON response

## Package Structure
A table of each internal package, one sentence per package describing its 
responsibility. Do not describe implementation details, only the contract.

## Concurrency Model
How the channel semaphore works. Why requests queue rather than fail.
What happens to queued requests if the server shuts down.

## Security Model
What nsjail provides (namespace isolation, cgroup limits, syscall filtering).
What the Go layer adds on top (input validation, output capping, orphan sweep).
Why both layers are necessary.

## YAML Language Registry
How languages.yaml is loaded. How {{source}}, {{artifact}}, {{flags}} are resolved.
Why adding a language requires no Go code change.

## Tradeoffs and Decisions
3 ADR-style entries:
1. chi vs gin vs net/http
2. channel semaphore vs worker pool
3. os.MkdirTemp vs manual UID scheme
```

---

#### Commits 47–48 — `feat(bonus): add Zig and Go to languages.yaml`

**Prompt:**
```
Add two more bonus languages to goboxd's languages.yaml: Zig and Go itself.

For Zig (id: zig):
- Compiler path, source filename convention, artifact, typical limits
- Dockerfile install: Zig is not in apt. Show me how to download the binary 
  from ziglang.org in the Dockerfile (use a pinned version)

For Go (id: golang):
- The Go compiler is already in the builder stage of our Dockerfile
- How do I make it available in the final image without copying the entire Go SDK?
- Source filename: main.go
- Build: go build -o {{artifact}} {{source}}
- Important: go build needs GOPATH and GOCACHE set — show me the ENV lines 
  for the Dockerfile and the nsjail bindmount needed to expose /usr/local/go

For both languages: confirm no Go code change is needed.
```

---

#### Commits 49–50 — `docs(ai): complete prompts.md, adrs.md, plan-evolution.md, postmortem.md`

**Prompt:**
```
I need to complete the docs/ai/ folder for goboxd before submitting my PR.
This folder is a scored deliverable. Help me write quality entries.

1. adrs.md — write 3 ADR entries I should have documented during development:

ADR 1: Why chi over net/http, gin, or echo
ADR 2: Why channel semaphore over sync.Mutex or a worker pool for concurrency
ADR 3: Why os.MkdirTemp over a manual UID scheme for jail directories

For each ADR, use this format:
## [Title]
**Context:** ...
**Options considered:** 1. option A  2. option B  3. option C
**Decision:** ...
**Rationale:** (include what was decided against and why)

2. plan-evolution.md — give me a template for 2 pivot entries I should fill:
- What I thought the YAML registry would look like on Day 1 vs what I built
- How my understanding of nsjail invocation changed after reading the reference impl

3. For prompts.md — give me a good example entry showing the format correctly,
using the chi framework decision as the example (I actually used AI to research this).

4. For postmortem.md — give me the 5 questions I should answer honestly.
```

---

#### Commit 51 — `chore: final pre-PR checklist and lint`

**Prompt:**
```
I'm about to open my final PR for the goboxd hackathon. Run me through a complete 
pre-submission checklist.

Give me:
1. The exact commands to run locally before opening the PR (in order)
2. What each command should output (pass/fail criteria)
3. Common last-minute issues in Go projects and how to spot them quickly

Specifically check:
- go vet ./... — what errors to look for
- golangci-lint run — what warnings are OK to suppress vs must-fix
- go test ./... — what failure patterns mean test setup issues vs real bugs
- docker build — what layer failures mean
- docker run + curl /healthz — the smoke test that proves "it works"

Also: review the spec's README requirements one more time and give me a checklist 
of banned words/phrases to search for in my README.md before submitting.
```

---

#### Commit 52 — `docs: write PR description`

**Prompt:**
```
Write the PR description for my goboxd hackathon submission.

Context:
- Team: Sudo — Baratam Praneeth Gupta  
- Framework: chi/v5
- Branch: team/team-sudo
- Languages: C, C++, Java, Python 3, Bash, Node.js, Verilog, Rust, Go, Zig

The PR description must contain these sections (from the spec):

1. Team members (just me, solo)

2. Framework choice — one sentence with the reason
   (chi: composable, idiomatic, uses only standard library interfaces)

3. How to run locally — one paragraph pointing at the Makefile

4. Security holes closed — list all 7 with file:line links
   (use placeholder FILE:LINE format, I'll fill real line numbers)

5. Languages supported — list all 10

6. Link to docs/benchmarks.md

Additional sections I want:
7. What the AI log covers (brief reference to docs/ai/)
8. Architecture overview (one-sentence pointer to docs/architecture.md)

Rules for the PR description:
- No AI-generated marketing language
- No "elegant", "robust", "seamlessly"
- Factual, direct, as if written for a code review
- Total length: under 400 words
```

---

## Commit Map

| Range | Stage | Phase | Count |
|---|---|---|---|
| 1–5 | Stage 1 | Repo bootstrap | 5 |
| 6–10 | Stage 1 | Config & validation | 5 |
| 11–14 | Stage 1 | HTTP layer | 4 |
| 15–20 | Stage 1 | Sandbox core | 6 |
| 21 | Stage 1 | Concurrency | 1 |
| 22–24 | Stage 1 | Security | 3 |
| 25–28 | Stage 1 | Tests & polish | 4 |
| 29–40 | Stage 2 | Polyglot | 12 |
| 41–52 | Stage 3 | Harden & load | 12 |
| **Total** | | | **52 commits** |

---

## How to Use This File

1. Work commit-by-commit in order. Do not skip.
2. Before each commit: copy the prompt, paste into your AI tool, read the output.
3. After each commit: add an entry to `docs/ai/prompts.md`.
4. If the AI output is wrong or partially wrong: note what you discarded in the log entry.
5. Every commit must leave the repo buildable (`go build ./...` passes).
6. The AI log is **scored** — specific entries with honest "what I discarded" 
   notes score higher than vague entries or no entries.

---

*Team Sudo — Baratam Praneeth Gupta | May 24, 2026*
