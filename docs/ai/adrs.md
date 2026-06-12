# Architecture Decision Records (ADRs)
This document contains the Architecture Decision Records (ADRs) for the AI project. ADRs are a way to capture important architectural decisions made during the development of the project, along with their context and consequences.

## Choice of chi over net/http, gin and echo
 **Context**: The project requires a lightweight and efficient HTTP router that can handle high concurrency and provide good performance. I evaluated several popular Go web frameworks, including net/http, gin, echo, and chi. 
**Options considered**:
1. net/http: The standard library for HTTP in Go, which is simple and efficient but lacks some features provided by third-party frameworks.
2. gin: A popular web framework that offers a rich set of features, including middleware support, routing, and performance optimizations. However, it can be heavier than necessary for my use case.
3. echo: Another popular web framework that provides a similar feature set to gin, but with a different design philosophy. It is also heavier than necessary for my use case.
4. chi: A lightweight and modular HTTP router that focuses on simplicity and performance. It provides a minimalistic API and allows for easy composition of middleware, making it a good fit for this project.
**Decision**: I chose chi as the HTTP router for the project due to its lightweight nature, modular design, and good performance. It provides the necessary features for my use case without the overhead of more feature-rich frameworks like gin and echo. Additionally, chi's middleware composition allows for greater flexibility in handling requests and responses, which fits the project's requirements.
**Consequences**: I need to implement some features myself that are provided out-of-the-box by gin and echo, such as request validation and error handling. Overall, the decision to use chi aligns well with my project's goals of simplicity and efficiency while still providing the necessary functionality for my HTTP routing needs.

---

## Multi-Stage Docker Builds for Image Size and Reproducibility

**Context**: I needed efficient Docker builds that minimize final image size while keeping all build tools and compiled binaries reproducible across environments.

**Options considered**:
1. Single-stage build: All tools in final image (~2GB bloat)
2. External builds: Run builds outside Docker, COPY binaries (version mismatch risk)
3. Multi-stage builds: Separate stages for nsjail → Go app → final runtime

**Decision**: Use 3-stage builds: nsjail-builder (compile from source) → go-builder (compile Go) → toolchains (runtime only).

**Consequences**: 
-  Smaller final image (~500MB vs 2GB)
-  Reproducible: nsjail pinned via git submodule tag 3.4
-  Fast iteration: Docker layer caching across stages
-  Longer first build (~5-10 min with full compilation)

---

## Per-Request Sandbox Directory Isolation with Atomic Counter Naming

**Context**: Each POST /run request needs an isolated temporary directory (jail dir) for the nsjail sandbox. Under concurrent load, directory naming must be guaranteed unique to prevent races, collisions, and potential security issues. The Python reference implementation uses a UID range approach which suffers from collisions under high concurrency.

**Options considered**:
1. UUID-based naming: `/tmp/goboxd-{uuid}` — cryptographically unique but doesn't prevent reuse
2. UID range (30k-wide, retry 3x): `uid_{random}` — fast but prone to collisions under load
3. Atomic counter + PID + random: `goboxd-{pid}-{counter}-{hex}` — guaranteed unique per process, scalable
4. Timestamp-based: `goboxd-{timestamp}-{random}` — can collide under nanosecond accuracy limits

**Decision**: Use atomic counter combined with PID and 6-character random hex: `goboxd-{PID}-{counter}-{random}`. The atomically-incremented counter guarantees no two directories share the same name across all goroutines in the same process.

**Consequences**:
- **Security (hole #5):** UID collisions impossible even under 1000+ concurrent requests
- **Cleanup (hole #7):** mtime-based orphan sweeper can safely cleanup old `goboxd-*` directories without collision risk
- **Performance:** O(1) naming with no retry loops or filesystem checks
- **Tradeoff:** Directory names are less human-readable than UUIDs, but clarity is not a security requirement
- **Lifecycle:** Each request immediately defers cleanup via `defer cleanup()`, ensuring even panics trigger RemoveAll

---

## Separate Chroot Strategies for Build vs Run Phases in nsjail

**Context**: 
Compiled languages (like C++) require a build step to produce an executable, while interpreted languages (like Python) only require a run step. nsjail provides isolation via `--chroot`. Initially, I used `--chroot jailDir` for both build and run phases, and expanded source file placeholders to absolute host paths (e.g., `/tmp/goboxd-1-xxx/solution.py`). This failed completely:
1. When `--chroot jailDir` is used, the root of the filesystem *inside* the jail is `jailDir`. Passing an absolute host path like `/tmp/...` makes nsjail look for `/tmp/...` *inside* the jail, which doesn't exist, leading to `runtime_error` (`No such file or directory`).
2. Compilers (like `g++`) need a writable output directory to place artifacts, but nsjail mounts `--chroot` as read-only by default.
3. Compilers invoke other binaries (like `collect2` invoking `ld`) and require a valid `PATH` environment variable, which nsjail strips by default.

**Options considered**:
1. Mount the entire host filesystem (`--chroot /`) for both phases: Compromises security for the run phase.
2. Bind mount the host `/tmp` directory into the jail: Leaks host state and allows cross-container access.
3. Use two different nsjail configurations based on the phase (Build vs. Run).

**Decision**: 
Implemented Phase-Specific nsjail Arguments in `runner.go` (`buildNsjailBuildArgs` and `buildNsjailRunArgs`):
- **Build Phase**: Uses `--chroot /` (host root) with `--cwd jailDir` and a writable `--bindmount jailDir:jailDir`. This allows compilers to use absolute host paths and write artifacts correctly.
- **Run Phase**: Uses `--chroot jailDir` (maximum isolation) with `--cwd /`. Path placeholders expand to jail-relative absolute paths (e.g., `/solution.py`, `/solution`).
- Both phases inject `--env PATH=...` to ensure toolchains can find internal binaries like `ld`.

**Consequences**: 
- All integration tests pass, including C++ compilation and Python execution.
- Security boundary is maintained: user code executes completely restricted within `jailDir`.
- The runner layer now explicitly understands `phaseBuild` vs `phaseRun` isolation requirements, avoiding conflating compiler needs with untrusted-code restrictions.

---

## golangci-lint v1.x for Stable Linter Configuration

**Context**: Static analysis tools need a stable configuration format. `.golangci.yml` was written for golangci-lint v1.x (the long-term stable version used by most Go projects). When v2 was released, the configuration format changed and is not backward-compatible by default.

**Options considered**:
1. Update `.golangci.yml` to v2 format: Learn new schema, rewrite all linter settings, test compatibility
2. Install v1.x binary: Keep existing config, maintain stability across environments
3. Add `version: "2"` flag to config: Quick patch but risks subtle v2-specific behavior differences

**Decision**: Install golangci-lint v1.64.1 (stable) via official binary installer. The config file stays as-is (v1 format), ensuring consistent linting behavior across local and CI environments.

**Consequences**:
- Config remains unchanged and stable
- All linter settings work as documented
- No surprises from v2 format migrations
- Installation via binary (not snap) avoids confinement overhead
- Anyone building this must use v1.x

---

## Seccomp Kafel Policy for Syscall Filtering

**Context**: nsjail provides namespace-based isolation (PID, mount, network), but the kernel still exposes hundreds of syscalls to processes inside the jail. Advanced sandbox escapes have historically exploited syscalls like `ptrace`, `bpf`, `mount`, and `unshare` to break out of containers. A reference implementation ("Alpha") used a Kafel seccomp policy as its primary differentiator.

**Options considered**:
1. No seccomp filtering: Rely solely on namespaces and rlimits (current state)
2. Seccomp allowlist: Enumerate the ~50 syscalls user code actually needs; deny everything else
3. Seccomp denylist: Block only the known-dangerous syscalls; allow everything else

**Decision**: Use a Kafel denylist policy (`KILL_PROCESS`) blocking 28 specific dangerous syscalls. The policy is defined as a Go string constant `seccompPolicy` and injected via `--seccomp_string` to both build and run phases.

**Consequences**:
- Kernel-level defense-in-depth: even if namespace isolation is bypassed, the blocked syscalls prevent privilege escalation
- `KILL_PROCESS` (not `KILL`) ensures multi-threaded programs can't race past the filter
- Denylist over allowlist chosen because compilers (Java, Kotlin) use unpredictable syscall sets that are hard to enumerate
- Adding new blocked syscalls is a single line change in the constant
- Testing requires a Docker environment with nsjail; unit tests skip seccomp verification

---

## Per-Request cgroupv2 Slices for Memory Tracking

**Context**: The API response included a `memory_peak_kb` field, but it was hardcoded to `0`. Competing implementations read `memory.peak` from a cgroupv2 hierarchy to report actual peak memory from the kernel, giving judges precise memory usage data.

**Options considered**:
1. Parse nsjail log output for memory statistics (fragile, format varies by version)
2. Use `/proc/{pid}/status` VmPeak field (only available while process is alive; race with cleanup)
3. Create a per-request cgroupv2 directory, let nsjail manage it via `--cgroup_mem_parent`, read `memory.peak` after exit

**Decision**: Option 3. Before each `runCommand` call, create a directory under `/sys/fs/cgroup/` named `goboxd-{nanosecond-timestamp}`. Pass it to nsjail via `--cgroup_mem_parent`, `--cgroup_mem_swap_max 0`, `--detect_cgroupv2`, and `--cgroupv2_mount /sys/fs/cgroup`. After the process exits, read `memory.peak` from the cgroup directory, parse the byte count, convert to KiB, and populate `CommandResult.MemoryPeakKB`. Clean up the cgroup directory in a deferred function.

**Consequences**:
- `memory_peak_kb` is now sourced from the kernel, not estimated
- Swap is disabled (`--cgroup_mem_swap_max 0`), ensuring OOM kills happen immediately rather than silently swapping
- If cgroup creation fails (e.g., running without cgroupv2 or without privileges), the code falls back gracefully — `cgroupName` is empty, no cgroup flags are passed, and `memory_peak_kb` remains `0`
- The cgroup cleanup must handle nsjail-created child cgroups inside the parent before removing the parent directory

---

## Environment Allowlisting in nsjail

**Context**: By default, nsjail inherits the parent process's environment variables. In a Docker container, this can include `NSJAIL_PATH`, `LANGUAGES_CONFIG`, and any secrets injected via `docker-compose.yml` or Kubernetes. Leaking these to user code is an information disclosure risk.

**Options considered**:
1. Trust Docker's isolation — assume env vars inside the container are safe to expose
2. Use `--env` flags to explicitly set only the variables user code needs

**Decision**: Option 2. Both `buildNsjailRunArgs` and `buildNsjailBuildArgs` now explicitly pass only `--env HOME=/`, `--env TMP=/tmp`, `--env TMPDIR=/tmp`, and `--env PATH=...`. No other environment variables reach user code.

**Consequences**:
- Host secrets, config paths, and internal env vars are never visible to sandboxed processes
- Languages that depend on specific env vars (e.g., `JAVA_HOME`) would need explicit additions — currently Java works without it because the JDK is on `PATH`
- Build phase additionally gets `--env GOCACHE=/tmp` for Go compilation

---

## ADR-002: Per-language install scripts instead of monolithic Dockerfile RUN

**Date:** 2026-06-09
**Status:** Accepted

**Context:** Demo-day requires adding a language in 30 minutes with no Dockerfile change.
A monolithic `apt-get install` line requires Dockerfile edits for every new language.
The judging spec scores the plug-and-play language model at 25% of Stage 2.

**Options considered:**
1. Monolithic `RUN apt-get install` line — simple but requires Dockerfile change per language
2. Per-language scripts iterated at build time — adding a language = one .sh file + one YAML block
3. Downloading language binaries at runtime — fragile, network-dependent, breaks immutability

**Decision:** Use `scripts/lang_install/<id>.sh` scripts iterated at build time:
```dockerfile
COPY scripts/lang_install/ /tmp/lang_install/
RUN apt-get update \
    && for f in /tmp/lang_install/*.sh; do echo "== $f =="; bash "$f" || exit 1; done \
    && rm -rf /var/lib/apt/lists/* /tmp/lang_install
```

**Consequences:**
- Adding a language = one YAML block + one shell script. Zero Dockerfile change.
- The `|| exit 1` ensures Docker fails loudly if any script fails
- Scripts are alphabetically ordered by filename, so install order is deterministic
- The smoke-test `RUN` block at the end of the Dockerfile catches missing toolchains

---

## ADR-003: Separate MaxBodyBytes from MaxSourceBytes

**Date:** 2026-06-09
**Status:** Accepted

**Context:** Using `MaxSourceBytes` (256 KiB) as the `http.MaxBytesReader` body cap made
`source_too_large` unreachable. A 300 KiB source field inside a 301 KiB JSON body would
hit `MaxBytesReader` during `json.Decode()` and return `invalid_json`, not `source_too_large`.

**Options considered:**
1. Keep single limit, document `source_too_large` as alias for `invalid_json` — incorrect spec
2. Separate limits: `MaxBodyBytes` = 4 MiB (body cap), `MaxSourceBytes` = 256 KiB (field check)

**Decision:** Option 2. `MaxBodyBytes` = 4 MiB governs `http.MaxBytesReader`. After successful
JSON decode, `len(req.Source) > MaxSourceBytes` triggers `source_too_large`. Two distinct
limits, two distinct error codes, exactly as the spec requires.

**Consequences:**
- `source_too_large` is now reachable for the first time
- `invalid_json` still fires when the entire body exceeds 4 MiB
- `MaxBodyBytes` defaults to `4 * 1024 * 1024` in code and is also explicit in `languages.yaml`
- Validation boundary tests can now distinguish the two error codes

---

## ADR-004: Whole-string TrimSpace for output_whitespace_mismatch

**Date:** 2026-06-09
**Status:** Accepted

**Context:** The previous `CompareOutput` used `strings.Fields` + `strings.Join` to normalise
whitespace, then compared. This collapsed internal whitespace runs, so `"hello  world"` and
`"hello world"` were treated as equal under `output_whitespace_mismatch`. The spec says only
leading/trailing whitespace is trimmed for this status; internal differences are `wrong_output`.

**Options considered:**
1. Keep `strings.Fields` normalisation — fails spec for internal whitespace differences
2. Use `strings.TrimSpace` on the whole string — spec-correct, minimal change

**Decision:** Option 2. `CompareOutput` now uses `strings.TrimSpace`:
```go
func CompareOutput(actual, expected string) string {
    if actual == expected {
        return StatusAccepted
    }
    if strings.TrimSpace(actual) == strings.TrimSpace(expected) {
        return StatusOutputWhitespaceMismatch
    }
    return StatusWrongOutput
}
```

**Consequences:**
- `"hello  world\n"` vs `"hello world\n"` now correctly returns `wrong_output`
- `"hello\n"` vs `"hello"` (trailing newline vs none) still returns `output_whitespace_mismatch`
- `"  hello  \n"` vs `"hello"` still returns `output_whitespace_mismatch`
- Inline `TrimRight` comparison in `runTestCase` was removed; all paths go through `CompareOutput`
- `normalizeWhitespace` helper deleted (no longer used anywhere)

---

## ADR-005: Host cgroup Namespace for Docker Compose

**Date:** 2026-06-12
**Status:** Accepted

**Context:** On newer Linux distributions (like Ubuntu 22.04+) using cgroupv2, Docker defaults to private cgroup namespaces (`cgroupns: private`). When goboxd runs in a Docker container and tries to spawn nsjail sandboxes with per-request memory tracking (`--cgroup_mem_parent`), it fails if the container's view of the cgroup filesystem is isolated from the host. This caused memory limits to fail or be tracked incorrectly.

**Options considered:**
1. Let Docker manage cgroup namespaces (default) — causes cgroupv2 memory tracking to fail in nsjail.
2. Explicitly share the host's cgroup namespace (`cgroupns: host`).

**Decision:** Option 2. Added `cgroupns: host` to the `goboxd` service definition in `docker-compose.yml`.

**Consequences:**
- cgroupv2 memory limits and `memory.peak` tracking now work correctly on Ubuntu 22.04+.
- `docker-compose up` behaves identically to the `docker run --cgroupns=host` command.
