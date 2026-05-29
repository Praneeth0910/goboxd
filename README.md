<div align="center">

# goboxd

**A robust, secure Go HTTP service for executing untrusted code in isolated sandboxes.**

[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.23-00ADD8.svg?logo=go&logoColor=white)](https://go.dev)
[![Docker](https://img.shields.io/badge/Docker-Required-2496ED.svg?logo=docker&logoColor=white)](https://www.docker.com)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](https://github.com/thesouldev/goboxd/pulls)

[Getting Started](docs/getting-started.md) · [API Reference](docs/api.md) · [Architecture](docs/architecture.md) · [Development Guide](docs/development.md)

</div>

---

## What is goboxd?

**goboxd** safely compiles and runs untrusted code across multiple programming languages inside isolated [nsjail](https://github.com/google/nsjail) sandboxes. It supports real-time stdout matching against predefined test cases, making it perfect for:

- 🏆 **Competitive programming platforms** — judge submissions with multiple test cases
- 💻 **Online IDEs** — let users run code safely in the browser
- 📝 **Code assessment tools** — evaluate candidate solutions securely
- 🎓 **Educational platforms** — students can experiment without risk

---

## Features

- 🔒 **Strict Isolation** — Process separation using Linux namespaces and cgroups via `nsjail`. No network access, no filesystem escape.
- 🌐 **11 Languages** — Python, C, C++, Java, Go, Rust, Kotlin, Node.js, Bash, Ruby, Verilog — all configurable via `languages.yaml`.
- ⚡ **Bounded Concurrency** — Built-in request queuing with a channel semaphore prevents system overload.
- 📊 **Resource Limits** — Configurable per-request limits for wall time, memory, file descriptors, and processes.
- 🖥️ **Interactive Web UI** — Built-in Monaco Editor frontend for live code testing (no setup needed).
- 🐳 **Fully Containerized** — One `docker build`, one `docker compose up`. No host dependencies.
- 🛡️ **Security-First** — Path traversal prevention, compiler flag allowlists, symlink-safe file writes, output capping, slowloris protection.

---

## Quick Start (Step by Step)

> **Complete beginner?** Follow every step below. You'll have goboxd running in under 10 minutes.

### Prerequisites

You need these tools installed on your computer:

| Tool | How to check | Install guide |
|------|-------------|---------------|
| **Git** | `git --version` | [git-scm.com/downloads](https://git-scm.com/downloads) |
| **Docker** | `docker --version` | [docs.docker.com/get-docker](https://docs.docker.com/get-docker/) |
| **Docker Compose** | `docker compose version` | Bundled with Docker Desktop |
| **cURL** | `curl --version` | Pre-installed on macOS/Linux |

> **Windows users**: Install [Docker Desktop](https://www.docker.com/products/docker-desktop/) and enable the WSL2 backend. Run all commands inside a WSL2 terminal (Ubuntu recommended).

### Step 1 — Clone the repository

Open your terminal and run:

```bash
git clone https://github.com/Praneeth0910/goboxd.git
cd goboxd
```

### Step 2 — Build the Docker image

```bash
make build
```

> **What this does**: Runs `docker build -t goboxd .` which compiles nsjail, the Go server binary, and installs all language runtimes into a single Docker image. The first build takes **5–10 minutes** (subsequent builds use cache and are much faster).

> **If `make` is not installed**, run directly:
> ```bash
> docker build -t goboxd .
> ```

### Step 3 — Start the server

```bash
make run
```

This starts the server on **http://localhost:8080**.

> **Alternative**: Run in the background with:
> ```bash
> docker compose up -d
> ```

### Step 4 — Verify it's running

Open a **new terminal** and run:

```bash
curl -s http://localhost:8080/healthz
```

You should see:

```json
{"status":"ok"}
```

✅ **goboxd is running!**

### Step 5 — Run your first program

Send a Python program to the sandbox:

```bash
curl -s -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "language": "py3",
    "source": "print(\"Hello, World!\")",
    "tests": [{"stdin": "", "expected_stdout": "Hello, World!\n"}]
  }'
```

**Response:**

```json
{
  "status": "accepted",
  "tests": [
    {
      "status": "accepted",
      "stdout": "Hello, World!\n",
      "stderr": "",
      "duration_ms": 45
    }
  ]
}
```

`"status": "accepted"` means the output matched your expected value. 🎉

### Step 6 — Try more examples

**Python with stdin input:**

```bash
curl -s -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "language": "py3",
    "source": "n = int(input())\nprint(n * 2)",
    "tests": [
      {"stdin": "5\n", "expected_stdout": "10\n"},
      {"stdin": "100\n", "expected_stdout": "200\n"}
    ]
  }'
```

**C++ (compiled language):**

```bash
curl -s -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "language": "cpp",
    "source": "#include <iostream>\nint main() {\n    std::cout << \"Hello from C++!\" << std::endl;\n    return 0;\n}",
    "tests": [{"stdin": "", "expected_stdout": "Hello from C++!\n"}]
  }'
```

**Go:**

```bash
curl -s -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "language": "go",
    "source": "package main\nimport \"fmt\"\nfunc main() { fmt.Println(\"Hello from Go!\") }",
    "tests": [{"stdin": "", "expected_stdout": "Hello from Go!\n"}]
  }'
```

> **Tip**: Pipe any response through `jq` for pretty-printed output: `... | jq .`

### Step 7 — Use the Interactive Web UI

goboxd ships with a built-in web editor powered by Monaco (the same editor in VS Code).

1. Open a **new terminal** (keep the server running).
2. Start the web UI:

   ```bash
   cd docs/demo
   python3 -m http.server 8081
   ```

3. Open **http://localhost:8081** in your browser.

You'll see a code editor where you can:
- Select a language from the dropdown
- Write code with syntax highlighting
- Define test cases with stdin and expected stdout
- Click **Run** (or press `Ctrl+Enter`) to execute

### Step 8 — Stop the server

```bash
# If running in foreground: press Ctrl+C, then:
docker compose down

# If running with docker run:
docker stop goboxd && docker rm goboxd

# Full cleanup (removes Docker image too):
make clean
```

---

## How the API Works

### Request format

Send a `POST` request to `/run` with this JSON body:

```json
{
  "language": "py3",
  "source": "your source code here",
  "tests": [
    {
      "stdin": "input for your program",
      "expected_stdout": "expected output\n"
    }
  ]
}
```

### Response format

```json
{
  "status": "accepted",
  "build": { ... },
  "tests": [
    {
      "status": "accepted",
      "stdout": "actual output\n",
      "stderr": "",
      "duration_ms": 45
    }
  ]
}
```

### Status values

| Status | Meaning |
|--------|---------|
| `accepted` | ✅ Output matches expected |
| `wrong_output` | ❌ Output differs |
| `runtime_error` | 💥 Program crashed |
| `time_exceeded` | ⏱️ Hit wall time limit |
| `memory_exceeded` | 🧠 Hit memory limit |
| `build_failed` | 🔨 Compilation error |

### Available endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/run` | Execute code in sandbox |
| `GET` | `/healthz` | Liveness probe |
| `GET` | `/readyz` | Readiness probe (checks nsjail + languages) |
| `GET` | `/info` | Server info, languages, limits, stats |

*For the complete API reference, see [docs/api.md](docs/api.md).*

---

## Supported Languages

| ID | Language | Type | Wall Time | Memory |
|----|----------|------|-----------|--------|
| `py3` | Python 3 | Interpreted | 9s | 100 MB |
| `cpp` | C++ | Compiled | 3s (run) | 512 MB |
| `c` | C | Compiled | 3s (run) | 512 MB |
| `java` | Java | Compiled | 5s | 512 MB |
| `go` | Go | Compiled | 3s (run) | 512 MB |
| `rust` | Rust | Compiled | 3s (run) | 512 MB |
| `kotlin` | Kotlin | Compiled | 5s | 512 MB |
| `node` | Node.js | Interpreted | 9s | 100 MB |
| `bash` | Bash | Interpreted | 9s | 100 MB |
| `ruby` | Ruby | Interpreted | 9s | 100 MB |
| `verilog` | Verilog | Compiled | 5s | 256 MB |

Adding a new language requires **zero Go code changes** — just edit `languages.yaml` and `Dockerfile`. See the [Development Guide](docs/development.md#adding-a-new-language) for instructions.

---

## Project Structure

```text
goboxd/
├── cmd/goboxd/main.go          # Application entry point
├── internal/
│   ├── config/                 # YAML config loader & validator
│   ├── handler/                # HTTP handlers (health, run)
│   ├── middleware/             # CORS & structured JSON logging
│   ├── runner/                 # Sandbox execution engine
│   ├── sandbox/                # Jail directory & resource limits
│   ├── stats/                  # In-flight job counters
│   ├── status/                 # Status constants & output comparison
│   └── validate/               # Filename & compiler flag validation
├── docs/
│   ├── demo/index.html         # Interactive web UI
│   ├── api.md                  # API reference
│   ├── architecture.md         # Architecture deep dive
│   ├── benchmarks.md           # Load test results
│   ├── development.md          # Development & contribution guide
│   ├── getting-started.md      # Beginner quickstart guide
│   ├── how-to-use.md           # Detailed usage guide
│   ├── logging.md              # Structured logging docs
│   ├── security.md             # Security audit & threat model
│   └── testing.md              # Test suite documentation
├── tests/                      # Integration tests
├── scripts/                    # Load tests & attack simulations
├── Dockerfile                  # Multi-stage Docker build
├── docker-compose.yml          # Service orchestration
├── languages.yaml              # Language runtime configuration
├── Makefile                    # Build automation
└── .golangci.yml               # Linter configuration
```

---

## Security

goboxd was designed with security as a first-class concern. Key protections include:

| Threat | Mitigation |
|--------|-----------|
| **Path traversal** (`../../etc/passwd`) | Filename validation rejects separators, dots, traversal patterns |
| **Compiler flag injection** (`-fplugin=evil.so`) | Strict allowlist-only validation |
| **Symlink attacks (TOCTOU)** | `O_EXCL \| O_NOFOLLOW` file creation + `Lstat` verification |
| **Memory exhaustion** | `CapReader` truncates child process output |
| **Request flooding** | Bounded concurrency with channel semaphore |
| **Slowloris attacks** | Explicit `ReadTimeout`, `ReadHeaderTimeout`, `WriteTimeout` |
| **Stale jail data** | Automatic directory sweep at startup + every 10 minutes |
| **Oversized payloads** | `http.MaxBytesReader` enforces `max_source_bytes` |

For the full security audit, see [docs/security.md](docs/security.md).

---

## Troubleshooting

<details>
<summary><b>Port 8080 is already in use</b></summary>

Use a different port:

```bash
docker run -d --privileged --name goboxd -p 9090:8080 goboxd:latest
```

Then use `http://localhost:9090` for all API calls.
</details>

<details>
<summary><b>Permission denied when running Docker</b></summary>

Add your user to the `docker` group:

```bash
sudo usermod -aG docker $USER
```

Log out and log back in, then try again.
</details>

<details>
<summary><b>Connection refused when calling the API</b></summary>

The container may still be starting up. Wait a few seconds, then check:

```bash
docker ps                 # Is the container running?
docker logs goboxd        # Any errors in the logs?
```
</details>

<details>
<summary><b>First build is very slow</b></summary>

The first build compiles nsjail from source and installs 11 language runtimes. This is a one-time cost (~5–10 min). Subsequent builds use Docker's layer cache.
</details>

---

## 📖 Documentation

| Document | Description |
|----------|-------------|
| [Getting Started](docs/getting-started.md) | Step-by-step setup for absolute beginners |
| [How to Use](docs/how-to-use.md) | Detailed usage guide with examples |
| [API Reference](docs/api.md) | Complete endpoint documentation |
| [Architecture](docs/architecture.md) | System design and request lifecycle |
| [Security Audit](docs/security.md) | Threat model and mitigations |
| [Development Guide](docs/development.md) | Contributing, testing, CI/CD |
| [Benchmarks](docs/benchmarks.md) | Load test results and analysis |
| [Logging](docs/logging.md) | Structured JSON logging details |
| [Testing](docs/testing.md) | Test suite documentation |

---

## 🤝 Contributing

Contributions are welcome! Before submitting a pull request:

1. Open an issue to discuss significant changes.
2. Run linting and tests locally:

   ```bash
   make lint
   make test
   ```

3. Ensure your changes pass CI.

See the [Development Guide](docs/development.md) for the full setup and contribution workflow.

---

## 📄 License

This project is licensed under the [GNU General Public License v3.0](LICENSE).
