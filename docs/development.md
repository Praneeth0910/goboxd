# Development Guide

Everything you need to set up a development environment, understand the codebase, and contribute to goboxd.

---

## Table of Contents

- [Development Environment Setup](#development-environment-setup)
- [Project Structure](#project-structure)
- [Architecture Overview](#architecture-overview)
- [Building & Running](#building--running)
- [Configuration](#configuration)
- [Adding a New Language](#adding-a-new-language)
- [Testing](#testing)
- [Linting & Code Quality](#linting--code-quality)
- [CI/CD Pipeline](#cicd-pipeline)
- [Environment Variables](#environment-variables)
- [Makefile Targets](#makefile-targets)
- [Coding Conventions](#coding-conventions)
- [Security Considerations](#security-considerations)
- [Debugging Tips](#debugging-tips)

---

## Development Environment Setup

### Required tools

| Tool | Version | Purpose |
|------|---------|---------|
| **Go** | 1.22+ | Build and test the Go source code |
| **Docker** | 20+ | Build and run the sandbox container |
| **Docker Compose** | v2+ | Orchestrate the container |
| **Make** | any | Run build/test/lint shortcuts |
| **golangci-lint** | v1.64+ | Static analysis and linting |

### Optional tools

| Tool | Purpose |
|------|---------|
| **hey** | HTTP load testing (`go install github.com/rakyll/hey@latest`) |
| **jq** | Pretty-print JSON API responses |
| **Python 3** | Serve the web demo UI locally |

### Install Go (if not installed)

```bash
# Linux (amd64)
wget https://go.dev/dl/go1.22.12.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.22.12.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

### Install golangci-lint

```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8
```

Or use the official install script:

```bash
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.64.8
```

### Clone and bootstrap

```bash
git clone https://github.com/Praneeth0910/goboxd.git
cd goboxd
go mod download    # Download Go dependencies
make build         # Build the Docker image
```

---

## Project Structure

```
goboxd/
├── cmd/goboxd/
│   └── main.go                 # Application entry point
│
├── internal/                   # Private packages (not importable externally)
│   ├── config/
│   │   ├── config.go           # YAML config loader & validator
│   │   └── config_test.go      # Config unit tests
│   ├── handler/
│   │   ├── health.go           # GET /healthz, /readyz, /info handlers
│   │   ├── run.go              # POST /run handler (validation + orchestration)
│   │   └── run_test.go         # Handler unit tests
│   ├── middleware/
│   │   ├── cors.go             # CORS middleware
│   │   └── logger.go           # Structured JSON request logging
│   ├── runner/
│   │   ├── runner.go           # Core sandbox execution engine
│   │   ├── runner_test.go      # Runner unit tests
│   │   ├── probe.go            # Startup probes (nsjail + language checks)
│   │   └── sweep.go            # Orphaned jail directory cleanup
│   ├── sandbox/
│   │   ├── dir.go              # Unique jail directory creation
│   │   └── limits.go           # Resource limit merging & output capping
│   ├── stats/
│   │   └── stats.go            # Atomic in-flight job counters
│   ├── status/
│   │   └── status.go           # Status constants & comparison logic
│   └── validate/
│       ├── filename.go         # Filename security validation
│       ├── flags.go            # Compiler flag allowlist validation
│       ├── table_driven_test.go
│       └── validate_test.go
│
├── docs/
│   ├── demo/
│   │   └── index.html          # Interactive web UI (Monaco Editor)
│   ├── api.md                  # API reference
│   ├── architecture.md         # Architecture deep dive
│   ├── benchmarks.md           # Load test results
│   ├── development.md          # This file
│   ├── getting-started.md      # Beginner quickstart
│   ├── how-to-use.md           # Usage guide with examples
│   ├── logging.md              # Structured logging documentation
│   ├── security.md             # Security audit & threat model
│   └── testing.md              # Test suite documentation
│
├── tests/                      # Integration tests (require running container)
│   ├── config_test.go
│   ├── phase2_test.go
│   ├── phase3_test.go
│   └── status_test.go
│
├── scripts/                    # Utility & attack simulation scripts
│   ├── attack-test.go
│   ├── demo-attacks.sh
│   ├── demo-flag-attacks.sh
│   ├── fix-wsl.sh
│   ├── load-test.sh
│   ├── run_benchmarks.sh
│   └── test-filename-attacks.py
│
├── .github/workflows/ci.yml   # GitHub Actions CI pipeline
├── .gitlab-ci.yml              # GitLab CI pipeline
├── .golangci.yml               # Linter configuration
├── Dockerfile                  # Multi-stage Docker build
├── docker-compose.yml          # Docker Compose service definition
├── go.mod / go.sum             # Go module dependencies
├── languages.yaml              # Language runtime configuration
├── Makefile                    # Build automation
└── payload.json                # Sample API request payload
```

---

## Architecture Overview

goboxd follows a clean layered architecture:

```
HTTP Request
     │
     ▼
┌─────────────────────┐
│   Middleware Layer   │  CORS, structured JSON logging, panic recovery
├─────────────────────┤
│   Handler Layer      │  Request decoding, input validation, response encoding
├─────────────────────┤
│   Validate Layer     │  Filename security, compiler flag allowlists
├─────────────────────┤
│   Runner Layer       │  Jail dir creation, nsjail execution, output capture
├─────────────────────┤
│   Sandbox Layer      │  Directory isolation, resource limit enforcement
├─────────────────────┤
│   nsjail             │  Linux namespaces + cgroups (kernel-level isolation)
└─────────────────────┘
```

### Request lifecycle (POST /run)

1. Middleware logs the request and generates a unique request ID.
2. Handler decodes JSON, validates language, filenames, flags, and test count.
3. Handler acquires a concurrency semaphore slot (blocks if all slots are busy).
4. Runner creates a unique jail directory under `/tmp/goboxd/`.
5. Runner writes the source code safely (symlink-safe, `O_EXCL | O_NOFOLLOW`).
6. **Build phase** *(compiled languages only)*: nsjail executes the compiler.
7. **Run phase** *(per test case)*: nsjail executes the program with piped stdin.
8. Runner compares actual vs expected output and assigns statuses.
9. Handler serializes the result as JSON (always HTTP 200 for execution results).
10. Jail directory is cleaned up.

### Key design decisions

- **Channel semaphore** for bounded concurrency (not a worker pool or mutex).
- **User code failures never cause 5xx** — all execution results return HTTP 200.
- **No external dependencies** beyond the Go standard library, `chi` router, and `yaml.v3`.
- **Structured JSON logging** via `log/slog` (one log line per request).

---

## Building & Running

### Build the Docker image

```bash
make build
# or directly:
docker build -t goboxd .
```

### Run the server

```bash
make run
# or directly:
docker compose up
```

The server listens on `http://localhost:8080`.

### Run in background

```bash
docker compose up -d
```

### View logs

```bash
docker logs -f goboxd
```

### Stop the server

```bash
docker compose down
```

---

## Configuration

All language runtimes and global limits are defined in `languages.yaml`.

### Global settings

```yaml
max_source_bytes: 262144    # Max request body size (256 KiB)
max_tests: 50               # Max test cases per request
max_concurrent: 0           # 0 = auto-detect from CPU count
queue_timeout_s: 30         # Seconds to wait for a semaphore slot
```

### Language configuration

Each language entry defines:

```yaml
languages:
  py3:
    id: py3                       # Unique identifier (used in API)
    name: Python 3                # Human-readable name
    source_filename: solution.py  # Default filename for source code
    run:
      cmd: /usr/bin/python3       # Executable path inside container
      args:
        - "{{source}}"            # Template: replaced with actual filename
      limits:
        wall_time_s: 9            # Max execution time (seconds)
        memory_kb: 102400         # Max memory (100 MB)
        max_processes: 100        # Max child processes
```

### Template variables

| Variable | Replaced with |
|----------|---------------|
| `{{source}}` | The source filename (e.g., `solution.py`) |
| `{{artifact}}` | The compiled binary name (e.g., `solution`) |
| `{{flags}}` | User-provided compiler flags |
| `{{class}}` | Java class name |

---

## Adding a New Language

Adding a new language requires **zero Go code changes**. You only need to modify two files:

### Step 1: Add to `languages.yaml`

**For an interpreted language** (like Ruby):

```yaml
ruby:
  id: ruby
  name: Ruby
  source_filename: solution.rb
  run:
    cmd: /usr/bin/ruby
    args:
      - "{{source}}"
    limits:
      wall_time_s: 9
      memory_kb: 102400
      max_processes: 100
```

**For a compiled language** (like Rust):

```yaml
rust:
  id: rust
  name: Rust
  source_filename: solution.rs
  artifact: solution
  build:
    cmd: /usr/bin/rustc
    args:
      - "-O"
      - "-o"
      - "{{artifact}}"
      - "{{source}}"
    limits:
      wall_time_s: 10
      memory_kb: 1048576
      max_processes: 100
  run:
    cmd: ./{{artifact}}
    limits:
      wall_time_s: 3
      memory_kb: 524288
      max_processes: 64
```

### Step 2: Install the runtime in `Dockerfile`

Add the package to the `apt-get install` line in the runtime stage:

```dockerfile
RUN apt-get update && apt-get install -y --no-install-recommends \
    bash ca-certificates \
    python3 g++ gcc nodejs default-jdk iverilog \
    rustc kotlin ruby \
    your-new-language \       # <-- Add here
    libnl-route-3-200 libprotobuf32t64 \
    && rm -rf /var/lib/apt/lists/*
```

### Step 3: Rebuild and verify

```bash
make build
make run
# In another terminal, check the new language appears:
curl -s http://localhost:8080/info | jq '.languages[] | select(.id == "your_lang_id")'
```

---

## Testing

### Unit tests

Run all unit tests (no Docker required):

```bash
make test
# or directly:
go test -v -race -count=1 ./...
```

### Integration tests

Integration tests require a running goboxd container:

```bash
make integration
```

This will:
1. Build the Docker image.
2. Start the container on port 18080.
3. Verify the `/healthz` endpoint.
4. Run Go integration tests from the `tests/` directory.
5. Clean up the container.

### Test specific packages

```bash
# Validation tests only
go test -v ./internal/validate/...

# Table-driven validation tests
go test -v ./internal/validate -run TableDriven

# Config tests
go test -v ./internal/config/...

# Handler tests
go test -v ./internal/handler/...

# Runner tests
go test -v ./internal/runner/...
```

### Run with coverage

```bash
go test -cover ./...

# Generate HTML coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Benchmarks

```bash
go test -bench=. -benchmem ./internal/validate/...
```

### Smoke tests

Quick end-to-end test scripts:

```bash
./smoke_test.sh          # Basic smoke test
./test_endpoints.sh      # All endpoint tests
./test_endpoints_v2.sh   # Extended endpoint tests
```

### Load tests

```bash
make load
# or directly:
./scripts/load-test.sh
```

See [benchmarks.md](benchmarks.md) for detailed load test results and analysis.

---

## Linting & Code Quality

### Run the linter

```bash
make lint
# or directly:
golangci-lint run ./...
```

### Run `go vet`

```bash
make vet
# or directly:
go vet ./...
```

### Linter configuration

The project uses a comprehensive `golangci-lint` configuration (`.golangci.yml`) with 20+ enabled linters:

**Enabled linters include:**
- `errcheck` — unchecked error returns
- `staticcheck` — advanced static analysis
- `govet` — official Go vet checks
- `bodyclose` — HTTP response body leak detection
- `bidichk` — dangerous unicode character detection
- `gocritic` — opinionated style/performance checks
- `misspell` — spelling errors in comments

**Intentionally disabled linters:**
- `cyclop`/`gocyclo` — systems code can have complex functions
- `lll` — line length (editor wrapping preferred)
- `mnd` — magic numbers are legitimate in systems code

---

## CI/CD Pipeline

The project has CI configured for both **GitHub Actions** and **GitLab CI**.

### GitHub Actions (`.github/workflows/ci.yml`)

| Job | What it does |
|-----|-------------|
| **Lint** | Runs `golangci-lint` with v1.64.8 |
| **Build** | Compiles `go build ./cmd/goboxd` |
| **Test** | Runs `go test -race -cover` (unit tests only, excludes integration) |
| **Docker** | Builds the Docker image (no push) |

### GitLab CI (`.gitlab-ci.yml`)

Provides equivalent lint, build, and test stages for GitLab-hosted repositories.

### Running CI checks locally

Before pushing, run all CI checks locally:

```bash
make lint            # Linting
make vet             # Go vet
make test            # Unit tests
make build           # Docker image build
make integration     # Full integration tests
```

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `GOBOXD_MAX_CONCURRENT` | `runtime.NumCPU()` | Max concurrent sandbox executions |
| `NSJAIL_PATH` | `/usr/sbin/nsjail` | Path to the nsjail binary |
| `LANGUAGES_CONFIG` | `/etc/goboxd/languages.yaml` | Path to languages configuration |

### Tuning concurrency

For CPU-heavy compiled languages (C++, Rust, Java), reduce concurrency to avoid CPU contention:

```bash
GOBOXD_MAX_CONCURRENT=2 docker compose up
```

Or set it in `docker-compose.yml`:

```yaml
environment:
  - GOBOXD_MAX_CONCURRENT=2
```

---

## Makefile Targets

| Target | Description |
|--------|-------------|
| `make help` | Show all available targets |
| `make build` | Build the Docker image |
| `make run` | Start with docker-compose |
| `make test` | Run Go unit tests (`-race -count=1`) |
| `make integration` | Build, start container, run integration tests, cleanup |
| `make load` | Run load test script |
| `make lint` | Run golangci-lint |
| `make vet` | Run go vet |
| `make clean` | Remove binaries, images, temp files |

---

## Coding Conventions

### Go style

- Follow standard [Go code review comments](https://github.com/golang/go/wiki/CodeReviewComments).
- Use `log/slog` for all logging (structured JSON output).
- Avoid external dependencies unless absolutely necessary.
- All exported types and functions must have doc comments.

### Error handling

- Always check error returns (enforced by `errcheck` linter).
- Wrap errors with context: `fmt.Errorf("failed to X: %w", err)`.
- User code failures return HTTP 200 with a status field — never 5xx.
- Validation errors return HTTP 400 with structured JSON error codes.

### File organization

- `cmd/` — entry points only (minimal logic).
- `internal/` — all business logic (unexported outside the module).
- `tests/` — integration tests (require running container, tagged `integration`).
- `docs/` — all documentation.

### Commit messages

- Use imperative mood: "Add feature" not "Added feature".
- Reference issue numbers where applicable.

---

## Security Considerations

When contributing, keep these security invariants in mind:

1. **Never shell out** — use `os/exec` with explicit argument lists, never `sh -c`.
2. **Validate all user input** — filenames, flags, source code size, test count.
3. **Follow symlinks safely** — use `O_NOFOLLOW` when writing to jail directories.
4. **Cap all output** — use `CapReader` to prevent memory exhaustion from child processes.
5. **Enforce resource limits** — every sandbox execution has wall time, memory, and process limits.
6. **Allowlist, not blocklist** — compiler flags use strict allowlists.

See [security.md](security.md) for the full security audit and threat model.

---

## Debugging Tips

### View container logs

```bash
docker logs -f goboxd
```

All logs are structured JSON. Use `jq` to filter:

```bash
docker logs goboxd 2>&1 | jq 'select(.path == "/run")'
```

### Run the container interactively

```bash
docker run -it --privileged --rm goboxd:latest /bin/bash
```

This drops you into a shell inside the container where you can:
- Test nsjail manually: `nsjail --help`
- Check language versions: `python3 --version`, `g++ --version`
- Inspect the configuration: `cat /etc/goboxd/languages.yaml`

### Test nsjail directly inside the container

```bash
echo 'print("hello")' > /tmp/test.py
nsjail -Mo --user 65534 --group 65534 \
  -R /usr -R /lib -R /lib64 \
  -T /tmp -R /tmp/test.py:/tmp/test.py \
  --time_limit 5 -- /usr/bin/python3 /tmp/test.py
```

### Check the readiness probe

```bash
curl -s http://localhost:8080/readyz | jq .
```

This shows the status of nsjail and every configured language runtime, including version strings. If any probe fails, the response will be HTTP 503.

### Inspect jail directories

While the server is running, you can check the temporary jail directories:

```bash
docker exec goboxd ls -la /tmp/goboxd/
```

These are automatically cleaned up after each request and swept periodically (every 10 minutes) for orphaned directories.
