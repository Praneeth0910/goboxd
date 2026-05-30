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
**What I used / didn't use**: 
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
**Judgment Call**: AI returned a list-format YAML; I restructured it to a map keyed by language ID 
to match config.go's map[string]Language. Also switched from net/http to chi after evaluating 
routing needs (see adrs.md).
## 26-05-26 - Add languages.yaml with python3 and cpp
**prompt**: "Write a languages.yaml file for goboxd with exactly two language entries for Stage 1:

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
+ one Dockerfile install line, zero Go code changes."

**Response Summary**: 
```yaml
# languages.yaml defines the supported programming languages for goboxd.
# To add a new language, simply add a new block with the required fields and update the Dockerfile to install necessary compilers/interpreters. No Go code changes needed.
- id: py3
  name: Python 3
  source_filename: solution.py
  run:
    command: /usr/bin/python3 {{source}}
    limits:
      wall_time_s: 9
      memory_kb: 102400
      max_processes: 100
- id: cpp
  name: C++
  source_filename: solution.cpp
  artifact: solution
  build:
    command: /usr/bin/g++ {{flags}} -o {{artifact}} {{source}}
    limits:
      wall_time_s: 3
      memory_kb: 1048576
      max_processes: 100
    flag_allowlist:
      - -O0
      - -O1
      - -O2
      - -O3
      - -Wall
      - -Wextra
      - -std=*
  run:
    command: ./{{artifact}}
    limits:
      wall_time_s: 3
      memory_kb: 524288
      max_processes: 64
```
**What I used / didn't use**:
**Used**:
- YAML format for language configuration
- Two language entries: Python 3 and C++
- Build and run command templates with {{source}}, {{artifact}}, and {{flags}} placeholders
- Resource limits for both build and run stages
- Flag allowlist with glob pattern support for C++ standards
**Not Used**:
- Additional languages (e.g., Java, Go, Rust)
- More complex build/run steps (e.g., multi-stage builds, dependency installation)
- Environment variable support in commands
- Per-language custom error messages or validation rules

## 26-05-26 - Debugging nsjail and Path Expansion
**Prompt**: "fix all the fialed tests one by one" (followed by iterative terminal debugging and execution context investigation).

**Response Summary**: The agent methodically diagnosed the reason behind the `runtime_error` in all integration tests involving execution (Python and C++). By isolating the execution environment and running manual `nsjail` processes with varying configurations inside the Docker container, the agent identified that `runner.go` was constructing absolute host paths (e.g., `/tmp/goboxd-123/solution.py`) but placing them into a jail whose root (`--chroot`) was set to that exact `/tmp/goboxd-123` directory. This caused file-not-found errors because the path inside the jail was just `/solution.py`. Furthermore, C++ compilation was failing because it lacked a writable artifact output directory and the necessary `PATH` environment variables to find `collect2` and `ld`. The agent separated the execution logic into `buildNsjailBuildArgs` (using host root) and `buildNsjailRunArgs` (using jail root) and correctly injected the `PATH` environment variable.

**What I used / didn't use**:
**Used**:
- Interactive debugging with `docker exec` to run manual `nsjail` tests and isolate root causes.
- Structuring `runner.go` to explicitly differentiate between a `phaseBuild` and `phaseRun` execution context.
- Use of `chroot=/` combined with `--bindmount` and `--cwd` for the compilation phase to allow standard compiler pathing while maintaining isolation.
- Passing `PATH` explicitly via `--env` to the `nsjail` arguments.
**Not Used**:
- Bypassing nsjail entirely (maintained strict sandbox requirements).
- Changing the overall architecture of how requests were submitted (the fix was entirely isolated to `runner.go`).

## 30-05-26 - Security Hardening
**Prompt**: " 1. Add Seccomp Kafel Policy (Security — 15%) [...] 2. Per-request cgroupv2 with memory_peak_kb reporting [...] 3. Enhanced memory_exceeded detection [...] 4. Fix CI lint [...] 5. --cgroup_mem_swap_max 0 [...] 6. Hardening (--rlimit_core, --rlimit_stack, --rlimit_fsize) [...] 7. Dockerfile smoke tests [...] 8. Environment variable whitelisting. Focus on closing the gap, and commit and push to git after every fix."

**Response Summary**: The agent created an implementation plan covering all 8 items, then executed them sequentially. The seccomp policy was defined as a `const seccompPolicy` Kafel string blocking 28 syscalls and injected via `--seccomp_string` to both build and run nsjail argument builders. cgroupv2 integration added per-request cgroup directory creation, nsjail flags (`--cgroup_mem_parent`, `--detect_cgroupv2`, `--cgroupv2_mount`), and post-execution reading of `memory.peak`. Memory detection was refactored from fragile exit-code checks to a multi-pattern parser. Rlimits, swap disable, env whitelisting, Dockerfile smoke tests, and lint fixes were batched into the same execution run. Each item was committed and pushed individually.

**What I used / didn't use**:
**Used**:
- Deep analysis of the codebase as the driving prompt to prioritize work by judging-impact score
- Sequential commit-per-fix strategy for clean git history and easy rollback
- Implementation plan artifact to get user approval before execution
- `go build && go vet` as a fast local gate before each push

**Not Used**:
- Seccomp allowlist approach (too fragile for diverse compilers)
- Sampling-based memory tracking (kernel `memory.peak` is authoritative)
- Bulk commit of all changes (individual commits preferred for traceability)

## 30-05-26 - README Rewrite
**Prompt**: "generate a better readme, keep it informative and to the point with our emojies or fancy stuff, highlight the important differentiating points naturally"

**Response Summary**: Rewrote README.md from 1,789 words (emoji-heavy, marketing-style) to ~500 words with five sections: Isolation model, Languages, API, Quick start, Documentation. Security differentiators (seccomp, cgroupv2, env allowlist) were presented as natural technical facts in the Isolation model section rather than as marketing bullet points. A follow-up prompt added a detailed Security section with tables for rlimits and input validation attack surfaces.

**What I used / didn't use**:
**Used**:
- Factual prose style over bullet-point marketing
- Working curl example with realistic response including `memory_peak_kb`
- Tables for structured information (rlimits, attack surfaces)

**Not Used**:
- Emoji or decorative badges beyond the standard license/Go/Docker shields
- Troubleshooting, FAQ, or contributing sections (moved to docs/)
- The word "robust"