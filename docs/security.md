# Security Architecture

## Isolation Mechanisms

### Process Isolation
- Uses `nsjail` for namespace-based sandboxing
- Each execution runs in a separate Linux namespace
- Prevents access to host resources

### File System Isolation
- Isolated temporary directories per job
- `chroot` into sandbox rootfs to prevent escapes
- Read-only mounts for runtime/libraries

### Network Isolation
- Disabled by default
- Optional NAT mode for controlled network access
- Port binding restricted to loopback

### Resource Limits
- Memory limits enforced via cgroups
- CPU limits via CPU quotas
- Process count limits
- Open file descriptor limits
- Execution timeout

## Input Validation

### Code Input
- Accepts raw source code
- No pre-processing or transformation
- Size limits enforced (max 1MB per job)

### Filename Validation
- Path traversal prevention via `../` detection
- Safe path verification with base directory checks
- Directory separator rejection

### Flag Validation
- Per-language allowlist enforcement
- Prevents compiler/runtime flag injection
- Whitelist-based approach

## Threat Model

### Protected Against
- **Privilege Escalation**: Linux namespace isolation
- **Resource Exhaustion**: cgroup limits
- **File System Escape**: chroot + safe path checks
- **Code Injection**: Input validation + allowlist enforcement
- **Network Attacks**: Disabled by default

### Assumptions
- Kernel patches for namespaces are current
- nsjail binary is trusted
- Host system is secure
- Container is properly configured
