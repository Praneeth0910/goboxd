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
