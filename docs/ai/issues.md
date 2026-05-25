# Known Issues and Gaps

## Critical

### Issue: Timeout Enforcement Missing
- **Description**: Code execution has no timeout enforcement. Infinite loops never terminate.
- **Impact**: Can exhaust resources and hang the service
- **Severity**: Critical
- **Status**: Open
- **Proposed Fix**: Implement context-based timeout with context.WithTimeout

### Issue: Orphaned Sandbox Directories
- **Description**: Sandbox cleanup sometimes fails, leaving /tmp directories behind
- **Impact**: Disk space exhaustion over time
- **Severity**: High
- **Status**: Open
- **Root Cause**: Likely permission issues or in-use files during cleanup
- **Proposed Fix**: Improve error handling and implement async cleanup with retry

## High

### Issue: No Request Authentication
- **Description**: All endpoints are public, no authentication mechanism
- **Impact**: Unauthorized code execution
- **Severity**: High
- **Status**: Design Phase
- **Proposed Fix**: Implement API key authentication or OAuth2

### Issue: Cold Start Performance
- **Description**: First execution takes ~500ms, target is <100ms
- **Impact**: Poor user experience, throughput limited
- **Severity**: High
- **Status**: Investigation Needed
- **Proposed Fix**: Profile startup, consider runtime pooling

## Medium

### Issue: Limited Error Messages
- **Description**: Users get generic "execution failed" errors
- **Impact**: Hard to debug user code
- **Severity**: Medium
- **Status**: Design Phase
- **Proposed Fix**: Better error reporting, execution logs

### Issue: No Job History
- **Description**: Jobs are lost on restart, no tracking
- **Impact**: Can't debug or audit executions
- **Severity**: Medium
- **Status**: Design Phase
- **Proposed Fix**: Implement Redis-backed job queue

### Issue: Memory Limits Not Enforced
- **Description**: Resource limit configuration not actually applied to nsjail
- **Impact**: Out-of-memory kills possible
- **Severity**: Medium
- **Status**: In Investigation
- **Proposed Fix**: Review nsjail config generation, add cgroup setup

## Low

### Issue: No Streaming Output
- **Description**: Output only available after execution completes
- **Impact**: Long-running tasks have no feedback
- **Severity**: Low
- **Status**: Feature Request
- **Proposed Fix**: Implement WebSocket support for streaming

### Issue: Documentation Incomplete
- **Description**: API and security docs need expansion
- **Impact**: Unclear usage patterns
- **Severity**: Low
- **Status**: Ongoing
