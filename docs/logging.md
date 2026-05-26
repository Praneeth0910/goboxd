# Structured Logging in goboxd

## Overview
goboxd uses structured JSON logging with `log/slog` to provide detailed request tracking and debugging information. Each HTTP request generates exactly one log line with full request/response details.

## Log Format

Every request produces a JSON log line containing:
- **ts**: RFC3339 timestamp (request start time)
- **request_id**: Unique UUID per request (hex-encoded)
- **method**: HTTP method (GET, POST, etc.)
- **path**: Request URI path
- **status**: HTTP response status code
- **duration_ms**: Request duration in milliseconds
- **language**: Programming language (only for POST /run, empty otherwise)
- **job_status**: Top-level execution status (only for POST /run, empty otherwise)

## Example Log Lines

### GET /info
```json
{"ts":"2026-05-26T14:30:45Z","request_id":"a1b2c3d4...","method":"GET","path":"/info","status":200,"duration_ms":12,"language":"","job_status":""}
```

### POST /run (successful)
```json
{"ts":"2026-05-26T14:30:50Z","request_id":"e5f6g7h8...","method":"POST","path":"/run","status":200,"duration_ms":245,"language":"python3","job_status":"accepted"}
```

### POST /run (failed)
```json
{"ts":"2026-05-26T14:30:55Z","request_id":"i9j0k1l2...","method":"POST","path":"/run","status":200,"duration_ms":180,"language":"cpp","job_status":"build_failed"}
```

## Implementation Details

### Middleware (`internal/middleware/logger.go`)
- Wraps the response writer to capture HTTP status codes
- Generates a unique request_id per request using `crypto/rand`
- Stores language and job_status in request context (set by handlers)
- Logs all details after the handler completes

### Context Keys
Two typed context keys are used to pass data from handlers to the logger:
- `language`: Set by POST /run handler after validating the requested language
- `job_status`: Set by POST /run handler after execution completes

### Usage in Handlers
```go
// In POST /run handler, after language validation:
r = middleware.SetLanguage(r, req.Language)

// After execution result is obtained:
r = middleware.SetJobStatus(r, result.Status)
```

## Router Integration

In `main.go`, the custom logger middleware is registered first in the chi router:
```go
r.Use(mw.Logger)  // Structured JSON logging (replaces chi's middleware.Logger)
```

This replaces chi's built-in logger with our structured version.

## Benefits
- **Structured**: Every field is explicitly named, enabling easy log aggregation and analysis
- **Performant**: Single log line per request (no noise)
- **Request Tracking**: Unique request_id allows tracing related logs
- **Execution Visibility**: language and job_status fields provide immediate insight into job outcomes
- **Simple**: Only uses Go standard library (`log/slog`, `crypto/rand`, `context`)
