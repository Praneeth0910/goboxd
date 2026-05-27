#!/usr/bin/env bash
# =============================================================================
# run_benchmarks.sh
# Runs real hey benchmarks against a live goboxd container and writes
# the results into docs/benchmarks.md.
#
# Prerequisites:
#   - goboxd container running on localhost:8080
#     (docker compose up -d, or docker run ...)
#   - hey installed (this script auto-installs it on Ubuntu/Debian if missing)
#   - curl + awk + jq available
#
# Usage:
#   bash scripts/run_benchmarks.sh [HOST]
#   HOST defaults to http://localhost:8080
# =============================================================================

set -euo pipefail

HOST="${1:-http://localhost:8080}"
ENDPOINT="$HOST/run"
DOCS_FILE="$(cd "$(dirname "$0")/.." && pwd)/docs/benchmarks.md"
CONCURRENCIES=(1 10 50 100)
REQUESTS=200

# ---------------------------------------------------------------------------
# 1. Install hey if missing
# ---------------------------------------------------------------------------
install_hey() {
  echo "[setup] hey not found – installing via go install..."
  if command -v go &>/dev/null; then
    go install github.com/rakyll/hey@latest
    export PATH="$PATH:$(go env GOPATH)/bin"
  else
    echo "[setup] go not found, trying apt..."
    sudo apt-get update -qq
    # hey is not in Ubuntu repos, fall back to binary download
    local arch
    arch=$(dpkg --print-architecture)
    local url
    case "$arch" in
      amd64)  url="https://hey-release.s3.us-east-2.amazonaws.com/hey_linux_amd64" ;;
      arm64)  url="https://hey-release.s3.us-east-2.amazonaws.com/hey_linux_arm64" ;;
      *)      echo "Unknown arch $arch, cannot auto-install hey"; exit 1 ;;
    esac
    sudo curl -sSL "$url" -o /usr/local/bin/hey
    sudo chmod +x /usr/local/bin/hey
  fi
}

if ! command -v hey &>/dev/null; then
  install_hey
fi

# ---------------------------------------------------------------------------
# 2. Verify the container is reachable
# ---------------------------------------------------------------------------
echo "[check] Pinging $HOST/healthz ..."
if ! curl -sf "$HOST/healthz" >/dev/null; then
  echo "ERROR: goboxd is not reachable at $HOST."
  echo "Start it with:  docker compose up -d"
  exit 1
fi
echo "[check] Container is up."

# ---------------------------------------------------------------------------
# 3. Collect environment metadata
# ---------------------------------------------------------------------------
CPU_CORES=$(nproc)
RAM=$(free -h | awk '/^Mem:/{print $2}')
KERNEL=$(uname -r)
DOCKER_VERSION=$(docker version --format '{{.Server.Version}}' 2>/dev/null || echo "N/A")
DATE=$(date -u +"%Y-%m-%d")

# Detect whether nsjail is active by inspecting the container env
NSJAIL_STATUS="unknown"
if docker inspect goboxd &>/dev/null 2>&1; then
  NSJAIL_PATH=$(docker inspect goboxd \
    --format '{{range .Config.Env}}{{println .}}{{end}}' \
    | grep NSJAIL_PATH | cut -d= -f2 || true)
  if [ -n "$NSJAIL_PATH" ]; then
    NSJAIL_STATUS="enabled (NSJAIL_PATH=$NSJAIL_PATH)"
  else
    NSJAIL_STATUS="disabled (NSJAIL_PATH not set)"
  fi
fi

# ---------------------------------------------------------------------------
# 4. Payloads
# ---------------------------------------------------------------------------
PY_PAYLOAD='{"language":"py3","source":"print(\"Hello, World!\")","tests":[{"stdin":"","expected_stdout":"Hello, World!\n"}]}'

CPP_PAYLOAD='{"language":"cpp","source":"#include <iostream>\nint main(){std::cout<<\"Hello, World!\"<<std::endl;return 0;}","tests":[{"stdin":"","expected_stdout":"Hello, World!\n"}]}'

# ---------------------------------------------------------------------------
# 5. Run hey and extract p50 / p95 / p99 / rps / errors
# ---------------------------------------------------------------------------
# hey output format (relevant lines):
#   Requests/sec:   NNN.NN
#   ...
#   Response time histogram:
#   ...
#   Latency distribution:
#     10% in 0.0012 secs
#     25% in 0.0014 secs
#     50% in 0.0019 secs     <-- p50
#     75% in 0.0023 secs
#     90% in 0.0031 secs
#     95% in 0.0041 secs     <-- p95
#     99% in 0.0081 secs     <-- p99
#   ...
#   [200] NNN responses       <-- success count
#   [4xx] NNN responses       <-- error count (may be absent)

run_hey() {
  local label="$1"
  local concurrency="$2"
  local payload="$3"

  echo "[bench] $label  c=$concurrency  n=$REQUESTS" >&2
  hey -n "$REQUESTS" -c "$concurrency" \
      -m POST \
      -H "Content-Type: application/json" \
      -d "$payload" \
      "$ENDPOINT"
}

extract() {
  # $1 = full hey output, $2 = metric (p50|p95|p99|rps|errors)
  local out="$1" metric="$2"
  case "$metric" in
    p50)
      echo "$out" | awk '/50%/{printf "%.1f", $1*1000; exit}'
      ;;
    p95)
      echo "$out" | awk '/95%/{printf "%.1f", $1*1000; exit}'
      ;;
    p99)
      echo "$out" | awk '/99%/{printf "%.1f", $1*1000; exit}'
      ;;
    rps)
      echo "$out" | awk '/Requests\/sec/{printf "%.1f", $2; exit}'
      ;;
    errors)
      # Total requests minus [200] responses
      local total ok err
      total=$REQUESTS
      ok=$(echo "$out" | grep -oP '\[200\] \K[0-9]+' || echo "$total")
      err=$(( total - ok ))
      echo "$err"
      ;;
  esac
}

# ---------------------------------------------------------------------------
# 6. Run all benchmarks, accumulate table rows
# ---------------------------------------------------------------------------
declare -a ROWS

for c in "${CONCURRENCIES[@]}"; do
  echo ""
  echo "=== Python  c=$c ==="
  PY_OUT=$(run_hey "Python" "$c" "$PY_PAYLOAD")
  echo "$PY_OUT"
  PY_P50=$(extract "$PY_OUT" p50)
  PY_P95=$(extract "$PY_OUT" p95)
  PY_P99=$(extract "$PY_OUT" p99)
  PY_RPS=$(extract "$PY_OUT" rps)
  PY_ERR=$(extract "$PY_OUT" errors)
  ROWS+=("| Python  | $c  | $REQUESTS | $PY_P50 | $PY_P95 | $PY_P99 | $PY_RPS | $PY_ERR |")

  echo ""
  echo "=== C++  c=$c ==="
  CPP_OUT=$(run_hey "C++" "$c" "$CPP_PAYLOAD")
  echo "$CPP_OUT"
  CPP_P50=$(extract "$CPP_OUT" p50)
  CPP_P95=$(extract "$CPP_OUT" p95)
  CPP_P99=$(extract "$CPP_OUT" p99)
  CPP_RPS=$(extract "$CPP_OUT" rps)
  CPP_ERR=$(extract "$CPP_OUT" errors)
  ROWS+=("| C++     | $c  | $REQUESTS | $CPP_P50 | $CPP_P95 | $CPP_P99 | $CPP_RPS | $CPP_ERR |")
done

# ---------------------------------------------------------------------------
# 7. Write docs/benchmarks.md
# ---------------------------------------------------------------------------
cat > "$DOCS_FILE" <<MARKDOWN
# Benchmark Results

## Test Environment

| Field | Value |
|---|---|
| Date | $DATE |
| Kernel | $KERNEL |
| CPU cores | $CPU_CORES |
| RAM | $RAM |
| Docker version | $DOCKER_VERSION |
| nsjail | $NSJAIL_STATUS |
| Tool | hey |
| Requests per run | $REQUESTS |
| Endpoint | POST $ENDPOINT |

> Results collected on a live container with the commands in \`scripts/run_benchmarks.sh\`.

---

## Python 3 — \`print("Hello, World!")\`

### Payload
\`\`\`json
{
  "language": "py3",
  "source": "print(\"Hello, World!\")",
  "tests": [{"stdin": "", "expected_stdout": "Hello, World!\n"}]
}
\`\`\`

## C++ — \`std::cout << "Hello, World!"\`

### Payload
\`\`\`json
{
  "language": "cpp",
  "source": "#include <iostream>\nint main(){std::cout<<\"Hello, World!\"<<std::endl;return 0;}",
  "tests": [{"stdin": "", "expected_stdout": "Hello, World!\n"}]
}
\`\`\`

---

## Results

| Language | Clients | Requests | p50 (ms) | p95 (ms) | p99 (ms) | RPS | Errors |
|----------|---------|----------|----------|----------|----------|-----|--------|
MARKDOWN

for row in "${ROWS[@]}"; do
  echo "$row" >> "$DOCS_FILE"
done

cat >> "$DOCS_FILE" <<'MARKDOWN'

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
MARKDOWN

echo ""
echo "==================================================="
echo " Done! Results written to:"
echo "   $DOCS_FILE"
echo "==================================================="
