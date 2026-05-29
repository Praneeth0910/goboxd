# Getting Started with goboxd

A step-by-step guide for complete beginners to get goboxd up and running on your machine.

---

## What is goboxd?

**goboxd** is a secure code execution sandbox — an HTTP API that accepts source code, compiles it (if needed), runs it inside an isolated jail, and returns the output. Think of it like the backend that powers online coding judges or browser-based IDEs.

Every submitted program runs inside [nsjail](https://github.com/google/nsjail), which uses Linux namespaces and cgroups to completely isolate it from the host system. Malicious code **cannot** access your files, network, or other processes.

---

## Prerequisites

Before you begin, make sure the following tools are installed on your system.

| Tool | Why it's needed | Install guide |
|------|----------------|---------------|
| **Git** | Clone the repository | [git-scm.com/downloads](https://git-scm.com/downloads) |
| **Docker** (v20+) | Build & run the sandbox container | [docs.docker.com/get-docker](https://docs.docker.com/get-docker/) |
| **Docker Compose** (v2+) | Orchestrate the container | Bundled with Docker Desktop; on Linux: [install plugin](https://docs.docker.com/compose/install/linux/) |
| **cURL** | Send API requests from the terminal | Pre-installed on macOS/Linux; Windows: `winget install curl` |
| **Python 3** *(optional)* | Serve the web UI locally | Pre-installed on most systems |
| **jq** *(optional)* | Pretty-print JSON responses | `sudo apt install jq` / `brew install jq` |

### Verify your installation

Open a terminal and run:

```bash
git --version        # e.g., git version 2.43.0
docker --version     # e.g., Docker version 27.x.x
docker compose version  # e.g., Docker Compose version v2.x.x
curl --version       # e.g., curl 8.x.x
```

If any command is not found, install that tool before continuing.

> **WSL2 users (Windows)**: Make sure Docker Desktop is configured to use the WSL2 backend. Open Docker Desktop → Settings → General → check "Use the WSL 2 based engine".

---

## Step 1: Clone the Repository

```bash
git clone https://github.com/Praneeth0910/goboxd.git
cd goboxd
```

You should see the project files:

```
goboxd/
├── cmd/goboxd/       # Go entry point
├── internal/         # Core packages (handler, runner, config, etc.)
├── docs/             # Documentation & web demo
├── Dockerfile        # Multi-stage Docker build
├── docker-compose.yml
├── languages.yaml    # Language runtime configuration
├── Makefile          # Build/test/run shortcuts
└── README.md
```

---

## Step 2: Build the Docker Image

The entire sandbox (Go binary, nsjail, compilers, interpreters) is packaged as a single Docker image. Build it with:

```bash
make build
```

This runs `docker build -t goboxd .` under the hood. The first build will take **5–10 minutes** because it:

1. Compiles **nsjail** from source (C++ build).
2. Compiles the **goboxd** Go binary.
3. Installs runtime dependencies (Python, GCC, Node.js, Java, Rust, etc.).

Subsequent builds are much faster thanks to Docker layer caching.

> [!TIP]
> If `make` is not available on your system, you can run the Docker command directly:
> ```bash
> docker build -t goboxd .
> ```

---

## Step 3: Start the Server

### Option A: Using `make` (recommended)

```bash
make run
```

This starts the container via `docker-compose up`, mapping port **8080** on your host to port **8080** inside the container.

### Option B: Using `docker compose` directly

```bash
docker compose up
```

Add `-d` to run in the background:

```bash
docker compose up -d
```

### Option C: Using `docker run` directly

```bash
docker run -d \
  --privileged \
  --name goboxd \
  -p 8080:8080 \
  goboxd:latest
```

> [!IMPORTANT]
> The `--privileged` flag is **required**. goboxd uses Linux namespaces and cgroups (via nsjail) to isolate processes, which needs elevated container permissions.

---

## Step 4: Verify the Server is Running

Check the health endpoint:

```bash
curl -s http://localhost:8080/healthz
```

**Expected output:**

```json
{"status":"ok"}
```

For detailed server info (supported languages, versions, limits):

```bash
curl -s http://localhost:8080/info | jq .
```

**Expected output (excerpt):**

```json
{
  "build_info": {
    "version": "0.1.0",
    "go_version": "go1.22.12"
  },
  "languages": [
    { "id": "py3", "name": "Python 3" },
    { "id": "cpp", "name": "C++" },
    { "id": "go",  "name": "Go" },
    { "id": "java","name": "Java" },
    { "id": "node","name": "Node.js (javascript)" },
    { "id": "rust","name": "Rust" },
    ...
  ]
}
```

---

## Step 5: Run Your First Program

### Example 1: Python — Hello World

Send a Python program to the sandbox:

```bash
curl -s -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "language": "py3",
    "source": "print(\"Hello, World!\")",
    "tests": [
      {
        "stdin": "",
        "expected_stdout": "Hello, World!\n"
      }
    ]
  }' | jq .
```

**Expected response:**

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

✅ **`"status": "accepted"`** means the output matched your expected value.

### Example 2: Python — With stdin input

```bash
curl -s -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "language": "py3",
    "source": "n = int(input())\nprint(n * 2)",
    "tests": [
      {
        "stdin": "5\n",
        "expected_stdout": "10\n"
      },
      {
        "stdin": "100\n",
        "expected_stdout": "200\n"
      }
    ]
  }' | jq .
```

This sends **two test cases** — the sandbox runs your code twice with different inputs and checks each output.

### Example 3: C++ — Compiled language

```bash
curl -s -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "language": "cpp",
    "source": "#include <iostream>\nint main() {\n    std::cout << \"Hello from C++!\" << std::endl;\n    return 0;\n}",
    "tests": [
      {
        "stdin": "",
        "expected_stdout": "Hello from C++!\n"
      }
    ]
  }' | jq .
```

**Response (notice the `build` field for compiled languages):**

```json
{
  "status": "accepted",
  "build": {
    "status": "ok",
    "stdout": "",
    "stderr": "",
    "duration_ms": 230
  },
  "tests": [
    {
      "status": "accepted",
      "stdout": "Hello from C++!\n",
      "stderr": "",
      "duration_ms": 10
    }
  ]
}
```

---

## Step 6: Use the Interactive Web UI

goboxd ships with a built-in web interface featuring the Monaco Editor (the same editor that powers VS Code).

### Start the Web UI

Open a **new terminal** (keep the server running) and serve the demo page:

```bash
cd docs/demo
python3 -m http.server 8081
```

### Open in your browser

Navigate to **[http://localhost:8081](http://localhost:8081)**.

You will see:
- A **language dropdown** (auto-populated from the running API).
- A **code editor** with syntax highlighting.
- **Test case panels** where you can add stdin and expected stdout.
- A **Run** button (or press `Ctrl+Enter`) to execute your code.

> [!TIP]
> The web UI communicates with the API at `http://localhost:8080`. If your API is on a different port, you may need to modify the `API_URL` constant in `docs/demo/index.html`.

---

## Step 7: Understanding the API Response

Every response from `POST /run` uses the same structure. Here's what each field means:

| Field | Description |
|-------|-------------|
| `status` | Overall result: `accepted`, `wrong_output`, `runtime_error`, `build_failed`, etc. |
| `build` | *(compiled languages only)* Compilation result with status, stdout, stderr, duration. |
| `tests[]` | Array of per-test results. |
| `tests[].status` | Individual test result. |
| `tests[].stdout` | Actual program output. |
| `tests[].stderr` | Error output (compiler warnings, runtime errors). |
| `tests[].duration_ms` | Execution time in milliseconds. |

### Status values explained

| Status | What it means |
|--------|--------------|
| `accepted` | ✅ Output matches expected — test passed |
| `wrong_output` | ❌ Output differs from expected |
| `output_whitespace_mismatch` | ⚠️ Output differs only in whitespace (trailing newline, etc.) |
| `runtime_error` | 💥 Program crashed (non-zero exit code) |
| `time_exceeded` | ⏱️ Program took too long (hit wall time limit) |
| `memory_exceeded` | 🧠 Program used too much memory (killed by OOM) |
| `build_failed` | 🔨 Compilation failed (syntax errors, etc.) |

---

## Supported Languages

goboxd supports **11 languages** out of the box:

| Language ID | Language | Type |
|-------------|----------|------|
| `py3` | Python 3 | Interpreted |
| `cpp` | C++ | Compiled |
| `c` | C | Compiled |
| `java` | Java | Compiled |
| `go` | Go | Compiled |
| `rust` | Rust | Compiled |
| `kotlin` | Kotlin | Compiled |
| `node` | Node.js (JavaScript) | Interpreted |
| `bash` | Bash (sh) | Interpreted |
| `ruby` | Ruby | Interpreted |
| `verilog` | Verilog | Compiled |

---

## Stopping the Server

### If started with `make run` or `docker compose up`

Press `Ctrl+C` in the terminal, then:

```bash
docker compose down
```

### If started with `docker run`

```bash
docker stop goboxd && docker rm goboxd
```

### Full cleanup (remove the Docker image too)

```bash
make clean
```

---

## Troubleshooting

### Port 8080 is already in use

```
Error: Port 8080 is already allocated
```

**Fix:** Use a different port:

```bash
docker run -d --privileged --name goboxd -p 9090:8080 goboxd:latest
```

Then make requests to `http://localhost:9090` instead.

### Docker build fails — permission denied

```
permission denied while trying to connect to the Docker daemon socket
```

**Fix:** Add your user to the `docker` group:

```bash
sudo usermod -aG docker $USER
```

Then **log out and log back in** (or restart your terminal).

### Connection refused when calling the API

**Fix:** Wait a few seconds after starting the container. Check if it's running:

```bash
docker ps
```

Check the logs:

```bash
docker logs goboxd
```

### Build takes too long

The first build compiles nsjail from source and installs multiple language runtimes. This is a **one-time cost**. Subsequent builds use Docker's cache and are much faster.

---

## Next Steps

- 📖 Read the [API Reference](api.md) for all endpoints and request/response formats.
- 🏗️ Read the [Architecture Guide](architecture.md) to understand how the sandbox works internally.
- 🔒 Read the [Security Audit](security.md) to learn about the threat model and mitigations.
- 🛠️ Read the [Development Guide](development.md) to contribute to goboxd.
