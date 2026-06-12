#!/bin/bash
# Run 3 load test — matches the challenge spec exactly:
#   - TIMEOUT=10s  (challenge: "per-request timeout is 10 seconds")
#   - ENDPOINT: goboxd-run3 on port 8082
#   - Config: max_concurrent=8, queue_timeout_s=8, Xmx=400m, MaxMetaspaceSize=32m
#
# Key insight vs Run 2 (TIMEOUT=40s):
#   Run 2 let requests queue for up to 30s — not what SEEK is measuring.
#   At 10s timeout with ~2.57s per-request latency:
#     max throughput = 8 slots / 2.57s = ~3.1 RPS
#     queue budget   = 10s - 2.57s     = ~7.4s of allowed wait
#   server queue_timeout_s=8 ensures clean HTTP 429 before client deadline,
#   so failures are graceful (non-2xx) not silent timeouts (status 0).

set -euo pipefail

ENDPOINT="http://localhost:8082/run"
PAYLOAD="$(dirname "$0")/run-request.json"
DURATION=30
TIMEOUT=10s
RATES="5 10 25 50 75 100 150 200 300 400"
VEGETA="/home/praneeth_0910/go/bin/vegeta"
OUTDIR="$(dirname "$0")/run3"

mkdir -p "${OUTDIR}/raw"

# Build vegeta target file
TARGET_FILE="${OUTDIR}/raw/target.txt"
cat > "${TARGET_FILE}" << EOF
POST ${ENDPOINT}
Content-Type: application/json
@${PAYLOAD}
EOF

echo "target_rps,throughput_rps,duration_s,requests,success,failed,error_pct,p50_ms,p95_ms,p99_ms,max_ms" > "${OUTDIR}/results.csv"

for rate in $RATES; do
  echo "=== Running at ${rate} RPS for ${DURATION}s (timeout=${TIMEOUT}) ==="
  $VEGETA attack \
    -rate="${rate}/1s" \
    -duration="${DURATION}s" \
    -timeout="${TIMEOUT}" \
    -targets="${TARGET_FILE}" \
    > "${OUTDIR}/raw/report-${rate}.bin"

  $VEGETA report -type=json "${OUTDIR}/raw/report-${rate}.bin" \
    > "${OUTDIR}/raw/report-${rate}.json"

  # Print live summary to terminal
  $VEGETA report "${OUTDIR}/raw/report-${rate}.bin"

  jq -r --arg r "$rate" '
    [ $r,
      (.throughput | tostring),
      ((.duration/1e9) | tostring),
      (.requests | tostring),
      ((.status_codes["200"] // 0) | tostring),
      ((.requests - (.status_codes["200"] // 0)) | tostring),
      (((1 - .success) * 100) | tostring),
      ((.latencies["50th"]/1e6) | tostring),
      ((.latencies["95th"]/1e6) | tostring),
      ((.latencies["99th"]/1e6) | tostring),
      ((.latencies.max/1e6) | tostring)
    ] | @csv' \
    "${OUTDIR}/raw/report-${rate}.json" >> "${OUTDIR}/results.csv"

  echo "Done at ${rate} RPS. Sleeping 15s for container recovery..."
  sleep 15
done

echo ""
echo "Run 3 complete. Results in ${OUTDIR}/results.csv"
echo "Plot with:  cd docs/loadtest/run3 && ../../../venv/bin/python3 ../plot.py"
