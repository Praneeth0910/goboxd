# Design Patterns Used

## Pattern: Handler-based Request Routing

**Where**: `internal/handler/` package

**Pattern**:
```go
func HealthHandler(w http.ResponseWriter, r *http.Request) {
    // Handle request
}
```

**Benefits**:
- Simple and idiomatic Go
- No framework dependencies
- Easy to test and mock

---

## Pattern: Configuration Validation

**Where**: `internal/config/config.go`

**Pattern**:
```go
type Config struct { /* ... */ }
func (c *Config) Validate() error { /* ... */ }
```

**Benefits**:
- Fails fast on invalid config
- Clear error messages
- Reusable validation logic

---

## Pattern: Atomic Counters for Metrics

**Where**: `internal/stats/stats.go`

**Pattern**:
```go
func (s *Stats) IncrementJobs() {
    atomic.AddInt64(&s.totalJobs, 1)
}
```

**Benefits**:
- Thread-safe without locks
- Low overhead
- Fast metric updates

---

## Pattern: Resource Cleanup with Defer

**Where**: `internal/runner/runner.go`

**Pattern**:
```go
sbx := sandbox.NewDir(path)
defer sbx.Cleanup()
// Use sandbox
```

**Benefits**:
- Ensures cleanup happens
- Works even on error paths
- Clear resource lifecycle

---

## Pattern: Allowlist Validation

**Where**: `internal/validate/flags.go`

**Pattern**:
```go
allowed := AllowedFlags[lang]
// Check against whitelist
```

**Benefits**:
- Explicit security policy
- Easy to audit
- Prevents flag injection

---

## Pattern: Middleware Chain

**Where**: `internal/middleware/logger.go`

**Pattern**:
```go
func Logger(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w, r) {
        // Log request, call next
    })
}
```

**Benefits**:
- Composable request processing
- Separation of concerns
- Reusable across routes
