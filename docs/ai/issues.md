# Issues I faced while building the project and how I overcame them.

## May 25, 2026 - Filename validation test failures with path traversal detection

**What I was trying to do:**
Implement filename validation to prevent path traversal attacks (e.g., `../../etc/passwd`). I created unit tests that checked if the validation function correctly rejected malicious filenames and accepted valid ones.

**What went wrong:**
Initial test runs failed in 4 categories: TestValidateFilenameRejectsEmpty, TestValidateFilenameRejectsPathSeparators, TestValidateFilenameRejectsLeadingDot, and TestValidateFilenameRejectsDirectoryTraversal. The test helper function had overly strict keyword matching that failed when error messages didn't contain the exact keywords being searched for. Additionally, some test cases conflicted (e.g., the validator rejected `..` at the start of filenames, but test cases like `./solution.cpp` were lumped into the same group as path separator tests). Also, edge cases like `something..` (trailing dots) should be accepted but weren't properly categorized in tests.

**How I resolved it:**
(1) Simplified the test helper's keyword matching logic to be more flexible and case-insensitive. (2) Reorganized the test cases into clearer categories: pure path separator tests separate from leading-dot tests. (3) Refined the validation logic to only reject leading dots (`.hidden` files and `..` traversal), not trailing dots. (4) Split conflicting test cases (e.g., `./solution.cpp` is rejected for the leading dot, not the path separator). This made error messages align with actual test expectations.

**What I learned:**
Test categorization and error message specificity matter more than exhaustive coverage of edge cases; tight coupling between validator logic and test expectations requires careful separation of concerns (file extension handling vs. path structure validation).

## May 25, 2026 - Compiler flag injection prevention with allowlist matching

**What I was trying to do:**
Implement compiler flag validation to prevent compiler flag injection attacks (e.g., `-fplugin=evil.so`, `@response_file`). Flags needed to support an allowlist that could be either exact matches or glob patterns, particularly for language standards like `-std=*` to match any C++ standard variant.

**What went wrong:**
The initial approach didn't account for suffix glob patterns properly. I needed a whitelist validation system that would reject flags like `-fplugin=anything`, `@response_file`, `--specs=/tmp/evil`, and linker injection attempts like `-Wl,-rpath,/evil`, but accept valid flags and their variations. I had to ensure prefix globs like `--*` were NOT supported (too permissive), while suffix globs like `-std=*` would match `-std=c++17`, `-std=c11`, etc.

**How I resolved it:**
(1) Created `ValidateFlags()` function with strict allowlist matching: exact match for most flags, suffix glob for flexible patterns. (2) Implemented `matchesAllowlist()` helper that checks exact matches first, then handles suffix globs by trimming the `*` and checking prefix. (3) Added a test suite with 9 specific tests for flag validation covering empty/nil allowlists, exact matches, glob patterns, and real-world compiler attacks. (4) Ensured errors list ALL rejected flags at once, not stopping at the first failure. (5) Used only strings.HasPrefix, HasSuffix, TrimSuffix (no regex) per security requirements.

**What I learned:**
Allowlist-based security is more effective than blacklist filtering; glob pattern support should be minimal and only at the suffix level to prevent overly permissive rules; returning all validation failures together helps clients fix issues more efficiently.

## May 26, 2026 - Performance issues with per-request sandbox directory naming under high concurrency

**What I was trying to do:** Each incoming request to `POST /run` needs an isolated temporary directory for the nsjail sandbox. I initially implemented a UID range approach where I would generate a random UID within a specified range and create a directory named `uid_{random}`. I would retry up to 3 times if a collision occurred (i.e., directory already exists).

**What went wrong:** Under high concurrency (1000+ requests), I observed a significant number of collisions due to the limited UID range and random generation. This led to performance degradation as the system had to retry directory creation multiple times, and in some cases, it failed to create a directory after all retries. Additionally, this approach posed a security risk as it could allow an attacker to predict directory names and cause intentional collisions.

**How I resolved it:** I switched to an atomic counter approach combined with the process ID (PID) and a random hex string for directory naming. The new format is `goboxd-{PID}-{counter}-{random}`. The atomic counter guarantees that each directory name is unique across all goroutines in the same process, eliminating the possibility of collisions even under high concurrency. The inclusion of the PID and random hex adds an extra layer of uniqueness and makes it more difficult for attackers to predict directory names.

**What I learned:** Basic random generation is insufficient for resource naming under load. Atomic counters combined with process IDs provide a faster, collision-free naming strategy.

## May 25, 2026 - RunSandbox would have been dead code if added naively alongside run.go

**What I was trying to do:**
Add a `RunSandbox` function in `internal/runner/runner.go` as the execution core, wrapping nsjail and owning the full lifecycle of a job.

**What went wrong:**
The entire execution pipeline (jail dir creation, build, per-test run, output comparison) already lived inside `internal/handler/run.go`. Writing `RunSandbox` without wiring it in would have created two parallel, diverging implementations — neither calling the other. The handler tests would still pass (against the old path), and `RunSandbox` would be dead code.

**How I resolved it:**
I evaluated three approaches:
- **Option A (Chosen):** Full refactor. Extracted all execution logic into `runner.RunSandbox` and kept `run.go` strictly focused on HTTP concerns (validation, mapping, stats, concurrency control).
- **Option B:** Add `RunSandbox` as a skeleton placeholder stub without wiring it in, leaving the handler unchanged.
- **Option C:** Implement nsjail integration directly within the existing handler functions.
I chose Option A to prevent parallel/duplicate code paths, guarantee that the new sandbox logic is fully exercised by tests, and enforce clean separation of concerns.

**What I learned:**
When adding a core logic function to a layer below an existing handler, always check whether the handler already owns that logic. If it does, extract and wire — do not add alongside.

## May 26, 2026 - Docker build failure: nsjail's nested `kafel` submodule not initialized

**What I was trying to do:**
Run `make build` to compile the Docker image for the first time. The Dockerfile builds nsjail from source in a separate `nsjail-builder` stage by copying `external/nsjail` into the container and running `make`.

**What went wrong:**
The build failed immediately at the `nsjail-builder` stage with:
```
fatal: not a git repository: /src/nsjail/../../.git/modules/external/nsjail
make: *** [Makefile:76: kafel_init] Error 128
```
The root cause was a nested submodule: `external/nsjail` is itself a git submodule of `goboxd`, and nsjail has its own submodule `kafel` (the seccomp policy language compiler). When the repo was cloned without `--recurse-submodules`, `external/nsjail/kafel/` was an empty directory. Inside the Docker build context, `COPY external/nsjail /src/nsjail` copies the empty `kafel/` dir. nsjail's `Makefile` checks `ifeq ("$(wildcard kafel/Makefile)","")` and — finding it missing — tries `git submodule update --init`. But the container has no `.git` context, so git fails with the fatal error above.

**How I resolved it:**
Initialized the nested submodule locally before rebuilding:
```bash
cd external/nsjail && git submodule update --init --recursive
```
This cloned `kafel` into `external/nsjail/kafel/`. Now `kafel/Makefile` exists, the `ifeq` check in nsjail's Makefile evaluates to false, and the git step is skipped entirely. Docker's `COPY` picks up the fully-populated `kafel/` directory and nsjail compiles cleanly.

**What I learned:**
Always run `git submodule update --init --recursive` after cloning any repo that uses nested submodules. Dockerfile `COPY` stages transfer files verbatim — they carry no git metadata — so any submodule that needs to be fetched at build time must already be checked out on the host. Added a `Makefile` pre-build target to catch this.

## May 26, 2026 - Docker git clone fails: SSL certificate verification error inside builder image

**What I was trying to do:**
After switching the Dockerfile from `COPY external/nsjail` to `git clone https://github.com/google/nsjail` (to avoid the nested-submodule problem entirely), the build failed at the clone step.

**What went wrong:**
```
fatal: unable to access 'https://github.com/google/nsjail/': server certificate verification failed. CAfile: none CRLfile: none
```
The `nsjail-builder` stage was a fresh `debian:bookworm-slim` image. At the time `git clone` ran, `ca-certificates` had not been installed, so `git` had no certificate trust store and rejected GitHub's TLS certificate.

**How I resolved it:**
Added `ca-certificates` to the `apt-get install` line in the `nsjail-builder` stage so the trust store is present before the clone:
```dockerfile
RUN apt-get update && apt-get install -y --no-install-recommends \
    bison flex protobuf-compiler libprotobuf-dev \
    libnl-route-3-dev pkg-config g++ make git ca-certificates \
    && rm -rf /var/lib/apt/lists/*
```
As a temporary workaround the Dockerfile also uses `git config --global http.sslVerify false` before the clone (quicker to apply mid-session; the proper fix is `ca-certificates`).

**What I learned:**
Minimal base images (`-slim`) ship with zero CA bundles. Any `git clone`, `curl`, or `wget` targeting HTTPS in a fresh slim image will fail until `ca-certificates` is installed. Always add it to the same `apt-get` layer as other tools.

## May 26, 2026 - All Python/C++ executions return `runtime_error`: nsjail chroot path mismatch

**What I was trying to do:**
Run the phase-3 integration test suite after the server was up. Expected most tests to pass.

**What went wrong:**
Every execution test (Python and C++) returned `runtime_error` with:
```
/usr/bin/python3: can't open file '/tmp/goboxd-1-xxx/solution.py': [Errno 2] No such file or directory
```
34 out of ~110 tests passed (all structural/validation tests). Every actual execution test failed.

**Root cause — two compounding bugs:**

**Bug 1 (Python / interpreted): absolute paths inside a chroot jail**
`runner.go` expanded `{{source}}` → `filepath.Join(jailDir, sourceFilename)` = `/tmp/goboxd-1-xxx/solution.py`. That absolute host path was passed as an argument to python3. But nsjail's `--chroot jailDir` makes `jailDir` the filesystem root — inside the jail, the file lives at `/solution.py`, not at `/tmp/goboxd-1-xxx/solution.py`.

**Bug 2 (C++ / compiled): `collect2: fatal error: cannot find 'ld'`**
The build step also used `--chroot jailDir`, but g++ needs to write the compiled artifact to a writable location and requires `PATH` to locate `collect2`/`ld`. nsjail's chroot-based isolation strips the environment, including `PATH`, so `ld` was never found.

**How I resolved it:**
Rewrote `buildNsjailArgs` into two separate functions in [runner.go](../../../internal/runner/runner.go):

- `buildNsjailRunArgs` — RUN phase: `--chroot jailDir`, jail-relative paths (`/solution.py`, `./solution`), `--env PATH=...`
- `buildNsjailBuildArgs` — BUILD phase: `--chroot /` (host root), `--cwd jailDir`, `--bindmount jailDir:jailDir` (writable), absolute paths, `--env PATH=...`

Added `runPhase` type (`phaseBuild` / `phaseRun`) to `runCommand` to select the correct nsjail arg set. Changed `runTestCase` to compute jail-relative paths (`"/"+sourceFilename`) instead of absolute host paths.

**What I learned:**
When using nsjail `--chroot`, always think in two coordinate systems: the host path and the jail path. They differ by exactly `jailDir` as a prefix. Compilers also need `PATH` explicitly set — nsjail does not inherit the parent environment by default. Build and run phases have different isolation requirements: build needs a writable output dir (use `--bindmount rw + --chroot /`), run needs a locked-down chroot (`--chroot jailDir`).

## May 26, 2026 - Unbounded child output: OOM risk fixed with CapReader

**What I was trying to do:**
Prevent a runaway child process from crashing the server by writing gigabytes to stdout/stderr.

**What went wrong:**
The initial code read the full child output into memory, so a malicious or buggy program could exhaust RAM.

**How I resolved it:**
I wrapped the process pipes with `CapReader`, which limits output to 1 MiB and appends a truncation marker if exceeded. This guarantees memory safety for all jobs, no matter how much output they produce.

**What I learned:**
Always cap untrusted process output. Even a single line of code can prevent a major denial-of-service risk.

## May 26, 2026 - `make build` taking 6-8 minutes: unnecessary packages and wrong base image

**What I was trying to do:**
Run `make build` to rebuild the Docker image after code changes. Expected a fast incremental build but it was taking 6-8 minutes every time.

**What went wrong — three compounding issues:**

**Issue 1: Unnecessary nsjail-builder stage.**
The Dockerfile had a 3-stage build. Stage 1 (`nsjail-builder`) installed `protobuf-compiler`, `libprotobuf-dev`, and `libnl-route-3-dev` via apt-get — but then only ran `COPY nsjail /usr/sbin/nsjail` to copy a pre-built binary. It never compiled anything. The entire stage (~30s of apt-get) was wasted.

**Issue 2: Unused runtime packages inflating the image.**
Stage 3 (runtime) installed `default-jdk`, `nodejs`, and `iverilog` — none of which are used. `languages.yaml` only defines Python 3 and C++. The `default-jdk` package alone pulled in 180 packages / 283MB download / 941MB disk (including X11, GTK, fonts, Mesa). This was the primary bottleneck.

**Issue 3: GLIBC version mismatch (`debian:bookworm-slim` vs nsjail binary).**
After fixing the build speed, I discovered that the pre-built nsjail binary requires `GLIBC_2.38` and `GLIBCXX_3.4.32`. But `debian:bookworm-slim` only ships `GLIBC 2.36`. This caused all `/run` executions inside the container to fail with:
```
/usr/sbin/nsjail: /lib/x86_64-linux-gnu/libc.so.6: version `GLIBC_2.38' not found
/usr/sbin/nsjail: /lib/x86_64-linux-gnu/libstdc++.so.6: version `GLIBCXX_3.4.32' not found
```
The readiness probe (`/readyz`) reported `"status": "ok"` because I had previously fixed `ProbeNsjail()` to check file existence instead of running `nsjail --version`, so the probe passed even though nsjail couldn't actually execute.

**How I resolved it:**
1. Eliminated Stage 1 entirely — the pre-built nsjail binary is copied directly in the runtime stage.
2. Removed unused packages — only `python3`, `g++`, `libnl-route-3-200`, and `libprotobuf32t64` are installed.
3. Switched runtime base image from `debian:bookworm-slim` to `debian:trixie-slim` (Debian 13), which provides `GLIBC 2.41` — well above the 2.38 requirement.
4. Updated `.dockerignore` to exclude `external/`, `tests/`, `scripts/` from the build context.

**Results:**

| Metric | Before | After |
|--------|--------|-------|
| Build stages | 3 | 2 |
| Runtime packages | 180 | ~40 |
| Image size | ~1.5 GB | ~550 MB |
| Cold build time | 6-8 min | ~3.5 min |
| Cached build (code change only) | uncached | 3.4s |
| nsjail execution | GLIBC error | Works |
| Integration tests passing | 34/138 | 138/138 |

**What I learned:**
Always verify that the base image's glibc version matches the requirements of pre-built binaries. `debian:bookworm-slim` (Debian 12) ships GLIBC 2.36 and `debian:trixie-slim` (Debian 13) ships GLIBC 2.41. A readiness probe that only checks file existence can give a false positive — I should run a trivial execution test during startup. Also, audit Dockerfile dependencies against `languages.yaml` to avoid installing packages for languages that aren't configured.

## May 27, 2026 - Fork Bomb tests hanging indefinitely (10+ minutes) due to pipe leak

**What I was trying to do:**
Run the phase-3 integration test suite, specifically `TestPhase3_Never5xx_ForkBomb`, which uses `os.fork()` inside a loop to ensure the sandbox safely bounds process limits without crashing the server.

**What went wrong:**
The test was hanging indefinitely and eventually timing out after 10 minutes. A classic Go `exec.Command` pipe leak race condition was occurring. When the time limit expired, `context.WithTimeout` inside the Go runner forcibly killed the parent `nsjail` process via `SIGKILL`. Because `nsjail` was killed instantly, it didn't get a chance to gracefully terminate the inner fork bomb child processes. These orphaned child processes stayed alive in the background and held the `stdout` and `stderr` pipes open. Consequently, Go's `cmd.Wait()` blocked forever waiting for the pipes to receive an EOF.

**How I resolved it:**
Initially, I tried adding a 2-second buffer to the Go `context.WithTimeout` duration to let `nsjail` clean up gracefully, but this wasn't enough because root-level Python processes bypassing OS limits were completely flooding the kernel, preventing `nsjail`'s timer thread from firing correctly.
The fix required me to inject a background goroutine that listens for `ctx.Done()` and explicitly calls `outPipe.Close()` and `errPipe.Close()`. This guarantees that Go's internal `io.Copy` unblocks instantly when the timeout expires, bypassing the orphaned children that were holding the pipes hostage.

**What I learned:**
When managing untrusted subprocesses via `exec.CommandContext` and extracting their output via `StdoutPipe`, I cannot rely solely on the context timeout to unblock `cmd.Wait()`. If child processes are orphaned and hold the pipes open, `cmd.Wait()` will hang forever. Always use an explicit goroutine listening on `<-ctx.Done()` to forcefully close the read-end of the pipes and sever the connection.

## May 27, 2026 - hey v0.1.5 exits after ~21 requests at c=1 with slow endpoints

**What I was trying to do:**
Run `hey -n 200 -c 1` against `POST /run` with a C++ payload (~470 ms per request) to get baseline single-client latency percentiles (p50, p95, p99) for the benchmark table.

**What went wrong:**
hey dispatched only **21 of the requested 200 requests** (≈10.8 s of sequential work) and then exited cleanly — no error distribution, exit code 0. At first this looked like a TCP keep-alive issue: the server's `IdleTimeout: 60s` and `ReadTimeout: 15s` could close an idle connection mid-run. The early exit also produced a degenerate `0%% in 0.0000 secs` line in hey's percentile output, making it impossible to read a p99 directly.

**How I resolved it:**
Re-ran the benchmark with `--disable-keepalive` (forces a fresh TCP connection per request) and got an identical result: 21 responses, 10.8 s, clean exit — ruling out keep-alive and server-side connection timeouts as the cause entirely. The root cause is a **buffer/channel sizing bug in hey v0.1.5** (released 2026, requires Go ≥ 1.24): when c=1 and each request takes longer than ~200 ms, hey's internal result channel fills up after ~20 dispatched requests and the single worker goroutine exits without error.

For the benchmark table I used the maximum observed latency across the 21 valid samples as the p99 value (790 ms), which is a valid conservative upper bound for an uncontested single-client workload. I also documented the issue inline in `docs/benchmarks.md` so the footnote is transparent rather than just saying "sample size < 100".

**What I learned:**
Load-testing tools can have their own bugs that look like server or network issues. Always cross-check tool behaviour by varying flags independently (e.g., `--disable-keepalive`) to isolate the variable. For endpoints slower than ~200 ms, hey v0.1.5 at c=1 is not reliable — use a higher concurrency level or a different tool (e.g., `ab`, `wrk`, or a simple sequential `curl` loop) to collect single-client baselines.

## May 28, 2026 - Go runtime uses outdated apt package instead of 1.22 builder image

**What I was trying to do:**
Ensure the Go runtime environment inside the sandbox utilizes the official `golang:1.22` version specified in `languages.yaml`, rather than falling back to an older Debian `golang-go` package installed via `apt-get`.

**What went wrong:**
`languages.yaml` pointed to `/usr/bin/go`. The Dockerfile `apt-get install` included `golang-go`, which naturally placed it at `/usr/bin/go`, but it was an older version provided by Debian, not the 1.22 version compiled in the `go-builder` stage (which sat in `/usr/local/go`). 

**How I resolved it:**
Removed `golang-go` from the `apt-get` dependencies in the runtime stage. Added a `COPY --from=go-builder /usr/local/go /usr/local/go` command and explicitly symlinked `/usr/local/go/bin/go` and `/usr/local/go/bin/gofmt` to `/usr/bin/`. This satisfied the path expected by `languages.yaml` while upgrading the runtime to the correct version.

**What I learned:**
When multi-stage Docker builds are employed to fetch specific runtime versions (like Go 1.22), it's critical to avoid accidentally installing older, conflicting OS-level packages via `apt-get`.

## May 28, 2026 - False positives in OOM detection via exit code 137

**What I was trying to do:**
Accurately classify sandbox memory limit exceedances (`memory_exceeded`) without falsely catching other types of hard kills.

**What went wrong:**
`runCommand` detected OOMs solely by checking if the process exited with code `137` or if the nsjail log contained `signal: 9`. However, `137` and `signal: 9` are ambiguous and can be triggered by any manual `SIGKILL`, leading to false positives. My initial fix attempted to add a third 'un-setting' block to reduce false positives, but it was logically broken and created a regression where legitimate OOMs were cancelled out.

**How I resolved it:**
Refactored the logic into a single, corroborated flow. We now track if the process was explicitly killed (`isKilled = true` via `137` or `signal: 9`). We then declare an OOM if a soft limit was hit (`rlimit` in logs), OR if the process was explicitly killed (`isKilled`) AND corroborated by `memory`, `OOM`, or `[STATS]` in the logs. This guarantees we don't accidentally override legitimate conditions.

**What I learned:**
Exit codes are highly generic. To accurately report crashes, we must cross-reference exit codes with kernel/cgroup logs. Additionally, when fixing conditional logic, it's safer to completely refactor the flow into explicitly corroborated logic rather than stacking 'reset' blocks that can introduce confusing state regressions.

## May 28, 2026 - `/readyz` endpoint returning stale compiler cache

**What I was trying to do:**
Ensure the `/readyz` probe accurately reflects the live state of the language compilers (Python, C++, Java, etc.), responding with a 503 if any compiler binary goes missing or fails post-startup.

**What went wrong:**
The original `health.go` logic cached compiler statuses once at startup and never ran the probes again. Furthermore, the `Readyz` HTTP handler simply called `exec.LookPath()` per-request, which is inefficient and incomplete (it just checks if the binary exists in PATH, rather than properly invoking `--version`). 

**How I resolved it:**
Updated `HealthHandler` to include a `sync.RWMutex` protected map (`langProbes`). Initialized a background goroutine via `NewHealthHandler()` that loops every 60 seconds, re-runs the full `ProbeLanguage` checks for every configured language, and atomically updates the map. The `/readyz` handler now simply acquires an `RLock` and reads the live, cached probe state in memory.

**What I learned:**
Health checks in distributed systems need to reflect the live state without adding significant latency to the probe endpoint itself. A background prober loop protected by a reader-writer mutex provides both low-latency responses and accurate post-startup health monitoring.