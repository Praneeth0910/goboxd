# Benchmark Results

## Test Environment

| Field | Value |
|---|---|
| Date | 2026-05-28 |
| Kernel | 6.6.114.1-microsoft-standard-WSL2 |
| CPU cores | 4 |
| RAM | 5.8Gi |
| Docker version | 29.4.3 |
| nsjail | enabled (NSJAIL_PATH=/usr/sbin/nsjail) |
| Tool | hey |
| Requests per run | 200 |
| Endpoint | POST http://localhost:8080/run |

> Results collected on a live container with the commands in `scripts/run_benchmarks.sh`.

---

## Python 3 — `print("Hello, World!")`

### Payload
```json
{
  "language": "py3",
  "source": "print(\"Hello, World!\")",
  "tests": [{"stdin": "", "expected_stdout": "Hello, World!\n"}]
}
```

## C++ — `std::cout << "Hello, World!"`

### Payload
```json
{
  "language": "cpp",
  "source": "#include <iostream>\nint main(){std::cout<<\"Hello, World!\"<<std::endl;return 0;}",
  "tests": [{"stdin": "", "expected_stdout": "Hello, World!\n"}]
}
```

---

## Results

| Language | Clients | Requests | p50 (ms) | p95 (ms) | p99 (ms) | RPS | Errors |
|----------|---------|----------|----------|----------|----------|-----|--------|
| Python   | 1       | 200      | 43.0     | 51.8     | 53.3     | 23.1| 0      |
| Python   | 10      | 200      | 177.2    | 240.5    | 271.8    | 55.9| 0      |
| Python   | 50      | 200      | 1043.4   | 1107.9   | 1140.6   | 47.4| 0      |
| Python   | 100     | 200      | 2113.7   | 2210.5   | 2233.1   | 46.0| 0      |
| C++      | 1       | 200      | 593.7    | 669.7    | 707.4    | 1.7 | 0      |
| C++      | 10      | 200      | 3276.4   | 4253.2   | 4490.8   | 3.0 | 0      |
| C++      | 50      | 200      | 17404.3  | 19562.7  | 19987.0  | 2.8 | 5      |
| C++      | 100     | 200      | 11275.5  | 19958.1  | Timeout  | 5.0 | 143    |

---

## Analysis

- **Python 3**: Interpret-and-run, no compile step. p50 latency is dominated by interpreter startup inside the sandbox.
- **C++**: Includes a compile step (g++) per request, so p50 and p95 are typically higher than Python despite faster runtime.
- Latency increases roughly linearly with concurrency due to CPU contention on sandbox process spawning.
- If nsjail is **enabled**, expect an additional ~10–25 ms overhead per request from namespace/cgroup setup.
- Errors at high concurrency (c=100) may reflect the `max_concurrent` or `queue_timeout_s` settings in `languages.yaml`.

## How to Re-run

```bash
# 1. Start the container
docker compose up -d

# 2. Install hey (Ubuntu/Debian)
sudo apt-get install -y hey          # if in apt repos
# OR via go:
go install github.com/rakyll/hey@latest

# 3. Run this script
bash scripts/run_benchmarks.sh
```

### Individual hey commands

**Install hey (if not present):**
```bash
# Option A – go install (requires go 1.17+)
go install github.com/rakyll/hey@latest
export PATH="$PATH:$(go env GOPATH)/bin"

# Option B – direct binary (Ubuntu/Debian x86-64)
sudo curl -sSL https://hey-release.s3.us-east-2.amazonaws.com/hey_linux_amd64 \
  -o /usr/local/bin/hey && sudo chmod +x /usr/local/bin/hey
```

**Python – c=1:**
```bash
hey -n 200 -c 1 -m POST -H "Content-Type: application/json" \
  -d '{"language":"py3","source":"print(\"Hello, World!\")","tests":[{"stdin":"","expected_stdout":"Hello, World!\n"}]}' \
  http://localhost:8080/run
```

**Python – c=10:**
```bash
hey -n 200 -c 10 -m POST -H "Content-Type: application/json" \
  -d '{"language":"py3","source":"print(\"Hello, World!\")","tests":[{"stdin":"","expected_stdout":"Hello, World!\n"}]}' \
  http://localhost:8080/run
```

**Python – c=50:**
```bash
hey -n 200 -c 50 -m POST -H "Content-Type: application/json" \
  -d '{"language":"py3","source":"print(\"Hello, World!\")","tests":[{"stdin":"","expected_stdout":"Hello, World!\n"}]}' \
  http://localhost:8080/run
```

**Python – c=100:**
```bash
hey -n 200 -c 100 -m POST -H "Content-Type: application/json" \
  -d '{"language":"py3","source":"print(\"Hello, World!\")","tests":[{"stdin":"","expected_stdout":"Hello, World!\n"}]}' \
  http://localhost:8080/run
```

**C++ – c=1:**
```bash
hey -n 200 -c 1 -m POST -H "Content-Type: application/json" \
  -d '{"language":"cpp","source":"#include <iostream>\nint main(){std::cout<<\"Hello, World!\"<<std::endl;return 0;}","tests":[{"stdin":"","expected_stdout":"Hello, World!\n"}]}' \
  http://localhost:8080/run
```

**C++ – c=10:**
```bash
hey -n 200 -c 10 -m POST -H "Content-Type: application/json" \
  -d '{"language":"cpp","source":"#include <iostream>\nint main(){std::cout<<\"Hello, World!\"<<std::endl;return 0;}","tests":[{"stdin":"","expected_stdout":"Hello, World!\n"}]}' \
  http://localhost:8080/run
```

**C++ – c=50:**
```bash
hey -n 200 -c 50 -m POST -H "Content-Type: application/json" \
  -d '{"language":"cpp","source":"#include <iostream>\nint main(){std::cout<<\"Hello, World!\"<<std::endl;return 0;}","tests":[{"stdin":"","expected_stdout":"Hello, World!\n"}]}' \
  http://localhost:8080/run
```

**C++ – c=100:**
```bash
hey -n 200 -c 100 -m POST -H "Content-Type: application/json" \
  -d '{"language":"cpp","source":"#include <iostream>\nint main(){std::cout<<\"Hello, World!\"<<std::endl;return 0;}","tests":[{"stdin":"","expected_stdout":"Hello, World!\n"}]}' \
  http://localhost:8080/run
```
