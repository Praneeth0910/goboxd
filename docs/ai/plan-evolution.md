# The changes of plan wrt the time

## 24-05-26 - Project Architecture & Build Strategy

**What we thought we'd do:**
Simple flat structure with single main.go handling all concerns (HTTP, validation, execution, cleanup). Basic Docker build with everything in one stage. Use chi web framework for routing.

**What we actually did:**
Created 8-package layered architecture (config, handler, runner, sandbox, validate, status, stats, middleware) with clear separation of concerns. Implemented 3-stage Docker builds (nsjail-builder → go-builder → toolchains). Stuck with Go standard library (net/http) instead of chi.

**Why it changed:**
Realized early that monolithic structure would become unmaintainable as features scale. Multi-stage builds needed because nsjail submodule requires compilation with heavy build tools (protobuf, libnl-dev, gcc). Standard library proved sufficient for our needs—no chi overhead required. Clear package boundaries enable independent testing and future feature additions (Redis queue, metrics, auth) without refactoring.

## 25-05-26 - Security Validation Logic

**What we thought we'd do:**
Basic input validation with simple string checks for path traversal and flag injection. Minimal test coverage with a few unit tests for common cases.

**What we actually did:**Implemented comprehensive validation logic for filename and compiler flags, including edge cases and glob pattern support. Developed a robust test suite with 13+ tests covering various scenarios for both filename and flag validation.

**Why it changed:**
Security is a top priority for this project, and initial validation logic was too simplistic to cover all attack vectors. As we fleshed out the requirements, it became clear that we needed a more robust validation system to prevent common vulnerabilities. The test suite was expanded to ensure that all edge cases were covered and that the validation logic was thoroughly vetted against potential attacks. This iterative process of refining the validation logic and expanding test coverage was essential to building a secure and reliable system.

## 26-05-26 - nsjail Execution Model (Pathing & Chroot)

**What we thought we'd do:**
We originally assumed we could build a single `buildNsjailArgs` function that would wrap our host absolute paths (e.g., `/tmp/goboxd-123/solution.py`) with `--chroot jailDir` and everything would run perfectly inside the jail.

**What we actually did:**
We split the nsjail execution into two distinct phases: `phaseBuild` and `phaseRun`. 
- For compilation (`phaseBuild`), we use the host root (`--chroot /`), explicitly bind-mount the jail directory as writable, and inject the `PATH` variable. 
- For execution (`phaseRun`), we use `--chroot jailDir` for maximum isolation and pass *jail-relative* absolute paths (e.g., `/solution.py`) to the runtime.

**Why it changed:**
The initial model completely misunderstood how `chroot` interacts with absolute paths. Passing an absolute host path into a chrooted environment causes the kernel to search for that path *relative to the chroot root*, which inevitably failed. Furthermore, compilers are complex beasts that invoke internal binaries (like `ld`) and require a writable output directory and a populated `PATH`. A one-size-fits-all `--chroot jailDir` model stripped the environment so severely that basic tools broke, forcing us to build a more nuanced, phase-aware execution engine.
