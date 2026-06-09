# Postmortem — Team Sudo, GoBoxD Phase 1

## What actually broke (in order of pain)

The first two days were mostly fighting the environment, not the problem. 
WSL kept disconnecting mid-session due to .bashrc printing output that 
broke the VS Code remote install script. Lost probably 3 hours to that 
before tracing it to shell startup noise corrupting the IDE's pipe.

The nsjail chroot coordinate system was the hardest bug. I assumed 
--chroot jailDir meant I could pass absolute host paths as arguments. 
Wrong — inside the jail, jailDir IS the root, so /tmp/goboxd-xxx/solution.py 
doesn't exist; /solution.py does. Every execution test returned runtime_error 
for two hours before I figured this out. The fix required splitting into 
two separate nsjail argument builders: one for build (chroot=/, writable 
bindmount) and one for run (chroot=jailDir, read-only). 

The fork bomb test hung for 10+ minutes for multiple times. I killed nsjail 
but orphaned child processes held the stdout/stderr pipes open, so cmd.Wait() 
blocked forever. Fixed by adding a goroutine that closes the pipes on ctx.Done().

## What surprised me

How much the base image matters. Switched from debian:bookworm-slim to 
debian:trixie-slim specifically for GLIBC 2.41 — the pre-built nsjail binary 
needs it and bookworm only ships 2.36. The readyz probe was returning "ok" 
the whole time because I was checking file existence, not actually running 
nsjail. Silent failures are worse than loud ones.

---

# Postmortem — Team Sudo, GoBoxD Phase 2 (Security Hardening)

## What actually broke (in order of pain)

The lint cycle was the biggest time sink. I pushed the seccomp + cgroup changes
after `go build && go vet` passed locally, assuming CI would be clean. It wasn't.
`golangci-lint` with gocritic enabled flagged three things `go vet` doesn't care
about: `paramTypeCombine` (consecutive same-type params), `errcheck` (discarded
`os.ReadDir` error), and `filepathJoin` (literal `/` in `filepath.Join` args).
The `filepathJoin` fix took three attempts — splitting `/sys/fs/cgroup` into
`"/", "sys", "fs", "cgroup"` still triggered it because `"/"` contains a separator.
The final fix was a `const cgroupBase = "/sys/fs/cgroup/"` with string concat.
Three commits for what should have been zero.

The cgroup scoping bug was more subtle. I declared `cgroupPath` inside the
`if nsjailAvailable()` block, but needed to read `memory.peak` from it after
`cmd.Wait()` — outside that block. The compiler caught it immediately, but it
was a design mistake: variables needed across the full function lifecycle should
be declared at function scope, not inside conditionals.

## What surprised me

How much of a differentiator the seccomp policy is. It's a string constant and
two `--seccomp_string` flags — maybe 40 lines of code total — but it blocks 28
syscalls at the kernel level and was worth 15% of the judging rubric. The ratio
of implementation effort to competitive impact was the best of all 8 items.

The `buildNsjailBuildArgs` cgroup insertion was more complex than expected.
I initially tried to insert cgroup flags at a specific position in the args
slice by iterating and looking for a sentinel value (`"--env"`, `"HOME=/"`).
This worked but was fragile. In hindsight, I should have just appended the
cgroup flags before the final `--chroot`/`--`/`cmd` block, which is always
the last thing added to the slice.

## What I'd do differently

Run `golangci-lint` locally before every push. The three-commit lint fix cycle
was entirely avoidable. I'd also declare all cross-scope variables (like
`cgroupPath`) at function scope from the start, not try to minimize their
visibility by scoping them inside conditionals.

For the README rewrite, I spent time on an intermediate version (~300 words,
too terse) before the user asked for a better one. Should have asked for
preferences upfront rather than defaulting to minimal.

---

# Postmortem — Team Sudo, GoBoxD Stage 2 Fixes

## What was fixed (and why it was broken)

**normalizeWhitespace was too aggressive.** The `CompareOutput` function used
`strings.Fields` to split on all whitespace, then rejoined with single spaces.
This meant `"hello  world"` and `"hello world"` matched as `output_whitespace_mismatch`
instead of `wrong_output`. The spec says only leading/trailing whitespace is trimmed
for that status — internal differences remain `wrong_output`. The fix was replacing
`normalizeWhitespace` entirely with `strings.TrimSpace`. Three lines of code, one
function deleted, spec-correct.

The parallel issue was in `runTestCase`: there was an inline comparison using
`strings.TrimRight(res.Stdout, "\r\n")` that bypassed `CompareOutput` entirely.
Two codepaths doing the same thing differently — a classic maintainability bug.
Fixed by removing the inline cases and routing all output comparison through
`status.CompareOutput`.

**source_too_large was unreachable.** The handler used `MaxSourceBytes` (256 KiB)
as the `http.MaxBytesReader` body limit. A request with a 300 KiB source field would
hit `MaxBytesReader` during `json.Decode()` and surface as `invalid_json` (read error),
not `source_too_large`. The fix required separating the limits: `MaxBodyBytes` = 4 MiB
caps the entire HTTP body; after JSON decode, `len(req.Source) > MaxSourceBytes` returns
`source_too_large`. Simple in hindsight, easy to miss when reading the code linearly.

**Dockerfile was a barrier to adding languages.** The original monolithic
`apt-get install` line listed every language toolchain inline. Adding a new language
required a Dockerfile edit, rebuild, and push — three steps that violate the "30-minute
demo-day add" requirement. The restructure to per-language scripts (`scripts/lang_install/`)
with a `for f in *.sh; do bash "$f"; done` loop makes adding a language a one-step YAML
+ one-step .sh file operation, with zero Dockerfile change.

## What surprised me

How small the fixes were relative to the scoring impact. The `normalizeWhitespace`
bug was 3 lines. The `MaxBodyBytes` separation was ~8 lines. Together they're worth
15% of the judging rubric (API contract conformance). The Dockerfile restructure
is 25% of the rubric but only ~10 lines of actual change — the work was creating 11
idempotent shell scripts.

The inline `TrimRight` comparison in `runTestCase` having subtly different semantics
from `CompareOutput` is the kind of divergence that's easy to introduce when two people
both "fix" the same thing in different places. A unit test (now committed as
`TestArtifactPlaceholderResolution`) prevents similar drift in the template expansion code.

## What I'd do differently

Write the `status` package unit tests before writing the implementation. A test for
`CompareOutput("hello  world\n", "hello world\n") == "wrong_output"` would have caught
the `normalizeWhitespace` bug immediately. Instead it took a spec re-read and a code audit.

For limits, define all limit constants in one place (`config.go`) with a clear comment
explaining what each limit governs. `MaxBodyBytes` and `MaxSourceBytes` being different
things that both live in `Config` is not obvious without the comment.
