# Architecture

## Overview

goboxd is a containerized code execution service that safely runs untrusted code in isolated sandboxes using Linux namespaces and `nsjail`.

## Components

### Handler Layer
- **Health**: Liveness and readiness probes
- **Run**: Main execution endpoint, request routing
- Responsible for HTTP request/response handling

### Runner
- Coordinates code execution lifecycle
- Invokes nsjail with appropriate sandbox configuration
- Manages job lifecycle and resource cleanup

### Sandbox Manager
- Creates per-request isolated directories
- Merges resource limits (defaults + overrides)
- Handles cleanup and orphan detection

### Validation Layer
- **Filename**: Path traversal prevention
- **Flags**: Per-language flag allowlisting
- Ensures input safety before execution

### Status & Stats
- Atomic counters for job metrics
- Status vocabulary (pending, running, success, error, timeout)
- Request/response tracking

### Middleware
- Structured JSON logging of all requests
- Performance metrics (duration, status codes)
- Request tracing

## Execution Flow

1. Client sends POST /run with code + language
2. Handler validates input (language, flags)
3. Runner creates sandbox directory
4. Code written to sandbox
5. nsjail invokes language runtime in sandbox
6. Output captured and returned
7. Sandbox cleaned up
8. Response sent to client

## External Dependencies

- `nsjail`: Container/sandbox execution
- Linux kernel: Namespace support
- Language runtimes: go, python3, node, rustc, java
