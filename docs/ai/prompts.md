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

## 12-06-26 - Add OCaml, TypeScript, and Scala Support
**Prompt**: "Add support for three new languages to languages.yaml: OCaml, TypeScript, and Scala. Follow the existing build/run pattern (id, source_filename, artifact, build.cmd/args, run.cmd/args, limits). Write the corresponding installer scripts in scripts/lang_install/ (ocaml.sh, typescript.sh, scala.sh), add them to the Dockerfile apt-get/install chain with smoke tests, and write integration tests covering compile + run for each. Don't change any Go code — this should prove the registry-driven extensibility model."

**Response Summary**: Added three new entries to `languages.yaml`: `ocaml` (compiled via `ocamlopt`, source `solution.ml`), `typescript` (transpiled via the Node/TS toolchain, source `solution.ts`), and `scala` (compiled via `scalac` to `solution.jar`, run via `scala` with `-J-Xmx` flags). Each got a dedicated install script under `scripts/lang_install/` and a Dockerfile smoke-test line. Integration tests were added covering a basic compile-and-run round trip per language. No `internal/` Go code was touched — confirming the "one YAML block + one install script = new language" promise from the original architecture.

**What I used / didn't use**:
**Used**:
- Existing `build`/`run` cmd+args YAML shape (no new schema fields)
- Per-language JVM flag tuning for Scala (`-J-Xmx512m` build, `-J-Xmx256m` run) consistent with later memory-tuning work
- Dockerfile smoke tests (`ocamlopt --version`, `tsc --version`, `scala --version`) to fail fast on broken installs
- Integration tests mirroring the existing py3/cpp pattern

**Not Used**:
- Any change to `config.go`, `runner.go`, or the validation layer
- Shared base images per language family (kept install scripts independent for clarity)

## 12-06-26 - Resource Limit Optimization + Custom Node.js/Swift Installs + Load Test Tooling
**Prompt**: "Go through languages.yaml and tighten memory_kb/wall_time_s for every runtime based on realistic workloads — we're shipping 14 languages in a 2GB container and the defaults are too generous. Also replace the apt-get Node.js and Swift installs with custom version-pinned installations (apt's versions are stale and bloated). Then build out automated load-testing tooling: a vegeta-based script (load-test.sh) and a Python plotting script (plot.py) so we can run repeatable benchmarks and generate breaking-point graphs."

**Response Summary**: Reviewed and re-tuned per-language `memory_kb`/`wall_time_s` limits across all 14 entries in `languages.yaml`, trimming oversized defaults (e.g. JVM-family memory_kb) to fit safely within a 2 vCPU / 2GB container running up to `max_concurrent` jails simultaneously. Replaced the apt-based Node.js install with a version-pinned binary install script, and the apt-based Swift install with the official Swift toolchain tarball, both reducing image size and startup variance. Added `docs/loadtest/load-test.sh` (vegeta attack at stepped RPS rates against `/run`) and `docs/loadtest/plot.py` (parses vegeta JSON reports into latency and breaking-point charts).

**What I used / didn't use**:
**Used**:
- vegeta for rate-stepped load generation (industry-standard, JSON output)
- matplotlib-based `plot.py` for `latency.png` / `breaking-point.png` charts
- Version-pinned upstream installers for Node.js and Swift over apt packages
- Conservative per-language memory_kb sized for worst-case concurrent execution

**Not Used**:
- hey or wrk (vegeta's JSON report format was easier to plot programmatically)
- Per-language container variants (single image, single languages.yaml remains the source of truth)

## 12-06-26 - Load Test Run 1: Establish Baseline
**Prompt**: "Run a load test against MemoryHog.java on a container limited to 2 vCPUs / 2GB RAM with a 10s per-request timeout. Use vegeta to step through RPS rates and find the breaking point. Document the container limits, reproduction steps, breaking point, what fails first, and the degradation behaviour in docs/loadtest/run1/README.md."

**Response Summary**: Started `goboxd-loadtest` with `--cpus=2 --memory=2g`, ran `load-test.sh` against `MemoryHog.java`, and plotted results. Found the breaking point at 5 RPS offered load against a sustainable throughput of ~0.58 RPS — each request costs ~3.4s (1.8s compile + 1.6s run with sleep) against `max_concurrent_jobs=2`. The first failure mode was `queue_timeout_s=30` triggering clean `429 queue_full` responses once the queue filled. No crashes, drops, or OOM — `nsjail`'s `-Xmx512m`/`memory_kb` limits held throughout.

**What I used / didn't use**:
**Used**:
- vegeta stepped-rate attacks (1 → 5 RPS) as the standard methodology for all subsequent runs
- `memory.peak` / nsjail memory limits as proof against JVM process pile-up
- Documented per-request cost breakdown (compile time + run time) to explain the throughput ceiling

**Not Used**:
- Any code changes — Run 1 is purely measurement, establishing the baseline for Runs 2-4

## 12-06-26 - Load Test Run 2: JVM Tuning + Concurrency Increase
**Prompt**: "Based on Run 1's findings, tune the JVM execution flags to reduce per-request compile/run cost — try -XX:TieredStopAtLevel=1, -XX:+UseSerialGC, -XX:CICompilerCount=1, -XX:-UsePerfData, and pre-allocate heap with -Xms256m. Also bump max_concurrent from 4 to 6. Re-run the same load test, vegeta TIMEOUT=40s, and document the throughput improvement vs Run 1 in docs/loadtest/run2/README.md."

**Response Summary**: Applied the JVM tuning flags to javac/java invocations and raised `max_concurrent` to 6. Re-ran the load test with `vegeta TIMEOUT=40s` against the same `MemoryHog.java`. Breaking point remained at 5 RPS offered, but sustained throughput rose from ~0.76 RPS (Run 1 baseline figure) to ~1.22 RPS — a **+60% improvement**. The first failure mode shifted to queue timeout: `queue_timeout_s=30` with `p50_ms≈30,003ms` confirmed requests sat in queue for the full 30s before a clean `429`. Degradation remained graceful — no crashes, no OOM.

**What I used / didn't use**:
**Used**:
- JIT-disabling JVM flags (`TieredStopAtLevel=1`, `CICompilerCount=1`) to cut compile-phase overhead for short-lived processes
- `SerialGC` over default GC (lower overhead for small, short-lived heaps)
- Increased `max_concurrent` (4→6) backed by Run 1's measured per-request cost
- Percentage-quantified comparison against Run 1 baseline

**Not Used**:
- Heap size reduction (deferred to Run 3 — Run 2 focused on GC/JIT flags only)
- Tightening `queue_timeout_s` (kept at 30s to isolate the effect of JVM tuning alone)

## 12-06-26 - Load Test Run 3: Push Concurrency to 8, Discover Timeout Misalignment
**Prompt**: "Push max_concurrent to 8, reduce -Xmx to 400m / -Xms to 200m and add -XX:MaxMetaspaceSize=32m to fit more concurrent JVMs in the 2GB box, and drop queue_timeout_s to 8. Align vegeta TIMEOUT to the strict 10s challenge spec this time instead of 40s. Run the load test and document what happens — including any unexpected status codes — in docs/loadtest/run3/README.md."

**Response Summary**: Applied the tighter memory settings and `max_concurrent=8`, set `queue_timeout_s=8`, and ran vegeta with `TIMEOUT=10s`. The breaking point hit immediately at 5 RPS with a mixed failure mode: status breakdown `0:38, 200:12, 429:100`. Diagnosed the root cause via arithmetic: `queue_timeout_s(8) + execution_time(~2.57s) = 10.57s`, which exceeds the 10s client timeout — so 38 requests grabbed a slot but were killed client-side mid-execution (silent `status 0`) instead of receiving a clean server response. Documented the fix formula for Run 4: `queue_timeout_s < client_timeout − execution_time` → `queue_timeout_s ≤ 7`.

**What I used / didn't use**:
**Used**:
- Strict alignment of vegeta TIMEOUT to the 10s challenge spec (exposing the bug Run 2's 40s timeout had masked)
- Status-code breakdown table (`0 / 200 / 429` counts) as the diagnostic tool
- Derived a general formula (`queue_timeout_s + execution_time < client_timeout`) rather than a one-off fix, so it generalizes to other languages/configs

**Not Used**:
- Reverting max_concurrent back to 6 (kept at 8 — the bug was a timeout misalignment, not a concurrency problem)
- Ignoring the status-0 results as "noise" — treated as a real degradation-quality issue worth fixing

## 12-06-26 - Load Test Run 4: Apply Fix, Validate 3x Improvement
**Prompt**: "Apply the Run 3 fix: set queue_timeout_s to 7 so the formula queue_timeout_s + execution_time < 10s holds. Keep everything else the same as Run 3 (max_concurrent=8, tuned -Xmx/-Xms/Metaspace, vegeta TIMEOUT=10s). Re-run the load test, compare against Run 3, and write up the final conclusion for the challenge in docs/loadtest/run4/README.md — this should be the definitive result."

**Response Summary**: Set `queue_timeout_s=7` (only change vs Run 3) and re-ran the load test on `goboxd-run4` (port 8083). Results: 30-43 successful requests per step, a **3x improvement** over Run 3's flat 12 successes per step, with near-zero `status 0` timeouts — the vast majority of overload requests now received clean `429 Too Many Requests` instead of silent client-side failures. Breaking point remains 5 RPS (throughput maxes ~1.1 RPS under full concurrent load due to CPU contention from parallel `javac`/JVM execution), but the degradation quality at that breaking point is now "pristine" — fully graceful. Wrote up the conclusion: Run 4 strictly obeys the 10s timeout and 2GB/2vCPU bounds while running 8 concurrent JVMs safely, and stands as the final load-test result for the challenge.

**What I used / didn't use**:
**Used**:
- Single-variable change from Run 3 (`queue_timeout_s: 8→7`) to cleanly isolate and prove the fix
- Before/after comparison table (Run 3 vs Run 4) as the core evidence
- `breaking-point.png` / `latency.png` visual artifacts to support the written conclusion
- Framed as the "definitive"/final result, closing the baseline→tune→break→diagnose→fix→validate arc across all 4 runs

**Not Used**:
- Further concurrency increases beyond 8 (5 RPS breaking point is CPU-bound, not queue-bound, at this point — diminishing returns)
- A Run 5 — Run 4 was deliberately scoped as the closing result for the challenge submission