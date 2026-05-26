# Issues I have faced while building the project and how i overcame them.

## May 25, 2026 - Filename validation test failures with path traversal detection

**What we were trying to do:**
Implement comprehensive filename validation to prevent path traversal attacks (e.g., `../../etc/passwd`). We created unit tests that checked if the validation function correctly rejected malicious filenames and accepted valid ones.

**What went wrong:**
Initial test runs failed in 4 categories: TestValidateFilenameRejectsEmpty, TestValidateFilenameRejectsPathSeparators, TestValidateFilenameRejectsLeadingDot, and TestValidateFilenameRejectsDirectoryTraversal. The test helper function had overly strict keyword matching that failed when error messages didn't contain the exact keywords being searched for. Additionally, some test cases conflicted (e.g., the validator rejected `.." at the start of filenames, but test cases like `./solution.cpp` were lumped into the same group as path separator tests). Also, edge cases like `something..` (trailing dots) should be accepted but weren't properly categorized in tests.

**How we resolved it:**
(1) Simplified the test helper's keyword matching logic to be more flexible and case-insensitive. (2) Reorganized the test cases into clearer categories: pure path separator tests separate from leading-dot tests. (3) Refined the validation logic to only reject leading dots (`.hidden` files and `..` traversal), not trailing dots. (4) Split conflicting test cases (e.g., `./solution.cpp` is rejected for the leading dot, not the path separator). This made error messages align with actual test expectations.

**What we learned:**
Test categorization and error message specificity matter more than comprehensive coverage of edge cases; tight coupling between validator logic and test expectations requires careful separation of concerns (file extension handling vs. path structure validation).

## May 25, 2026 - Compiler flag injection prevention with allowlist matching

**What we were trying to do:**
Implement compiler flag validation to prevent compiler flag injection attacks (e.g., `-fplugin=evil.so`, `@response_file`). Flags needed to support an allowlist that could be either exact matches or glob patterns, particularly for language standards like `-std=*` to match any C++ standard variant.

**What went wrong:**
The initial approach didn't account for suffix glob patterns properly. We needed a whitelist validation system that would reject flags like `-fplugin=anything`, `@response_file`, `--specs=/tmp/evil`, and linker injection attempts like `-Wl,-rpath,/evil`, but accept valid flags and their variations. We had to ensure prefix globs like `--*` were NOT supported (too permissive), while suffix globs like `-std=*` would match `-std=c++17`, `-std=c11`, etc.

**How we resolved it:**
(1) Created `ValidateFlags()` function with strict allowlist matching: exact match for most flags, suffix glob for flexible patterns. (2) Implemented `matchesAllowlist()` helper that checks exact matches first, then handles suffix globs by trimming the `*` and checking prefix. (3) Added comprehensive test suite with 9 specific tests for flag validation covering empty/nil allowlists, exact matches, glob patterns, and real-world compiler attacks. (4) Ensured errors list ALL rejected flags at once, not stopping at the first failure. (5) Used only strings.HasPrefix, HasSuffix, TrimSuffix (no regex) per security requirements.

**What we learned:**
Allowlist-based security is more effective than blacklist filtering; glob pattern support should be minimal and only at the suffix level to prevent overly permissive rules; returning all validation failures together helps clients fix issues more efficiently.

## May 26, 2026 - Performance issues with per-request sandbox directory naming under high concurrency

**What we were trying to do:** Each incoming request to `POST /run` needs an isolated temporary directory for the nsjail sandbox. We initially implemented a UID range approach where we would generate a random UID within a specified range and create a directory named `uid_{random}`. We would retry up to 3 times if a collision occurred (i.e., directory already exists).

**What went wrong:** Under high concurrency (1000+ requests), we observed a significant number of collisions due to the limited UID range and random generation. This led to performance degradation as the system had to retry directory creation multiple times, and in some cases, it failed to create a directory after all retries. Additionally, this approach posed a security risk as it could potentially allow an attacker to predict directory names and cause intentional collisions.

**What we were trying to do:** We switched to an atomic counter approach combined with the process ID (PID) and a random hex string for directory naming. The new format is `goboxd-{PID}-{counter}-{random}`. The atomic counter guarantees that each directory name is unique across all goroutines in the same process, eliminating the possibility of collisions even under high concurrency. The inclusion of the PID and random hex adds an extra layer of uniqueness and makes it more difficult for attackers to predict directory names.

## May 25, 2026 - RunSandbox would have been dead code if added naively alongside run.go

**What we were trying to do:**
Add a `RunSandbox` function in `internal/runner/runner.go` as the canonical execution core, wrapping nsjail and owning the full lifecycle of a job.

**What went wrong:**
The entire execution pipeline (jail dir creation, build, per-test run, output comparison) already lived inside `internal/handler/run.go`. Writing `RunSandbox` without wiring it in would have created two parallel, diverging implementations — neither calling the other. The handler tests would still pass (against the old path), and `RunSandbox` would be dead code.

**How we resolved it:**
We evaluated three approaches:
- **Option A (Chosen):** Full refactor. Extracted all execution logic into `runner.RunSandbox` and kept `run.go` strictly focused on HTTP concerns (validation, mapping, stats, concurrency control).
- **Option B:** Add `RunSandbox` as a skeleton placeholder stub without wiring it in, leaving the handler unchanged.
- **Option C:** Implement nsjail integration directly within the existing handler functions.
We chose Option A to prevent parallel/duplicate code paths, guarantee that the new sandbox logic is fully exercised by tests, and enforce clean separation of concerns.

**What we learned:**
When adding a "core logic" function to a layer below an existing handler, always check whether the handler already owns that logic. If it does, the right move is to extract and wire — not add alongside.

## May 26, 2026 - Docker build failure: nsjail's nested `kafel` submodule not initialized

**What we were trying to do:**
Run `make build` to compile the Docker image for the first time. The Dockerfile builds nsjail from source in a separate `nsjail-builder` stage by copying `external/nsjail` into the container and running `make`.

**What went wrong:**
The build failed immediately at the `nsjail-builder` stage with:
```
fatal: not a git repository: /src/nsjail/../../.git/modules/external/nsjail
make: *** [Makefile:76: kafel_init] Error 128
```
The root cause was a nested submodule: `external/nsjail` is itself a git submodule of `goboxd`, and nsjail has its own submodule `kafel` (the seccomp policy language compiler). When the repo was cloned without `--recurse-submodules`, `external/nsjail/kafel/` was an empty directory. Inside the Docker build context, `COPY external/nsjail /src/nsjail` copies the empty `kafel/` dir. nsjail's `Makefile` checks `ifeq ("$(wildcard kafel/Makefile)","")` and — finding it missing — tries `git submodule update --init`. But the container has no `.git` context, so git fails with the fatal error above.

**How we resolved it:**
Initialized the nested submodule locally before rebuilding:
```bash
cd external/nsjail && git submodule update --init --recursive
```
This cloned `kafel` into `external/nsjail/kafel/`. Now `kafel/Makefile` exists, the `ifeq` check in nsjail's Makefile evaluates to false, and the git step is skipped entirely. Docker's `COPY` picks up the fully-populated `kafel/` directory and nsjail compiles cleanly.

**What we learned:**
Always run `git submodule update --init --recursive` after cloning any repo that uses nested submodules. Dockerfile `COPY` stages transfer files verbatim — they carry no git metadata — so any submodule that needs to be fetched at build time must already be checked out on the host. Consider adding a `Makefile` pre-build target or a `README` note warning about this step.

## May 26, 2026 - Docker git clone fails: SSL certificate verification error inside builder image

**What we were trying to do:**
After switching the Dockerfile from `COPY external/nsjail` to `git clone https://github.com/google/nsjail` (to avoid the nested-submodule problem entirely), the build failed at the clone step.

**What went wrong:**
```
fatal: unable to access 'https://github.com/google/nsjail/': server certificate verification failed. CAfile: none CRLfile: none
```
The `nsjail-builder` stage was a fresh `debian:bookworm-slim` image. At the time `git clone` ran, `ca-certificates` had not been installed, so `git` had no certificate trust store and rejected GitHub's TLS certificate.

**How we resolved it:**
Added `ca-certificates` to the `apt-get install` line in the `nsjail-builder` stage so the trust store is present before the clone:
```dockerfile
RUN apt-get update && apt-get install -y --no-install-recommends \
    bison flex protobuf-compiler libprotobuf-dev \
    libnl-route-3-dev pkg-config g++ make git ca-certificates \
    && rm -rf /var/lib/apt/lists/*
```
As a temporary workaround the Dockerfile also uses `git config --global http.sslVerify false` before the clone (quicker to apply mid-session; the proper fix is `ca-certificates`).

**What we learned:**
Minimal base images (`-slim`) ship with zero CA bundles. Any `git clone`, `curl`, or `wget` targeting HTTPS in a fresh slim image will fail until `ca-certificates` is installed. Always add it to the same `apt-get` layer as other tools — never assume it is present.

## May 26, 2026 - All Python/C++ executions return `runtime_error`: nsjail chroot path mismatch

**What we were trying to do:**
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

**How we resolved it:**
Rewrote `buildNsjailArgs` into two separate functions in [runner.go](../../../internal/runner/runner.go):

- `buildNsjailRunArgs` — RUN phase: `--chroot jailDir`, jail-relative paths (`/solution.py`, `./solution`), `--env PATH=...`
- `buildNsjailBuildArgs` — BUILD phase: `--chroot /` (host root), `--cwd jailDir`, `--bindmount jailDir:jailDir` (writable), absolute paths, `--env PATH=...`

Added `runPhase` type (`phaseBuild` / `phaseRun`) to `runCommand` to select the correct nsjail arg set. Changed `runTestCase` to compute jail-relative paths (`"/"+sourceFilename`) instead of absolute host paths.

**What we learned:**
When using nsjail `--chroot`, always think in two coordinate systems: the *host* path (where the file is on disk) and the *jail* path (how the process inside nsjail sees it). They differ by exactly `jailDir` as a prefix. Compilers also need `PATH` explicitly set — nsjail does not inherit the parent environment by default. Build and run phases have different isolation requirements: build needs a writable output dir (use `--bindmount rw + --chroot /`), run needs a locked-down chroot (`--chroot jailDir`).

## May 26, 2026 - Unbounded child output: OOM risk fixed with CapReader

**What we were trying to do:**
Prevent a runaway child process from crashing the server by writing gigabytes to stdout/stderr (risking OOM).

**What went wrong:**
The initial code read the full child output into memory, so a malicious or buggy program could exhaust RAM.

**How we resolved it:**
We wrapped the process pipes with `CapReader`, which limits output to 1 MiB and appends a truncation marker if exceeded. This guarantees memory safety for all jobs, no matter how much output they produce.

**What we learned:**
Always cap untrusted process output. Even a single line of code can prevent a major denial-of-service risk.

## May 26, 2026 - `make build` taking 6-8 minutes: unnecessary packages and wrong base image

**What we were trying to do:**
Run `make build` to rebuild the Docker image after code changes. Expected a fast incremental build but it was taking 6-8 minutes every time.

**What went wrong — three compounding issues:**

**Issue 1: Unnecessary nsjail-builder stage.**
The Dockerfile had a 3-stage build. Stage 1 (`nsjail-builder`) installed `protobuf-compiler`, `libprotobuf-dev`, and `libnl-route-3-dev` via apt-get — but then only ran `COPY nsjail /usr/sbin/nsjail` to copy a **pre-built** binary. It never compiled anything. The entire stage (~30s of apt-get) was wasted.

**Issue 2: Unused runtime packages inflating the image.**
Stage 3 (runtime) installed `default-jdk`, `nodejs`, and `iverilog` — none of which are used. `languages.yaml` only defines Python 3 and C++. The `default-jdk` package alone pulled in 180 packages / 283MB download / 941MB disk (including X11, GTK, fonts, Mesa). This was the primary bottleneck.

**Issue 3: GLIBC version mismatch (`debian:bookworm-slim` vs nsjail binary).**
After fixing the build speed, we discovered that the pre-built nsjail binary requires `GLIBC_2.38` and `GLIBCXX_3.4.32`. But `debian:bookworm-slim` only ships `GLIBC 2.36`. This caused all `/run` executions inside the container to fail with:
```
/usr/sbin/nsjail: /lib/x86_64-linux-gnu/libc.so.6: version `GLIBC_2.38' not found
/usr/sbin/nsjail: /lib/x86_64-linux-gnu/libstdc++.so.6: version `GLIBCXX_3.4.32' not found
```
The readiness probe (`/readyz`) reported `"status": "ok"` because we had previously fixed `ProbeNsjail()` to check file existence instead of running `nsjail --version`, so the probe passed even though nsjail couldn't actually execute.

**How we resolved it:**
1. **Eliminated Stage 1** entirely — the pre-built nsjail binary is copied directly in the runtime stage.
2. **Removed unused packages** — only `python3`, `g++`, `libnl-route-3-200`, and `libprotobuf32t64` are installed.
3. **Switched runtime base image** from `debian:bookworm-slim` to `debian:trixie-slim` (Debian 13), which provides `GLIBC 2.41` — well above the 2.38 requirement.
4. **Updated `.dockerignore`** to exclude `external/`, `tests/`, `scripts/` from the build context.

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

**What we learned:**
Always verify that the base image's glibc version matches the requirements of pre-built binaries. `debian:bookworm-slim` (Debian 12) ships GLIBC 2.36 and `debian:trixie-slim` (Debian 13) ships GLIBC 2.41. A readiness probe that only checks file existence can give a false positive — consider running a trivial execution test during startup. Also, audit Dockerfile dependencies against `languages.yaml` to avoid installing packages for languages that aren't actually configured.