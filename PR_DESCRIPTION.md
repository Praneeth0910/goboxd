# Pull Request: goboxd — Secure Code Execution Sandbox

## 🎯 Summary

This PR implements **goboxd**, a production-grade HTTP service written in Go that compiles and executes untrusted code inside isolated [nsjail](https://github.com/google/nsjail) sandboxes. It supports 11 programming languages, real-time test case evaluation, and includes a fully interactive web frontend.

**Branch:** `team/sudo` → `master`
**Commits:** 93
**Files changed:** 65 (+13,940 / -86 lines)

---

## 🏗️ What We Built

### Core Service

A stateless HTTP API (`POST /run`) that accepts source code, compiles it (if needed), executes it inside a sandboxed jail, and returns structured results with per-test-case pass/fail status.

### Key Capabilities

| Capability | Details |
|-----------|---------|
| **Languages** | Python 3, C, C++, Java, Go, Rust, Kotlin, Node.js, Bash, Ruby, Verilog |
| **Sandbox** | nsjail 3.4 — Linux namespaces (PID, mount, UTS, IPC, net) + cgroups |
| **Concurrency** | Channel semaphore with configurable `max_concurrent` + queue timeout |
| **Resource Limits** | Per-request wall time, memory, max processes (configurable via YAML) |
| **Web UI** | Monaco Editor-based frontend with live language loading and test case support |
| **API Endpoints** | `POST /run`, `GET /healthz`, `GET /readyz`, `GET /info` |
| **Observability** | Structured JSON logging (`log/slog`), atomic stats counters |
| **CI/CD** | GitHub Actions + GitLab CI (lint, build, test, Docker build) |

---

## 🔒 Security Hardening (9 Vulnerabilities Closed)

| # | Vulnerability | Severity | Mitigation |
|---|-------------|----------|------------|
| 1 | Path traversal via filename | Critical | Filename validation rejects separators, dots, traversal patterns |
| 2 | Shell-style directory commands | Critical | Go `os.MkdirTemp`/`os.RemoveAll` — never invoke a shell |
| 3 | Compiler flag injection | High | Strict allowlist-only validation per language |
| 4 | No request size limits | High | `http.MaxBytesReader` + `CapReader` |
| 5 | UID collisions under load | Medium | Atomic counter + PID + random hex for directory names |
| 6 | Unbounded child output | High | `CapReader` with truncation marker |
| 7 | Stale jail directories | Medium | Age-based sweep at startup + every 10 minutes |
| 8 | Slowloris HTTP attack | High | Explicit `ReadTimeout`, `ReadHeaderTimeout`, `WriteTimeout` |
| 9 | Symlink attack (TOCTOU) | High | `O_EXCL \| O_NOFOLLOW` + `Lstat` verification |

---

## 📊 Load Test Results

Tested with real traffic simulation (20/200/2000 concurrent users):

| Metric | 20 Users | 200 Users | 2000 Users |
|--------|----------|-----------|------------|
| Total Requests | 60 | 200 | 2000 |
| ✅ Accepted | 60 (100%) | 200 (100%) | 213/227 (93.8%) |
| HTTP Errors | 0 | 0 | 0 |
| Throughput | 6.2 req/s | 6.2 req/s | 2.0 req/s |
| Avg Latency | 2.7s | 15.9s | 31.7s |

**At 2000 users the server never crashed, never OOM'd, never returned a 5xx.** The semaphore queue correctly throttled requests and the 14 non-accepted results were `time_exceeded` (CPU contention during compilation), not server errors.

---

## 🧪 Testing

### Unit Tests
- **74 table-driven sub-tests** for filename and compiler flag validation (33 filename + 41 flags)
- Config loading, handler, runner, and status package tests
- All pass with `-race -count=1`

### Integration Tests
- End-to-end tests requiring a running container (`tests/` directory)
- Health, readiness, run endpoint validation across multiple languages
- Security boundary tests (path traversal, flag injection, resource limits)

### Benchmarks
- ValidateFilename: **7.6M ops/sec** (165 ns/op, 64B/op)
- ValidateFlags: **1.9M ops/sec** (619 ns/op, 112B/op)

### Attack Simulations
- `scripts/attack-test.go` — Automated security attack vectors
- `scripts/demo-attacks.sh` — Path traversal demonstrations
- `scripts/demo-flag-attacks.sh` — Compiler flag injection tests
- `scripts/test-filename-attacks.py` — Python-based filename fuzzing
- `scripts/traffic-sim.sh` — Multi-user concurrent load simulation

### How to Run

```bash
make test            # Unit tests
make lint            # golangci-lint (20+ linters)
make integration     # Full integration suite (auto-builds container)
make load            # Load test
bash scripts/traffic-sim.sh  # 20-user traffic simulation
```

---

## 📁 Project Structure

```
goboxd/
├── cmd/goboxd/main.go          # Entry point (chi router, slog, semaphore)
├── internal/
│   ├── config/                 # YAML config loader & validator
│   ├── handler/                # HTTP handlers (health, run)
│   ├── middleware/             # CORS & structured JSON request logging
│   ├── runner/                 # nsjail sandbox execution engine
│   ├── sandbox/                # Jail directory creation & resource limits
│   ├── stats/                  # Atomic in-flight job counters
│   ├── status/                 # Status constants & output comparison
│   └── validate/               # Filename & compiler flag validation
├── docs/
│   ├── demo/index.html         # Interactive web UI (Monaco Editor)
│   ├── getting-started.md      # Step-by-step beginner guide
│   ├── development.md          # Development & contribution guide
│   ├── api.md                  # API reference
│   ├── architecture.md         # System design deep dive
│   ├── security.md             # Security audit (9 vulnerabilities)
│   ├── benchmarks.md           # Load test results & analysis
│   ├── testing.md              # Test suite documentation
│   ├── logging.md              # Structured logging docs
│   └── how-to-use.md           # Usage guide with curl examples
├── tests/                      # Integration tests
├── scripts/                    # Load tests & attack simulations
├── Dockerfile                  # Multi-stage build (nsjail + Go + runtimes)
├── docker-compose.yml          # Service orchestration
├── languages.yaml              # Language configuration (zero-code extensible)
├── Makefile                    # build, run, test, lint, integration, clean
├── .golangci.yml               # 20+ linter config
└── .github/workflows/ci.yml   # CI pipeline (lint, build, test, docker)
```

---

## 🔑 Design Decisions

### Why nsjail over Docker-in-Docker?
nsjail provides lightweight per-request isolation using Linux namespaces without the overhead of spinning up a full container per execution. This gives us sub-second sandbox setup time.

### Why channel semaphore over worker pool?
Go's `net/http` already spawns a goroutine per request. A buffered channel (`make(chan struct{}, N)`) naturally provides bounded concurrency with FIFO queue behavior — no extra goroutines, workers, or job dispatching needed.

### Why user errors return HTTP 200?
Compilation failures and wrong outputs are expected outcomes, not server errors. HTTP 5xx is reserved for actual server-side failures. This follows the convention used by online judges and code execution APIs.

### Why zero Go code changes to add a language?
The language registry (`languages.yaml`) uses template variables (`{{source}}`, `{{artifact}}`, `{{flags}}`) that the runner expands at runtime. Adding a language only requires a YAML block and a Dockerfile install line.

---

## 📖 Documentation

| Document | Description |
|----------|-------------|
| [README.md](README.md) | Project overview with step-by-step quickstart |
| [Getting Started](docs/getting-started.md) | Beginner-friendly setup guide |
| [Development Guide](docs/development.md) | Contributing, testing, CI/CD, adding languages |
| [API Reference](docs/api.md) | Complete endpoint documentation |
| [Architecture](docs/architecture.md) | System design and request lifecycle |
| [Security Audit](docs/security.md) | 9 vulnerabilities identified and closed |
| [Benchmarks](docs/benchmarks.md) | Load test results with analysis |
| [Testing](docs/testing.md) | Test suite coverage documentation |
| [Logging](docs/logging.md) | Structured JSON logging |
| [How to Use](docs/how-to-use.md) | Detailed usage guide with examples |

---

## ✅ Submission Checklist

- [x] All code compiles without errors (`go build ./cmd/goboxd`)
- [x] All unit tests pass (`make test`)
- [x] All linting passes (`make lint` — 20+ linters)
- [x] Docker image builds successfully (`make build`)
- [x] Integration tests pass (`make integration`)
- [x] Health, readiness, and info endpoints work
- [x] Code execution works for all 11 languages
- [x] Security hardening — 9 vulnerabilities closed
- [x] Load tested at 20, 200, and 2000 concurrent users
- [x] Interactive web UI functional
- [x] Structured JSON logging implemented
- [x] CI/CD pipelines configured (GitHub Actions + GitLab CI)
- [x] Comprehensive documentation (10 docs)
- [x] Zero external dependencies beyond `chi` router and `yaml.v3`

---

## 🚀 How to Verify

```bash
# 1. Clone and build
git clone https://github.com/Praneeth0910/goboxd.git
cd goboxd
git checkout team/sudo
make build

# 2. Start the server
make run

# 3. (New terminal) Verify health
curl -s http://localhost:8080/healthz

# 4. Run a Python program
curl -s -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{"language":"py3","source":"print(int(input())*2)","tests":[{"stdin":"5\n","expected_stdout":"10\n"}]}'

# 5. Try the web UI
cd docs/demo && python3 -m http.server 8081
# Open http://localhost:8081

# 6. Run the test suite
make test
make lint

# 7. Run load test (20 concurrent users)
bash scripts/traffic-sim.sh
```

---

## 👥 Team

- **Praneeth0910** — 93 commits (core implementation, security, testing, documentation)
