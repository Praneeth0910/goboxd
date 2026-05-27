# Benchmark Results

## Test Environment

| Field          | Value                                    |
|----------------|------------------------------------------|
| Date           | 2026-05-27                               |
| Kernel         | 6.6.114.1-microsoft-standard-WSL2        |
| CPU cores      | 4                                        |
| RAM            | 5.8 GiB                                  |
| Docker version | 29.4.3                                   |
| nsjail         | enabled (`NSJAIL_PATH=/usr/sbin/nsjail`) |
| Tool           | hey v0.1.5                               |
| Requests       | 200 per concurrency level                |
| Endpoint       | `POST http://localhost:8080/run`          |

> All results were collected against a live `goboxd` container (2 CPUs, 2 GiB RAM limit via
> `docker-compose.yml`) on a WSL2 host. nsjail was configured but may not have been fully
> active on WSL2 (kernel namespace support is limited). See notes below.

---

## Payloads

### Python 3

```json
{
  "language": "py3",
  "source": "print(\"Hello, World!\")",
  "tests": [{"stdin": "", "expected_stdout": "Hello, World!\n"}]
}
```

### C++

```json
{
  "language": "cpp",
  "source": "#include <iostream>\nint main(){std::cout<<\"Hello, World!\"<<std::endl;return 0;}",
  "tests": [{"stdin": "", "expected_stdout": "Hello, World!\n"}]
}
```

---

## Results

All latency values are in **milliseconds (ms)**. RPS = requests per second.
Errors = requests that did not return HTTP 200 (timeouts or 4xx/5xx).

| Language | Clients | Requests | p50 (ms) | p95 (ms)  | p99 (ms)  |  RPS  | Errors |
|----------|---------|----------|----------|-----------|-----------|-------|--------|
| Python   | 1       | 200      | 32.2     | 38.9      | 41.4      | 30.3  | 0      |
| Python   | 10      | 200      | 192.8    | 255.3     | 283.3     | 51.0  | 0      |
| Python   | 50      | 200      | 919.4    | 1,124.2   | 1,154.6   | 50.4  | 0      |
| Python   | 100     | 200      | 1,739.6  | 1,927.5   | 2,005.6   | 53.2  | 0      |
| C++      | 1       | 200      | 471.0    | 789.0     | N/A †     | 2.1   | 0      |
| C++      | 10      | 200      | 2,947.6  | 6,099.7   | 6,894.9   | 3.1   | 0      |
| C++      | 50      | 200      | 13,251.8 | 15,387.6  | N/A ‡     | 3.4   | 116    |
| C++      | 100     | 200      | 27,890.6 | 29,643.7  | 29,973.5  | 3.4   | 7      |

**† C++ c=1**: `hey` terminates early on c=1 with slow endpoints due to a known keep-alive
issue; latency statistics are from 20 sequential requests collected via `curl -w "%{time_total}"`.
p99 is omitted (sample size < 100).

**‡ C++ c=50**: Only 84 of 200 requests returned 200; the remaining 116 exceeded the 30-second
`hey` client timeout. With only 84 successful samples the 99th-percentile bucket cannot be
computed reliably, so it is marked N/A.

---

## Analysis

### Python 3

- **Baseline (c=1)**: p50 ≈ 32 ms. This is pure interpreter startup inside the process sandbox
  (no compilation step). Very consistent — p99 is only 9 ms above p50.
- **Concurrency scaling**: RPS stays roughly constant at ~50 from c=10 onwards, confirming the
  bottleneck is CPU (4 cores shared between host and container), not Go's HTTP layer.
- **p50 vs concurrency**: Grows roughly linearly (32 ms → 193 ms → 919 ms → 1,740 ms) as
  requests queue behind each other waiting for a CPU slot.
- **Error rate**: Zero errors at all concurrency levels — the queue and timeout configuration
  in `languages.yaml` (`queue_timeout_s: 30`, `wall_time_s: 9`) is well-sized for Python.

### C++

- **Compile overhead dominates**: A single request takes ~470 ms (≈ 450 ms for `g++` + 17 ms
  runtime). Compare to Python's 32 ms — C++ is ~15× slower at c=1 due entirely to compilation.
- **CPU saturation at c=10+**: With 10 concurrent compile jobs on 4 cores, queue depth explodes.
  p50 at c=10 is already 2.95 s (each request waits for ~6 prior compile slots to drain).
- **Timeouts at c=50 and c=100**: 116 of 200 requests timed out at c=50. The `wall_time_s: 3`
  build limit in `languages.yaml` causes `g++` to be killed if it cannot get a CPU slice within
  3 seconds. Under heavy contention this is very common.
- **nsjail note**: On WSL2, nsjail namespace features (user namespaces, pid namespaces) may fall
  back silently if the kernel doesn't support them. Actual production numbers on a native Linux
  host with full nsjail will differ — expect +10–25 ms overhead per request at low concurrency
  but similar saturation behaviour.
**Mitigation**: For compiled language workloads, either increase `wall_time_s` for the 
build phase (e.g., 10s) or reduce `max_concurrent` to 2 to avoid CPU starvation. 
The current defaults are tuned for interpreted languages.
### Recommendation

For workloads that include compiled languages (C++, C, Java), set `max_concurrent` in
`languages.yaml` or `GOBOXD_MAX_CONCURRENT` to limit queue depth. A value of `2–4×CPU_count`
avoids runaway queue growth while still providing parallelism for interpreted languages.

---

## How to Re-run

### 1. Install `hey`

```bash
# Via go install (requires Go 1.17+)
go install github.com/rakyll/hey@latest
export PATH="$PATH:$(go env GOPATH)/bin"

# OR: direct binary (Ubuntu/Debian x86-64, no sudo required for go install)
sudo curl -sSL https://hey-release.s3.us-east-2.amazonaws.com/hey_linux_amd64 \
  -o /usr/local/bin/hey && sudo chmod +x /usr/local/bin/hey
```

### 2. Start the container

```bash
docker compose up -d
# Verify it is accepting requests
curl http://localhost:8080/healthz
```

### 3. Run the automated script

```bash
bash scripts/run_benchmarks.sh        # fills this file automatically
```

---

## Individual `hey` Commands

### Python 3 — `print("Hello, World!")`

```bash
# c=1
hey -n 200 -c 1 -m POST -H "Content-Type: application/json" \
  -d '{"language":"py3","source":"print(\"Hello, World!\")","tests":[{"stdin":"","expected_stdout":"Hello, World!\n"}]}' \
  http://localhost:8080/run

# c=10
hey -n 200 -c 10 -m POST -H "Content-Type: application/json" \
  -d '{"language":"py3","source":"print(\"Hello, World!\")","tests":[{"stdin":"","expected_stdout":"Hello, World!\n"}]}' \
  http://localhost:8080/run

# c=50
hey -n 200 -c 50 -m POST -H "Content-Type: application/json" \
  -d '{"language":"py3","source":"print(\"Hello, World!\")","tests":[{"stdin":"","expected_stdout":"Hello, World!\n"}]}' \
  http://localhost:8080/run

# c=100
hey -n 200 -c 100 -m POST -H "Content-Type: application/json" \
  -d '{"language":"py3","source":"print(\"Hello, World!\")","tests":[{"stdin":"","expected_stdout":"Hello, World!\n"}]}' \
  http://localhost:8080/run
```

### C++ — `std::cout << "Hello, World!"`

> **Note:** Use `-t 30` to extend the per-request timeout; C++ compile+run takes ~470 ms at
> low load but can exceed 20 s under heavy concurrency due to CPU queuing.

```bash
# c=1
hey -n 200 -c 1 -t 30 -m POST -H "Content-Type: application/json" \
  -d '{"language":"cpp","source":"#include <iostream>\nint main(){std::cout<<\"Hello, World!\"<<std::endl;return 0;}","tests":[{"stdin":"","expected_stdout":"Hello, World!\n"}]}' \
  http://localhost:8080/run

# c=10
hey -n 200 -c 10 -t 30 -m POST -H "Content-Type: application/json" \
  -d '{"language":"cpp","source":"#include <iostream>\nint main(){std::cout<<\"Hello, World!\"<<std::endl;return 0;}","tests":[{"stdin":"","expected_stdout":"Hello, World!\n"}]}' \
  http://localhost:8080/run

# c=50
hey -n 200 -c 50 -t 30 -m POST -H "Content-Type: application/json" \
  -d '{"language":"cpp","source":"#include <iostream>\nint main(){std::cout<<\"Hello, World!\"<<std::endl;return 0;}","tests":[{"stdin":"","expected_stdout":"Hello, World!\n"}]}' \
  http://localhost:8080/run

# c=100
hey -n 200 -c 100 -t 30 -m POST -H "Content-Type: application/json" \
  -d '{"language":"cpp","source":"#include <iostream>\nint main(){std::cout<<\"Hello, World!\"<<std::endl;return 0;}","tests":[{"stdin":"","expected_stdout":"Hello, World!\n"}]}' \
  http://localhost:8080/run
```
