<div align="center">

# goboxd

A Go HTTP service that compiles and runs untrusted code inside nsjail sandboxes. It accepts source code and test cases via a JSON API, executes each test in isolation, and returns per-test results with stdout, stderr, wall time, and peak memory.

[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.23-00ADD8.svg?logo=go&logoColor=white)](https://go.dev)
[![Docker](https://img.shields.io/badge/Docker-Required-2496ED.svg?logo=docker&logoColor=white)](https://www.docker.com)
[![Getting Started](https://img.shields.io/badge/Getting%20Started-2496ED.svg?logo=docker&logoColor=white)](https://www.docker.com)

</div>

---

## Isolation model

Each execution runs in a separate nsjail jail with:

- Linux PID, mount, UTS, IPC, and network namespaces — no host access, no outbound network
- A Kafel seccomp policy that kills any process calling one of 28 restricted syscalls (`ptrace`, `bpf`, `mount`, `kexec_load`, `unshare`, and others)
- A per-request cgroupv2 slice that hard-limits memory, disables swap (`memory.swap.max = 0`), and exposes peak usage via `memory.peak` — so `memory_peak_kb` in the response is read from the kernel, not estimated
- Strict rlimits: no core dumps, 8 MB stack cap, 100 MB file write cap
- An environment allowlist: only `HOME`, `TMPDIR`, and `PATH` are passed in — host environment variables are never inherited

These apply to both the build phase (compiler) and the run phase (user program).

---

## Languages

11 runtimes ship in the Docker image: Python 3, C, C++, Java, Go, Rust, Kotlin, Node.js, Bash, Ruby, and Verilog.

Languages are defined entirely in `languages.yaml`. Adding a new language means editing that file and the `Dockerfile` `apt-get` line — no Go code changes. The Dockerfile smoke-tests every toolchain at build time (`python3 --version && node --version && ...`) so a broken runtime is caught before deployment.

---

## API

`POST /run` — execute code and compare against test cases.

```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "language": "py3",
    "source": "n = int(input())\nprint(n * 2)",
    "tests": [
      {"stdin": "5\n", "expected_stdout": "10\n"},
      {"stdin": "21\n", "expected_stdout": "42\n"}
    ]
  }'
```

```json
{
  "status": "accepted",
  "tests": [
    {"status": "accepted", "stdout": "10\n", "duration_ms": 38, "memory_peak_kb": 6144},
    {"status": "accepted", "stdout": "42\n", "duration_ms": 34, "memory_peak_kb": 6112}
  ]
}
```

All execution outcomes — wrong output, runtime error, time limit, memory limit, compile failure — return HTTP 200 with a structured status field. 5xx means the server itself failed.

Other endpoints: `GET /healthz` (liveness), `GET /readyz` (readiness, checks nsjail + each language binary), `GET /info` (build info, language list, resource limits, in-flight stats).

---

## Quick start

**Requirements:** Docker, Docker Compose, Make.

Refer [Getting Started](docs/getting-started.md) and [How to Run](docs/how-to-run.md) for step by step installation guide.
Also i recommend to read all the docs before using the service.
To use interactive UI service refer [Getting Started](docs/getting-started.md) from line 274 (Must try!!)

```bash
git clone https://github.com/Praneeth0910/goboxd.git
cd goboxd
make build   # compiles nsjail, Go binary, installs runtimes (~5 min first run)
make run     # starts on http://localhost:8080
```

```bash
curl -s http://localhost:8080/healthz
# {"status":"ok"}
```

```bash
make test        # unit tests
make integration # builds image, starts container, runs integration tests, tears down
make lint        # golangci-lint
```

---

## Security

The sandbox is designed to contain adversarial code that actively tries to escape.

**Kernel-level isolation**

Every nsjail invocation creates a fresh set of Linux namespaces (PID, mount, UTS, IPC, network). Processes inside cannot see the host process tree, cannot reach the network, and cannot access the host filesystem beyond the read-only bind mounts explicitly listed.

**Syscall filtering**

A Kafel seccomp BPF policy is loaded for every execution — both the build phase and the run phase. Any attempt to call one of 28 blocked syscalls results in `SIGKILL` of the offending process. Blocked calls include `ptrace`, `process_vm_readv`, `bpf`, `mount`, `unshare`, `setns`, `kexec_load`, `reboot`, `userfaultfd`, `add_key`, and others commonly used in sandbox escapes.

**Memory containment**

Each request gets its own cgroupv2 hierarchy. Swap is disabled (`memory.swap.max = 0`), so hitting the memory limit results in an immediate OOM kill rather than silent swapping. After the sandbox exits, `memory.peak` is read from the cgroup and returned as `memory_peak_kb` in the response — the figure comes from the kernel, not from sampling or estimation.

**Resource limits (rlimits)**

Beyond wall time and address space, the sandbox enforces:

| Limit | Value |
|-------|-------|
| Core dumps | Disabled (`rlimit_core = 0`) |
| Stack size | 8 MB cap |
| File write size | 100 MB cap |
| Max open files | 1024 |
| Max processes | Configurable per language |

**Input validation**

| Attack surface | Protection |
|----------------|------------|
| Path traversal in filenames (`../../etc/passwd`) | `ValidateFilename` rejects separators, leading dots, and traversal sequences; `SafeJoin` double-checks the resolved path stays inside the jail |
| Compiler flag injection (`-fplugin=evil.so`) | Strict per-language allowlist; any unlisted flag rejects the entire request |
| Oversized payloads | `http.MaxBytesReader` enforces `max_source_bytes` before JSON decoding begins |
| Symlink attacks on source file write | File is created with `O_EXCL | O_NOFOLLOW`; existence verified with `Lstat` before use |
| Output-based memory exhaustion | `CapReader` truncates stdout/stderr from child processes to a hard byte cap |
| Slowloris / slow client attacks | Explicit `ReadTimeout`, `ReadHeaderTimeout`, and `WriteTimeout` on the HTTP server |

**Environment isolation**

The nsjail invocation passes only `HOME=/`, `TMPDIR=/tmp`, and `PATH=...` into the sandbox. No host environment variables are inherited. The host's secrets, credentials, and configuration are never visible to user code.

The full threat model with per-mitigation code references is in [docs/security.md](docs/security.md).

---

## Documentation

| Document | Contents |
|----------|----------|
| [API Reference](docs/api.md) | Full request/response schema, all status codes, error codes |
| [Architecture](docs/architecture.md) | Component map, request lifecycle, sandbox isolation details |
| [Security Audit](docs/security.md) | Threat model, all mitigations with code references |
| [Development Guide](docs/development.md) | Setup, adding languages, testing, CI/CD, coding conventions |
| [Getting Started](docs/getting-started.md) | Step-by-step walkthrough for first-time setup |

---

## License

[GNU General Public License v3.0](LICENSE)
