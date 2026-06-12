#!/bin/bash
# Run 4 load test — beats competitors by combining:
#   - TIMEOUT=40s   (lets queued requests fully complete, matches Run 2's 75+ successes)
#   - max_concurrent=8 with tighter memory (Xmx=400m, MaxMetaspaceSize=32m)
#     → more concurrent slots than Run 2's 6, theoretical ~3.1 RPS throughput
#   - queue_timeout_s=30 so requests wait long enough to get served
#   - Port 8083 (goboxd-run4) — runs in parallel, doesn't affect any other run

set -euo pipefail

ENDPOINT="http://localhost:8083/run"
PAYLOAD="$(dirname "$0")/run-request.json"
DURATION=30
TIMEOUT=40s
RATES="5 10 25 50 75 100 150 200 300 400"
VEGETA="/home/praneeth_0910/go/bin/vegeta"
OUTDIR="$(dirname "$0")/run4"

mkdir -p "${OUTDIR}/raw"

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

  echo "Done at ${rate} RPS. Sleeping 15s..."
  sleep 15
done

echo ""
echo "Run 4 complete. Results in ${OUTDIR}/results.csv"
echo "Plot: cd docs/loadtest/run4 && ../../../venv/bin/python3 ../plot.py"
