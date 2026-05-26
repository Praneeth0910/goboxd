# Architecture Decision Records (ADRs)
This document contains the Architecture Decision Records (ADRs) for the AI project. ADRs are a way to capture important architectural decisions made during the development of the project, along with their context and consequences.
## Choice of chi over net/http, gin and echo
 **Context**: The project requires a lightweight and efficient HTTP router that can handle high concurrency and provide good performance. I evaluated several popular Go web frameworks, including net/http, gin, echo, and chi. 
**Options considered**:
1. net/http: The standard library for HTTP in Go, which is simple and efficient but lacks some features provided by third-party frameworks.
2. gin: A popular web framework that offers a rich set of features, including middleware support, routing, and performance optimizations. However, it can be heavier than necessary for our use case.
3. echo: Another popular web framework that provides a similar feature set to gin, but with a different design philosophy. It is also heavier than necessary for our use case.
4. chi: A lightweight and modular HTTP router that focuses on simplicity and performance. It provides a minimalistic API and allows for easy composition of middleware, making it a good fit for our project.
**Decision**: I chose chi as the HTTP router for the project due to its lightweight nature, modular design, and good performance. It provides the necessary features for our use case without the overhead of more feature-rich frameworks like gin and echo. Additionally, chi's middleware composition allows for greater flexibility in handling requests and responses, which is beneficial for our project's requirements.
**Consequences**: I need to implement some features ourselves that are provided out-of-the-box by gin and echo, such as request validation and error handling. Overall, the decision to use chi aligns well with my project's goals of simplicity and efficiency while still providing the necessary functionality for our HTTP routing needs.

---

## Multi-Stage Docker Builds for Image Size and Reproducibility

**Context**: Need efficient Docker builds that minimize final image size while keeping all build tools and compiled binaries reproducible across environments.

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
Compiled languages (like C++) require a build step to produce an executable, while interpreted languages (like Python) only require a run step. nsjail provides isolation via `--chroot`. Initially, we used `--chroot jailDir` for both build and run phases, and expanded source file placeholders to absolute host paths (e.g., `/tmp/goboxd-1-xxx/solution.py`). This failed completely:
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
2. Install v1.x binary: Keep existing config, maintain stability across team environments
3. Add `version: "2"` flag to config: Quick patch but risks subtle v2-specific behavior differences

**Decision**: Install golangci-lint v1.64.1 (stable) via official binary installer. The config file stays as-is (v1 format), ensuring consistent linting behavior across local and CI environments.

**Consequences**:
- Config remains unchanged and stable
- All linter settings work as documented
- No surprises from v2 format migrations
- Installation via binary (not snap) avoids confinement overhead
- Team members must use v1.x; document in CONTRIBUTING guide if needed
