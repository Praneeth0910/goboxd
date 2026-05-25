# Architecture Decision Records

## ADR-001: Use nsjail for Sandboxing

**Decision**: Implement container isolation using nsjail instead of Docker.

**Rationale**:
- Lightweight compared to Docker containers
- Lower overhead for ephemeral jobs
- Direct control over namespace configuration
- Better resource isolation at kernel level

**Consequences**:
- Requires Linux kernel with namespace support
- Dependency on nsjail binary (external, maintained separately)
- Different semantics than Docker (no OCI compliance)

**Status**: Accepted

---

## ADR-002: Whitelist-Based Flag Validation

**Decision**: Use per-language allowlists for compiler/runtime flags.

**Rationale**:
- Prevents flag-based code injection attacks
- Explicit security policy
- Easy to audit and update
- Better than trying to blacklist dangerous flags

**Consequences**:
- Requires maintaining allowlists per language
- May restrict legitimate use cases
- Need education for users on limitations

**Status**: Accepted

---

## ADR-003: In-Memory Job Tracking

**Decision**: Track jobs only in-memory with atomic counters.

**Rationale**:
- Simplicity for MVP
- No external dependencies
- Sufficient for initial use case

**Consequences**:
- Job data lost on restart
- No distributed support initially
- Will need migration to Redis later

**Status**: Accepted, Planning Migration

---

## ADR-004: Structured JSON Logging

**Decision**: Use JSON format for all structured logs.

**Rationale**:
- Easy to parse and aggregate
- Better for log analysis tools
- Standard in cloud environments

**Consequences**:
- Less human-readable console output
- Requires tooling to view locally
- Need log format documentation

**Status**: Accepted
