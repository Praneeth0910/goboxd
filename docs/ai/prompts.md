# AI/ML Development Prompts

## System Prompt for Code Generation

You are an expert Go developer building a secure, scalable code execution sandbox. Focus on:
- Safety through namespace isolation and strict input validation
- Clean architecture with clear separation of concerns
- Comprehensive error handling and logging
- Performance optimization for sandbox overhead

## Context and Requirements

### Security Requirements
- Path traversal prevention
- Resource exhaustion protection
- Network isolation
- Flag/argument validation
- Sandbox escape prevention

### Performance Requirements
- <1s response time for typical code
- Support 1000+ req/sec throughput
- Efficient resource cleanup
- Memory footprint <50MB base

### Code Quality
- Comprehensive unit tests
- Clear interfaces and abstractions
- Well-documented public APIs
- Error propagation and logging

## Example Prompts

### Implementing a New Feature
```
Implement a job queue with Redis backend for persistent execution tracking.
Requirements:
- Support enqueuing jobs with priority
- Track job status (pending, running, complete)
- Expire old jobs after 24 hours
- Atomic operations for thread safety
```

### Debugging Issues
```
The sandbox is sometimes not cleaning up temporary directories. 
The sweep operation removes directories older than 1 hour, but we see orphans.
Debug this issue and implement a more robust cleanup strategy.
```

### Optimization
```
We need to reduce cold start time from 500ms to <100ms.
Profile the startup sequence and identify bottlenecks.
Consider: lazy initialization, connection pooling, or caching strategies.
```

## Testing Strategy

- Unit tests for validation, limits, and status
- Integration tests for full execution flow
- Load tests for throughput and latency
- Stress tests for resource limits
