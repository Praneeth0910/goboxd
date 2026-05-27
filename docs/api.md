# goboxd API Reference

## Base URL

```
http://localhost:8080
```

---

## Endpoints

### GET /healthz

Liveness probe. Always returns 200 if the process is running.

**Response** `200 OK`

```json
{
  "status": "ok"
}
```

---

### GET /readyz

Readiness probe. Returns 200 if nsjail and all languages are operational, 503 if degraded.

**Response** `200 OK` or `503 Service Unavailable`

```json
{
  "status": "ok",
  "nsjail": {
    "ok": true,
    "version": "unknown"
  },
  "languages": {
    "py3": {
      "ok": true,
      "version": "Python 3.11.2"
    },
    "cpp": {
      "ok": true,
      "version": "g++ (Debian 12.2.0-14+deb12u1) 12.2.0"
    }
  }
}
```

---

### GET /info

Returns build info, configured languages, global limits, and runtime stats. Always 200.

**Response** `200 OK`

```json
{
  "build_info": {
    "version": "0.1.0",
    "commit": "abc1234",
    "go_version": "go1.22.12"
  },
  "nsjail": {
    "path": "/usr/sbin/nsjail",
    "version": "unknown"
  },
  "languages": [
    {
      "id": "py3",
      "name": "Python 3",
      "version": "Python 3.11.2",
      "default_run_limits": {
        "wall_time_s": 9,
        "memory_kb": 102400,
        "max_processes": 100
      }
    },
    {
      "id": "cpp",
      "name": "C++",
      "version": "g++ (Debian 12.2.0-14+deb12u1) 12.2.0",
      "default_run_limits": {
        "wall_time_s": 3,
        "memory_kb": 524288,
        "max_processes": 64
      }
    }
  ],
  "limits": {
    "max_source_bytes": 262144,
    "max_tests": 50,
    "max_concurrent_jobs": 4
  },
  "stats": {
    "in_flight_jobs": 0,
    "jobs_total": 42,
    "jobs_failed_internal": 0,
    "disk_free_bytes_jail_dir": 1014416347136
  }
}
```

---

### GET /

Redirects to `/info` (HTTP 302).

---

### POST /run

Execute source code in a sandboxed environment and run test cases against it.

**Request Headers**

| Header         | Value              |
|----------------|--------------------|
| Content-Type   | application/json   |

**Request Body**

```json
{
  "language": "py3",
  "source": "print(\"hello\")",
  "source_filename": "solution.py",
  "artifact_filename": "solution",
  "build": {
    "flags": ["-O2", "-std=c++17"],
    "limits": {
      "wall_time_s": 5,
      "memory_kb": 524288,
      "max_processes": 100
    }
  },
  "run": {
    "limits": {
      "wall_time_s": 10,
      "memory_kb": 102400,
      "max_processes": 100
    }
  },
  "tests": [
    {
      "stdin": "5\n",
      "expected_stdout": "10\n"
    }
  ]
}
```

| Field               | Type        | Required | Description                                                                 |
|---------------------|-------------|----------|-----------------------------------------------------------------------------|
| `language`          | string      | Yes      | Language ID from `languages.yaml` (e.g., `py3`, `cpp`)                      |
| `source`            | string      | Yes      | Source code to execute                                                       |
| `source_filename`   | string      | No       | Filename for the source file (defaults to language config)                   |
| `artifact_filename` | string      | No       | Output binary name for compiled languages (defaults to language config)      |
| `build`             | object      | No       | Build-phase options (compiled languages only)                                |
| `build.flags`       | string[]    | No       | Compiler flags (validated against per-language allowlist)                     |
| `build.limits`      | object      | No       | Override build resource limits                                               |
| `run`               | object      | No       | Run-phase options                                                            |
| `run.limits`        | object      | No       | Override run resource limits                                                 |
| `tests`             | object[]    | Yes      | Array of test cases (1 to 50)                                                |
| `tests[].stdin`     | string      | Yes      | Input to provide via stdin                                                   |
| `tests[].expected_stdout` | string | Yes   | Expected stdout for comparison                                               |

**Successful Response** `200 OK`

All execution results (including failures) return 200. User code errors never cause 5xx.

```json
{
  "status": "accepted",
  "build": {
    "status": "ok",
    "stdout": "",
    "stderr": "",
    "wall_time_ms": 125
  },
  "tests": [
    {
      "status": "accepted",
      "stdout": "10\n",
      "stderr": "",
      "wall_time_ms": 18
    }
  ]
}
```

The `build` field is only present for compiled languages.

**Status Values**

| Status                        | Description                                        |
|-------------------------------|----------------------------------------------------|
| `accepted`                    | Output matches expected                            |
| `wrong_output`               | Output differs from expected (non-whitespace diff) |
| `output_whitespace_mismatch` | Output differs only in whitespace                  |
| `runtime_error`              | Non-zero exit code                                 |
| `time_exceeded`              | Execution exceeded wall time limit                 |
| `memory_exceeded`            | Process killed by OOM                              |
| `build_failed`               | Compilation failed                                 |
| `not_executed`               | Test was skipped (e.g., build failed)              |
| `internal_error`             | Server-side error (should not occur)               |

**Top-level Status Rule**: The overall `status` is `accepted` only if build succeeded AND all tests are `accepted`. If build fails, status is `build_failed`. Otherwise, the first non-accepted test status is returned.

---

## Error Responses

Validation errors return HTTP 400 with an error object:

```json
{
  "error": {
    "code": "unknown_language",
    "message": "language \"rust\" is not configured"
  }
}
```

| Error Code         | HTTP | Trigger                                      |
|--------------------|------|----------------------------------------------|
| `invalid_json`     | 400  | Malformed JSON or unknown fields             |
| `unknown_language` | 400  | Language not in `languages.yaml`             |
| `invalid_filename` | 400  | Path traversal, hidden file, or separators   |
| `disallowed_flag`  | 400  | Compiler flag not in allowlist               |
| `invalid_test_count` | 400 | 0 tests or more than 50 tests               |
| `queue_full`       | 429  | Server is too busy, all concurrency slots full |
| `internal_error`   | 500  | Server-side error during execution           |

Oversized request bodies (exceeding `max_source_bytes`) are rejected by `http.MaxBytesReader` before decoding.
