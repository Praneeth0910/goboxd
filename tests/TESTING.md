# goboxd Manual Testing Guide

Complete curl command reference for testing all endpoints before submission.

## Prerequisites

Start the service in one terminal:

```bash
make run
# or: docker run --privileged --cgroupns=host -p 8080:8080 goboxd:latest
```

The service will listen on `http://localhost:8080`

---

## Health & Info Endpoints

### GET /healthz - Liveness probe
```bash
curl -s http://localhost:8080/healthz | jq .
```

Expected output:
```json
{"status":"ok"}
```

### GET /readyz - Readiness probe
```bash
curl -s http://localhost:8080/readyz | jq .
```

Expected output:
```json
{"status":"ok"}
```

### GET /info - Service info (probes, config, stats)
```bash
curl -s http://localhost:8080/info | jq .
```

Expected output:
```json
{
  "version": "dev",
  "commit": "...",
  "ready": true,
  "nsjail": {"available": true},
  "languages": {
    "py3": {"available": true, "language": "Python 3"},
    "cpp": {"available": true, "language": "C++"}
  },
  "stats": {...}
}
```

---

## Successful Execution Tests

### POST /run - Python 3 hello world
```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "language": "py3",
    "source": "print(\"Hello, World!\")",
    "source_filename": "solution.py",
    "tests": [
      {
        "stdin": "",
        "expected_stdout": "Hello, World!\n"
      }
    ]
  }' | jq .
```

Expected output:
```json
{
  "status": "accepted",
  "tests": [
    {
      "status": "accepted",
      "stdout": "Hello, World!\n",
      "stderr": "",
      "duration_ms": 45
    }
  ]
}
```

### POST /run - C++ hello world (with build)
```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "language": "cpp",
    "source": "#include <iostream>\nint main() { std::cout << \"Hello, World!\" << std::endl; return 0; }",
    "source_filename": "solution.cpp",
    "artifact_filename": "solution",
    "tests": [
      {
        "stdin": "",
        "expected_stdout": "Hello, World!\n"
      }
    ]
  }' | jq .
```

Expected output:
```json
{
  "status": "accepted",
  "build": {
    "status": "ok",
    "stdout": "",
    "stderr": "",
    "duration_ms": 125
  },
  "tests": [
    {
      "status": "accepted",
      "stdout": "Hello, World!\n",
      "stderr": "",
      "duration_ms": 18
    }
  ]
}
```

### POST /run - Python with multiple tests
```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "language": "py3",
    "source": "x = int(input())\nprint(x * 2)",
    "source_filename": "solution.py",
    "tests": [
      {
        "stdin": "5",
        "expected_stdout": "10\n"
      },
      {
        "stdin": "10",
        "expected_stdout": "20\n"
      }
    ]
  }' | jq .
```

Expected output:
```json
{
  "status": "accepted",
  "tests": [
    {"status": "accepted", ...},
    {"status": "accepted", ...}
  ]
}
```

---

## Security & Validation Tests

### POST /run - Path traversal attack (../../etc/passwd)
```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "language": "py3",
    "source": "print(1)",
    "source_filename": "../../etc/passwd",
    "tests": []
  }' | jq .
```

Expected output: HTTP 400 with error
```json
{
  "error": {
    "code": "invalid_filename",
    "message": "..."
  }
}
```

### POST /run - Leading dot attack (.bashrc)
```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "language": "py3",
    "source": "print(1)",
    "source_filename": ".bashrc",
    "tests": []
  }' | jq .
```

Expected output: HTTP 400 with error

### POST /run - Compiler flag injection (-fPIC)
```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "language": "cpp",
    "source": "int main() { return 0; }",
    "source_filename": "solution.cpp",
    "artifact_filename": "solution",
    "build": {
      "flags": ["-fPIC"]
    },
    "tests": []
  }' | jq .
```

Expected output: HTTP 400 with error
```json
{
  "error": {
    "code": "invalid_flag",
    "message": "..."
  }
}
```

### POST /run - Allowed flag variant (-std=c++17)
```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "language": "cpp",
    "source": "#include <iostream>\nint main() { std::cout << \"OK\" << std::endl; return 0; }",
    "source_filename": "solution.cpp",
    "artifact_filename": "solution",
    "build": {
      "flags": ["-std=c++17"]
    },
    "tests": [
      {
        "stdin": "",
        "expected_stdout": "OK\n"
      }
    ]
  }' | jq .
```

Expected output: HTTP 200 with status "accepted"

---

## Error Handling Tests

### POST /run - Missing language field
```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "source": "print(1)",
    "tests": []
  }' | jq .
```

Expected output: HTTP 400, error code "unknown_language" or "invalid_json"

### POST /run - Unknown language
```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "language": "rust",
    "source": "fn main() {}",
    "tests": []
  }' | jq .
```

Expected output: HTTP 400, error code "unknown_language"

### POST /run - Oversized source (>256KB)
```bash
# Generate a 300KB source string
python3 -c 'print("{\"language\": \"py3\", \"source\": \"" + "x = 1\n" * 50000 + "\", \"tests\": []}")' | \
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d @-
```

Expected output: HTTP 413 (Payload Too Large)

### POST /run - Invalid JSON
```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{"language": "py3", "source": "print(1)"' # Missing closing brace
```

Expected output: HTTP 400, error code "invalid_json"

---

## Testing Workflow

Run these commands in sequence:

```bash
# 1. Start the service
make run &

# 2. Wait for readiness (repeat until 200)
sleep 2
curl http://localhost:8080/readyz

# 3. Run all health checks
curl -s http://localhost:8080/healthz | jq .
curl -s http://localhost:8080/info | jq .

# 4. Run Python hello world test
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "language": "py3",
    "source": "print(\"Hello, World!\")",
    "source_filename": "solution.py",
    "tests": [{"stdin": "", "expected_stdout": "Hello, World!\n"}]
  }' | jq .

# 5. Run C++ hello world test
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "language": "cpp",
    "source": "#include <iostream>\nint main() { std::cout << \"Hello, World!\" << std::endl; return 0; }",
    "source_filename": "solution.cpp",
    "artifact_filename": "solution",
    "tests": [{"stdin": "", "expected_stdout": "Hello, World!\n"}]
  }' | jq .

# 6. Run security tests
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{"language": "py3", "source": "print(1)", "source_filename": "../../etc/passwd", "tests": []}' | jq .

# Expected: {"error": {"code": "invalid_filename", ...}}

# 7. Verify static analysis
make lint
go vet ./...
```

---

## Troubleshooting

### Service not responding
- Check if it's running: `ps aux | grep goboxd`
- Check logs: `docker logs <container-id>`
- Verify port: `netstat -tlnp | grep 8080` (Linux) or `lsof -i :8080` (macOS)

### Python test fails
- Verify Python 3 is installed in container: `docker exec <container> python3 --version`
- Check if solution.py is being created: `docker exec <container> ls -la /tmp/goboxd-*/`

### C++ test fails
- Verify g++ is installed: `docker exec <container> g++ --version`
- Check compile output in `build.stderr` field

### Timeouts
- Increase `wall_time_s` in your test payload
- Check if nsjail is available: curl the `/info` endpoint

---

**Last Updated**: May 26, 2026
