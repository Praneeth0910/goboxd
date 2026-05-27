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

## What I'd do differently

Build nsjail from source inside the Dockerfile instead of copying a pre-built 
binary. It makes the build slower but removes the GLIBC dependency entirely 
and guarantees it runs on any amd64 machine. I made this tradeoff for speed 
during the hackathon — it's the right call to reverse before production.