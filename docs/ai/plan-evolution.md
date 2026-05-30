# The changes of plan wrt the time

## 24-05-26 - Project Architecture & Build Strategy

**What I thought I'd do:**
Simple flat structure with single main.go handling all concerns (HTTP, validation, execution, cleanup). Basic Docker build with everything in one stage. Use chi web framework for routing.

**What I actually did:**
Created 8-package layered architecture (config, handler, runner, sandbox, validate, status, stats, middleware) with clear separation of concerns. Implemented 3-stage Docker builds (nsjail-builder → go-builder → toolchains). Stuck with Go standard library (net/http) instead of chi.

**Why it changed:**
Realized early that monolithic structure would become unmaintainable as features scale. Multi-stage builds needed because nsjail submodule requires compilation with heavy build tools (protobuf, libnl-dev, gcc). Standard library proved sufficient for my needs—no chi overhead required. Clear package boundaries enable independent testing and future feature additions (Redis queue, metrics, auth) without refactoring.

## 25-05-26 - Security Validation Logic

**What I thought I'd do:**
Basic input validation with simple string checks for path traversal and flag injection. Minimal test coverage with a few unit tests for common cases.

**What I actually did:**
Implemented validation logic for filename and compiler flags, including edge cases and glob pattern support. Developed a test suite with 13+ tests covering various scenarios for both filename and flag validation.

**Why it changed:**
Security is a top priority for this project, and initial validation logic was too simplistic to cover all attack vectors. As I fleshed out the requirements, it became clear that I needed a better validation system to prevent common vulnerabilities. The test suite was expanded to ensure that all edge cases were covered and that the validation logic was vetted against potential attacks. This iterative process of refining the validation logic and expanding test coverage was essential to building a secure and reliable system.

## 26-05-26 - nsjail Execution Model (Pathing & Chroot)

**What I thought I'd do:**
I originally assumed I could build a single `buildNsjailArgs` function that would wrap my host absolute paths (e.g., `/tmp/goboxd-123/solution.py`) with `--chroot jailDir` and everything would run perfectly inside the jail.

**What I actually did:**
I split the nsjail execution into two distinct phases: `phaseBuild` and `phaseRun`. 
- For compilation (`phaseBuild`), I use the host root (`--chroot /`), explicitly bind-mount the jail directory as writable, and inject the `PATH` variable. 
- For execution (`phaseRun`), I use `--chroot jailDir` for maximum isolation and pass *jail-relative* absolute paths (e.g., `/solution.py`) to the runtime.

**Why it changed:**
The initial model completely misunderstood how `chroot` interacts with absolute paths. Passing an absolute host path into a chrooted environment causes the kernel to search for that path *relative to the chroot root*, which inevitably failed. Furthermore, compilers are complex beasts that invoke internal binaries (like `ld`) and require a writable output directory and a populated `PATH`. A one-size-fits-all `--chroot jailDir` model stripped the environment so severely that basic tools broke, forcing me to build a more nuanced, phase-aware execution engine.

## 30-05-26 - Security Hardening: From "Namespace-Only" to Defense-in-Depth

**What I thought I'd do:**
Rely on nsjail's namespace isolation as the sole security boundary. Namespaces provide PID, mount, UTS, IPC, and network separation, which seemed sufficient. Memory limits were enforced via `--rlimit_as` only, with no cgroup integration. The `memory_peak_kb` API field was a placeholder returning `0`. No syscall filtering was in place.

**What I actually did:**
Implemented a full defense-in-depth stack across 8 changes:
1. Added a Kafel seccomp BPF policy blocking 28 dangerous syscalls (`ptrace`, `bpf`, `mount`, etc.) — applied to both build and run phases.
2. Created per-request cgroupv2 slices with `--cgroup_mem_parent` for precise `memory.peak` tracking from the kernel.
3. Disabled swap (`--cgroup_mem_swap_max 0`) so OOM kills are immediate, not deferred to swap.
4. Added strict rlimits: `--rlimit_core 0` (no core dumps), `--rlimit_stack 8` (8 MB stack cap), `--rlimit_fsize 100` (100 MB file write cap).
5. Switched from inheriting the host environment to an explicit allowlist: only `HOME`, `TMPDIR`, `PATH`, and (for build phase) `GOCACHE`.
6. Added Dockerfile smoke tests that verify every language toolchain at image build time.
7. Refactored memory-exceeded detection from fragile exit-code-137 checks to multi-pattern nsjail log parsing.
8. Fixed three categories of CI lint failures introduced by the changes.

**Why it changed:**
A competitive analysis against a reference implementation ("Alpha") revealed that namespace isolation alone left significant gaps. Alpha's seccomp policy was its biggest differentiator (15% of the judging rubric). The `memory_peak_kb: 0` placeholder was a visible zero-score item. The environment inheritance was a real information-disclosure risk. Closing these gaps required moving from a single-layer isolation model to a multi-layer one: namespaces + seccomp + cgroups + rlimits + env allowlist.

## 29-05-26 - Documentation Restructuring & Web UI Polish

**What I thought I'd do:**
Keep all documentation in the README and STAGE1-CHECKLIST/QUICK-REF files. The web UI was functional but rough — no instructions, starter code clobbered user edits.

**What I actually did:**
Deleted STAGE1-CHECKLIST.md, STAGE1-QUICK-REF.md, and SUBMISSION-GUIDE.md. Created dedicated docs: `getting-started.md` (429 lines, step-by-step setup for beginners), `development.md` (650 lines, contributing, testing, CI/CD), and a PR_DESCRIPTION.md. Updated the web UI to cache user code per-language, auto-load starter templates only on first visit, and added an instructions popup modal that auto-shows for first-time visitors. Also fixed the JVM warning filter in `probe.go` to return clean version strings.

**Why it changed:**
The README had grown to 1,789 words by absorbing setup instructions, troubleshooting, and contribution guidelines. Moving these to dedicated docs made the README scannable and made each doc independently useful. The web UI changes came from dogfooding — I kept losing my code when switching languages, and new users had no idea what the UI could do without the instructions popup.

## 30-05-26 - README: From Marketing Document to Technical Reference

**What I thought I'd do:**
Keep the existing 1,789-word README with emoji, step-by-step beginner walkthrough, troubleshooting FAQ, language table, project structure tree, and contributing guide.

**What I actually did:**
Rewrote it to ~500 words across five sections (Isolation model, Languages, API, Quick start, Documentation), then added a detailed Security section with tables. Moved all beginner content, troubleshooting, and contributing guidelines to `docs/`.

**Why it changed:**
The spec called for a short README. The existing one was a standalone tutorial that duplicated content already in `docs/getting-started.md`, `docs/how-to-use.md`, and `docs/api.md`. The rewrite leads with the isolation model (the project's core differentiator) rather than burying it below a 200-word quickstart. The security section was added separately at the user's request to ensure judges see the defense-in-depth story without needing to navigate to `docs/security.md`.
