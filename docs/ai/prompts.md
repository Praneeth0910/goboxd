# Record of prompts
This page is a record of prompts used in the development of the features in this project. It serves as a reference for future improvements and adjustments to the AI's behavior.

## 23-05-26 - Designing the Project Architecture
**Prompt**: "Design a high-level architecture for a secure code execution platform that can run untrusted code in a sandboxed environment. The platform should support multiple programming languages and provide an API for users to submit code and retrieve results. Consider using nsjail for sandboxing and Go for the API server. Provide a diagram of the architecture and explain the components involved." 
**Response Summary**: The architecture for the secure code execution platform consists of the following components:
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
**What we used / didnt used**: 
**Used**:
- Multi-stage Docker builds (3 stages for optimized image)
- nsjail as git submodule (pinned to tag 3.4)
- Go standard library (net/http) for HTTP routing
- YAML config loader with validation
- Input validation layer (path traversal, flag allowlist)
- Resource limits via cgroups
- Atomic metrics counters (thread-safe)
- Structured JSON logging middleware
- docker-compose for local development
- Makefile for build/test/deploy workflows

**Not Used** (designed but deferred):
- Redis job queue (in-memory only for MVP)
- WebSocket streaming output
- Advanced authentication (API keys, OAuth2)
- Prometheus metrics export
- Distributed execution
- Job history persistence
- Request rate limiting