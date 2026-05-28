<div align="center">

# goboxd

**A robust, secure Go HTTP service for executing untrusted code in isolated sandboxes.**

[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.23-00ADD8.svg?logo=go&logoColor=white)](https://go.dev)
[![Docker](https://img.shields.io/badge/Docker-Required-2496ED.svg?logo=docker&logoColor=white)](https://www.docker.com)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](https://github.com/thesouldev/goboxd/pulls)

</div>

---

## Overview

`goboxd` safely compiles and runs untrusted code across multiple programming languages inside isolated `nsjail` sandboxes. It supports real-time stdout matching against predefined test cases, making it perfect for competitive programming platforms, online IDEs, and code assessment tools.

## Features

-  **Strict Isolation**: Process separation using Linux namespaces and cgroups via `nsjail`.
-  **Plug & Play Languages**: Easily add or configure runtimes via a central `languages.yaml`. Supports 10+ languages (Go, Python, C++, Java, Node, Rust, etc.).
-  **Bounded Concurrency**: Built-in request queuing to prevent system overload.
-  **Resource Limits**: Configurable per-request limits for time, memory, file descriptors, and processes.
-  **Interactive UI Included**: Features a built-in web frontend (`docs/demo`) powered by Monaco Editor for live testing.
-  **Fully Containerized**: No host dependencies other than Docker.

## Project Structure

```text
.
├── cmd/
│   └── goboxd/
│       └── main.go
├── docs/
│   ├── ai/
│   │   ├── adrs.md
│   │   ├── issues.md
│   │   ├── patterns.md
│   │   ├── plan-evolution.md
│   │   ├── postmortem.md
│   │   └── prompts.md
│   ├── demo/
│   │   └── index.html
│   ├── api.md
│   ├── architecture.md
│   ├── benchmarks.md
│   ├── how-to-use.md
│   ├── languages.md
│   ├── logging.md
│   ├── security.md
│   └── testing.md
├── internal/
│   ├── config/
│   │   ├── config.go
│   │   └── config_test.go
│   ├── handler/
│   │   ├── health.go
│   │   ├── run.go
│   │   └── run_test.go
│   ├── middleware/
│   │   ├── cors.go
│   │   └── logger.go
│   ├── runner/
│   │   ├── probe.go
│   │   ├── runner.go
│   │   ├── runner_test.go
│   │   └── sweep.go
│   ├── sandbox/
│   │   ├── dir.go
│   │   └── limits.go
│   ├── stats/
│   │   └── stats.go
│   ├── status/
│   │   └── status.go
│   └── validate/
│       ├── filename.go
│       ├── flags.go
│       ├── table_driven_test.go
│       └── validate_test.go
├── scripts/
│   ├── attack-test.go
│   ├── demo-attacks.sh
│   ├── demo-flag-attacks.sh
│   ├── fix-wsl.sh
│   ├── load-test.sh
│   ├── run_benchmarks.sh
│   └── test-filename-attacks.py
├── tests/
│   ├── config_test.go
│   ├── phase2_test.go
│   ├── phase3_test.go
│   └── status_test.go
├── Dockerfile
├── docker-compose.yml
├── go.mod
├── go.sum
├── languages.yaml
├── LICENSE
├── Makefile
├── nsjail
├── payload.json
├── project_test.py
├── README.md
├── smoke_test.sh
├── STAGE1-CHECKLIST.md
├── STAGE1-QUICK-REF.md
├── SUBMISSION-GUIDE.md
├── test_docker.sh
├── test_endpoints.sh
├── test_endpoints_v2.sh
└── TESTING.md
```

## Quick Start & Usage

### 1. Build and Run the Server
Ensure Docker is installed, then build and start the sandbox API (runs on `http://localhost:8080`):
```bash
make build
make run
```

### 2. Access the Interactive Web UI
In a new terminal, serve the frontend demo to interact with your running `goboxd` instance:
```bash
cd docs/demo
python3 -m http.server 8081
```
Open **[http://localhost:8081](http://localhost:8081)** in your browser to write and test code interactively!

### 3. Or Use the API Directly
Send a `POST` request to `/run` with your code:
```bash
curl -s -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{"language":"py3","source":"print(int(input())*2)","tests":[{"stdin":"5","expected_stdout":"10\n"}]}'
```

*For more detailed API commands and troubleshooting, see the [How to Use Guide](docs/how-to-use.md).*

## 🔗 Quick Links

- [📖 How to Use goboxd](docs/how-to-use.md)
- [🏗 Architecture Details](docs/architecture.md)
- [🔌 Supported Languages](docs/languages.md)
- [🔒 Security & Threat Model](docs/security.md)

## 🤝 Contributing
Contributions are welcome! Please open an issue to discuss significant changes before submitting a pull request. Run `make test` and `make lint` locally before pushing.

## 📄 License
This project is licensed under the [GNU General Public License v3.0](LICENSE).
