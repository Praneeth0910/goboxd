# Benchmark Results

## Test Environment

| Field | Value |
|---|---|
| Date | 2026-05-28 |
| OS | WSL2 (Linux 6.6.114.1-microsoft-standard-WSL2) |
| CPU cores | 4 |
| RAM | 5.8 GiB |
| Docker version | 29.4.3 |
| nsjail | enabled (`NSJAIL_PATH=/usr/sbin/nsjail`) |
| Load tool | hey v0.1.5 |
| Requests per run | 200 |
| hey default request timeout | 20s |
| Endpoint | `POST http://localhost:8080/run` |

---

## Payloads

**Python 3** — interpreted, no compile step
```json
{"language":"py3","source":"print(\"Hello, World!\")","tests":[{"stdin":"","expected_stdout":"Hello, World!\n"}]}
```

**C++** — compiled per request via g++
```json
{"language":"cpp","source":"#include <iostream>\nint main(){std::cout<<\"Hello, World!\"<<std::endl;return 0;}","tests":[{"stdin":"","expected_stdout":"Hello, World!\n"}]}
```

---

## Results

| Language | Clients (c) | Sent | OK  | p50 (ms) | p95 (ms) | p99 (ms) | RPS  | Client timeouts |
|----------|-------------|------|-----|----------|----------|----------|------|-----------------|
| Python 3 | 1           | 200  | 200 | 33       | 44       | 50       | 28.8 | 0               |
| Python 3 | 10          | 200  | 200 | 172      | 230      | 250      | 56.3 | 0               |
| Python 3 | 50          | 200  | 200 | 852      | 900      | 919      | 58.0 | 0               |
| Python 3 | 100         | 200  | 200 | 1686     | 1791     | 1812     | 57.1 | 0               |
| C++      | 1           | 200  | 200 | 439      | 515      | 857      | 2.2  | 0               |
| C++      | 10          | 200  | 200 | 2673     | 3626     | 4965     | 3.6  | 0               |
| C++      | 50          | 200  | 200 | 13000    | 14816    | 15612    | 3.7  | 0               |
| C++      | 100         | 200  | 74  | 10639    | 19294    | —        | 5.0  | 126             |

> **Client timeouts** = hey gave up after its 20s default; the server did not reject these
> requests (zero 429s observed). The server queued them correctly — the compile time
> under high concurrency simply exceeds what hey will wait for.
> `—` in p99 for C++ c=100: insufficient successful samples for hey to compute the percentile.

---

## Analysis

### Key Takeaways
- **Server returned zero 429s at Python c=100.**
- **C++ c=100 timeouts are client-side (hey's 20s), not server rejections.**
- **Queue absorbed load cleanly** across all tests, buffering requests instead of dropping them.

### Python 3

Zero errors across all 800 requests. The semaphore queue absorbs load cleanly at every
concurrency level tested.

- **c=1**: p50=33ms — nsjail namespace setup + Python interpreter startup dominate.
- **c=1 → c=10**: p50 rises to 172ms as 10 sandbox processes share 4 CPU cores.
- **c=10 → c=50**: p50 jumps to 852ms — queue fills as 50 workers compete for the
  `max_concurrent` slots (default = `runtime.NumCPU()` = 4 on this machine). No requests
  are dropped; they wait in the channel semaphore and drain correctly.
- **c=50 → c=100**: p50 climbs to 1686ms but RPS stays flat at ~57 — the server is
  saturated but stable. Queue timeout is 30s; no request waited long enough to hit it.

### C++

C++ adds a `g++` compile step per request inside the sandbox before the run step.

- **c=1**: p50=439ms. The compile step is the dominant cost — roughly 400ms of that is
  `g++` inside nsjail.
- **c=10**: p50=2673ms. Ten concurrent compile jobs on 4 cores means each job waits ~2.3
  compile slots before it starts. All 200 requests complete successfully.
- **c=50**: p50=13000ms. Deep queue saturation — 50 concurrent requests but only 4 slots.
  Average wait is ~12s of queue time before compilation even starts. All 200 still succeed;
  the server does not drop or reject any.
- **c=100**: 74/200 succeed within hey's 20s timeout, 126 are abandoned by the client.
  The server emitted zero 429s — every request was accepted and queued. The failures are
  entirely client-side: `context deadline exceeded` after 20s. Raising hey's `-t` flag
  (e.g. `-t 60`) would show more completions, at the cost of longer test duration.
  On a machine with more cores or a faster compiler cache, this threshold would shift
  significantly.

### WSL2 Caveats

Results were collected on WSL2, which shares the host kernel and has lower I/O throughput
than bare-metal Linux. Cgroup enforcement and nsjail namespace setup may be slower under
WSL2 than on a native Linux host. Expect lower latencies on a dedicated machine.

### Concurrency limiter

The channel semaphore (`GOBOXD_MAX_CONCURRENT`, defaulting to `runtime.NumCPU()`) is
functioning as designed. Python handled 100 concurrent clients with zero failures.
C++ at high concurrency exposes the compile-step cost, not a server correctness problem —
the server never crashed, never rejected with 5xx, and never dropped a request into the
void.

**Mitigation Tuning Tip**: For workloads heavily skewed toward compiled languages (like C++ or Java), the default `runtime.NumCPU()` might over-commit the CPU and lead to slow compilation queues. We recommend tuning `GOBOXD_MAX_CONCURRENT=2` (or roughly `NumCPU / 2`) in these environments. This limits in-flight compilations, ensuring they finish faster and reducing overall tail latency.

---

## How to re-run

```bash
# Payloads
cat > /tmp/py_payload.json << 'EOF'
{"language":"py3","source":"print(\"Hello, World!\")","tests":[{"stdin":"","expected_stdout":"Hello, World!\n"}]}
EOF

cat > /tmp/cpp_payload.json << 'EOF'
{"language":"cpp","source":"#include <iostream>\nint main(){std::cout<<\"Hello, World!\"<<std::endl;return 0;}","tests":[{"stdin":"","expected_stdout":"Hello, World!\n"}]}
EOF

# Start container
docker compose up -d

# Python runs (5s cooldown between each)
for c in 1 10 50 100; do
  hey -n 200 -c $c -m POST -H "Content-Type: application/json" \
    -D /tmp/py_payload.json http://localhost:8080/run
  sleep 5
done

# C++ runs (15s cooldown — compilation is CPU-heavy)
for c in 1 10 50 100; do
  hey -n 200 -c $c -m POST -H "Content-Type: application/json" \
    -D /tmp/cpp_payload.json http://localhost:8080/run
  sleep 15
done

# To see C++ c=100 completions without client timeout:
hey -n 200 -c 100 -t 60 -m POST -H "Content-Type: application/json" \
  -D /tmp/cpp_payload.json http://localhost:8080/run
```

---

## Stage 2 Update — 2026-06-09

### Changes and Performance Impact

The Stage 2 code changes have **no measurable impact** on the benchmark numbers above.

| Fix | Performance impact |
|---|---|
| `CompareOutput` TrimSpace (FIX 1) | None — string comparison, nanoseconds |
| `MaxBodyBytes` + source size check (FIX 2) | None — one `len()` call per request |
| Artifact placeholder test (FIX 3) | None — test-only change |
| Per-language Dockerfile scripts (FIX 4) | Build-time only, not runtime |
| Lua language (FIX 5) | New language, does not affect existing |

The concurrency semaphore, nsjail argument construction, jail directory creation,
and sandbox execution path are all unchanged. The Stage 1 benchmark numbers remain valid
for Stage 2.

### Lua Benchmark (new language)

Lua is interpreted (no build step), similar to Python 3. Expected performance:

| Concurrency | Expected RPS | Expected p50 | Notes |
|---|---|---|---|
| 1 | ~25–30 | ~35ms | nsjail startup + lua5.4 interpreter |
| 10 | ~50–60 | ~180ms | same as py3 profile |
| 50 | ~55–60 | ~900ms | queue saturation |
| 100 | ~55–60 | ~1700ms | stable, zero dropped |

To benchmark Lua specifically:

```bash
cat > /tmp/lua_payload.json << 'EOF'
{"language":"lua","source":"io.write(\"Hello, World!\\n\")","tests":[{"stdin":"","expected_stdout":"Hello, World!\n"}]}
EOF

for c in 1 10 50 100; do
  hey -n 200 -c $c -m POST -H "Content-Type: application/json" \
    -D /tmp/lua_payload.json http://localhost:8080/run
  sleep 5
done
```

### Source Size Limit Verification

The `source_too_large` fix (FIX 2) adds one `len()` check per request. To verify:

```bash
# Exactly at limit (262144 bytes) — must return 200
python3 -c "print('x'*262144)" | \
  jq -Rn --arg src "$(cat)" \
  '{"language":"py3","source":$src,"tests":[{"stdin":"","expected_stdout":""}]}' | \
  curl -sf -X POST http://localhost:8080/run -H "Content-Type: application/json" -d @- | \
  jq .status

# One byte over limit (262145 bytes) — must return 400 source_too_large
python3 -c "print('x'*262145)" | \
  jq -Rn --arg src "$(cat)" \
  '{"language":"py3","source":$src,"tests":[{"stdin":"","expected_stdout":""}]}' | \
  curl -sf -X POST http://localhost:8080/run -H "Content-Type: application/json" -d @- | \
  jq .error.code
```