# Incident Postmortem: Disk Space Exhaustion (May 2026)

## Executive Summary
Service became unresponsive on May 23, 2026 due to disk space exhaustion from orphaned sandbox directories. 6-hour downtime. RCA: Race condition in sandbox cleanup during force kill.

## Timeline
- **14:32**: Monitoring alerts for high disk usage
- **14:45**: On-call engineer investigates
- **15:02**: Identified /tmp/goboxd with 150GB of directories
- **15:15**: Emergency manual cleanup performed
- **15:23**: Service restored, customer impact assessment begun
- **16:00**: Root cause analysis meeting

## Root Cause
During resource limit enforcement, code execution context cancellation could race with sandbox cleanup. If the process was killed before cleanup completed, the directory remained with ownership by the killed process, preventing deletion without elevated privileges.

Specific sequence:
1. Job execution exceeds memory limit
2. nsjail process killed by cgroup OOM handler
3. Cleanup routine tries to remove directory
4. Permission denied due to process still holding file descriptor
5. Error logged but not retried
6. Directory remains indefinitely

## Impact
- 6 hours of service downtime
- ~1000 failed user requests
- 150GB disk cleanup required
- Reputational damage with customers

## Resolution Steps Taken
1. Implemented proper error handling in cleanup with retry logic
2. Added async cleanup job that runs periodically
3. Deployed improved sudo/elevated permission handling
4. Added disk usage monitoring/alerting at 70%, 85%, 95%

## Follow-up Actions
- [ ] Implement distributed tracing for job lifecycle
- [ ] Add comprehensive audit logging for cleanup operations
- [ ] Create runbook for manual disk cleanup
- [ ] Set up automated cleanup script with cron
- [ ] Implement circuit breaker for resource limit enforcement
- [ ] Add "max retries" for cleanup operations

## Lessons Learned
- Race conditions in cleanup are critical
- Permission issues need explicit handling
- Need better observability for lifecycle operations
- Monitoring must cover disk usage and resource limits
- Async operations require robust retry strategies

## Severity: CRITICAL - Prevented by: Monitoring, Testing
